package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/ws"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

type fakePush struct {
	results []services.PushResult
	dists   []pushDistribution
	err     error
}

type pushDistribution struct {
	company, key, docType string
	at                    time.Time
}

func (f *fakePush) EmitResult(_ context.Context, r services.PushResult) error {
	f.results = append(f.results, r)
	return f.err
}

func (f *fakePush) EmitDistribution(_ context.Context, company, key, docType string, at time.Time) error {
	f.dists = append(f.dists, pushDistribution{company, key, docType, at})
	return f.err
}

func pushConsumer(p PushEmitter) *ResultsConsumer {
	return (&ResultsConsumer{registry: ws.NewMemoryRegistry(), cache: cache.NewMemoryBackend(16)}).WithPush(p)
}

// Every terminal document result reaches the push service with what it needs to decide, and nothing else does.
func TestDispatch_TerminalDocumentResultsReachPush(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"authorized", "rejected", "failed"} {
		push := &fakePush{}
		if err := pushConsumer(push).dispatch(context.Background(), resultMessage(documentResult(status), "1800000000123")); err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if len(push.results) != 1 || len(push.dists) != 0 {
			t.Fatalf("%s: results=%d distributions=%d", status, len(push.results), len(push.dists))
		}
		r := push.results[0]
		if r.CompanyPK != "COMP1" || r.Table != "nfes" || r.Status != status || r.DocumentKey != "35250512345678000195550010000000011000000015" || r.SentAt.UnixMilli() != 1800000000123 {
			t.Fatalf("%s: result = %+v", status, r)
		}
	}
}

// An event on a document is not a document result, and the push service decides what an in-flight status is worth; the
// consumer only forwards document results.
func TestDispatch_EventResultsDoNotReachPush(t *testing.T) {
	t.Parallel()
	push := &fakePush{}
	event := map[string]any{"result_kind": "event", "table_name": "nfes", "status": "authorized", "doc_pk": "prod#C", "access_key": "k"}
	if err := pushConsumer(push).dispatch(context.Background(), resultMessage(event, "")); err != nil {
		t.Fatal(err)
	}
	if len(push.results) != 0 || len(push.dists) != 0 {
		t.Errorf("an event result reached push: %+v %+v", push.results, push.dists)
	}
}

func TestDispatch_ADistributionReachesPushWithItsDocumentType(t *testing.T) {
	t.Parallel()
	for name, event := range map[string]map[string]any{
		"doc_type given": {"type": "new_distribution_nfe", "org_pk": "C", "access_key": "k", "doc_type": "cte"},
		"from the type":  {"type": "new_distribution_mdfe", "org_pk": "C", "access_key": "k"},
	} {
		push := &fakePush{}
		if err := pushConsumer(push).dispatch(context.Background(), resultMessage(event, "1800000000000")); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(push.dists) != 1 || len(push.results) != 0 {
			t.Fatalf("%s: dists=%d results=%d", name, len(push.dists), len(push.results))
		}
		d := push.dists[0]
		want := map[string]string{"doc_type given": "cte", "from the type": "mdfe"}[name]
		if d.company != "C" || d.key != "k" || d.docType != want {
			t.Errorf("%s: %+v, want type %q", name, d, want)
		}
	}
}

// A failed hand-off keeps the message on the queue (the push service released its markers), and push being unwired
// is no reason to drop anything.
func TestDispatch_APushFailureLeavesTheMessageOnTheQueue(t *testing.T) {
	t.Parallel()
	push := &fakePush{err: errors.New("lambda throttled")}
	if err := pushConsumer(push).dispatch(context.Background(), resultMessage(documentResult("rejected"), "")); err == nil {
		t.Fatal("a failed push hand-off was swallowed; the redelivery that retries it would never come")
	}
}

func TestDispatch_NoPushWiredStillDispatches(t *testing.T) {
	t.Parallel()
	c := &ResultsConsumer{registry: ws.NewMemoryRegistry(), cache: cache.NewMemoryBackend(16)}
	if err := c.dispatch(context.Background(), resultMessage(documentResult("rejected"), "")); err != nil {
		t.Fatal(err)
	}
}
