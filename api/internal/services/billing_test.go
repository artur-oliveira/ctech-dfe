package services

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/billingclient"
)

// SnapshotFrom is where the product's rules actually live — which subscription
// governs the account, what a quota means, which meters the worker reports — so
// it is tested without a network or a database.

func proSubscription() billingclient.EntitlementSubscription {
	return billingclient.EntitlementSubscription{
		ID:       "sub_pro",
		Status:   "ACTIVE",
		Entitled: true,
		Plan:     "pro",
		Period:   billingclient.Period{Start: "2026-08-01", End: "2026-09-01"},
		Items: []billingclient.EntitlementItem{{
			PriceID: "price_dfe_pro_monthly",
			Type:    "fixed",
			Metadata: map[string]string{
				"plan":            "pro",
				"quota_companies": "10",
				"quota_users":     "25",
				"quota_nfe":       "1200",
				"quota_cte":       "1200",
			},
		}},
	}
}

func TestSnapshotCarriesThePlanAndItsQuotas(t *testing.T) {
	snap := SnapshotFrom("user-1", &billingclient.Entitlements{
		CustomerID:    "cus_1",
		Entitled:      true,
		Subscriptions: []billingclient.EntitlementSubscription{proSubscription()},
	})

	if snap.SubscriptionID != "sub_pro" || snap.Plan != "pro" || snap.Status != "ACTIVE" {
		t.Fatalf("snapshot = %+v", snap)
	}
	if !GrantsService(snap) {
		t.Fatal("an ACTIVE subscription must grant service")
	}
	for meter, want := range map[string]int64{"companies": 10, "users": 25, "nfe": 1200, "cte": 1200} {
		got, ok := Quota(snap, meter)
		if !ok || got != want {
			t.Errorf("quota %s = %d (present=%t), want %d", meter, got, ok, want)
		}
	}
	// A meter the plan does not mention is not unlimited — it is not granted.
	if _, ok := Quota(snap, "nfse"); ok {
		t.Error("a quota the plan never declared must read as absent, not as a limit")
	}
	// Fixed plans have no meters: nothing is reported per document.
	if len(snap.Meters) != 0 {
		t.Errorf("a fixed plan reports no usage, got meters %v", snap.Meters)
	}
}

// TestSnapshotOfAMeteredPlanCarriesTheMeters: the presence of a meter is what
// tells the worker to report an emission at all, and which price to report it
// against. Getting the price wrong bills NFC-e volume at the CT-e rate.
func TestSnapshotOfAMeteredPlanCarriesTheMeters(t *testing.T) {
	snap := SnapshotFrom("user-2", &billingclient.Entitlements{
		CustomerID: "cus_2",
		Subscriptions: []billingclient.EntitlementSubscription{{
			ID:       "sub_ondemand",
			Status:   "ACTIVE",
			Entitled: true,
			Plan:     "ondemand",
			Items: []billingclient.EntitlementItem{
				{PriceID: "price_dfe_ondemand_nfe", Type: "metered",
					Metadata: map[string]string{"plan": "ondemand", "meter": "nfe"}},
				{PriceID: "price_dfe_ondemand_nfce", Type: "metered",
					Metadata: map[string]string{"plan": "ondemand", "meter": "nfce"}},
			},
		}},
	})

	if snap.Meters["nfe"] != "price_dfe_ondemand_nfe" || snap.Meters["nfce"] != "price_dfe_ondemand_nfce" {
		t.Fatalf("meters = %v", snap.Meters)
	}
	// Usage-based means no ceiling to enforce — the customer pays for what they
	// use — so there are no quota keys at all.
	if len(snap.Quotas) != 0 {
		t.Errorf("a usage-based plan declares no quotas, got %v", snap.Quotas)
	}
}

// TestIncompleteDoesNotGrantService pins the decision that separates this
// service's answer from billing's own `entitled`.
//
// INCOMPLETE is exactly "chose the paid plan and never paid". Treating it as a
// grace period would make the first month free for anyone who abandons the
// checkout. The open invoice rides along so the screen can offer the way out.
func TestIncompleteDoesNotGrantService(t *testing.T) {
	snap := SnapshotFrom("user-3", &billingclient.Entitlements{
		CustomerID: "cus_3",
		Subscriptions: []billingclient.EntitlementSubscription{{
			ID:       "sub_new",
			Status:   "INCOMPLETE",
			Entitled: false,
			Plan:     "pro",
			OpenInvoice: &billingclient.EntitlementInvoice{
				ID: "in_1", TotalCents: 35000, DueDate: "2026-08-16",
				CheckoutURL: "https://billing.aoctech.app/checkout?token=x",
			},
		}},
	})

	if GrantsService(snap) {
		t.Fatal("a subscription whose first invoice was never paid must not grant service")
	}
	if snap.OpenInvoice == nil || snap.OpenInvoice.CheckoutURL == "" {
		t.Fatalf("the way to pay must survive into the snapshot, got %+v", snap.OpenInvoice)
	}
}

