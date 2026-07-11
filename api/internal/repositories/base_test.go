package repositories

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestBuildUpdateExpr_SetAndRemove(t *testing.T) {
	expr, names, values, err := buildUpdateExpr(map[string]any{
		"name": "X",
		"cest": nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(expr, "SET #name = :name") {
		t.Errorf("expected SET clause for name, got %q", expr)
	}
	if !strings.Contains(expr, "REMOVE #cest") {
		t.Errorf("expected REMOVE clause for cest, got %q", expr)
	}
	if _, ok := values[":cest"]; ok {
		t.Errorf("nil value must not be in ExpressionAttributeValues")
	}
	if names["#cest"] != "cest" {
		t.Errorf("expected name mapping for cest")
	}
}

func TestBuildUpdateExpr_RemoveOnly(t *testing.T) {
	expr, _, values, err := buildUpdateExpr(map[string]any{"cest": nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(expr, "SET") {
		t.Errorf("expected no SET clause, got %q", expr)
	}
	if !strings.HasPrefix(expr, "REMOVE") {
		t.Errorf("expected REMOVE-only expression, got %q", expr)
	}
	if len(values) != 0 {
		t.Errorf("expected no expression values, got %d", len(values))
	}
}

func TestBase_BuildPutTxItem(t *testing.T) {
	b := Base{TableName: "test_table"} // no client needed — these builders only read TableName, exactly like existing base_test.go tests construct pure inputs (see TestBuildUpdateExpr_* in the same file) without a DynamoDB client
	item := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "PK1"},
		"sk": &types.AttributeValueMemberS{Value: "SK1"},
	}
	txItem := b.BuildPutTxItem(item)
	if txItem.Put == nil {
		t.Fatal("expected Put transact item, got nil")
	}
	if *txItem.Put.TableName != b.TableName {
		t.Errorf("table name = %q, want %q", *txItem.Put.TableName, b.TableName)
	}
	if txItem.Put.Item["pk"].(*types.AttributeValueMemberS).Value != "PK1" {
		t.Error("item not carried through unchanged")
	}
}

func TestBase_BuildUpdateTxItem(t *testing.T) {
	b := Base{TableName: "test_table"}
	sk := "SK1"
	txItem, err := b.BuildUpdateTxItem("PK1", &sk, map[string]any{"name": "new-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txItem.Update == nil {
		t.Fatal("expected Update transact item, got nil")
	}
	if *txItem.Update.ConditionExpression != "attribute_exists(pk)" {
		t.Errorf("condition = %q, want attribute_exists(pk)", *txItem.Update.ConditionExpression)
	}
	if txItem.Update.Key["sk"].(*types.AttributeValueMemberS).Value != "SK1" {
		t.Error("sk not set on key")
	}
}

func TestBase_BuildDeleteTxItem(t *testing.T) {
	b := Base{TableName: "test_table"}
	txItem := b.BuildDeleteTxItem("PK1", "SK1")
	if txItem.Delete == nil {
		t.Fatal("expected Delete transact item, got nil")
	}
	if *txItem.Delete.ConditionExpression != "attribute_exists(pk)" {
		t.Errorf("condition = %q, want attribute_exists(pk)", *txItem.Delete.ConditionExpression)
	}
}
