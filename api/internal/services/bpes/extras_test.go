package bpes

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/validation"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// Substituição, agência, PDF417 e os tipos de compra do TA: o documento montado cumpre o schema e as regras do MOC.

const testSubKey = "35261112345678000190630010000000101012345671"

func testAgency() *BpeAgency {
	return &BpeAgency{
		Name: "Agencia Teste", CNPJ: "11222333000181",
		Address: BpeAddress{Street: "Rua A", Number: "10", Neighborhood: "Centro", IBGECode: "3550308", City: "Sao Paulo", UF: "SP"},
	}
}

func testBuyer() *BpeBuyer {
	return &BpeBuyer{
		Name: "Comprador Teste", CNPJ: strp("11222333000181"),
		Address: BpeAddress{Street: "Rua B", Number: "20", Neighborhood: "Centro", IBGECode: "3550308", City: "Sao Paulo", UF: "SP"},
	}
}

func checkDoc(t *testing.T, family, root string, body map[string]any) map[string]any {
	t.Helper()
	doc, ok := body[root].(map[string]any)
	if !ok {
		t.Fatalf("raiz %s ausente: %v", root, body)
	}
	s, err := validate.Load(family)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range s.Validate(doc, validate.Options{Skip: []string{"Signature"}}) {
		t.Errorf("schema: %s %s %s", v.Path, v.Code, v.Message)
	}
	found, _ := validate.Apply(family, doc, nil)
	for _, v := range found {
		t.Errorf("regra %s: %s", v.Rule, v.Message)
	}
	return doc["infBPe"].(map[string]any)
}

func TestBuildBPeSubstitutionAgencyAndBoardPass(t *testing.T) {
	p := testParams(t)
	p.req.Substitution = &BpeSubstitution{OriginalKey: testSubKey, Type: "1"}
	p.req.Agency = testAgency()
	p.req.BoardPass = strp(strings.Repeat("A", 60))
	body := BuildBPe(p)
	inf := checkDoc(t, "bpe", "BPe", body)
	if inf["ide"].(map[string]any)["tpBPe"] != "3" {
		t.Errorf("tpBPe = %v", inf["ide"])
	}
	if sub := inf["infBPeSub"].(map[string]any); sub["chBPe"] != testSubKey || sub["tpSub"] != "1" {
		t.Errorf("infBPeSub = %v", sub)
	}
	if ag := inf["agencia"].(map[string]any); ag["CNPJ"] != "11222333000181" || ag["enderAgencia"] == nil {
		t.Errorf("agencia = %v", ag)
	}
	if supl := body["BPe"].(map[string]any)["infBPeSupl"].(map[string]any); supl["boardPassBPe"] != strings.Repeat("A", 60) {
		t.Errorf("infBPeSupl = %v", supl)
	}
}

func TestBuildBPeTASubstitutionAndAgency(t *testing.T) {
	p := testTAParams(t)
	p.ta.Substitution = &BpeSubstitution{OriginalKey: testSubKey, Type: "5"}
	p.ta.Agency = testAgency()
	inf := checkDoc(t, "bpeta", "BPeTA", BuildBPeTA(p))
	if inf["ide"].(map[string]any)["tpBPe"] != "3" || inf["infBPeSub"].(map[string]any)["tpSub"] != "5" || inf["agencia"] == nil {
		t.Errorf("inf = %v", inf)
	}
}

func TestBuildBPeTAMultiple(t *testing.T) {
	p := testTAParams(t)
	p.ta.PurchaseType = PurchaseMultiple
	p.ta.Passage.Passenger = nil
	p.ta.Buyer = testBuyer()
	inf := checkDoc(t, "bpeta", "BPeTA", BuildBPeTA(p))
	if inf["ide"].(map[string]any)["tpCompra"] != "1" {
		t.Errorf("tpCompra = %v", inf["ide"])
	}
	if err := validateTABody(p.ta, 2026); err != nil {
		t.Errorf("múltiplo válido recusado: %v", err)
	}
}

