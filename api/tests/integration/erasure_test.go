//go:build integration

package integration_test

import (
	"context"
	"testing"

	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/testsupport"
)

const (
	erasureDocsBucket  = "docs"
	erasureCertsBucket = "certs"
)

func newErasureSvc(objects *testsupport.ObjectStore, subject services.SubjectLookup) (*services.ErasureService, *erasure.Store) {
	store := erasure.NewStore(db, tablePrefix, 0)
	return services.NewErasureService(
		repositories.NewErasureRepository(db, cfg), certRepo, auditRepo,
		repositories.NewSerieClaimRepository(db, cfg), store, objects,
		erasureDocsBucket, erasureCertsBucket, subject, memCache,
	), store
}

type seedRow struct {
	table string
	item  map[string]any
}

// companySeed is one row in every kind of table the company purge reaches.
func companySeed(c, accountOrg, tax string) []seedRow {
	k := "NFEKEY-" + c
	dps := "DPS-" + c
	return []seedRow{
		{"organizations", map[string]any{"pk": c, "organization_id": accountOrg, "tax_id": tax, "tax_id_kind": "cnpj"}},
		{"organization_nfe_configs", map[string]any{"pk": c, "prod_current_serie": 1}},
		{"organization_nfse_configs", map[string]any{"pk": c}},
		{"serie_claims", map[string]any{"pk": repositories.SerieClaimPK(tax, services.ModelNFe, services.AmbienteProd, 1), "company_id": c}},
		{"nfes", map[string]any{"pk": "prod#" + c, "sk": k, "xml_s3_key": "nfe/prod/CNPJ_" + tax + "/" + k + ".xml"}},
		{"nfe_events", map[string]any{"pk": k, "sk": "EV1", "xml_s3_key": "nfe/prod/CNPJ_" + tax + "/" + k + "_110111_001.xml"}},
		{"nfe_events", map[string]any{"pk": services.InutEventPK(services.EnvHom, c), "sk": "EV2"}},
		{"worker_outbox", map[string]any{"pk": "nfes#" + k, "sk": "command"}},
		{"nfses", map[string]any{"pk": "hom#" + c, "sk": dps, "xml_s3_key": "nfse/hom/" + c + "/" + dps + ".xml", "dps_xml_s3_key": "nfse/hom/" + c + "/" + dps + "_dps.xml"}},
		{"nfse_events", map[string]any{"pk": dps, "sk": "EV3"}},
		{"nfe_distributions", map[string]any{"pk": "hom#" + c, "nsu": 7, "xml_s3_key": "nfe-distribution/hom/" + c + "/NSU_000000000000007.xml"}},
		{"organization_products", map[string]any{"pk": c, "sk": "PRODUCT_1"}},
		{repositories.TableFuelPumps, map[string]any{"pk": c, "sk": "FUELPUMP_1"}},
		{"organization_certificates", map[string]any{"pk": c, "sk": "CERTIFICATE_m", "md5": "m", "s3_key": "certs/" + c + "/m.pfx", "password": "pw"}},
		{"audit_logs", map[string]any{"pk": c, "sk": "PRODUCT#PRODUCT_1#01", "user_id": "u-x", "created_at": "2026-10-07T00:00:00Z"}},
		{"organization_users", map[string]any{"pk": c, "sk": "USER_u-x"}},
		{"organization_invitations", map[string]any{"pk": "INVITE_" + c, "org_pk": c, "created_at": "2026-10-07T00:00:00Z"}},
	}
}

// seed writes rows and, for every object key a row carries, two versions of
// the object (versioned buckets keep the old one).
func seed(t *testing.T, objects *testsupport.ObjectStore, rows []seedRow) {
	t.Helper()
	for _, r := range rows {
		putItem(t, r.table, r.item)
		for _, attr := range []string{"xml_s3_key", "dps_xml_s3_key"} {
			if key, ok := r.item[attr].(string); ok {
				objects.Put(erasureDocsBucket, key, "v1")
				objects.Put(erasureDocsBucket, key, "v2")
			}
		}
		if key, ok := r.item["s3_key"].(string); ok {
			objects.Put(erasureCertsBucket, key, "pfx")
		}
	}
}

func seedKey(r seedRow) map[string]any {
	key := map[string]any{"pk": r.item["pk"]}
	for _, tb := range services.ErasureCatalog() {
		if tb.Name == r.table && tb.SortKey != "" {
			key[tb.SortKey] = r.item[tb.SortKey]
		}
	}
	return key
}

func TestPurgeCompany_ErasesEveryCompanyRow(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, _ := newErasureSvc(objects, nil)
	c, tax := "erase-co-"+randomCNPJ(), randomCNPJ()
	rows := companySeed(c, "acct-org-"+c, tax)
	seed(t, objects, rows)
	objects.Put(erasureDocsBucket, "pdfs/nfe/"+c+"/v1/NFEKEY-"+c+"-authorized.pdf", "pdf")

	for range 2 { // a rerun must succeed and change nothing
		if _, err := svc.PurgeCompany(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range rows {
		got := getItem(t, r.table, seedKey(r))
		if r.table == "organizations" {
			if got == nil {
				t.Error("the organizations item goes only after the settle window (Purge, not PurgeCompany)")
			}
			continue
		}
		if got != nil {
			t.Errorf("%s %v survived the company purge", r.table, seedKey(r))
		}
	}
	if left := objects.Keys(erasureDocsBucket); len(left) != 0 {
		t.Errorf("document objects left: %v", left)
	}
	if left := objects.Keys(erasureCertsBucket); len(left) != 0 {
		t.Errorf("certificate objects left: %v", left)
	}
}

// Two companies may hold the same CNPJ (ctech-billing ADR 0022). Emitted XML
// sits in a folder named after the CNPJ, and a branch's certificate row points
// at the matriz's PFX. Erasing one company must leave the other's objects.
func TestPurgeCompany_LeavesASiblingCompanyOnTheSameCNPJ(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, _ := newErasureSvc(objects, nil)
	tax := randomCNPJ()
	c, d := "erase-c-"+randomCNPJ(), "keep-d-"+randomCNPJ()
	seed(t, objects, companySeed(c, "acct-c-"+c, tax))
	dRows := companySeed(d, "acct-d-"+d, tax) // same CNPJ; its série claim row wins
	seed(t, objects, dRows)
	putItem(t, "organization_certificates", map[string]any{
		"pk": c, "sk": "CERTIFICATE_d", "md5": "d", "s3_key": "certs/" + d + "/m.pfx", "password": "pw",
	})

	if _, err := svc.PurgeCompany(ctx, c); err != nil {
		t.Fatal(err)
	}
	for _, r := range dRows {
		if getItem(t, r.table, seedKey(r)) == nil {
			t.Errorf("sibling row %s %v was erased", r.table, seedKey(r))
		}
		if key, ok := r.item["xml_s3_key"].(string); ok && objects.Versions(erasureDocsBucket, key) == 0 {
			t.Errorf("sibling object %s was erased", key)
		}
	}
	if objects.Versions(erasureCertsBucket, "certs/"+d+"/m.pfx") == 0 {
		t.Error("the sibling's PFX, shared by a branch row of the erased company, was erased")
	}
}
