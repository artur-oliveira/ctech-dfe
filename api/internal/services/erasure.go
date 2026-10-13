package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/api-commons/observability"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// ErasureServiceName is this service's id in erasure messages, acks and the
// SNS filter policy.
const ErasureServiceName = "dfe"

const (
	erasureSettleWindow = 10 * time.Minute // > the 300 s worker Lambda timeout

	organizationIDIndex = "organization-index" // existing since #130; sort key created_at
	orgInviteIndex      = "org-invite-index"
	userIndex           = "user-index"
	userIDIndex         = "user-id-index"
	outboxCommandSK     = "command"

	tblOrganizations  = "organizations"
	tblOrgUsers       = "organization_users"
	tblInvitations    = "organization_invitations"
	tblAuditLogs      = "audit_logs"
	tblCertificates   = "organization_certificates"
	tblOutbox         = "worker_outbox"
	tblUsers          = "users"
	tblAccountBilling = "account_billing"

	countS3Versions  = "s3_object_versions"
	countSerieClaims = "serie_claims_released"
)

var (
	erasureEnvs     = []string{EnvHom, EnvProd}
	erasureDocTypes = []string{"nfe", "nfce", "cte", "mdfe", "nfse"}
	// serieConfigModels are the fiscal configs whose séries are claimed
	// (NFS-e is municipal and claims none).
	serieConfigModels = map[string]string{
		"organization_nfe_configs":  ModelNFe,
		"organization_nfce_configs": ModelNFCe,
		"organization_cte_configs":  ModelCTe,
		"organization_mdfe_configs": ModelMDFe,
	}
)

// SubjectLookup returns the CPF of the user of a purging request (ctech-account
// GET /internal/erasure/{request_id}/subject). The CPF stays in memory.
type SubjectLookup func(ctx context.Context, requestID string) (string, error)

// ErasureService is dfe's side of the account-deletion saga
// (docs/plans/2026-10-07-account-deletion-participant.md).
type ErasureService struct {
	repo        *repositories.ErasureRepository
	certs       *repositories.CertificateRepository
	audit       *repositories.AuditLogRepository
	claims      *repositories.SerieClaimRepository
	store       *erasure.Store
	objects     ObjectStore
	docsBucket  string
	certsBucket string
	subject     SubjectLookup
	cache       cache.Backend
	settle      time.Duration
	now         func() time.Time
}

func NewErasureService(
	repo *repositories.ErasureRepository,
	certs *repositories.CertificateRepository,
	audit *repositories.AuditLogRepository,
	claims *repositories.SerieClaimRepository,
	store *erasure.Store,
	objects ObjectStore,
	docsBucket, certsBucket string,
	subject SubjectLookup,
	c cache.Backend,
) *ErasureService {
	return &ErasureService{
		repo: repo, certs: certs, audit: audit, claims: claims, store: store, objects: objects,
		docsBucket: docsBucket, certsBucket: certsBucket, subject: subject, cache: c,
		settle: erasureSettleWindow, now: time.Now,
	}
}

// WithSettle overrides the settle window (tests use 0).
func (s *ErasureService) WithSettle(d time.Duration) *ErasureService {
	s.settle = d
	return s
}

func pkKey(pk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": repositories.SAV(pk)}
}

func rowKey(item map[string]types.AttributeValue, sortKey string) map[string]types.AttributeValue {
	key := map[string]types.AttributeValue{"pk": item["pk"]}
	if sortKey != "" {
		key[sortKey] = item[sortKey]
	}
	return key
}

func avNum(item map[string]types.AttributeValue, key string) int {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		n, _ := strconv.Atoi(v.Value)
		return n
	}
	return 0
}

// PurgeCompany erases every row and object of one company except its
// organizations item, which Purge deletes once the settle window has passed.
func (s *ErasureService) PurgeCompany(ctx context.Context, company string) (map[string]int, error) {
	counts := map[string]int{}
	return counts, s.purgeCompany(ctx, company, counts)
}