func TestBuildBPeTALinked(t *testing.T) {
	p := testTAParams(t)
	p.ta.PurchaseType = PurchaseLinked
	p.ta.MultipleKey = strp(testSubKey)
	p.ta.Components, p.ta.Payments, p.ta.Discount, p.ta.DiscountType, p.ta.DiscountDesc, p.ta.Tax = nil, nil, nil, nil, nil, BpeTATax{}
	inf := checkDoc(t, "bpeta", "BPeTA", BuildBPeTA(p))
	if inf["ide"].(map[string]any)["tpCompra"] != "2" {
		t.Errorf("tpCompra = %v", inf["ide"])
	}
	if v := inf["infBPeVinculado"].(map[string]any); v["chBPeMultiplo"] != testSubKey {
		t.Errorf("infBPeVinculado = %v", v)
	}
	for _, g := range []string{"infValorBPe", "imp", "pag"} {
		if _, has := inf[g]; has {
			t.Errorf("o vinculado não leva %s", g)
		}
	}
	if err := validateTABody(p.ta, 2026); err != nil {
		t.Errorf("vinculado válido recusado: %v", err)
	}
}

func TestValidateTAPurchaseRules(t *testing.T) {
	cases := map[string]func(b *BpeTAEmitBody){
		"múltiplo com passageiro":     func(b *BpeTAEmitBody) { b.PurchaseType = PurchaseMultiple; b.Buyer = testBuyer() },
		"múltiplo sem comprador":      func(b *BpeTAEmitBody) { b.PurchaseType = PurchaseMultiple; b.Passage.Passenger = nil },
		"vinculado sem chave":         func(b *BpeTAEmitBody) { b.PurchaseType = PurchaseLinked },
		"vinculado com valores":       func(b *BpeTAEmitBody) { b.PurchaseType = PurchaseLinked; b.MultipleKey = strp(testSubKey) },
		"normal sem passageiro":       func(b *BpeTAEmitBody) { b.Passage.Passenger = nil },
		"chave do múltiplo no normal": func(b *BpeTAEmitBody) { b.MultipleKey = strp(testSubKey) },
		"normal sem imposto":          func(b *BpeTAEmitBody) { b.Tax = BpeTATax{} },
	}
	for name, mutate := range cases {
		b := testTABody()
		mutate(&b)
		if err := validateTABody(b, 2026); err == nil {
			t.Errorf("%s: deveria ser recusado", name)
		}
	}
}

func TestValidateCommonSubstitutionType(t *testing.T) {
	if err := validateCommonSubstitution(nil); err != nil {
		t.Error(err)
	}
	if err := validateCommonSubstitution(&BpeSubstitution{OriginalKey: testSubKey, Type: "3"}); err != nil {
		t.Error(err)
	}
	if err := validateCommonSubstitution(&BpeSubstitution{OriginalKey: testSubKey, Type: "5"}); err == nil {
		t.Error("tpSub 5 vale só no TA")
	}
}

func TestSubstitutionBodyTags(t *testing.T) {
	if err := validation.Struct(BpeSubstitution{OriginalKey: "123", Type: "1"}); err == nil {
		t.Error("chave curta deveria ser recusada")
	}
	if err := validation.Struct(BpeSubstitution{OriginalKey: testSubKey, Type: "8"}); err == nil {
		t.Error("tpSub 8 não existe")
	}
}

func TestFactsOfBuyerDocument(t *testing.T) {
	f := factsOf(BpePlace{UF: "SP", IBGECode: "3550308"}, BpePlace{UF: "RJ", IBGECode: "3304557"}, testBuyer())
	if f.buyerDoc != "11222333000181" || f.originUF != "SP" || f.destIBGE != "3304557" {
		t.Errorf("facts = %+v", f)
	}
	if got := factsOf(BpePlace{}, BpePlace{}, nil).buyerDoc; got != "" {
		t.Errorf("sem comprador o documento é vazio: %q", got)
	}
}
