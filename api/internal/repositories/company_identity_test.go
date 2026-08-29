package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// The migration writes these attributes and the read path reads them. A typo in
// either is a silently empty document, not a failure — which is why the names
// are constants and why this pins them.
func TestCompanyFromItemReadsTheIdentity(t *testing.T) {
	const pk = "0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70"
	item := map[string]types.AttributeValue{
		AttrOrganizationID: &types.AttributeValueMemberS{Value: "org_1"},
		AttrTaxID:          &types.AttributeValueMemberS{Value: "12ABC34501DE35"},
		AttrTaxIDKind:      &types.AttributeValueMemberS{Value: TaxKindCNPJ},
		AttrLegalName:      &types.AttributeValueMemberS{Value: "ACME LTDA"},
	}
	got := CompanyFromItem(pk, item)
	if got.CompanyID != pk || got.OrganizationID != "org_1" {
		t.Errorf("ids = %q / %q", got.CompanyID, got.OrganizationID)
	}
	if got.TaxID != "12ABC34501DE35" || got.TaxIDKind != TaxKindCNPJ {
		t.Errorf("tax id = %q %q", got.TaxID, got.TaxIDKind)
	}
	if got.LegalName != "ACME LTDA" {
		t.Errorf("legal name = %q", got.LegalName)
	}
}

// A record that predates the migration reads as empty, not as garbage. The
// callers fall back to the legacy key, and a half-read record would make that
// fallback unreachable.
func TestCompanyFromItemOnAnUnmigratedRecord(t *testing.T) {
	got := CompanyFromItem("CNPJ_11222333000181", map[string]types.AttributeValue{
		"name": &types.AttributeValueMemberS{Value: "ACME"},
	})
	if got.TaxID != "" || got.TaxIDKind != "" || got.OrganizationID != "" {
		t.Errorf("an unmigrated record read as %+v, want empty identity", got)
	}
	if got.CompanyID != "CNPJ_11222333000181" {
		t.Errorf("CompanyID = %q, want the key it was read under", got.CompanyID)
	}
	// And with no tax id there is no raiz, so it claims no siblings.
	if got.CNPJRoot() != "" {
		t.Errorf("CNPJRoot = %q, want empty", got.CNPJRoot())
	}
}

// An attribute of the wrong DynamoDB type reads as empty rather than panicking.
// A migration that wrote a number where a string belongs must not take the
// issuance path down.
func TestCompanyFromItemIgnoresAWrongType(t *testing.T) {
	got := CompanyFromItem("cmp", map[string]types.AttributeValue{
		AttrTaxID: &types.AttributeValueMemberN{Value: "11222333000181"},
	})
	if got.TaxID != "" {
		t.Errorf("TaxID = %q, want empty for a non-string attribute", got.TaxID)
	}
}
