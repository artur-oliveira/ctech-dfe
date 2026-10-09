package v1

import (
	"context"

	"gopkg.aoctech.app/api-commons/observability"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// wsReach is the question the realtime channel asks ctech-account — the same
// one the HTTP routes ask (middleware/reach.go). Nil when the reach flip is off.
type wsReach interface {
	MayAct(ctx context.Context, companyID, userID string) (string, bool, error)
}

// reachApplies mirrors middleware.authorize: a legacy CNPJ_/CPF_ key has no
// company id to ask about, and an unconfigured reach client leaves the row as
// the access record. Everywhere else the edge decides reach.
func reachApplies(reach wsReach, orgPK string) bool {
	return reach != nil && repositories.IsCompanyKey(orgPK)
}

// wsMayConnect decides whether a socket may subscribe to a company's events.
//
// The row is required, and where reach applies the edge must grant too — fail
// closed, exactly like the HTTP routes. Before this, a row that survived a
// revoked edge kept receiving the company's realtime events.
func wsMayConnect(ctx context.Context, reach wsReach, orgPK, userID string, row *services.Membership) bool {
	if row == nil {
		return false
	}
	if !reachApplies(reach, orgPK) {
		return true
	}
	_, mayAct, err := reach.MayAct(ctx, orgPK, userID)
	return err == nil && mayAct
}

// wsStillAllowed is the heartbeat's re-check. A refusal — no row, or an edge
// that no longer grants — closes the socket. An error keeps it: dropping every
// live connection on a ctech-account or DynamoDB blip would be an outage of our
// own making, and the next tick asks again.
func wsStillAllowed(ctx context.Context, reach wsReach, orgPK, userID string, row *services.Membership, rowErr error) bool {
	if rowErr != nil {
		observability.Warn(ctx, "ws membership refresh failed", rowErr, "org", orgPK)
		return true
	}
	if row == nil {
		return false
	}
	if !reachApplies(reach, orgPK) {
		return true
	}
	_, mayAct, err := reach.MayAct(ctx, orgPK, userID)
	if err != nil {
		observability.Warn(ctx, "ws reach refresh failed", err, "org", orgPK)
		return true
	}
	return mayAct
}