func (s *ErasureService) purgeCompany(ctx context.Context, company string, counts map[string]int) error {
	if err := s.releaseSerieClaims(ctx, company, counts); err != nil {
		return fmt.Errorf("serie claims: %w", err)
	}
	for _, t := range erasureCatalog {
		if err := s.purgeTable(ctx, t, company, counts); err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
	}
	for _, p := range companyDocumentPrefixes(company) {
		n, err := purgeObjects(ctx, s.objects, s.docsBucket, p, false)
		if err != nil {
			return err
		}
		counts[countS3Versions] += n
	}
	n, err := purgeObjects(ctx, s.objects, s.certsBucket, companyCertPrefix(company), false)
	if err != nil {
		return err
	}
	counts[countS3Versions] += n
	for _, p := range []string{"dfe:res:" + company + ":", "dfe:member:" + company + ":", "dfe:reach:" + company + ":"} {
		if err := s.cache.DeletePrefix(ctx, p); err != nil {
			observability.Warn(ctx, "erasure: cache prefix delete failed", err, "prefix", p)
		}
	}
	return nil
}

func (s *ErasureService) purgeTable(ctx context.Context, t ErasureTable, company string, counts map[string]int) error {
	switch t.Scope {
	case ScopeOrgPartition:
		return s.purgePartition(ctx, company, t.Name, t.SortKey, company, counts)
	case ScopeOrgItem:
		existed, err := s.repo.DeleteKey(ctx, t.Name, pkKey(company))
		if existed {
			counts[t.Name]++
		}
		return err
	case ScopeEnvDocs:
		for _, env := range erasureEnvs {
			if err := s.purgeDocs(ctx, company, t, env+"#"+company, counts); err != nil {
				return err
			}
		}
	case ScopeEnvPartition:
		for _, env := range erasureEnvs {
			if err := s.purgePartition(ctx, company, t.Name, t.SortKey, env+"#"+company, counts); err != nil {
				return err
			}
		}
	case ScopeDocEvents:
		if t.Inut {
			for _, env := range erasureEnvs {
				if err := s.purgePartition(ctx, company, t.Name, t.SortKey, InutEventPK(env, company), counts); err != nil {
					return err
				}
			}
		}
	case ScopeInvitations:
		return s.purgeIndexed(ctx, t.Name, orgInviteIndex, "org_pk", company, counts)
	case ScopeKept, ScopePerson, ScopeAccountBilling, ScopeOrganization, ScopeOutbox, ScopeSerieClaims:
		// Kept; person and billing purge (Purge); after the settle window; through each document;
		// before the fiscal configs go (releaseSerieClaims).
	default:
		return fmt.Errorf("erasure: scope %d has no purge rule", t.Scope)
	}
	return nil
}

