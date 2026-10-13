//go:build integration

package integration_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"software.sslmate.com/src/go-pkcs12"

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
		{"organizations", map[string]any{"pk": c, "organization_id": accountOrg, "created_at": "2026-10-07T00:00:00Z", "tax_id": tax, "tax_id_kind": "cnpj"}},
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

const erasureTestCPF = "12345678909"

func ecpfPFX(t *testing.T, cn string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pfx, err := pkcs12.Modern2023.Encode(key, cert, nil, "pw")
	if err != nil {
		t.Fatal(err)
	}
	return string(pfx)
}

func sAttr(item map[string]types.AttributeValue, name string) string {
	if v, ok := item[name].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func eraseMessage(requestID, sub string, orgs ...string) erasure.Message {
	return erasure.Message{
		Version: erasure.Version, Type: erasure.TypeErase, RequestID: requestID, Sub: sub,
		Scope: erasure.ScopeAccount, Services: []string{services.ErasureServiceName},
		Organizations: orgs, Attempt: 1, IssuedAt: time.Now(),
	}
}

func cpfOf(cpf string) services.SubjectLookup {
	return func(context.Context, string) (string, error) { return cpf, nil }
}

func TestPurge_EndToEndTwice(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, store := newErasureSvc(objects, cpfOf(erasureTestCPF))
	svc.WithSettle(0)

	u, v := "user-"+randomCNPJ(), "user-"+randomCNPJ()
	o1 := "acct-o1-" + randomCNPJ()
	c1, c2, c3 := "c1-"+randomCNPJ(), "c2-"+randomCNPJ(), "c3-"+randomCNPJ()
	c1Rows := companySeed(c1, o1, randomCNPJ())
	seed(t, objects, c1Rows)
	putItem(t, "organization_users", map[string]any{"pk": c1, "sk": "USER_" + u})

	// c2 survives: u is an ADMIN beside v; u's e-CPF is installed there and
	// shared by a branch row in c3.
	putItem(t, "organizations", map[string]any{"pk": c2, "organization_id": "acct-o2-" + c2, "created_at": "2026-10-07T00:00:00Z"})
	putItem(t, "organization_users", map[string]any{"pk": c2, "sk": "USER_" + u, "name": "Fulano"})
	putItem(t, "organization_users", map[string]any{"pk": c2, "sk": "USER_" + v})
	objects.Put(erasureCertsBucket, "certs/"+c2+"/b.pfx", ecpfPFX(t, "FULANO DE TAL:"+erasureTestCPF))
	objects.Put(erasureCertsBucket, "certs/"+c2+"/b.pfx", ecpfPFX(t, "FULANO DE TAL:"+erasureTestCPF))
	objects.Put(erasureCertsBucket, "certs/"+c2+"/c.pfx", ecpfPFX(t, "OUTRA PESSOA:98765432100"))
	putItem(t, "organization_certificates", map[string]any{"pk": c2, "sk": "CERTIFICATE_b", "md5": "b", "s3_key": "certs/" + c2 + "/b.pfx", "password": "pw"})
	putItem(t, "organization_certificates", map[string]any{"pk": c2, "sk": "CERTIFICATE_c", "md5": "c", "s3_key": "certs/" + c2 + "/c.pfx", "password": "pw"})
	putItem(t, "organization_certificates", map[string]any{"pk": c3, "sk": "CERTIFICATE_b", "md5": "b", "s3_key": "certs/" + c2 + "/b.pfx", "password": "pw"})
	putItem(t, "audit_logs", map[string]any{"pk": c2, "sk": "PRODUCT#P#1", "user_id": u, "user_name": "Fulano", "created_at": "2026-10-07T00:00:01Z"})
	putItem(t, "audit_logs", map[string]any{"pk": c2, "sk": "CERTIFICATE#b#1", "user_id": v, "created_at": "2026-10-07T00:00:02Z",
		"modifications": []map[string]any{{"name": "alias", "after": "FULANO DE TAL:" + erasureTestCPF}}})
	putItem(t, "audit_logs", map[string]any{"pk": c2, "sk": "MEMBER#" + u + "#1", "user_id": v, "created_at": "2026-10-07T00:00:03Z",
		"modifications": []map[string]any{{"name": "name", "after": "Fulano"}}})
	putItem(t, "organization_invitations", map[string]any{"pk": "INVITE_acc_" + u, "org_pk": c2, "accepted_by": u, "created_at": "2026-10-07T00:00:04Z"})
	putItem(t, "organization_invitations", map[string]any{"pk": "INVITE_sent_" + u, "org_pk": c2, "invited_by": u, "invited_by_name": "Fulano", "accepted_by": v, "created_at": "2026-10-07T00:00:05Z"})
	putItem(t, "users", map[string]any{"pk": "USER_" + u})
	putItem(t, "users", map[string]any{"pk": "USER_" + v})
	putItem(t, "account_billing", map[string]any{"pk": "USER_" + u, "period_start": "2026-10-01"})
	putItem(t, "account_billing", map[string]any{"pk": repositories.UsageCounterPK(u, "2026-10-01")})

	m := eraseMessage("req-e2e-"+u, u, o1, "acct-org-with-no-dfe-company")
	for i := range 2 {
		ack, err := svc.Purge(ctx, m)
		if err != nil || ack.Result != erasure.ResultDone {
			t.Fatalf("run %d: ack %+v, err %v", i+1, ack, err)
		}
	}

	for _, r := range c1Rows {
		if getItem(t, r.table, seedKey(r)) != nil {
			t.Errorf("c1 row %s %v survived", r.table, seedKey(r))
		}
	}
	if rec, err := store.Get(ctx, erasure.OrgKey(c1)); err != nil || rec == nil || rec.State != erasure.StateErased {
		t.Errorf("company tombstone = %+v (%v), want erased", rec, err)
	}
	if getItem(t, "organizations", map[string]any{"pk": c2}) == nil {
		t.Error("the surviving organization was erased")
	}
	if getItem(t, "organization_users", map[string]any{"pk": c2, "sk": "USER_" + u}) != nil {
		t.Error("u's membership in the surviving org remains")
	}
	if getItem(t, "organization_users", map[string]any{"pk": c2, "sk": "USER_" + v}) == nil {
		t.Error("v's membership was erased")
	}
	for _, k := range []map[string]any{{"pk": c2, "sk": "CERTIFICATE_b"}, {"pk": c3, "sk": "CERTIFICATE_b"}} {
		if getItem(t, "organization_certificates", k) != nil {
			t.Errorf("e-CPF row %v remains", k)
		}
	}
	if objects.Versions(erasureCertsBucket, "certs/"+c2+"/b.pfx") != 0 {
		t.Error("a version of the e-CPF PFX remains")
	}
	if getItem(t, "organization_certificates", map[string]any{"pk": c2, "sk": "CERTIFICATE_c"}) == nil {
		t.Error("another person's e-CPF was erased")
	}
	row := getItem(t, "audit_logs", map[string]any{"pk": c2, "sk": "PRODUCT#P#1"})
	if sAttr(row, "user_id") == u || sAttr(row, "user_name") != "Usuário removido" {
		t.Errorf("audit actor not anonymized: %v", row)
	}
	for _, sk := range []string{"CERTIFICATE#b#1", "MEMBER#" + u + "#1"} {
		mods, _ := getItem(t, "audit_logs", map[string]any{"pk": c2, "sk": sk})["modifications"].(*types.AttributeValueMemberL)
		if mods == nil || len(mods.Value) != 0 {
			t.Errorf("audit %s still carries personal modifications", sk)
		}
	}
	notice, err := repositories.NewErasureRepository(db, cfg).QueryPrefix(ctx, "audit_logs", c2, "CERTIFICATE#b#", nil)
	if err != nil {
		t.Fatal(err)
	}
	system := 0
	for _, it := range notice.Items {
		if sAttr(it, "user_id") == "SYSTEM" {
			system++
		}
	}
	if system != 1 {
		t.Errorf("want exactly one SYSTEM audit notice for the erased certificate, got %d", system)
	}
	if getItem(t, "organization_invitations", map[string]any{"pk": "INVITE_acc_" + u}) != nil {
		t.Error("an invitation u received remains")
	}
	sent := getItem(t, "organization_invitations", map[string]any{"pk": "INVITE_sent_" + u})
	if sent == nil || sAttr(sent, "invited_by") == u || sAttr(sent, "invited_by_name") == "Fulano" {
		t.Errorf("an invitation u sent was not anonymized: %v", sent)
	}
	if getItem(t, "users", map[string]any{"pk": "USER_" + u}) != nil || getItem(t, "users", map[string]any{"pk": "USER_" + v}) == nil {
		t.Error("users table: u must be gone and v kept")
	}
	if getItem(t, "account_billing", map[string]any{"pk": "USER_" + u}) != nil ||
		getItem(t, "account_billing", map[string]any{"pk": repositories.UsageCounterPK(u, "2026-10-01")}) != nil {
		t.Error("u's billing snapshot or current usage remains")
	}
}

// Review Focus 1: a distribution job may write for a company while it is being
// purged. The first delivery locks and empties the company but keeps its
// organizations item; only a delivery after the settle window finishes.
func TestPurge_WaitsForSettleWindowBeforeFinishingCompanies(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, store := newErasureSvc(objects, cpfOf(""))
	u, o, c := "user-"+randomCNPJ(), "acct-"+randomCNPJ(), "co-"+randomCNPJ()
	seed(t, objects, companySeed(c, o, randomCNPJ()))
	m := eraseMessage("req-settle-"+u, u, o)

	if _, err := svc.Purge(ctx, m); !errors.Is(err, services.ErrErasureSettling) {
		t.Fatalf("first delivery err = %v, want ErrErasureSettling", err)
	}
	if rec, _ := store.Get(ctx, erasure.OrgKey(c)); rec == nil || rec.State != erasure.StateLocked {
		t.Fatalf("company record = %+v, want locked (workers drop its jobs)", rec)
	}
	if getItem(t, "organizations", map[string]any{"pk": c}) == nil {
		t.Fatal("organizations item deleted before the settle window: a rerun could no longer find the company")
	}
	putItem(t, "nfe_distributions", map[string]any{"pk": "prod#" + c, "nsu": 99}) // a late job's write

	ack, err := svc.WithSettle(0).Purge(ctx, m)
	if err != nil || ack.Result != erasure.ResultDone {
		t.Fatalf("redelivery: %+v, %v", ack, err)
	}
	if getItem(t, "nfe_distributions", map[string]any{"pk": "prod#" + c, "nsu": 99}) != nil {
		t.Error("the late write survived")
	}
	if getItem(t, "organizations", map[string]any{"pk": c}) != nil {
		t.Error("organizations item remains after the settle window")
	}
}

// Review Focus 2: the purge crashed after deleting an e-CPF's PFX and before
// deleting its row. The marker left on the row lets the rerun finish without
// being able to open the PFX.
func TestPurge_FinishesAHalfErasedCertificate(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, _ := newErasureSvc(objects, cpfOf(erasureTestCPF))
	u, c := "user-"+randomCNPJ(), "half-"+randomCNPJ()
	requestID := "req-half-" + u
	putItem(t, "organization_certificates", map[string]any{
		"pk": c, "sk": "CERTIFICATE_h", "md5": "h", "s3_key": "certs/" + c + "/h.pfx", "password": "pw",
		"erasure_request": requestID,
	})

	if _, err := svc.WithSettle(0).Purge(ctx, eraseMessage(requestID, u)); err != nil {
		t.Fatal(err)
	}
	if getItem(t, "organization_certificates", map[string]any{"pk": c, "sk": "CERTIFICATE_h"}) != nil {
		t.Fatal("the half-erased certificate row (with its password) remains")
	}
}

// R15: an erased organization's local billing rows go with it, and the key
// kinds are the ones the repository builds.
func TestPurge_ErasesOrganizationBilling(t *testing.T) {
	ctx := context.Background()
	svc, _ := newErasureSvc(testsupport.NewObjectStore(), cpfOf(""))
	svc.WithSettle(0)
	u, o := "user-"+randomCNPJ(), "acct-bill-"+randomCNPJ()
	month := time.Now().Format("2006-01")
	keys := []string{
		repositories.OrgBillingPK(o),
		repositories.QuotaGuardPK(o, services.MeterNFe),
		repositories.QuotaGuardPK(o, services.MeterCompanies),
		repositories.LevelMarkerPK(o, services.MeterNFe),
		repositories.UsageCounterPK(o, month),
		repositories.UsageCounterPK(o, "2026-10-01T00:00:00Z"),
	}
	putItem(t, "account_billing", map[string]any{"pk": keys[0], "period_start": "2026-10-01T00:00:00Z"})
	for _, k := range keys[1:] {
		putItem(t, "account_billing", map[string]any{"pk": k})
	}
	other := repositories.OrgBillingPK("acct-other-" + o)
	putItem(t, "account_billing", map[string]any{"pk": other})

	if _, err := svc.Purge(ctx, eraseMessage("req-bill-"+u, u, o)); err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if getItem(t, "account_billing", map[string]any{"pk": k}) != nil {
			t.Errorf("billing row %s survived", k)
		}
	}
	if getItem(t, "account_billing", map[string]any{"pk": other}) == nil {
		t.Error("another organization's billing snapshot was erased")
	}
}