// TestPastDueDoesNotGrantServiceEvenThoughBillingSaysEntitled is the deliberate
// disagreement (D2). Billing counts PAST_DUE as entitled because dunning has not
// given up; the DF-e blocks it. Both facts are kept.
func TestPastDueDoesNotGrantServiceEvenThoughBillingSaysEntitled(t *testing.T) {
	sub := proSubscription()
	sub.Status = "PAST_DUE"
	sub.Entitled = true

	snap := SnapshotFrom("user-4", &billingclient.Entitlements{
		CustomerID:    "cus_4",
		Subscriptions: []billingclient.EntitlementSubscription{sub},
	})

	if GrantsService(snap) {
		t.Fatal("PAST_DUE must not grant service in the DF-e")
	}
	if !snap.Entitled {
		t.Fatal("billing's own answer must be kept rather than overwritten, so the disagreement stays visible")
	}
}

// TestALiveSubscriptionWinsOverACancelledOne: an account is meant to have one
// subscription, but a cancelled row lying beside a new one must not shadow it.
func TestALiveSubscriptionWinsOverACancelledOne(t *testing.T) {
	dead := proSubscription()
	dead.ID, dead.Status, dead.Entitled, dead.Plan = "sub_old", "CANCELED", false, "free"

	snap := SnapshotFrom("user-5", &billingclient.Entitlements{
		CustomerID:    "cus_5",
		Subscriptions: []billingclient.EntitlementSubscription{dead, proSubscription()},
	})

	if snap.SubscriptionID != "sub_pro" || snap.Plan != "pro" {
		t.Fatalf("the live subscription must govern, got %+v", snap)
	}
}

// TestAnAccountWithNoSubscriptionIsNotAnError: never having chosen a plan is an
// ordinary state, and it must be distinguishable from a cancelled one — the
// screens differ.
func TestAnAccountWithNoSubscriptionIsNotAnError(t *testing.T) {
	empty := SnapshotFrom("user-6", &billingclient.Entitlements{})
	if empty.SubscriptionID != "" || empty.Status != "" || GrantsService(empty) {
		t.Fatalf("snapshot = %+v", empty)
	}

	cancelled := proSubscription()
	cancelled.Status, cancelled.Entitled = "CANCELED", false
	ended := SnapshotFrom("user-6", &billingclient.Entitlements{
		Subscriptions: []billingclient.EntitlementSubscription{cancelled},
	})
	if ended.Status != "CANCELED" {
		t.Fatalf("a cancelled subscription must still report its status, got %+v", ended)
	}
	if GrantsService(ended) {
		t.Fatal("CANCELED must not grant service")
	}
}

// TestAnUnreadableQuotaIsDroppedRatherThanGuessed: a malformed value in the seed
// must not become unlimited (giving the product away) nor zero (blocking a
// paying customer). It is dropped, which reads as "not granted", and logged.
func TestAnUnreadableQuotaIsDroppedRatherThanGuessed(t *testing.T) {
	sub := proSubscription()
	sub.Items[0].Metadata["quota_mdfe"] = "muitos"

	snap := SnapshotFrom("user-7", &billingclient.Entitlements{
		Subscriptions: []billingclient.EntitlementSubscription{sub},
	})

	if _, ok := Quota(snap, "mdfe"); ok {
		t.Fatal("an unreadable quota must not become a limit")
	}
	if got, ok := Quota(snap, "nfe"); !ok || got != 1200 {
		t.Fatalf("the readable quotas beside it must survive, got %d (present=%t)", got, ok)
	}
}

// TestNoChargeModeGrantsEverything: a deployment without billing configured runs
// unlimited, and says so rather than claiming a plan nobody bought.
func TestNoChargeModeGrantsEverything(t *testing.T) {
	snap := noChargeSnapshot("user-8")
	if !GrantsService(snap) || !snap.NoCharge {
		t.Fatalf("snapshot = %+v", snap)
	}
	// Every meter is unlimited, including ones no plan declares — there is no
	// plan.
	for _, meter := range []string{"nfe", "nfce", "cte", "mdfe", "nfse", "companies", "users"} {
		got, ok := Quota(snap, meter)
		if !ok || got != QuotaUnlimited {
			t.Errorf("quota %s = %d (present=%t), want unlimited", meter, got, ok)
		}
	}
}

// TestGrantsServiceOnNothing: the gate must answer for a nil snapshot rather
// than panicking, because "no row yet" reaches it on an account's first request.
func TestGrantsServiceOnNothing(t *testing.T) {
	if GrantsService(nil) {
		t.Fatal("nothing grants no service")
	}
	if _, ok := Quota(nil, "nfe"); ok {
		t.Fatal("nothing grants no quota")
	}
	if snap := SnapshotFrom("user-9", nil); snap.UserID != "user-9" {
		t.Fatalf("snapshot = %+v", snap)
	}
}
