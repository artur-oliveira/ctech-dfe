//go:build integration

package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// TestAccountSnapshotRoundTrip: the snapshot is what the issuance path reads to
// decide a quota, so every field has to survive the trip through DynamoDB —
// including the maps, which is where an encoding mistake would show up as a plan
// with no limits rather than as an error.
func TestAccountSnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	const userID = "snap-user-1"

	if got, err := repo.Get(ctx, userID); err != nil || got != nil {
		t.Fatalf("an account with no row must read as absent, got %+v (%v)", got, err)
	}

	want := &repositories.AccountSnapshot{
		UserID:            userID,
		CustomerID:        "cus_1",
		SubscriptionID:    "sub_1",
		Status:            "ACTIVE",
		Plan:              "pro",
		Entitled:          true,
		CancelAtPeriodEnd: true,
		PeriodStart:       "2026-08-01",
		PeriodEnd:         "2026-09-01",
		Quotas:            map[string]int64{"nfe": 1200, "users": 25, "cte": -1},
		Meters:            map[string]string{"nfe": "price_dfe_ondemand_nfe"},
		OpenInvoice: &repositories.OpenInvoice{
			ID: "in_1", TotalCents: 35000, DueDate: "2026-08-16",
			CheckoutURL: "https://billing.aoctech.app/checkout?token=x",
		},
	}
	if err := repo.Put(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, userID)
	if err != nil || got == nil {
		t.Fatalf("Get after Put: %+v (%v)", got, err)
	}
	if got.SubscriptionID != want.SubscriptionID || got.Status != want.Status || got.Plan != want.Plan {
		t.Errorf("snapshot = %+v", got)
	}
	if !got.CancelAtPeriodEnd || !got.Entitled {
		t.Errorf("the booleans did not survive: %+v", got)
	}
	for meter, limit := range want.Quotas {
		if got.Quotas[meter] != limit {
			t.Errorf("quota %s = %d, want %d", meter, got.Quotas[meter], limit)
		}
	}
	if got.Meters["nfe"] != "price_dfe_ondemand_nfe" {
		t.Errorf("meters = %v", got.Meters)
	}
	if got.OpenInvoice == nil || got.OpenInvoice.CheckoutURL != want.OpenInvoice.CheckoutURL {
		t.Errorf("open invoice = %+v", got.OpenInvoice)
	}
	if got.SyncedAt == "" {
		t.Error("Put must stamp synced_at — a snapshot with no age cannot be reasoned about")
	}
}

// TestPutReplacesRatherThanMerges: a snapshot is one consistent picture of a
// moment in billing. Merging a new subscription's fields over an old one's would
// produce a row describing neither — an old plan's quotas beside a new plan's id.
func TestPutReplacesRatherThanMerges(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	const userID = "snap-user-2"

	if err := repo.Put(ctx, &repositories.AccountSnapshot{
		UserID: userID, SubscriptionID: "sub_pro", Plan: "pro", Status: "ACTIVE",
		Quotas:      map[string]int64{"nfe": 1200},
		OpenInvoice: &repositories.OpenInvoice{ID: "in_old", TotalCents: 35000},
	}); err != nil {
		t.Fatal(err)
	}
	// Downgraded to free: fewer quotas, and nothing outstanding.
	if err := repo.Put(ctx, &repositories.AccountSnapshot{
		UserID: userID, SubscriptionID: "sub_free", Plan: "free", Status: "ACTIVE",
		Quotas: map[string]int64{"nfe": 3},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubscriptionID != "sub_free" || got.Plan != "free" || got.Quotas["nfe"] != 3 {
		t.Fatalf("snapshot = %+v", got)
	}
	if got.OpenInvoice != nil {
		t.Errorf("the previous plan's invoice must not survive the replacement, got %+v", got.OpenInvoice)
	}
}

// TestEventIsProcessedExactlyOnce: billing delivers at least once, so two copies
// of one event can be in flight together. The conditional write is what lets
// exactly one through — a read-then-write would let both.
func TestEventIsProcessedExactlyOnce(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	const eventID = "evt_once"

	fresh, err := repo.MarkEventProcessed(ctx, eventID)
	if err != nil || !fresh {
		t.Fatalf("first delivery must be fresh: %t (%v)", fresh, err)
	}
	again, err := repo.MarkEventProcessed(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("a redelivery must not read as fresh")
	}

	// And under real contention, which is the case the condition exists for: a
	// sequential re-check would pass even with a read-then-write implementation.
	const concurrent = 8
	var wg sync.WaitGroup
	results := make([]bool, concurrent)
	errs := make([]error, concurrent)
	for i := range concurrent {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = repo.MarkEventProcessed(ctx, "evt_race")
		}(i)
	}
	wg.Wait()

	winners := 0
	for i := range concurrent {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if results[i] {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("%d deliveries were treated as fresh, want exactly 1", winners)
	}
}

// TestQuotaIsClaimedAtomically is the property the whole limit rests on.
//
// Ten requests race for three slots. A read-then-write implementation passes the
// sequential version of this test and fails here — which is the point, because
// two people issuing an invoice at the same moment is not an edge case, it is
// Monday morning.
func TestQuotaIsClaimedAtomically(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	const userID, period, meter = "quota-user", "2026-08-01", "nfe"
	const limit, attempts = 3, 10

	var wg sync.WaitGroup
	granted := make([]bool, attempts)
	for i := range attempts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := repo.ReserveUsage(ctx, userID, period, meter, limit)
			granted[i] = err == nil
		}(i)
	}
	wg.Wait()

	won := 0
	for _, ok := range granted {
		if ok {
			won++
		}
	}
	if won != limit {
		t.Fatalf("%d reservations succeeded, want exactly %d", won, limit)
	}

	usage, err := repo.GetUsage(ctx, userID, period)
	if err != nil {
		t.Fatal(err)
	}
	if usage[meter] != limit {
		t.Fatalf("counter = %d, want %d — the counter and the grants must agree", usage[meter], limit)
	}
}

