package services

import (
	"context"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// maxPendingInvitations caps outstanding invitations per org (anti-spam).
const maxPendingInvitations = 50

// invitableRoles are the roles an invitation may grant — never OWNER (which is
// reserved for the org creator / ownership transfer), to prevent privilege
// escalation via a leaked link.
var invitableRoles = map[string]bool{
	repositories.RoleAdmin:  true,
	repositories.RoleUser:   true,
	repositories.RoleViewer: true,
}

// InvitationPreview is the non-consuming view of an invitation shown to the
// invitee before they accept or decline.
type InvitationPreview struct {
	OrgPK         string `json:"org_pk"`
	OrgName       string `json:"org_name"`
	Role          string `json:"role"`
	InvitedByName string `json:"invited_by_name"`
	Status        string `json:"status"`
	Expired       bool   `json:"expired"`
	AlreadyMember bool   `json:"already_member"`
}

// InvitationService manages organization invitation links.
type InvitationService struct {
	invRepo     *repositories.OrgInvitationRepository
	orgUserRepo *repositories.OrgUserRepository
	orgRepo     *repositories.OrganizationRepository
	auditRepo   *repositories.AuditLogRepository
	memberSvc   *MembershipService
}

func NewInvitationService(
	invRepo *repositories.OrgInvitationRepository,
	orgUserRepo *repositories.OrgUserRepository,
	orgRepo *repositories.OrganizationRepository,
	auditRepo *repositories.AuditLogRepository,
	memberSvc *MembershipService,
) *InvitationService {
	return &InvitationService{invRepo: invRepo, orgUserRepo: orgUserRepo, orgRepo: orgRepo, auditRepo: auditRepo, memberSvc: memberSvc}
}

// Create issues a new invitation and returns the raw token (shown only once, in
// the link) plus the stored item.
func (s *InvitationService) Create(ctx context.Context, orgPK, role, invitedBy, invitedByName string) (string, map[string]any, error) {
	if !invitableRoles[role] {
		return "", nil, problem.BadRequest("função inválida para convite")
	}
	pending, err := s.invRepo.CountPendingByOrg(ctx, orgPK)
	if err != nil {
		return "", nil, err
	}
	if pending >= maxPendingInvitations {
		return "", nil, problem.Conflict("limite de convites pendentes atingido")
	}
	raw, hash, err := repositories.GenerateInvitationToken()
	if err != nil {
		return "", nil, problem.InternalServer("falha ao gerar token de convite")
	}
	item, err := s.invRepo.Create(ctx, hash, orgPK, role, invitedBy, invitedByName, nil)
	if err != nil {
		return "", nil, err
	}
	out, err := attributeMapToPlain(item)
	if err != nil {
		return "", nil, err
	}
	return raw, out, nil
}

// ListPending returns an org's pending invitations.
func (s *InvitationService) ListPending(ctx context.Context, orgPK string) ([]map[string]any, error) {
	items, err := s.invRepo.ListPendingByOrg(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m, mErr := attributeMapToPlain(it)
		if mErr != nil {
			return nil, mErr
		}
		out = append(out, m)
	}
	return out, nil
}

// Revoke revokes a pending invitation (org-scoped).
func (s *InvitationService) Revoke(ctx context.Context, orgPK, invitationPK string) error {
	ok, err := s.invRepo.Revoke(ctx, invitationPK, orgPK)
	if err != nil {
		return err
	}
	if !ok {
		return problem.NotFound("convite não encontrado")
	}
	return nil
}

// Preview returns a non-consuming view of an invitation for the invitee.
func (s *InvitationService) Preview(ctx context.Context, rawToken, userID string) (*InvitationPreview, error) {
	inv, err := s.invRepo.GetByToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, problem.NotFound("convite não encontrado")
	}
	orgPK := attrStrAV(inv, "org_pk")
	preview := &InvitationPreview{
		OrgPK:         orgPK,
		Role:          attrStrAV(inv, "role"),
		InvitedByName: attrStrAV(inv, "invited_by_name"),
		Status:        attrStrAV(inv, "status"),
		Expired:       invitationExpired(attrStrAV(inv, "expires_at")),
	}
	if org, oErr := s.orgRepo.GetOrganization(ctx, orgPK); oErr == nil && org != nil {
		preview.OrgName = attrStrAV(org, "name")
	}
	if m, mErr := s.memberSvc.Get(ctx, orgPK, userID); mErr == nil && m != nil {
		preview.AlreadyMember = true
	}
	return preview, nil
}

// Accept consumes the invitation and creates the membership atomically. Returns
// the resulting membership (org + role) so the caller can mirror it and refresh
// the UI without waiting on the GSI.
func (s *InvitationService) Accept(ctx context.Context, rawToken, userID, userName string) (*Membership, error) {
	inv, err := s.invRepo.GetByToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, problem.NotFound("convite não encontrado")
	}
	orgPK := attrStrAV(inv, "org_pk")
	role := attrStrAV(inv, "role")
	invitedBy := attrStrAV(inv, "invited_by")
	status := attrStrAV(inv, "status")

	if status != repositories.InvitationPending {
		return nil, problem.Conflict("convite já utilizado ou revogado")
	}
	if invitationExpired(attrStrAV(inv, "expires_at")) {
		return nil, problem.Conflict("convite expirado")
	}
	if m, mErr := s.memberSvc.Get(ctx, orgPK, userID); mErr == nil && m != nil {
		return nil, problem.Conflict("Você já faz parte desta organização")
	}

	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceMember, repositories.RawUserID(userID), repositories.AuditActionCreate,
		userID, userName, Diff(nil, map[string]any{"role": role, "invited_by": invitedBy}),
	)
	if err != nil {
		return nil, err
	}

	txItems := []types.TransactWriteItem{
		s.invRepo.BuildAcceptTxItem(attrStrAV(inv, "pk"), repositories.RawUserID(userID)),
		s.orgUserRepo.BuildCreateTxItem(orgPK, userID, role, invitedBy, userName, nil),
		auditTx,
	}
	if err := s.invRepo.TransactWrite(ctx, txItems); err != nil {
		if repositories.IsConditionFailed(err) {
			return nil, problem.Conflict("convite já utilizado ou expirado")
		}
		return nil, err
	}
	s.memberSvc.Invalidate(ctx, orgPK, userID)
	return &Membership{OrgPK: orgPK, UserID: repositories.RawUserID(userID), Role: role}, nil
}

// Decline marks an invitation revoked at the invitee's request.
func (s *InvitationService) Decline(ctx context.Context, rawToken string) error {
	inv, err := s.invRepo.GetByToken(ctx, rawToken)
	if err != nil {
		return err
	}
	if inv == nil {
		return problem.NotFound("convite não encontrado")
	}
	if attrStrAV(inv, "status") != repositories.InvitationPending {
		return nil // already consumed/revoked — idempotent
	}
	_, err = s.invRepo.Revoke(ctx, attrStrAV(inv, "pk"), attrStrAV(inv, "org_pk"))
	return err
}

// invitationExpired reports whether the ISO expires_at is in the past.
func invitationExpired(expiresAt string) bool {
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(t)
}
