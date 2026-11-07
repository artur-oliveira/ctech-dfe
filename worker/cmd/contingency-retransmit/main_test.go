package main

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/worker/internal/contingency"
)

type fakeStore struct {
	item    map[string]types.AttributeValue
	updated bool
}

func (f *fakeStore) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{Item: f.item}, nil
}
func (f *fakeStore) UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.updated = true
	return &dynamodb.UpdateItemOutput{}, nil
}

func TestOverdueLeavesTheIndexAndInTimeStaysPending(t *testing.T) {
	now := time.Now()
	row := func(deadline time.Time, state string) map[string]types.AttributeValue {
		return map[string]types.AttributeValue{
			contingency.StateAttr:    &types.AttributeValueMemberS{Value: state},
			contingency.DeadlineAttr: &types.AttributeValueMemberS{Value: deadline.UTC().Format(time.RFC3339)},
		}
	}
	for _, c := range []struct {
		name    string
		item    map[string]types.AttributeValue
		updated bool
	}{
		{"past deadline", row(now.Add(-time.Hour), contingency.StatePending), true},
		{"in time", row(now.Add(time.Hour), contingency.StatePending), false},
		{"already reconciled", row(now.Add(-time.Hour), contingency.StateAuthorized), false},
		{"gone", nil, false},
	} {
		f := &fakeStore{item: c.item}
		if err := handle(context.Background(), "dev", f, contingency.Job{Table: "nfces", PK: "p", SK: "s"}, now); err != nil {
			t.Fatal(err)
		}
		if f.updated != c.updated {
			t.Errorf("%s: updated=%v want %v", c.name, f.updated, c.updated)
		}
	}
}