// TestQuotaOfZeroGrantsNothing: the Free plan says `quota_cte: 0`, and that has
// to mean "no CT-e" rather than "unlimited CT-e". The very first reservation
// must fail, which is the case an `attribute_not_exists` shortcut would get
// wrong.
func TestQuotaOfZeroGrantsNothing(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)

	if _, err := repo.ReserveUsage(ctx, "zero-user", "2026-08-01", "cte", 0); !errors.Is(err, repositories.ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded on the first attempt", err)
	}
}

// TestUnlimitedStillCounts: a usage-based plan has no ceiling but every emission
// is money, and the usage screen shows the number. Unlimited skips the condition,
// never the increment.
func TestUnlimitedStillCounts(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	const userID, period = "unlimited-user", "2026-08-01"

	for range 5 {
		if _, err := repo.ReserveUsage(ctx, userID, period, "nfce", -1); err != nil {
			t.Fatal(err)
		}
	}
	usage, err := repo.GetUsage(ctx, userID, period)
	if err != nil {
		t.Fatal(err)
	}
	if usage["nfce"] != 5 {
		t.Fatalf("counter = %d, want 5", usage["nfce"])
	}
}

// TestRefundGivesTheSlotBackButNeverGoesNegative.
//
// The floor matters: a refund is replayed whenever the worker's message is
// redelivered, and a counter that could go below zero would hand out free
// headroom every time.
func TestRefundGivesTheSlotBackButNeverGoesNegative(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	const userID, period, meter = "refund-user", "2026-08-01", "nfe"

	if _, err := repo.ReserveUsage(ctx, userID, period, meter, 3); err != nil {
		t.Fatal(err)
	}
	// Three refunds against one reservation — a redelivered message, twice over.
	for range 3 {
		if err := repo.RefundUsage(ctx, userID, period, meter); err != nil {
			t.Fatal(err)
		}
	}
	usage, err := repo.GetUsage(ctx, userID, period)
	if err != nil {
		t.Fatal(err)
	}
	if usage[meter] != 0 {
		t.Fatalf("counter = %d, want 0 — a refund must not drive it negative", usage[meter])
	}
	// And the slot is genuinely available again.
	if _, err := repo.ReserveUsage(ctx, userID, period, meter, 1); err != nil {
		t.Fatalf("the refunded slot must be reusable: %v", err)
	}
}

// TestPeriodsAreCountedSeparately: the counter is filed under the
// subscription's period, so a new period starts from zero without anything
// having to reset it.
func TestPeriodsAreCountedSeparately(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	const userID, meter = "period-user", "nfe"

	if _, err := repo.ReserveUsage(ctx, userID, "2026-08-01", meter, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReserveUsage(ctx, userID, "2026-08-01", meter, 1); !errors.Is(err, repositories.ErrQuotaExceeded) {
		t.Fatalf("the period must be exhausted, got %v", err)
	}
	if _, err := repo.ReserveUsage(ctx, userID, "2026-09-01", meter, 1); err != nil {
		t.Fatalf("the next period starts empty: %v", err)
	}
}
