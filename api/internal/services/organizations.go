package services

import (
	"context"
	"strings"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const orgCacheTTL = 300

// RequireOrgIE returns a BadRequest problem if cpfOrCNPJ is a CNPJ and regs
// is empty. Organizations (always the fiscal emitter) must declare at least
// one state registration; persons (destinatário/counterparty) are exempt —
// IE-when-contribuinte is a per-emission choice (indIEDest), not a cadastro
// requirement. See docs/superpowers/specs/2026-07-11-pessoas-organizacoes-cadastro-design.md.
func RequireOrgIE(cpfOrCNPJ string, regs []StateRegistrationEntry) error {
	v := normalizeDoc(cpfOrCNPJ)
	if len(v) == 14 && len(regs) == 0 {
		return problem.BadRequest("ao menos uma inscrição estadual é obrigatória para organização com CNPJ")
	}
	return nil
}

// OrganizationService mirrors api/app/services/organizations.py.
type OrganizationService struct {
	repo      *repositories.OrganizationRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
}

func NewOrganizationService(repo *repositories.OrganizationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *OrganizationService {
	return &OrganizationService{repo: repo, auditRepo: auditRepo, cache: c}
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
	_ = s.cache.Delete(ctx, "org:"+orgPK)
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