func (s *ErasureService) purgePartition(ctx context.Context, company, table, sortKey, pk string, counts map[string]int) error {
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryPartition(ctx, table, pk, start)
		if err != nil {
			return err
		}
		if err := s.deleteRows(ctx, company, table, sortKey, page.Items, counts); err != nil {
			return err
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// purgeDocs erases a document partition: each document's events and outbox
// command first, then its objects and the document row itself.
func (s *ErasureService) purgeDocs(ctx context.Context, company string, t ErasureTable, pk string, counts map[string]int) error {
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryPartition(ctx, t.Name, pk, start)
		if err != nil {
			return err
		}
		for _, doc := range page.Items {
			sk := avAttr(doc, skKey)
			if err := s.purgePartition(ctx, company, t.Events, skKey, sk, counts); err != nil {
				return err
			}
			existed, err := s.repo.DeleteKey(ctx, tblOutbox, map[string]types.AttributeValue{
				"pk": repositories.SAV(t.Name + "#" + sk), "sk": repositories.SAV(outboxCommandSK),
			})
			if err != nil {
				return err
			}
			if existed {
				counts[tblOutbox]++
			}
		}
		if err := s.deleteRows(ctx, company, t.Name, t.SortKey, page.Items, counts); err != nil {
			return err
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// deleteRows erases the objects the rows point to, then the rows. The row is
// the only index to its objects, so it goes last: a crash in between leaves
// both to the rerun.
func (s *ErasureService) deleteRows(ctx context.Context, company, table, sortKey string, items []map[string]types.AttributeValue, counts map[string]int) error {
	if len(items) == 0 {
		return nil
	}
	keys := make([]map[string]types.AttributeValue, 0, len(items))
	for _, it := range items {
		if err := s.purgeRowObjects(ctx, company, it, counts); err != nil {
			return err
		}
		keys = append(keys, rowKey(it, sortKey))
	}
	if err := s.repo.DeleteKeys(ctx, table, keys); err != nil {
		return err
	}
	counts[table] += len(keys)
	return nil
}

// purgeRowObjects deletes the objects a row names (ruling R7). A certificate
// object is deleted only under the company's own certs/ folder: a branch's row
// points at its matriz's PFX, which belongs to the matriz.
func (s *ErasureService) purgeRowObjects(ctx context.Context, company string, item map[string]types.AttributeValue, counts map[string]int) error {
	for _, attr := range []string{"xml_s3_key", "dps_xml_s3_key"} {
		n, err := purgeObjects(ctx, s.objects, s.docsBucket, avAttr(item, attr), true)
		if err != nil {
			return err
		}
		counts[countS3Versions] += n
	}
	if key := avAttr(item, "s3_key"); strings.HasPrefix(key, companyCertPrefix(company)) {
		n, err := purgeObjects(ctx, s.objects, s.certsBucket, key, true)
		if err != nil {
			return err
		}
		counts[countS3Versions] += n
	}
	return nil
}

func (s *ErasureService) purgeIndexed(ctx context.Context, table, index, attr, value string, counts map[string]int) error {
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, table, index, attr, value, start)
		if err != nil {
			return err
		}
		keys := make([]map[string]types.AttributeValue, 0, len(page.Items))
		for _, it := range page.Items {
			keys = append(keys, rowKey(it, ""))
		}
		if err := s.repo.DeleteKeys(ctx, table, keys); err != nil {
			return err
		}
		counts[table] += len(keys)
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// releaseSerieClaims frees the séries the company's fiscal configs claim. It
// runs before the configs are deleted, because the claims are derived from
// them. A série held by another company on the same CNPJ is not ours to free.
func (s *ErasureService) releaseSerieClaims(ctx context.Context, company string, counts map[string]int) error {
	org, err := s.repo.GetItem(ctx, tblOrganizations, pkKey(company))
	if err != nil || org == nil {
		return err
	}
	taxID, _ := IssuerDocAV(org, company)
	if taxID == "" {
		return nil
	}
	for table, modelo := range serieConfigModels {
		cfg, err := s.repo.GetItem(ctx, table, pkKey(company))
		if err != nil {
			return err
		}
		for _, c := range SerieClaimsFor(modelo, avNum(cfg, "prod_current_serie"), avNum(cfg, "hom_current_serie")) {
			err := s.claims.Release(ctx, taxID, c.Modelo, c.Ambiente, c.Serie, company)
			if errors.Is(err, repositories.ErrSerieTaken) {
				continue
			}
			if err != nil {
				return err
			}
			counts[countSerieClaims]++
		}
	}
	return nil
}

// companyDocumentPrefixes are the documents-bucket folders named after the
// company id, which no other company shares (unlike the CNPJ_ folders).
func companyDocumentPrefixes(company string) []string {
	var out []string
	for _, t := range erasureDocTypes {
		out = append(out, "pdfs/"+t+"/"+company+"/")
		for _, env := range erasureEnvs {
			out = append(out, t+"/"+env+"/"+company+"/", t+"-distribution/"+env+"/"+company+"/")
		}
	}
	return out
}

func companyCertPrefix(company string) string { return "certs/" + company + "/" }
