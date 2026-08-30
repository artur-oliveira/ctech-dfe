package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// LinkService adopts a company created in ctech-account.
//
// This is the DF-e half of the organization handoff: somebody needed a company
// that did not exist, was sent to ctech-account to create it, and came back with
// two ids. This turns those ids into a local record — the identity cached, the
// fiscal side empty and waiting.
//
// It is deliberately NOT a create: the company already exists, and this product
// does not get to decide whether it should. What it decides is whether the
// person asking may act for it.
type LinkService struct {
	orgRepo     *repositories.OrganizationRepository
	orgUserRepo *repositories.OrgUserRepository
	reach       *ReachService
	identity    companyIdentitySource
	userSvc     *UserService
}

// companyIdentitySource reads a company's names from ctech-account.
type companyIdentitySource interface {
	Company(ctx context.Context, organizationID, companyID string) (*accountclient.Identity, error)
}

func NewLinkService(
	orgRepo *repositories.OrganizationRepository,
	orgUserRepo *repositories.OrgUserRepository,
	reach *ReachService,
	identity companyIdentitySource,
	userSvc *UserService,
) *LinkService {
	return &LinkService{orgRepo: orgRepo, orgUserRepo: orgUserRepo, reach: reach, identity: identity, userSvc: userSvc}
}

// Enabled reports a service that can actually link. Without the ctech-account
// credential it cannot verify reach OR read identity, and linking on trust
// would let anybody adopt any company id they can type.
func (s *LinkService) Enabled() bool {
	return s != nil && s.reach != nil && s.identity != nil
}

// Link adopts the company, or explains why not.
//
// **Idempotent.** A refresh on the landing page replays the same ids, and the
// second call must find the record rather than fail or duplicate — the spec
// requires it because a browser reload is not a user error.
//
// **It verifies reach first, against ctech-account.** The ids arrive on a URL
// the person controls, so without this anybody could adopt any company id they
// can guess. The handoff's own validation protects the redirect, not this call.
func (s *LinkService) Link(ctx context.Context, organizationID, companyID, userID, userName string) (map[string]types.AttributeValue, error) {
	if !s.Enabled() {
		return nil, problem.BadRequest("a integração com a conta CTech não está configurada nesta instalação")
	}
	if organizationID == "" || companyID == "" {
		return nil, problem.BadRequest("organization_id e company_id são obrigatórios")
	}
	if !repositories.IsCompanyKey(companyID) {
		return nil, problem.BadRequest("company_id inválido")
	}

	// Reach before anything else, and from ctech-account rather than from a
	// local row: a local row is what this call is about to create.
	gotOrg, mayAct, err := s.reach.MayAct(ctx, companyID, userID)
	if _, prob := checkReach(organizationID, gotOrg, mayAct, err); prob != nil {
		return nil, prob
	}

	if existing, err := s.orgRepo.GetOrganization(ctx, companyID); err != nil {
		return nil, err
	} else if existing != nil {
		// Already linked. The membership is ensured anyway: a first run that
		// died between the two would otherwise leave a company nobody can open.
		if err := s.ensureOwner(ctx, companyID, userID, userName); err != nil {
			return nil, err
		}
		return existing, nil
	}

	ident, err := s.identity.Company(ctx, organizationID, companyID)
	if err != nil {
		if errors.Is(err, accountclient.ErrCompanyNotFound) {
			return nil, problem.NotFound("empresa não encontrada na conta CTech")
		}
		return nil, fmt.Errorf("reading the company identity: %w", err)
	}

	item := map[string]types.AttributeValue{
		"name":        &types.AttributeValueMemberS{Value: ident.LegalName},
		"description": &types.AttributeValueMemberS{Value: ident.TradeName},
		// The identity, cached. ctech-account owns it; this is what the issuer
		// document is read from (services.IssuerDoc) and what the emit node
		// carries.
		repositories.AttrOrganizationID: &types.AttributeValueMemberS{Value: ident.OrganizationID},
		repositories.AttrTaxID:          &types.AttributeValueMemberS{Value: ident.TaxID},
		repositories.AttrTaxIDKind:      &types.AttributeValueMemberS{Value: ident.TaxIDKind},
		repositories.AttrLegalName:      &types.AttributeValueMemberS{Value: ident.LegalName},
		repositories.AttrOwnerUserID:    &types.AttributeValueMemberS{Value: repositories.RawUserID(userID)},
	}
	if err := s.orgRepo.CreateOrganization(ctx, companyID, item); err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, companyID, userID, userName); err != nil {
		return nil, err
	}

	// No certificate and no fiscal configuration: those are what this product
	// asks for next, on its own screens. A company linked and not configured
	// cannot emit — and costs nothing, because the quota counts what is enabled
	// (ctech-billing ADR 0021).
	s.userSvc.InvalidateCache(ctx, userID)
	return s.orgRepo.GetOrganization(ctx, companyID)
}

// ensureOwner writes the caller's OWNER row if it is not there.
//
// OWNER because they are the person who created the company in ctech-account and
// linked it here, and because ADR 0023 answers "who may hand out grants" with
// this row. Idempotent: Create is a no-op when it already exists.
func (s *LinkService) ensureOwner(ctx context.Context, companyID, userID, userName string) error {
	return s.orgUserRepo.Create(ctx, companyID, userID, repositories.RoleOwner, userID, userName, nil)
}

// checkReach decides whether a link may proceed, given what ctech-account said.
//
// Split from Link so the decision is testable without a database, because it is
// the security-relevant half: everything else is a write.
//
// **One answer for every refusal.** A caller must not be able to tell a wrong
// organization from a missing edge from an outage — those distinctions would
// make this endpoint a probe for which companies belong to which organization.
// The cause goes in the log, where the person debugging sees it and the person
// guessing does not.
func checkReach(wantOrg, gotOrg string, mayAct bool, err error) (string, *problem.Problem) {
	const denied = "você não tem acesso a esta empresa"
	switch {
	case err != nil:
		// Fail closed. An unreachable ctech-account cannot be read as consent.
		return "", problem.Forbidden(denied)
	case !mayAct:
		return "", problem.Forbidden(denied)
	case gotOrg != wantOrg:
		// The URL named one organization and the edge says another. Trusting
		// the URL would let somebody attach a company they reach to an
		// organization they do not.
		return "", problem.Forbidden(denied)
	}
	return gotOrg, nil
}
