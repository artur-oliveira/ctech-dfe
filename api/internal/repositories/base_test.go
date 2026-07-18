package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// buildUpdateExpr's own unit coverage (TestBuildUpdateExpr_SetAndRemove,
// TestBuildUpdateExpr_RemoveOnly) now lives in gopkg.aoctech.app/api-commons/dynamo,
// since the helper itself moved there and is no longer defined in this package.

func TestBase_BuildPutTxItem(t *testing.T) {
	b := Base{TableName: "test_table"} // no client needed — these builders only read TableName
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
	txItem, err := b.BuildUpdateTxItem("PK1", new("SK1"), map[string]any{"name": "new-name"})
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
