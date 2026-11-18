//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/testsupport"
)

func newPushRepo() *repositories.PushRepository { return repositories.NewPushRepository(db, cfg) }

func pushSub(n int) repositories.PushSubscription {
	return repositories.PushSubscription{
		Endpoint: fmt.Sprintf("https://fcm.googleapis.com/fcm/send/token-%d", n), P256dh: "p", Auth: "a", VapidKID: "kid", Device: "Chrome no Linux",
	}
}

// A device renews on its own row; the eleventh replaces the one seen least recently and the count never passes ten.
func TestPushRepo_UpsertIsIdempotentAndCapsAtTen(t *testing.T) {
	ctx := context.Background()
	repo := newPushRepo()
	u := "user-" + randomCNPJ()

	first, err := repo.UpsertSubscription(ctx, u, pushSub(0))
	if err != nil {
		t.Fatal(err)
	}
	again, err := repo.UpsertSubscription(ctx, u, pushSub(0))
	if err != nil || again != first {
		t.Fatalf("same endpoint must land on the same row: %q %q %v", first, again, err)
	}
	for i := 1; i < repositories.MaxPushSubscriptionsPerUser; i++ {
		time.Sleep(2 * time.Millisecond) // last_seen_at orders the eviction
		if _, err := repo.UpsertSubscription(ctx, u, pushSub(i)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.ListSubscriptions(ctx, u)
	if err != nil || len(list) != repositories.MaxPushSubscriptionsPerUser {
		t.Fatalf("devices = %d, want %d (%v)", len(list), repositories.MaxPushSubscriptionsPerUser, err)
	}
	// Renew device 0 so device 1 is now the one seen least recently.
	time.Sleep(2 * time.Millisecond)
	if _, err := repo.UpsertSubscription(ctx, u, pushSub(0)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertSubscription(ctx, u, pushSub(99)); err != nil {
		t.Fatal(err)
	}
	list, _ = repo.ListSubscriptions(ctx, u)
	if len(list) != repositories.MaxPushSubscriptionsPerUser {
		t.Fatalf("after the eleventh: %d devices", len(list))
	}
	ids := map[string]bool{}
	for _, d := range list {
		ids[d.ID] = true
	}
	if !ids[repositories.PushSubscriptionID(pushSub(99).Endpoint)] || ids[repositories.PushSubscriptionID(pushSub(1).Endpoint)] {
		t.Errorf("the new device must be in and the least recently seen (1) out: %v", ids)
	}

	// Deleting frees a slot, and a second delete of the same device is a not-found.
	if err := repo.DeleteSubscription(ctx, u, first); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteSubscription(ctx, u, first); err == nil {
		t.Error("deleting a device twice must report not found")
	}
	if _, err := repo.UpsertSubscription(ctx, u, pushSub(100)); err != nil {
		t.Fatal(err)
	}
	if list, _ = repo.ListSubscriptions(ctx, u); len(list) != repositories.MaxPushSubscriptionsPerUser {
		t.Errorf("devices after delete and add = %d", len(list))
	}
}

// Preferences: seeding never overwrites a choice, the fan-out lists only who ticked the event, and an empty choice
// leaves no row.
func TestPushRepo_PreferencesAndRecipients(t *testing.T) {
	ctx := context.Background()
	repo := newPushRepo()
	company, a, b := "CNPJ_"+randomCNPJ(), "user-a-"+randomCNPJ(), "user-b-"+randomCNPJ()

	if wrote, err := repo.SeedPreference(ctx, company, a, []string{"distribution.received"}); err != nil || !wrote {
		t.Fatalf("seed: %v %v", wrote, err)
	}
	if err := repo.PutPreference(ctx, company, a, []string{"distribution.received", "document.rejected"}); err != nil {
		t.Fatal(err)
	}
	if wrote, _ := repo.SeedPreference(ctx, company, a, []string{"distribution.received"}); wrote {
		t.Error("seeding must not overwrite a choice")
	}
	if err := repo.PutPreference(ctx, company, b, []string{"document.rejected"}); err != nil {
		t.Fatal(err)
	}

	page, err := repo.ListRecipients(ctx, company, "document.rejected", 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(page.Subs)
	want := []string{a, b}
	slices.Sort(want)
	if !slices.Equal(page.Subs, want) {
		t.Errorf("rejected recipients = %v, want %v", page.Subs, want)
	}
	if page, _ = repo.ListRecipients(ctx, company, "document.authorized", 100, nil); len(page.Subs) != 0 {
		t.Errorf("nobody ticked authorized: %v", page.Subs)
	}

	if err := repo.PutPreference(ctx, company, a, nil); err != nil {
		t.Fatal(err)
	}
	if events, _ := repo.GetPreference(ctx, company, a); events != nil {
		t.Errorf("an empty choice must leave no row: %v", events)
	}
}

// The markers: an event is claimed once, a window opens once until it closes, and a released claim can be taken again.
func TestPushRepo_Markers(t *testing.T) {
	ctx := context.Background()
	repo := newPushRepo()
	company := "CNPJ_" + randomCNPJ()

	if ok, err := repo.MarkEvent(ctx, company, "ev1"); err != nil || !ok {
		t.Fatalf("first claim: %v %v", ok, err)
	}
	if ok, _ := repo.MarkEvent(ctx, company, "ev1"); ok {
		t.Error("a second claim of the same event must lose")
	}
	if err := repo.UnmarkEvent(ctx, company, "ev1"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := repo.MarkEvent(ctx, company, "ev1"); !ok {
		t.Error("a released claim must be takeable again")
	}

	if ok, err := repo.OpenWindow(ctx, company, "document.rejected", time.Hour); err != nil || !ok {
		t.Fatalf("first window: %v %v", ok, err)
	}
	if ok, _ := repo.OpenWindow(ctx, company, "document.rejected", time.Hour); ok {
		t.Error("an open window must refuse a second event")
	}
	if ok, _ := repo.OpenWindow(ctx, company, "document.failed", time.Hour); !ok {
		t.Error("windows are per event type")
	}
	if err := repo.CloseWindow(ctx, company, "document.rejected"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := repo.OpenWindow(ctx, company, "document.rejected", time.Hour); !ok {
		t.Error("a closed window must reopen")
	}
}

// Erasing a person removes their devices and their choice in every company, through the sparse user-index, and
// leaves another person's rows alone.
func TestPurge_ErasesAPersonsPush(t *testing.T) {
	ctx := context.Background()
	svc, _ := newErasureSvc(testsupport.NewObjectStore(), cpfOf(""))
	svc.WithSettle(0)
	repo := newPushRepo()
	u, other := "user-"+randomCNPJ(), "user-other-"+randomCNPJ()
	c1, c2 := "CNPJ_"+randomCNPJ(), "CNPJ_"+randomCNPJ()

	for _, who := range []string{u, other} {
		if _, err := repo.UpsertSubscription(ctx, who, pushSub(len(who))); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []string{c1, c2} {
		for _, who := range []string{u, other} {
			if err := repo.PutPreference(ctx, c, who, []string{"distribution.received"}); err != nil {
				t.Fatal(err)
			}
		}
	}

	ack, err := svc.Purge(ctx, eraseMessage("req-push-"+u, u))
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := repo.ListSubscriptions(ctx, u); len(list) != 0 {
		t.Errorf("the person's devices survived: %d", len(list))
	}
	for _, c := range []string{c1, c2} {
		if events, _ := repo.GetPreference(ctx, c, u); events != nil {
			t.Errorf("the person's choice in %s survived: %v", c, events)
		}
		if events, _ := repo.GetPreference(ctx, c, other); len(events) != 1 {
			t.Errorf("another person's choice in %s was erased: %v", c, events)
		}
	}
	if list, _ := repo.ListSubscriptions(ctx, other); len(list) != 1 {
		t.Errorf("another person's devices were erased: %d", len(list))
	}
	if ack.Counts[repositories.TablePush] < 4 { // 1 device + counter row + 2 preferences
		t.Errorf("counts[push] = %d, want at least 4", ack.Counts[repositories.TablePush])
	}
}
