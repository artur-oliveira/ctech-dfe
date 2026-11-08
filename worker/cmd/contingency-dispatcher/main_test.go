package main

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"gopkg.aoctech.app/dfe/worker/internal/config"
)

type fakeDB struct{ calls []*dynamodb.QueryInput }

func (f *fakeDB) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	f.calls = append(f.calls, in)
	if *in.TableName != "dev_ctes" {
		return &dynamodb.QueryOutput{}, nil
	}
	return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{{
		"pk": &types.AttributeValueMemberS{Value: "CNPJ_1"}, "sk": &types.AttributeValueMemberS{Value: "KEY1"},
	}}}, nil
}

type fakeInv struct {
	bodies []string
	kinds  []lambdaTypes.InvocationType
}

func (f *fakeInv) Invoke(_ context.Context, in *lambdaSDK.InvokeInput, _ ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error) {
	f.bodies = append(f.bodies, string(in.Payload))
	f.kinds = append(f.kinds, in.InvocationType)
	return &lambdaSDK.InvokeOutput{}, nil
}

func TestDispatchUsesSparseIndexAndInvokesAsync(t *testing.T) {
	db, inv := &fakeDB{}, &fakeInv{}
	cfg := &config.Config{TablePrefix: "dev", ContingencyFunctionName: "dev-contingency-retransmit-worker"}
	if err := dispatch(context.Background(), cfg, db, inv); err != nil {
		t.Fatal(err)
	}
	if len(db.calls) != 4 {
		t.Fatalf("one Query per document table, got %d", len(db.calls))
	}
	for _, c := range db.calls {
		if *c.IndexName != "contingency-index" {
			t.Fatalf("must read the sparse index: %v", *c.IndexName)
		}
	}
	if len(inv.bodies) != 1 || !strings.Contains(inv.bodies[0], `"table":"ctes"`) || inv.kinds[0] != lambdaTypes.InvocationTypeEvent {
		t.Fatalf("%v %v", inv.bodies, inv.kinds)
	}
}

// The probe lists only the sparse auto-contingency-index of the four configuration tables (no Scan) and invokes the
// worker with kind=probe, one per row; the production switch is off, so the Lambda handler never calls it today.
func TestDispatchProbesUsesSparseIndexPerConfigTable(t *testing.T) {
	db, inv := &probeDB{}, &fakeInv{}
	cfg := &config.Config{TablePrefix: "dev", ContingencyFunctionName: "dev-contingency-retransmit-worker"}
	dispatchProbes(context.Background(), cfg, db, inv)
	if len(db.calls) != 4 {
		t.Fatalf("one Query per configuration table, got %d", len(db.calls))
	}
	for _, c := range db.calls {
		if *c.IndexName != "auto-contingency-index" || !strings.Contains(*c.TableName, "_configs") {
			t.Fatalf("must read the sparse index of a config table: %s %s", *c.TableName, *c.IndexName)
		}
	}
	if len(inv.bodies) != 1 || !strings.Contains(inv.bodies[0], `"kind":"probe"`) ||
		!strings.Contains(inv.bodies[0], `"table":"organization_cte_configs"`) || !strings.Contains(inv.bodies[0], `"pk":"ORG1"`) ||
		inv.kinds[0] != lambdaTypes.InvocationTypeEvent {
		t.Fatalf("%v %v", inv.bodies, inv.kinds)
	}
}

type probeDB struct{ calls []*dynamodb.QueryInput }

func (f *probeDB) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	f.calls = append(f.calls, in)
	if *in.TableName != "dev_organization_cte_configs" {
		return &dynamodb.QueryOutput{}, nil
	}
	return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{{"pk": &types.AttributeValueMemberS{Value: "ORG1"}}}}, nil
}
