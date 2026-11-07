package main

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/worker/internal/contingency"
	"gopkg.aoctech.app/dfe/worker/internal/service"
)

type fakeStore struct {
	item    map[string]types.AttributeValue
	updated map[string]types.AttributeValue
}

func (f *fakeStore) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{Item: f.item}, nil
}
func (f *fakeStore) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.updated = in.ExpressionAttributeValues
	return &dynamodb.UpdateItemOutput{}, nil
}

// fakeSend stands in for DfeService.Process: it records the message and writes the cStat the
// egress answered into the stored item, the way the real worker does.
type fakeSend struct {
	store *fakeStore
	cStat string
	calls int
	got   service.WorkerMessage
}

func (f *fakeSend) Process(_ context.Context, m service.WorkerMessage) error {
	f.calls++
	f.got = m
	if f.cStat != "" {
		f.store.item["sefaz_status"] = &types.AttributeValueMemberS{Value: f.cStat}
	}
	return nil
}

func sv(s string) types.AttributeValue { return &types.AttributeValueMemberS{Value: s} }

func row(deadline time.Time, state string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		contingency.StateAttr:    sv(state),
		contingency.DeadlineAttr: sv(deadline.UTC().Format(time.RFC3339)),
		contingency.MsgAttr:      sv(`{"access_key":"K","sefaz_service":"NFeAutorizacao","body":{"@signed_xml":"<x/>"}}`),
	}
}

func TestResendAndClassify(t *testing.T) {
	now := time.Now()
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	for _, c := range []struct {
		name      string
		item      map[string]types.AttributeValue
		cStat     string
		sends     int
		wantState string // "" = not updated
	}{
		{"authorized in time", row(future, contingency.StatePending), "100", 1, contingency.StateAuthorized},
		{"authorized past the deadline counts as success", row(past, contingency.StatePending), "100", 1, contingency.StateAuthorizedLate},
		{"cStat 150 is a success", row(future, contingency.StatePending), "150", 1, contingency.StateAuthorizedLate},
		{"rejected ends it", row(future, contingency.StatePending), "225", 1, contingency.StateRejectedLate},
		{"no answer in time stays pending", row(future, contingency.StatePending), "", 1, ""},
		{"no answer past the deadline is overdue", row(past, contingency.StatePending), "", 1, contingency.StateOverdue},
		{"already reconciled is skipped (idempotent)", row(past, contingency.StateAuthorized), "100", 0, ""},
		{"gone", nil, "", 0, ""},
	} {
		st := &fakeStore{item: c.item}
		send := &fakeSend{store: st, cStat: c.cStat}
		if err := handle(context.Background(), "dev", st, send, contingency.Job{Table: "nfces", PK: "p", SK: "s"}, now); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if send.calls != c.sends {
			t.Errorf("%s: sends=%d want %d", c.name, send.calls, c.sends)
		}
		if c.sends == 1 && send.got.SefazService != "NFeAutorizacao" {
			t.Errorf("%s: message not forwarded: %+v", c.name, send.got)
		}
		got := ""
		if st.updated != nil {
			got = st.updated[":o"].(*types.AttributeValueMemberS).Value
		}
		if got != c.wantState {
			t.Errorf("%s: state=%q want %q", c.name, got, c.wantState)
		}
	}
}
