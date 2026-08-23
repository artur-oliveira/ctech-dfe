package services

import (
	"context"
	"strings"
	"time"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const orgCacheTTL = 300

// OrganizationService mirrors api/app/services/organizations.py.
type OrganizationService struct {
	repo        *repositories.OrganizationRepository
	auditRepo   *repositories.AuditLogRepository
	certRepo    *repositories.CertificateRepository
	orgUserRepo *repositories.OrgUserRepository
	certSvc     *CertificateService
	memberSvc   *MembershipService
	cache       cache.Backend
}

func NewOrganizationService(
	repo *repositories.OrganizationRepository,
	auditRepo *repositories.AuditLogRepository,
	certRepo *repositories.CertificateRepository,
	orgUserRepo *repositories.OrgUserRepository,
	certSvc *CertificateService,
	memberSvc *MembershipService,
	c cache.Backend,
) *OrganizationService {
	return &OrganizationService{
		repo:        repo,
		auditRepo:   auditRepo,
		certRepo:    certRepo,
		orgUserRepo: orgUserRepo,
		certSvc:     certSvc,
		memberSvc:   memberSvc,
		cache:       c,
	}
}

func (s *OrganizationService) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	cacheKey := "org:" + orgPK
	if v, ok := cacheGetItem(ctx, s.cache, cacheKey); ok {
		return v, nil
	}
	item, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if item != nil {
		cacheSetItem(ctx, s.cache, cacheKey, item, orgCacheTTL)
	}
	return item, nil
}

// SetOwnerUserID stamps the paying account on an organization.
//
// It exists only for the read-fallback repair of organizations created before
// the field did (BillingService.OwnerOf), which is why it takes no actor and
// writes no audit row: it records something that was already true rather than
// deciding it. **Ownership transfer, when it exists, must not use this** — that
// is a decision with an actor, and it moves the OWNER membership in the same
// transaction.
func (s *OrganizationService) SetOwnerUserID(ctx context.Context, orgPK, userID string) error {
	if err := s.repo.UpdateOrganization(ctx, orgPK, map[string]any{
		repositories.AttrOwnerUserID: repositories.RawUserID(userID),
	}); err != nil {
		return err
	}
	cacheDelete(ctx, s.cache, "org:"+orgPK)
	return nil
}

func (s *OrganizationService) Create(ctx context.Context, cpfOrCNPJ string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	existing, err := s.Get(ctx, cpfOrCNPJ)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	if err := s.repo.CreateOrganization(ctx, cpfOrCNPJ, fields); err != nil {
		return nil, err
	}
	return s.repo.GetOrganization(ctx, cpfOrCNPJ)
}

// cnpjRoot returns the 8-digit CNPJ root (raiz) of an org PK, or "" for a CPF
// org (no branch concept).
func cnpjRoot(orgPK string) string {
	if cnpj, ok := strings.CutPrefix(orgPK, "CNPJ_"); ok && len(cnpj) >= 8 {
		return cnpj[:8]
	}
	return ""
}

func certNotExpired(item map[string]types.AttributeValue) bool {
	av, ok := item["expires_at"].(*types.AttributeValueMemberS)
	if !ok {
		return false
	}
	exp, err := time.Parse(time.RFC3339, av.Value)
	if err != nil {
		return false
	}
	return time.Now().UTC().Before(exp)
}

// branchCertificate returns a valid, non-expired certificate from a sibling org
// that shares this org's CNPJ root and that the user already belongs to — the
// matriz certificate that also covers this filial. Returns nil when there is
// none, meaning a certificate is required to create the org. CPF orgs never
// qualify (they have no root).
func (s *OrganizationService) branchCertificate(ctx context.Context, userID, cpfOrCNPJ string) (map[string]types.AttributeValue, error) {
	orgPK, err := repositories.ParseOrgPK(cpfOrCNPJ)
	if err != nil {
		return nil, err
	}
	root := cnpjRoot(orgPK)
	if root == "" {
		return nil, nil
	}
	memberships, err := s.memberSvc.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, m := range memberships {
		if m.OrgPK == orgPK || cnpjRoot(m.OrgPK) != root {
			continue
		}
		certs, err := s.certRepo.List(ctx, m.OrgPK)
		if err != nil {
			return nil, err
		}
		for _, c := range certs {
			if certNotExpired(c) {
				return c, nil
			}
		}
	}
	return nil, nil
}

// CertificateRequired reports whether creating the given org requires a
// certificate upload (true unless the user can inherit a sibling/matriz
// certificate for the same CNPJ root).
func (s *OrganizationService) CertificateRequired(ctx context.Context, userID, cpfOrCNPJ string) (bool, error) {
	cert, err := s.branchCertificate(ctx, userID, cpfOrCNPJ)
	if err != nil {
		return true, err
	}
	return cert == nil, nil
}

