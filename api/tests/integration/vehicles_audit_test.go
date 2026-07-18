//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestVehicleService_CreateUpdateDelete_WriteAuditLogs(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := vehicleSvc.Create(ctx, orgPK, map[string]any{"plate": "ABC1D23", "plate_uf": "SP"}, "user-1", "Jane Doe")
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

	// Partial update: only "plate" is supplied. The audit row's
	// modifications must list ONLY that field, not every field on the
	// record compared against nil (regression test for the false
	// "changed to null" bug when Diff was called with the raw partial
	// updates map instead of a merged before+updates snapshot).
	if _, err := vehicleSvc.Update(ctx, orgPK, sk, map[string]any{"plate": "ABC1D24"}, "user-1", "Jane Doe"); err != nil {
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
	if mods[0].Name != "plate" {
		t.Errorf("UPDATE modification name = %q, want %q", mods[0].Name, "plate")
	}
	if mods[0].After != "ABC1D24" {
		t.Errorf("UPDATE modification after = %v, want %q", mods[0].After, "ABC1D24")
	}

	if err := vehicleSvc.Delete(ctx, orgPK, sk, "user-1", "Jane Doe"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	logs, _ = auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if len(logs.Items) != 3 {
		t.Fatalf("audit_logs rows after Delete = %d, want 3", len(logs.Items))
	}
	if got, err := vehicleSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("vehicle should be gone after Delete, got %v (err %v)", got, err)
	}
}
