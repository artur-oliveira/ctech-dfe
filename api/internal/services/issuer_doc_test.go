package services

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const testCompanyKey = "0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70"

// After the migration the document comes off the record. The key is a company
// id and carries nothing.
func TestIssuerDocReadsTheRecord(t *testing.T) {
	doc, isPJ := IssuerDoc("11222333000181", "cnpj", testCompanyKey)
	if doc != "11222333000181" || !isPJ {
		t.Fatalf("got %q, isPJ=%v", doc, isPJ)
	}
}

func TestIssuerDocReadsCpfOrCnpjCompatibilityAlias(t *testing.T) {
	av := map[string]types.AttributeValue{
		"cpf_or_cnpj": &types.AttributeValueMemberS{Value: "11.222.333/0001-81"},
	}
	plain := map[string]any{"cpf_or_cnpj": "11.222.333/0001-81"}

	if doc, isPJ := IssuerDocAV(av, testCompanyKey); doc != "11222333000181" || !isPJ {
		t.Errorf("IssuerDocAV compatibility alias = %q/%v", doc, isPJ)
	}
	if doc, isPJ := IssuerDocMap(plain, testCompanyKey); doc != "11222333000181" || !isPJ {
		t.Errorf("IssuerDocMap compatibility alias = %q/%v", doc, isPJ)
	}
}

// A natural-person issuer — produtor rural, MEI pessoa física — must pick the
// CPF tag. Emitting them as CNPJ is what SEFAZ rejects.
func TestIssuerDocKeepsANaturalPersonNatural(t *testing.T) {
	doc, isPJ := IssuerDoc("52998224725", "cpf", testCompanyKey)
	if doc != "52998224725" || isPJ {
		t.Fatalf("got %q, isPJ=%v", doc, isPJ)
	}
}

// Before the migration, and during a rollback, the record has no tax id and the
// legacy key is the only source. Both eras must answer.
func TestIssuerDocFallsBackToTheLegacyKey(t *testing.T) {
	doc, isPJ := IssuerDoc("", "", "CNPJ_11222333000181")
	if doc != "11222333000181" || !isPJ {
		t.Fatalf("CNPJ: got %q, isPJ=%v", doc, isPJ)
	}
	doc, isPJ = IssuerDoc("", "", "CPF_52998224725")
	if doc != "52998224725" || isPJ {
		t.Fatalf("CPF: got %q, isPJ=%v", doc, isPJ)
	}
}

// The record wins over the key. During the migration a row carries both, and
// the record is the one ctech-account owns.
func TestTheRecordWinsOverTheKey(t *testing.T) {
	doc, _ := IssuerDoc("12ABC34501DE35", "cnpj", "CNPJ_11222333000181")
	if doc != "12ABC34501DE35" {
		t.Fatalf("got %q, want the record's tax id", doc)
	}
}

// The regression this task exists for. A company id with no record behind it
// must NOT come back as a document — returning the UUID is exactly how it
// reached a signed XML, and an empty answer is what makes the caller fail
// loudly instead.
func TestIssuerDocNeverReturnsACompanyID(t *testing.T) {
	doc, isPJ := IssuerDoc("", "", testCompanyKey)
	if doc != "" {
		t.Fatalf("got %q, want empty — a company id is not a document", doc)
	}
	if isPJ {
		t.Error("an unknown issuer defaulted to a legal person")
	}
}

// A record whose tax id is empty is the same case: nothing to emit.
func TestAnEmptyTaxIDIsNotADocument(t *testing.T) {
	if doc, _ := IssuerDoc("", "cnpj", testCompanyKey); doc != "" {
		t.Fatalf("got %q, want empty", doc)
	}
}

// The tag follows the same decision. Two spellings of "is this issuer a CNPJ"
// is how one of them ends up wrong.
func TestIssuerDocTagAgreesWithIssuerDoc(t *testing.T) {
	if got := IssuerDocTag("52998224725", "cpf", testCompanyKey); got != TagCPF {
		t.Fatalf("tag = %q, want CPF", got)
	}
	if got := IssuerDocTag("11222333000181", "cnpj", testCompanyKey); got != TagCNPJ {
		t.Fatalf("tag = %q, want CNPJ", got)
	}
	// The legacy path, which every existing emission still takes.
	if got := IssuerDocTag("", "", "CPF_52998224725"); got != TagCPF {
		t.Fatalf("legacy CPF tag = %q", got)
	}
}

// The two adapters must agree with the core and with each other: the codebase
// carries an organization item in two shapes, and a rule that answered
// differently depending on which one reached it would be worse than no rule.
func TestBothAdaptersAgreeWithTheCore(t *testing.T) {
	av := map[string]types.AttributeValue{
		"tax_id":      &types.AttributeValueMemberS{Value: "11222333000181"},
		"tax_id_kind": &types.AttributeValueMemberS{Value: "cnpj"},
	}
	plain := map[string]any{"tax_id": "11222333000181", "tax_id_kind": "cnpj"}

	wantDoc, wantPJ := IssuerDoc("11222333000181", "cnpj", testCompanyKey)
	if doc, isPJ := IssuerDocAV(av, testCompanyKey); doc != wantDoc || isPJ != wantPJ {
		t.Errorf("IssuerDocAV = %q/%v, want %q/%v", doc, isPJ, wantDoc, wantPJ)
	}
	if doc, isPJ := IssuerDocMap(plain, testCompanyKey); doc != wantDoc || isPJ != wantPJ {
		t.Errorf("IssuerDocMap = %q/%v, want %q/%v", doc, isPJ, wantDoc, wantPJ)
	}
	if got := IssuerDocTagAV(av, testCompanyKey); got != TagCNPJ {
		t.Errorf("IssuerDocTagAV = %q", got)
	}
	if got := IssuerDocTagMap(plain, testCompanyKey); got != TagCNPJ {
		t.Errorf("IssuerDocTagMap = %q", got)
	}
}

// An adapter over an empty item falls through to the key, like the core does.
func TestTheAdaptersFallBackToTheKeyToo(t *testing.T) {
	if doc, isPJ := IssuerDocAV(nil, "CNPJ_11222333000181"); doc != "11222333000181" || !isPJ {
		t.Errorf("IssuerDocAV = %q/%v", doc, isPJ)
	}
	if doc, isPJ := IssuerDocMap(nil, "CPF_52998224725"); doc != "52998224725" || isPJ {
		t.Errorf("IssuerDocMap = %q/%v", doc, isPJ)
	}
}
