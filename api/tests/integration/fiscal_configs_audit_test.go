//go:build integration

package integration_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
)

// nfeConfigFields builds a base NF-e config field set. prodNSU, when > 0, is
// set explicitly by the caller (simulating a config that already has an
// internal-process cursor); pass 0 to omit it entirely from the write.
func nfeConfigFields(timezone string, prodNSU int, notes string) map[string]types.AttributeValue {
	fields := map[string]types.AttributeValue{
		"timezone":            &types.AttributeValueMemberS{Value: timezone},
		"environment":         &types.AttributeValueMemberN{Value: "2"},
		"prod_current_serie":  &types.AttributeValueMemberN{Value: "1"},
		"prod_current_number": &types.AttributeValueMemberN{Value: "1"},
		"hom_current_serie":   &types.AttributeValueMemberN{Value: "1"},
		"hom_current_number":  &types.AttributeValueMemberN{Value: "1"},
	}
	if prodNSU > 0 {
		fields["prod_nsu"] = &types.AttributeValueMemberN{Value: strconv.Itoa(prodNSU)}
	}
	if notes != "" {
		fields["notes"] = &types.AttributeValueMemberS{Value: notes}
	}
	return fields
}

func TestNfeConfigService_Upsert_WritesAuditLog(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	// First write: no existing record, so this must be a CREATE. Caller
	// explicitly sets prod_nsu=5 (allowed — preserve only kicks in to
	// override with the EXISTING value on later calls; on first write there
	// is no existing value, so the caller's explicit value stands). Also
	// sets "notes", a plain non-preserve field, to be dropped on the next
	// write (full-replace semantics: a field genuinely absent from the new
	// submission must disappear, unlike Tasks 7-11's partial-Update merge).
	created, err := nfeConfigSvc.Upsert(ctx, orgPK, nfeConfigFields("America/Sao_Paulo", 5, "first note"), "user-1", "Jane Doe")
	if err != nil {
		t.Fatalf("Upsert (create): %v", err)
	}
	if av, ok := created["prod_nsu"].(*types.AttributeValueMemberN); !ok || av.Value != "5" {
		t.Fatalf("created prod_nsu = %v, want N(5)", created["prod_nsu"])
	}

	logs, err := auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if len(logs.Items) != 1 {
		t.Fatalf("audit_logs rows after first Upsert = %d, want 1", len(logs.Items))
	}
	createLog := logs.Items[0]
	if v := attrStrTest(createLog, "resource_type"); v != repositories.AuditResourceNfeConfig {
		t.Errorf("resource_type = %q, want %q", v, repositories.AuditResourceNfeConfig)
	}
	if v := attrStrTest(createLog, "resource_id"); v != "nfe_config" {
		t.Errorf("resource_id = %q, want %q", v, "nfe_config")
	}
	if v := attrStrTest(createLog, "action"); v != repositories.AuditActionCreate {
		t.Errorf("action = %q, want %q (first write must be CREATE, not UPDATE)", v, repositories.AuditActionCreate)
	}

	// Second write: existing record present, so this must be an UPDATE.
	// - "timezone" genuinely changes (America/Sao_Paulo -> America/Recife):
	//   must appear in modifications.
	// - "prod_nsu" is omitted from the caller's fields this time; it is a
	//   preserve field, so Upsert must carry forward the existing value (5)
	//   unchanged. It must NOT appear in modifications — a false "changed"
	//   entry here would mean an internal-process field was misreported as
	//   a user-initiated change.
	// - "notes" is also omitted, but it is NOT a preserve field, so under
	//   full-replace semantics it is genuinely cleared. It MUST appear in
	//   modifications as before="first note", after=nil.
	updated, err := nfeConfigSvc.Upsert(ctx, orgPK, nfeConfigFields("America/Recife", 0, ""), "user-1", "Jane Doe")
	if err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}
	if av, ok := updated["prod_nsu"].(*types.AttributeValueMemberN); !ok || av.Value != "5" {
		t.Fatalf("updated prod_nsu = %v, want N(5) (preserved from existing)", updated["prod_nsu"])
	}
	if _, present := updated["notes"]; present {
		t.Fatalf("updated notes = %v, want absent (genuinely cleared, not a preserve field)", updated["notes"])
	}

	logs, err = auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if len(logs.Items) != 2 {
		t.Fatalf("audit_logs rows after second Upsert = %d, want 2", len(logs.Items))
	}

	var updateLog map[string]types.AttributeValue
	for _, item := range logs.Items {
		if attrStrTest(item, "action") == repositories.AuditActionUpdate {
			updateLog = item
			break
		}
	}
	if updateLog == nil {
		t.Fatal("no UPDATE audit_logs row found")
	}
	if v := attrStrTest(updateLog, "resource_type"); v != repositories.AuditResourceNfeConfig {
		t.Errorf("resource_type = %q, want %q", v, repositories.AuditResourceNfeConfig)
	}
	if v := attrStrTest(updateLog, "resource_id"); v != "nfe_config" {
		t.Errorf("resource_id = %q, want %q", v, "nfe_config")
	}

	modsAV, ok := updateLog["modifications"].(*types.AttributeValueMemberL)
	if !ok {
		t.Fatalf("modifications attribute missing or wrong type: %v", updateLog["modifications"])
	}
	var mods []repositories.Modification
	if err := attributevalue.UnmarshalList(modsAV.Value, &mods); err != nil {
		t.Fatalf("unmarshal modifications: %v", err)
	}

	byName := make(map[string]repositories.Modification, len(mods))
	for _, m := range mods {
		byName[m.Name] = m
	}

	if _, present := byName["prod_nsu"]; present {
		t.Errorf("modifications falsely includes preserved field prod_nsu: %+v", byName["prod_nsu"])
	}

	tzMod, ok := byName["timezone"]
	if !ok {
		t.Fatalf("modifications missing genuinely-changed field timezone; got %+v", mods)
	}
	if tzMod.Before != "America/Sao_Paulo" || tzMod.After != "America/Recife" {
		t.Errorf("timezone modification = %+v, want before=America/Sao_Paulo after=America/Recife", tzMod)
	}

	notesMod, ok := byName["notes"]
	if !ok {
		t.Fatalf("modifications missing genuinely-cleared field notes; got %+v", mods)
	}
	if notesMod.Before != "first note" || notesMod.After != nil {
		t.Errorf("notes modification = %+v, want before=%q after=nil", notesMod, "first note")
	}
}

// attrStrTest extracts a string attribute from a DynamoDB item, or "" if absent.
func attrStrTest(item map[string]types.AttributeValue, key string) string {
	if av, ok := item[key].(*types.AttributeValueMemberS); ok {
		return av.Value
	}
	return ""
}
