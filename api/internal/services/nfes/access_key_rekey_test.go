package nfes

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const rekeyCompanyKey = "0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70"

func orgWithUFAndTaxID(uf, taxID, kind string) map[string]types.AttributeValue {
	org := orgWithUF(uf)
	org["tax_id"] = &types.AttributeValueMemberS{Value: taxID}
	org["tax_id_kind"] = &types.AttributeValueMemberS{Value: kind}
	return org
}

// The single most consequential site in the re-key. Positions 7-20 of the chave
// de acesso are the issuer's CNPJ, and the builder truncates to fourteen
// characters — so a company id would have embedded "0199f3a1-8c42-", hyphens
// included, in the document's identity at SEFAZ. That cannot be corrected
// afterwards: the key IS the document.
func TestTheAccessKeyCarriesTheDocumentAndNeverTheCompanyID(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	org := orgWithUFAndTaxID("SP", "11222333000181", "cnpj")

	key, err := generateAccessKey(rekeyCompanyKey, org, 1, 1, now, nfModel55, tpEmisNormal)
	if err != nil {
		t.Fatalf("generateAccessKey: %v", err)
	}
	if len(key) != 44 {
		t.Fatalf("key length = %d, want 44", len(key))
	}
	if got := key[6:20]; got != "11222333000181" {
		t.Errorf("issuer positions = %q, want the record's tax id", got)
	}
	if strings.Contains(key, "-") {
		t.Errorf("the key carries a hyphen, so a company id reached it: %q", key)
	}
	if strings.Contains(strings.ToLower(key), "0199f3a1") {
		t.Errorf("the company id reached the access key: %q", key)
	}
}

// An issuer whose document is unknown must refuse, not pad. The builder used to
// right-pad with zeroes to fourteen, which would have produced a well-formed
// key for a company that has none — a document SEFAZ accepts and nobody can
// trace back.
func TestTheAccessKeyRefusesAnIssuerWithNoDocument(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	if _, err := generateAccessKey(rekeyCompanyKey, orgWithUF("SP"), 1, 1, now, nfModel55, tpEmisNormal); err == nil {
		t.Fatal("an issuer with no document produced an access key")
	}
}

// The legacy key still produces the same chave de acesso it always did. Every
// organization carries one until the migration runs, and the rollback puts them
// all back — a key that changed shape would orphan every document.
func TestTheAccessKeyIsUnchangedOnALegacyKey(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	key, err := generateAccessKey("CNPJ_11222333000181", orgWithUF("SP"), 1, 1, now, nfModel55, tpEmisNormal)
	if err != nil {
		t.Fatalf("generateAccessKey: %v", err)
	}
	if got := key[6:20]; got != "11222333000181" {
		t.Errorf("issuer positions = %q", got)
	}
}
