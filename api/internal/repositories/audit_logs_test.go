package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestAuditLogRepository_BuildLogTxItem(t *testing.T) {
	r := &AuditLogRepository{Base: Base{TableName: "dev_dfe_audit_logs"}}

	txItem, err := r.BuildLogTxItem(
		"CNPJ_12345678000195", "PRODUCT", "PRODUCT_abc123", "UPDATE",
		"user-1", "Jane Doe",
		[]Modification{{Name: "description", Before: "old", After: "new"}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txItem.Put == nil {
		t.Fatal("expected Put transact item, got nil")
	}
	item := txItem.Put.Item
	if item["pk"].(*types.AttributeValueMemberS).Value != "CNPJ_12345678000195" {
		t.Errorf("pk = %v, want org pk", item["pk"])
	}
	sk := item["sk"].(*types.AttributeValueMemberS).Value
	wantPrefix := "PRODUCT#PRODUCT_abc123#"
	if len(sk) <= len(wantPrefix) || sk[:len(wantPrefix)] != wantPrefix {
		t.Errorf("sk = %q, want prefix %q", sk, wantPrefix)
	}
	if item["action"].(*types.AttributeValueMemberS).Value != "UPDATE" {
		t.Errorf("action = %v, want UPDATE", item["action"])
	}
	if item["user_id"].(*types.AttributeValueMemberS).Value != "user-1" {
		t.Errorf("user_id = %v, want user-1", item["user_id"])
	}
	mods := item["modifications"].(*types.AttributeValueMemberL).Value
	if len(mods) != 1 {
		t.Fatalf("modifications len = %d, want 1", len(mods))
	}
}
