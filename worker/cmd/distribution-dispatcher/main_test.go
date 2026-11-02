package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"gopkg.aoctech.app/dfe/worker/internal/config"
)

type fakeDB struct {
	pages  map[string][][]string // table -> pages of pks
	errFor string
	calls  []*dynamodb.QueryInput
}

func (f *fakeDB) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	cp := *in
	f.calls = append(f.calls, &cp)
	table := *in.TableName
	if table == f.errFor {
		return nil, errors.New("boom")
	}
	pages := f.pages[table]
	if len(pages) == 0 {
		return &dynamodb.QueryOutput{}, nil
	}
	n := 0
	if in.ExclusiveStartKey != nil {
		n = int(in.ExclusiveStartKey["page"].(*types.AttributeValueMemberN).Value[0] - '0')
	}
	out := &dynamodb.QueryOutput{}
	for _, pk := range pages[n] {
		out.Items = append(out.Items, map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: pk}})
	}
	if n+1 < len(pages) {
		out.LastEvaluatedKey = map[string]types.AttributeValue{"page": &types.AttributeValueMemberN{Value: string(rune('0' + n + 1))}}
	}
	return out, nil
}

type fakeInvoker struct {
	bodies []string
	types  []lambdaTypes.InvocationType
	fns    []string
}

func (f *fakeInvoker) Invoke(_ context.Context, in *lambdaSDK.InvokeInput, _ ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error) {
	f.bodies = append(f.bodies, string(in.Payload))
	f.types = append(f.types, in.InvocationType)
	f.fns = append(f.fns, *in.FunctionName)
	return &lambdaSDK.InvokeOutput{}, nil
}

func TestDispatchQueriesSweepIndexAcrossPagesAndSurvivesATableError(t *testing.T) {
	db := &fakeDB{
		pages: map[string][][]string{
			"dev_organization_nfe_configs":  {{"CNPJ_1"}, {"CNPJ_2"}},
			"dev_organization_mdfe_configs": {{"CNPJ_3"}},
		},
		errFor: "dev_organization_cte_configs",
	}
	q := &fakeInvoker{}
	cfg := &config.Config{TablePrefix: "dev", DistributionFunctionName: "dev-distribution-worker"}

	if err := dispatch(context.Background(), cfg, db, q); err != nil {
		t.Fatal(err)
	}
	if len(q.bodies) != 3 {
		t.Fatalf("want 3 jobs (2 nfe pages + 1 mdfe, cte failed, nfse empty), got %d: %v", len(q.bodies), q.bodies)
	}
	for _, want := range []string{`"org_pk":"CNPJ_1"`, `"org_pk":"CNPJ_2"`, `"org_pk":"CNPJ_3"`} {
		found := false
		for _, b := range q.bodies {
			found = found || strings.Contains(b, want)
		}
		if !found {
			t.Errorf("missing job %s in %v", want, q.bodies)
		}
	}
	for i := range q.bodies {
		if q.types[i] != lambdaTypes.InvocationTypeEvent || q.fns[i] != "dev-distribution-worker" {
			t.Fatalf("job must be an async invoke of the distribution worker, got %v %s", q.types[i], q.fns[i])
		}
	}
	for _, c := range db.calls {
		if c.IndexName == nil || *c.IndexName != sweepIndex {
			t.Fatalf("every read must go through %s, got %+v", sweepIndex, c)
		}
	}
}