// CreateWithOwner atomically creates an organization, its certificate row, the
// founding OWNER membership, and an audit row — enforcing KYC: either a valid
// A1 certificate matching the org's document is supplied, or the user inherits
// a matriz certificate for the same CNPJ root (filial). An already-registered
// org returns the existing item if the caller is already a member, else 409.
func (s *OrganizationService) CreateWithOwner(
	ctx context.Context, cpfOrCNPJ, userID, userName string,
	fields map[string]types.AttributeValue, pfx []byte, password string,
) (map[string]types.AttributeValue, error) {
	orgPK, err := repositories.ParseOrgPK(cpfOrCNPJ)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		m, mErr := s.memberSvc.Get(ctx, orgPK, userID)
		if mErr != nil {
			return nil, mErr
		}
		if m != nil {
			return existing, nil // idempotent for an existing member
		}
		return nil, problem.Conflict("organização já cadastrada")
	}

	// Determine the certificate first, so KYC failures return before any org
	// item is built or uploaded.
	var certTx types.TransactWriteItem
	switch {
	case len(pfx) > 0:
		info, s3Key, upErr := s.certSvc.StageUpload(ctx, orgPK, pfx, password)
		if upErr != nil {
			return nil, upErr
		}
		certTx, _ = s.certSvc.BuildCertTxItem(orgPK, "", info.MD5, password, s3Key, info.CN, info.NotAfter.Format(time.RFC3339))
	default:
		branchCert, bErr := s.branchCertificate(ctx, userID, cpfOrCNPJ)
		if bErr != nil {
			return nil, bErr
		}
		if branchCert == nil {
			return nil, problem.BadRequest("certificado A1 é obrigatório para cadastrar esta empresa")
		}
		// Reuse the matriz PFX (same S3 object) so the filial can emit.
		certTx, _ = s.certRepo.BuildCreateTxItem(orgPK,
			attrStrAV(branchCert, "alias"), attrStrAV(branchCert, "md5"),
			attrStrAV(branchCert, "password"), attrStrAV(branchCert, "s3_key"),
			attrStrAV(branchCert, "expires_at"))
	}

	// owner_user_id is written here and nowhere else, in the same transaction as
	// the OWNER membership it mirrors.
	//
	// It is a field rather than a lookup because it answers a question asked on
	// the billing path — whose subscription governs this organization — and
	// deriving it would mean listing every member to find the one OWNER, on a
	// request that is already deciding whether to let a document be issued. It
	// stays honest because ownership is fixed at creation: an organization has
	// exactly one OWNER, and only an explicit transfer of ownership (which does
	// not exist yet) may rewrite either of them, together.
	if fields == nil {
		fields = map[string]types.AttributeValue{}
	}
	fields[repositories.AttrOwnerUserID] = &types.AttributeValueMemberS{Value: repositories.RawUserID(userID)}

	orgTx, orgItem, err := s.repo.BuildCreateTxItem(cpfOrCNPJ, fields)
	if err != nil {
		return nil, err
	}
	txItems := []types.TransactWriteItem{
		orgTx,
		certTx,
		s.orgUserRepo.BuildCreateTxItem(orgPK, userID, repositories.RoleOwner, "", userName, nil),
	}

	afterMap, err := attributeMapToPlain(orgItem)
	if err != nil {
		return nil, err
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceOrganization, orgPK, repositories.AuditActionCreate,
		userID, userName, Diff(nil, afterMap),
	)
	if err != nil {
		return nil, err
	}
	txItems = append(txItems, auditTx)

	if err := s.repo.TransactWrite(ctx, txItems); err != nil {
		if repositories.IsConditionFailed(err) {
			return nil, problem.Conflict("organização já cadastrada")
		}
		return nil, err
	}
	s.memberSvc.Invalidate(ctx, orgPK, userID)
	return s.repo.GetOrganization(ctx, orgPK)
}

