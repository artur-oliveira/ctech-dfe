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

func TestMarshalMapOmitNull(t *testing.T) {
	t.Run("top-level nulls omitted", func(t *testing.T) {
		in := map[string]any{
			"name":  "Vasilhame",
			"cest":  nil,
			"value": "20.00",
			"empty": "",
		}
		out, err := MarshalMapOmitNull(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := out["cest"]; ok {
			t.Errorf("expected null attribute 'cest' to be omitted, got %#v", out["cest"])
		}
		if _, ok := out["name"]; !ok {
			t.Errorf("expected 'name' to be present")
		}
		if s, ok := out["empty"].(*types.AttributeValueMemberS); !ok || s.Value != "" {
			t.Errorf("expected empty string to be preserved, got %#v", out["empty"])
		}
	})

	t.Run("nested nulls omitted", func(t *testing.T) {
		in := map[string]any{
			"name": "X",
			"addr": map[string]any{"city": "POA", "complement": nil},
			"list": []any{map[string]any{"x": "1", "y": nil}},
		}
		out, err := MarshalMapOmitNull(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Nested map: addr.complement must be gone, addr.city must remain.
		addrAV, ok := out["addr"].(*types.AttributeValueMemberM)
		if !ok {
			t.Fatalf("expected 'addr' to be a map, got %T", out["addr"])
		}
		if _, exists := addrAV.Value["complement"]; exists {
			t.Errorf("expected nested null 'addr.complement' to be omitted")
		}
		if _, exists := addrAV.Value["city"]; !exists {
			t.Errorf("expected 'addr.city' to be present")
		}

		// List element map: list[0].y must be gone, list[0].x must remain.
		listAV, ok := out["list"].(*types.AttributeValueMemberL)
		if !ok {
			t.Fatalf("expected 'list' to be a list, got %T", out["list"])
		}
		if len(listAV.Value) == 0 {
			t.Fatalf("expected list to have one element")
		}
		elemAV, ok := listAV.Value[0].(*types.AttributeValueMemberM)
		if !ok {
			t.Fatalf("expected list element to be a map, got %T", listAV.Value[0])
		}
		if _, exists := elemAV.Value["y"]; exists {
			t.Errorf("expected nested null 'list[0].y' to be omitted")
		}
		if _, exists := elemAV.Value["x"]; !exists {
			t.Errorf("expected 'list[0].x' to be present")
		}
	})
}
