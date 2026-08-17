//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// These exercise the money path end to end: a document reaches a terminal
// status, and either billing is told to charge for it or the quota slot comes
// back. Every other test in this package runs in no-charge mode, so this is the
// only place a real billing client is wired.

// usageCall is one recorded POST /v1.0/usage.
type usageCall struct {
	SubscriptionID string `json:"subscription_id"`
	PriceID        string `json:"price_id"`
	Quantity       int64  `json:"quantity"`
	IdempotencyKey string `json:"idempotency_key"`
}

// billingStub stands in for ctech-billing: it mints a token and records usage.
func billingStub(t *testing.T, calls *[]usageCall) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/v1.0/usage", func(w http.ResponseWriter, r *http.Request) {
		var call usageCall
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			t.Errorf("usage body: %v", err)
		}
		// The HTTP idempotency key and the usage event key must be the same
		// value: billing dedupes on the second, and a mismatch would let one
		// emission be charged twice under two different keys.
		if got := r.Header.Get("Idempotency-Key"); got != call.IdempotencyKey {
			t.Errorf("Idempotency-Key = %q, body key = %q", got, call.IdempotencyKey)
		}
		*calls = append(*calls, call)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// chargingBilling builds a BillingService that actually charges, against the
// stub. Each call gets its own cache so a snapshot written by one test is never
// read by another.
func chargingBilling(t *testing.T, srv *httptest.Server) *services.BillingService {
	t.Helper()
	client := billingclient.New(billingclient.Config{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/v1.0/token",
		ClientID:     "dfe-billing",
		ClientSecret: "secret",
		Cache:        cache.NewMemoryBackend(16),
	})
	if client == nil {
		t.Fatal("billing client must be enabled for these tests")
	}
	return services.NewBillingService(
		repositories.NewAccountBillingRepository(db, cfg),
		client, nil, memberSvc, orgSvc, cache.NewMemoryBackend(16),
	)
}

// seedPayingOrg creates an organization owned by userID and files the account's
// billing snapshot, which is what the issuance path reads.
func seedPayingOrg(t *testing.T, userID string, snap *repositories.AccountSnapshot) string {
	t.Helper()
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	if err := orgRepo.CreateOrganization(ctx, orgPK, map[string]types.AttributeValue{}); err != nil {
		t.Fatal(err)
	}
	if err := orgSvc.SetOwnerUserID(ctx, orgPK, userID); err != nil {
		t.Fatal(err)
	}
	snap.UserID = userID
	if err := repositories.NewAccountBillingRepository(db, cfg).Put(ctx, snap); err != nil {
		t.Fatal(err)
	}
	return orgPK
}

// TestAuthorisedIssuanceIsReportedToBilling is the usage-based plan's whole
// revenue path: no report, no invoice line, no money.
func TestAuthorisedIssuanceIsReportedToBilling(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))

	orgPK := seedPayingOrg(t, "usage-owner", &repositories.AccountSnapshot{
		SubscriptionID: "sub_ondemand", Status: "ACTIVE", Plan: "ondemand", Entitled: true,
		PeriodStart: "2026-08-01", PeriodEnd: "2026-09-01",
		Quotas: map[string]int64{services.MeterNFe: services.QuotaUnlimited},
		Meters: map[string]string{services.MeterNFe: "price_dfe_ondemand_nfe"},
	})

	const accessKey = "41260800000000000000550010000000011000000017"
	if err := svc.ReportUsage(ctx, orgPK, services.MeterNFe, accessKey); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("%d usage calls, want 1", len(calls))
	}
	got := calls[0]
	if got.SubscriptionID != "sub_ondemand" || got.PriceID != "price_dfe_ondemand_nfe" || got.Quantity != 1 {
		t.Fatalf("usage call = %+v", got)
	}
	if got.IdempotencyKey != accessKey {
		t.Fatalf("event key = %q, want the access key — a redelivery must report the same emission", got.IdempotencyKey)
	}

	// A redelivered SQS message reports again, and it must carry the same key so
	// billing can recognise it. Deduplication is billing's side of the contract;
	// sending a stable key is this side's.
	if err := svc.ReportUsage(ctx, orgPK, services.MeterNFe, accessKey); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[1].IdempotencyKey != accessKey {
		t.Fatalf("second call = %+v", calls)
	}
}

// TestAFixedPlanReportsNothing: a Pro account already paid for its 1200 NF-e.
// Reporting each one as metered usage would invoice it a second time, per unit.
func TestAFixedPlanReportsNothing(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))

	orgPK := seedPayingOrg(t, "fixed-owner", &repositories.AccountSnapshot{
		SubscriptionID: "sub_pro", Status: "ACTIVE", Plan: "pro", Entitled: true,
		PeriodStart: "2026-08-01", PeriodEnd: "2026-09-01",
		Quotas: map[string]int64{services.MeterNFe: 1200},
		// No Meters: the fixed price carries no `meter` in its metadata.
	})

	if err := svc.ReportUsage(ctx, orgPK, services.MeterNFe, "chave-fixa"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("a fixed plan must not be charged per unit, got %+v", calls)
	}
}

// TestARejectedDocumentGivesItsSlotBackExactlyOnce.
//
// The slot is claimed when the request arrives, before SEFAZ has an opinion, so
// a rejection has to return it — a Free account with three NF-e cannot lose one
// to a typo in an address. Exactly once, because the results queue redelivers
// and a second refund would be a fourth NF-e nobody paid for.
func TestARejectedDocumentGivesItsSlotBackExactlyOnce(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))

	orgPK := seedPayingOrg(t, "refund-owner", &repositories.AccountSnapshot{
		SubscriptionID: "sub_free", Status: "ACTIVE", Plan: "free", Entitled: true,
		PeriodStart: "2026-08-01", PeriodEnd: "2026-09-01",
		Quotas: map[string]int64{services.MeterNFe: 3},
	})

	for range 3 {
		if err := svc.Reserve(ctx, orgPK, services.MeterNFe); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Reserve(ctx, orgPK, services.MeterNFe); err == nil {
		t.Fatal("the fourth NF-e must be refused")
	}

	const rejected = "41260800000000000000550010000000021000000028"
	svc.RefundOnce(ctx, orgPK, services.MeterNFe, rejected)
	svc.RefundOnce(ctx, orgPK, services.MeterNFe, rejected)

	usage, err := svc.Usage(ctx, "refund-owner")
	if err != nil {
		t.Fatal(err)
	}
	if usage[services.MeterNFe].Used != 2 {
		t.Fatalf("used = %d, want 2 — one refund for one rejection", usage[services.MeterNFe].Used)
	}
	// And the returned slot is genuinely usable again.
	if err := svc.Reserve(ctx, orgPK, services.MeterNFe); err != nil {
		t.Fatalf("the refunded slot must be reusable: %v", err)
	}
}