// Update writes the organization's company-data change and its UPDATE audit
// row atomically. Fetches the current item first so only actually-changed
// fields are logged.
func (s *OrganizationService) Update(ctx context.Context, orgPK string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	// Read directly from the repository (bypassing the service cache) so the
	// audit "before" snapshot always reflects the latest DynamoDB state, even
	// under concurrent updates hitting different replicas (cache is per-
	// instance, 300s TTL — see api/CLAUDE.md).
	current, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("organization not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}

	orgTx, err := s.repo.BuildUpdateTxItem(orgPK, updates)
	if err != nil {
		return nil, err
	}
	// updates is a partial map (only the fields the caller wants to change).
	// Merge it over beforeMap so Diff only reports fields that actually
	// changed, instead of treating every omitted field as "changed to nil".
	afterMap := make(map[string]any, len(beforeMap))
	for k, v := range beforeMap {
		afterMap[k] = v
	}
	for k, v := range updates {
		afterMap[k] = v
	}

	pk := attrStrAV(current, "pk")
	auditTx, err := s.auditRepo.BuildLogTxItem(
		pk, repositories.AuditResourceOrganization, pk, repositories.AuditActionUpdate,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{orgTx, auditTx}); err != nil {
		return nil, err
	}
	cacheDelete(ctx, s.cache, "org:"+orgPK)
	return s.repo.GetOrganization(ctx, orgPK)
}

const maxAuthorizedViewers = 10

// AuthorizedViewerEntry is the plain-value shape of an authorized_xml_viewers
// entry (stored as a list of maps on the organization item) — CPF/CNPJ+name
// pairs SEFAZ allows to view this organization's NF-e XMLs (autXML).
type AuthorizedViewerEntry struct {
	CpfOrCnpj string `json:"cpf_cnpj"`
	Name      string `json:"name"`
}

func extractAuthorizedViewers(item map[string]any) []AuthorizedViewerEntry {
	raw, _ := item["authorized_xml_viewers"].([]any)
	out := make([]AuthorizedViewerEntry, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		cpfCnpj, _ := m["cpf_cnpj"].(string)
		name, _ := m["name"].(string)
		out = append(out, AuthorizedViewerEntry{CpfOrCnpj: cpfCnpj, Name: name})
	}
	return out
}

func normalizeDoc(s string) string {
	return strings.NewReplacer(".", "", "-", "", "/", "").Replace(s)
}

// appendAuthorizedViewer returns the new list with v appended, or an error if
// v.CpfOrCnpj is already present or the list is already at the SEFAZ-imposed
// cap of 10.
func appendAuthorizedViewer(current []AuthorizedViewerEntry, v AuthorizedViewerEntry) ([]AuthorizedViewerEntry, error) {
	if len(current) >= maxAuthorizedViewers {
		return nil, problem.BadRequest("limite de 10 pessoas autorizadas atingido")
	}
	normalized := normalizeDoc(v.CpfOrCnpj)
	for _, existing := range current {
		if normalizeDoc(existing.CpfOrCnpj) == normalized {
			return nil, problem.Conflict("CPF/CNPJ já autorizado")
		}
	}
	v.CpfOrCnpj = normalized
	return append(current, v), nil
}

func removeAuthorizedViewerEntry(current []AuthorizedViewerEntry, cpfCnpj string) []AuthorizedViewerEntry {
	normalized := normalizeDoc(cpfCnpj)
	out := make([]AuthorizedViewerEntry, 0, len(current))
	for _, existing := range current {
		if normalizeDoc(existing.CpfOrCnpj) != normalized {
			out = append(out, existing)
		}
	}
	return out
}

// toAuthorizedViewerMaps converts to plain maps with snake_case keys before
// writing — attributevalue.Marshal has no dynamodbav tags to guide it here,
// so a raw []AuthorizedViewerEntry would be stored under its exported Go
// field names (CpfOrCnpj/Name) instead of the cpf_cnpj/name keys the read
// side (extractAuthorizedViewers) expects.
func toAuthorizedViewerMaps(entries []AuthorizedViewerEntry) []map[string]any {
	out := make([]map[string]any, len(entries))
	for i, e := range entries {
		out[i] = map[string]any{"cpf_cnpj": e.CpfOrCnpj, "name": e.Name}
	}
	return out
}

// AddAuthorizedViewer appends a person authorized to view this organization's
// NF-e XMLs (SEFAZ autXML, max 10, no duplicate CPF/CNPJ).
func (s *OrganizationService) AddAuthorizedViewer(ctx context.Context, orgPK string, v AuthorizedViewerEntry, userID, userName string) (map[string]types.AttributeValue, error) {
	current, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("organization not found")
	}
	currentMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}
	viewers, err := appendAuthorizedViewer(extractAuthorizedViewers(currentMap), v)
	if err != nil {
		return nil, err
	}
	return s.Update(ctx, orgPK, map[string]any{"authorized_xml_viewers": toAuthorizedViewerMaps(viewers)}, userID, userName)
}

// RemoveAuthorizedViewer removes an authorized viewer by CPF/CNPJ. No-op
// (not an error) if the CPF/CNPJ wasn't in the list.
func (s *OrganizationService) RemoveAuthorizedViewer(ctx context.Context, orgPK, cpfCnpj, userID, userName string) (map[string]types.AttributeValue, error) {
	current, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("organization not found")
	}
	currentMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}
	viewers := removeAuthorizedViewerEntry(extractAuthorizedViewers(currentMap), cpfCnpj)
	return s.Update(ctx, orgPK, map[string]any{"authorized_xml_viewers": toAuthorizedViewerMaps(viewers)}, userID, userName)
}
