//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestProductService_CreateUpdateDelete_WriteAuditLogs(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := productSvc.Create(ctx, orgPK, productFields("AUD001", "Audited Widget"), "user-1", "Jane Doe")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := created["sk"].(*types.AttributeValueMemberS)
	sk := skAV.Value

	logs, err := auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if len(logs.Items) != 1 {
		t.Fatalf("audit_logs rows after Create = %d, want 1", len(logs.Items))
	}

	// Partial update: only "description" is supplied. The audit row's
	// modifications must list ONLY that field, not every field on the
	// record compared against nil (regression test for the false
	// "changed to null" bug when Diff was called with the raw partial
	// updates map instead of a merged before+updates snapshot).
	if _, err := productSvc.Update(ctx, orgPK, sk, map[string]any{"description": "Audited Widget v2"}, "user-1", "Jane Doe"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	logs, _ = auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if len(logs.Items) != 2 {
		t.Fatalf("audit_logs rows after Update = %d, want 2", len(logs.Items))
	}

	var updateLog map[string]types.AttributeValue
	for _, item := range logs.Items {
		if av, ok := item["action"].(*types.AttributeValueMemberS); ok && av.Value == repositories.AuditActionUpdate {
			updateLog = item
			break
		}
	}
	if updateLog == nil {
		t.Fatal("no UPDATE audit_logs row found")
	}
	modsAV, ok := updateLog["modifications"].(*types.AttributeValueMemberL)
	if !ok {
		t.Fatalf("modifications attribute missing or wrong type: %v", updateLog["modifications"])
	}
	var mods []repositories.Modification
	if err := attributevalue.UnmarshalList(modsAV.Value, &mods); err != nil {
		t.Fatalf("unmarshal modifications: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("UPDATE modifications = %d entries, want 1 (got %+v)", len(mods), mods)
	}
	if mods[0].Name != "description" {
		t.Errorf("UPDATE modification name = %q, want %q", mods[0].Name, "description")
	}
	if mods[0].After != "Audited Widget v2" {
		t.Errorf("UPDATE modification after = %v, want %q", mods[0].After, "Audited Widget v2")
	}

	if err := productSvc.Delete(ctx, orgPK, sk, "user-1", "Jane Doe"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	logs, _ = auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if len(logs.Items) != 3 {
		t.Fatalf("audit_logs rows after Delete = %d, want 3", len(logs.Items))
	}
	if got, err := productSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("product should be gone after Delete, got %v (err %v)", got, err)
	}
}
