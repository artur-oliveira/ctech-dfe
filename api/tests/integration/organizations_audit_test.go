//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestOrganizationService_Update_WritesAuditLog(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	orgPK := "CNPJ_" + cnpj

	if _, err := orgSvc.Create(ctx, cnpj, orgFields(cnpj, "Before Update")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Partial update: only "name" is supplied. The audit row's modifications
	// must list ONLY that field, not every field on the record compared
	// against nil (regression test for the false "changed to null" bug when
	// Diff was called with the raw partial updates map instead of a merged
	// before+updates snapshot).
	if _, err := orgSvc.Update(ctx, orgPK, map[string]any{"name": "New Name"}, "user-1", "Jane Doe"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	logs, err := auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if len(logs.Items) != 1 {
		t.Fatalf("audit_logs rows after Update = %d, want 1", len(logs.Items))
	}

	logItem := logs.Items[0]
	if av, ok := logItem["action"].(*types.AttributeValueMemberS); !ok || av.Value != repositories.AuditActionUpdate {
		t.Errorf("action = %v, want %q", logItem["action"], repositories.AuditActionUpdate)
	}
	if av, ok := logItem["resource_type"].(*types.AttributeValueMemberS); !ok || av.Value != repositories.AuditResourceOrganization {
		t.Errorf("resource_type = %v, want %q", logItem["resource_type"], repositories.AuditResourceOrganization)
	}
	if av, ok := logItem["resource_id"].(*types.AttributeValueMemberS); !ok || av.Value != orgPK {
		t.Errorf("resource_id = %v, want %q", logItem["resource_id"], orgPK)
	}

	modsAV, ok := logItem["modifications"].(*types.AttributeValueMemberL)
	if !ok {
		t.Fatalf("modifications attribute missing or wrong type: %v", logItem["modifications"])
	}
	var mods []repositories.Modification
	if err := attributevalue.UnmarshalList(modsAV.Value, &mods); err != nil {
		t.Fatalf("unmarshal modifications: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("UPDATE modifications = %d entries, want 1 (got %+v)", len(mods), mods)
	}
	if mods[0].Name != "name" {
		t.Errorf("UPDATE modification name = %q, want %q", mods[0].Name, "name")
	}
	if mods[0].Before != "Before Update" {
		t.Errorf("UPDATE modification before = %v, want %q", mods[0].Before, "Before Update")
	}
	if mods[0].After != "New Name" {
		t.Errorf("UPDATE modification after = %v, want %q", mods[0].After, "New Name")
	}
}
