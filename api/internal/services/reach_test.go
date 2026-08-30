package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"gopkg.aoctech.app/api-commons/cache"
)

// fakeReachSource stands in for ctech-account. It records calls so a test can
// prove the cache is doing something, and can fail on demand.
type fakeReachSource struct {
	orgID string
	ok    bool
	err   error
	calls int
}

func (f *fakeReachSource) Reach(context.Context, string, string) (string, bool, error) {
	f.calls++
	return f.orgID, f.ok, f.err
}

func newReach(src reachSource) *ReachService {
	return NewReachService(src, cache.NewMemoryBackend(64))
}

// The rule the whole unification exists for. A fallback to the product's own
// membership row is the line somebody adds during an incident, and it is
// exactly the second lineage this removes.
//
// It is also the OPPOSITE of BillingService's snapshot, which degrades open on
// purpose so a billing outage does not stop issuance. The difference is the
// point: billing decides whether somebody SHOULD be allowed to pay, reach
// decides whether they are WHO THEY SAY THEY ARE, and guessing at the second
// turns an outage into an authorization bypass.
func TestAnUnreachableAccountRefusesRatherThanGuessing(t *testing.T) {
	src := &fakeReachSource{err: errors.New("connection refused")}
	_, ok, err := newReach(src).MayAct(context.Background(), "cmp_1", "usr_1")
	if ok {
		t.Fatal("an unreachable account granted reach")
	}
	if err == nil {
		t.Fatal("an outage answered as a plain refusal; the caller cannot tell it apart from a real one")
	}
}

// A refusal is cached. Without it, a stranger probing company ids costs one
// request to ctech-account per attempt.
func TestARefusalIsCached(t *testing.T) {
	src := &fakeReachSource{ok: false}
	svc := newReach(src)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, ok, err := svc.MayAct(ctx, "cmp_1", "usr_1"); ok || err != nil {
			t.Fatalf("attempt %d: %v %v", i, ok, err)
		}
	}
	if src.calls != 1 {
		t.Fatalf("asked ctech-account %d times for the same refusal", src.calls)
	}
}

// And a grant is cached, which is the whole reason this is not a round trip per
// request on the issuance path.
func TestAGrantIsCached(t *testing.T) {
	src := &fakeReachSource{orgID: "org_1", ok: true}
	svc := newReach(src)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		orgID, ok, err := svc.MayAct(ctx, "cmp_1", "usr_1")
		if !ok || err != nil || orgID != "org_1" {
			t.Fatalf("attempt %d: %q %v %v", i, orgID, ok, err)
		}
	}
	if src.calls != 1 {
		t.Fatalf("asked ctech-account %d times for the same grant", src.calls)
	}
}

// An outage is NOT cached. Caching it would turn a blip into minutes of
// refusals, and the answer it would cache is "we do not know".
func TestAnOutageIsNotCached(t *testing.T) {
	src := &fakeReachSource{err: errors.New("timeout")}
	svc := newReach(src)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, _, err := svc.MayAct(ctx, "cmp_1", "usr_1"); err == nil {
			t.Fatalf("attempt %d answered without an error", i)
		}
	}
	if src.calls != 3 {
		t.Fatalf("an outage was cached: %d calls for 3 attempts", src.calls)
	}
}

// Two people are two answers. A cache keyed on the company alone would hand one
// person another's reach, which is the worst bug this file could have.
func TestTheCacheIsKeyedByPersonAndCompany(t *testing.T) {
	src := &fakeReachSource{orgID: "org_1", ok: true}
	svc := newReach(src)
	ctx := context.Background()

	_, _, _ = svc.MayAct(ctx, "cmp_1", "usr_1")
	_, _, _ = svc.MayAct(ctx, "cmp_1", "usr_2")
	_, _, _ = svc.MayAct(ctx, "cmp_2", "usr_1")
	if src.calls != 3 {
		t.Fatalf("calls = %d, want 3 — the cache collapsed distinct questions", src.calls)
	}
}

// The struct has no membership dependency, so there is nothing for a fallback
// to reach for. Reflection, because the failure this guards is somebody wiring
// MembershipService in during an incident to make the alarm stop — and that
// change would compile, pass every other test, and quietly restore the second
// lineage the unification removed.
func TestReachServiceCannotReachTheProductsOwnRows(t *testing.T) {
	v := reflect.TypeOf(ReachService{})
	for i := 0; i < v.NumField(); i++ {
		name := v.Field(i).Type.String()
		if strings.Contains(name, "Membership") || strings.Contains(name, "OrgUser") {
			t.Errorf("ReachService grew %s (%s) — reach must never fall back to the product's own row (ctech-billing ADR 0023)",
				v.Field(i).Name, name)
		}
	}
}

// A cached grant expires. Without it a revoked edge keeps working until the
// process restarts, and a revocation nobody can rely on is not a revocation.
func TestACachedAnswerExpires(t *testing.T) {
	if reachCacheTTL <= 0 {
		t.Fatal("the cache never expires; a revoked edge would keep working")
	}
	if reachCacheTTL > 300 {
		t.Errorf("TTL is %ds — a revocation taking that long to land is not one anybody can rely on", reachCacheTTL)
	}
}

// Invalidate drops the answer, so a grant made in ctech-account can be made to
// land now rather than at the TTL.
func TestInvalidateDropsTheCachedAnswer(t *testing.T) {
	src := &fakeReachSource{ok: false}
	svc := newReach(src)
	ctx := context.Background()

	_, _, _ = svc.MayAct(ctx, "cmp_1", "usr_1")
	svc.Invalidate(ctx, "cmp_1", "usr_1")

	src.ok, src.orgID = true, "org_1"
	if _, ok, _ := svc.MayAct(ctx, "cmp_1", "usr_1"); !ok {
		t.Fatal("the stale refusal survived Invalidate")
	}
	if src.calls != 2 {
		t.Fatalf("calls = %d, want 2", src.calls)
	}
}
