package services

import (
	"context"
	"fmt"

	"gopkg.aoctech.app/api-commons/cache"
)

// reachCacheTTL is short on purpose. It has to be long enough that the issuance
// path is not a round trip per request, and short enough that a revoked edge
// stops working in seconds rather than minutes — a revocation nobody can rely on
// is a revocation that does not exist.
const reachCacheTTL = 60

// reachSource is the one call this service makes, behind an interface so the
// caching and the failure mode are testable without ctech-account.
type reachSource interface {
	// Reach answers whether userID may act for companyID, and which
	// organization the company belongs to. A refusal is (,"", false, nil); an
	// outage is a non-nil error, and the two must never be conflated.
	Reach(ctx context.Context, companyID, userID string) (string, bool, error)
}

// ReachService answers "may this person act for this company" by asking
// ctech-account, which owns that fact (ctech-billing ADR 0023).
//
// **It fails closed, and that is the opposite of BillingService's snapshot in
// this same package.** The snapshot degrades open on purpose so a billing outage
// does not stop a customer from issuing. This must not: billing decides whether
// somebody SHOULD be allowed to pay, reach decides whether they ARE WHO THEY SAY
// THEY ARE, and guessing at the second turns an outage into an authorization
// bypass. Copying the snapshot's shape into here is the mistake this paragraph
// exists to prevent.
//
// **It never falls back to the product's own membership row.** That fallback is
// the second lineage the unification removes, and it is the line somebody adds
// during an incident because it makes the alarm stop. There is deliberately no
// MembershipService on this struct for it to reach for.
type ReachService struct {
	src   reachSource
	cache cache.Backend
}

func NewReachService(src reachSource, c cache.Backend) *ReachService {
	return &ReachService{src: src, cache: c}
}

// reachCacheKey is keyed by BOTH the person and the company.
//
// Keying on the company alone would hand one person another's reach, which is
// the worst defect this file could carry — and it would look like a cache hit.
func reachCacheKey(companyID, userID string) string {
	return fmt.Sprintf("dfe:reach:%s:%s", companyID, userID)
}

// reachAnswer is what gets cached. A struct rather than a bool because the
// organization travels with the grant, and because a cached false has to be
// distinguishable from a cache miss.
type reachAnswer struct {
	OrganizationID string `json:"organization_id"`
	MayAct         bool   `json:"may_act"`
}

// MayAct answers reach, from cache when it can.
//
// Both outcomes are cached — a grant so the issuance path is not a round trip
// per request, and a refusal so a stranger probing company ids does not cost one
// request to ctech-account per attempt.
//
// An outage is NOT cached. The answer it would store is "we do not know", and
// caching that turns a blip into a minute of refusals for people who are
// perfectly entitled.
func (s *ReachService) MayAct(ctx context.Context, companyID, userID string) (string, bool, error) {
	key := reachCacheKey(companyID, userID)
	if v, ok := CacheGet[reachAnswer](ctx, s.cache, key); ok {
		return v.OrganizationID, v.MayAct, nil
	}

	orgID, mayAct, err := s.src.Reach(ctx, companyID, userID)
	if err != nil {
		// Fail closed: refuse, and say why. A caller that cannot tell an outage
		// from a refusal will eventually treat one as the other.
		return "", false, fmt.Errorf("resolving reach for %s: %w", companyID, err)
	}

	CacheSet(ctx, s.cache, key, reachAnswer{OrganizationID: orgID, MayAct: mayAct}, reachCacheTTL)
	return orgID, mayAct, nil
}

// Invalidate drops a cached answer, so a grant made in ctech-account can be made
// to land immediately rather than at the TTL.
//
// It exists for the case where this service is told about a change; it is not a
// substitute for the TTL, because nothing guarantees this instance is the one
// that hears.
func (s *ReachService) Invalidate(ctx context.Context, companyID, userID string) {
	cacheDelete(ctx, s.cache, reachCacheKey(companyID, userID))
}
