package v1

import (
	"context"
	"errors"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

const wsCompany = "01a04fc3-baa2-7cae-ac62-0ca3260a5888"

type fakeReach struct {
	mayAct bool
	err    error
	asked  int
}

func (f *fakeReach) MayAct(context.Context, string, string) (string, bool, error) {
	f.asked++
	return "org_1", f.mayAct, f.err
}

var wsRow = &services.Membership{OrgPK: wsCompany, UserID: "usr_1", Role: "OWNER"}

// The socket follows the HTTP rule (middleware/reach.go): reach from
// ctech-account first, then the row. A row that survived a revoked edge — or a
// company whose workspace is not an organization — must not subscribe anybody
// to that company's events.
func TestTheSocketAsksReachBeforeConnecting(t *testing.T) {
	ctx := context.Background()
	if !wsMayConnect(ctx, &fakeReach{mayAct: true}, wsCompany, "usr_1", wsRow) {
		t.Fatal("a member with reach was refused")
	}
	if wsMayConnect(ctx, &fakeReach{mayAct: false}, wsCompany, "usr_1", wsRow) {
		t.Fatal("a revoked edge connected on the strength of the local row")
	}
	// Fail closed at connect, like the HTTP routes: an outage is not consent.
	if wsMayConnect(ctx, &fakeReach{err: errors.New("timeout")}, wsCompany, "usr_1", wsRow) {
		t.Fatal("an outage was read as permission")
	}
	if wsMayConnect(ctx, &fakeReach{mayAct: true}, wsCompany, "usr_1", nil) {
		t.Fatal("reach without a row connected")
	}
}

// A legacy CNPJ_/CPF_ key has no company id to ask about, and no reach client
// means the flip is off: both keep today's row-only rule, as the HTTP side does.
func TestTheSocketKeepsTheRowRuleWhereHTTPDoes(t *testing.T) {
	ctx := context.Background()
	r := &fakeReach{mayAct: false}
	if !wsMayConnect(ctx, r, "CNPJ_11222333000181", "usr_1", wsRow) || r.asked != 0 {
		t.Fatalf("a legacy key asked reach (%d) or was refused", r.asked)
	}
	if !wsMayConnect(ctx, nil, wsCompany, "usr_1", wsRow) {
		t.Fatal("with reach unconfigured, a member was refused")
	}
}

// The heartbeat re-check closes on a refusal but keeps the socket through an
// outage — dropping every live connection on a ctech-account blip would be a
// self-inflicted outage, and the next tick asks again.
func TestTheHeartbeatClosesOnARefusalAndRidesOutAnOutage(t *testing.T) {
	ctx := context.Background()
	if wsStillAllowed(ctx, &fakeReach{mayAct: false}, wsCompany, "usr_1", wsRow, nil) {
		t.Fatal("a revoked edge stayed subscribed")
	}
	if !wsStillAllowed(ctx, &fakeReach{err: errors.New("timeout")}, wsCompany, "usr_1", wsRow, nil) {
		t.Fatal("an outage closed a live socket")
	}
	if wsStillAllowed(ctx, &fakeReach{mayAct: true}, wsCompany, "usr_1", nil, nil) {
		t.Fatal("a removed row stayed subscribed")
	}
	if !wsStillAllowed(ctx, &fakeReach{mayAct: true}, wsCompany, "usr_1", nil, errors.New("dynamo")) {
		t.Fatal("a row lookup error closed a live socket")
	}
	if !wsStillAllowed(ctx, &fakeReach{mayAct: true}, wsCompany, "usr_1", wsRow, nil) {
		t.Fatal("a member with reach was dropped")
	}
}
