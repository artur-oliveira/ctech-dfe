package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func s(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }

// A API nunca devolve o CSRT, então um PUT montado a partir de um GET nunca o
// traz de volta. Se a omissão apagasse o campo, salvar a série destruiria o
// segredo.
func TestUpsertMantemSegredoOmitido(t *testing.T) {
	repo := &FiscalConfigRepository{}
	fields := map[string]types.AttributeValue{"environment": s("2")}
	existing := map[string]types.AttributeValue{"csrt": s("segredo"), "prod_csc": s("csc")}

	_, final, err := repo.BuildUpsertTxItem("CNPJ_1", fields, existing)
	if err != nil {
		t.Fatal(err)
	}
	if final["csrt"] != existing["csrt"] || final["prod_csc"] != existing["prod_csc"] {
		t.Fatalf("segredo omitido deveria sobreviver: %v", final)
	}
}

// Mas informar um valor novo tem que sobrescrever — senão o segredo seria
// impossível de trocar.
func TestUpsertSobrescreveSegredoInformado(t *testing.T) {
	repo := &FiscalConfigRepository{}
	fields := map[string]types.AttributeValue{"csrt": s("novo")}
	existing := map[string]types.AttributeValue{"csrt": s("antigo")}

	_, final, err := repo.BuildUpsertTxItem("CNPJ_1", fields, existing)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := final["csrt"].(*types.AttributeValueMemberS)
	if got == nil || got.Value != "novo" {
		t.Fatalf("want novo, got %v", final["csrt"])
	}
}

// prod_current_number/hom_current_number do NfseConfigRepository não são
// campos "preserve" (diferente de prod_nsu/hom_nsu): um PUT que os informa
// tem que sobrescrever o valor já gravado, senão o usuário nunca consegue
// ajustar a numeração da DPS/RPS pela config. Regressão do bug em que
// current_number entrou por engano no preserve map e o PUT era silenciosamente
// ignorado.
func TestUpsertNfseSobrescreveCurrentNumberInformado(t *testing.T) {
	repo := &FiscalConfigRepository{preserve: map[string]any{
		"prod_nsu":              0,
		"hom_nsu":               0,
		"prod_last_dist_nsu_at": nil,
		"hom_last_dist_nsu_at":  nil,
	}}
	fields := map[string]types.AttributeValue{
		"prod_current_number": &types.AttributeValueMemberN{Value: "1"},
		"hom_current_number":  &types.AttributeValueMemberN{Value: "2"},
	}
	existing := map[string]types.AttributeValue{
		"prod_current_number": &types.AttributeValueMemberN{Value: "0"},
		"hom_current_number":  &types.AttributeValueMemberN{Value: "1"},
	}

	_, final, err := repo.BuildUpsertTxItem("CNPJ_1", fields, existing)
	if err != nil {
		t.Fatal(err)
	}
	gotProd, _ := final["prod_current_number"].(*types.AttributeValueMemberN)
	gotHom, _ := final["hom_current_number"].(*types.AttributeValueMemberN)
	if gotProd == nil || gotProd.Value != "1" {
		t.Fatalf("prod_current_number: want 1, got %v", final["prod_current_number"])
	}
	if gotHom == nil || gotHom.Value != "2" {
		t.Fatalf("hom_current_number: want 2, got %v", final["hom_current_number"])
	}
}

// Todo Put da configuração substitui o item inteiro: sem o `sweep`, uma edição
// tiraria a linha do sweep-index e a organização sumiria do dispatcher.
func TestUpsertCarimbaSweep(t *testing.T) {
	repo := &FiscalConfigRepository{}
	_, final, err := repo.BuildUpsertTxItem("CNPJ_1", map[string]types.AttributeValue{"environment": s("2")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := final[AttrSweep].(*types.AttributeValueMemberS); got == nil || got.Value != SweepValue {
		t.Fatalf("sweep ausente: %v", final)
	}
}

// A janela de contingência é aberta só pelo endpoint próprio: um PUT da
// configuração nem a abre (campo no corpo) nem a fecha (campo omitido).
func TestUpsertPreservaJanelaDeContingencia(t *testing.T) {
	repo := &FiscalConfigRepository{}
	existing := map[string]types.AttributeValue{AttrEmissionType: s(EmissionSVC), AttrContingencySince: s("2026-11-08T10:00:00Z")}

	_, final, _ := repo.BuildUpsertTxItem("CNPJ_1", map[string]types.AttributeValue{"environment": s("2")}, existing)
	if final[AttrEmissionType] != existing[AttrEmissionType] || final[AttrContingencySince] != existing[AttrContingencySince] {
		t.Fatalf("janela aberta deveria sobreviver ao PUT: %v", final)
	}
	_, final, _ = repo.BuildUpsertTxItem("CNPJ_1", map[string]types.AttributeValue{AttrEmissionType: s(EmissionSVC)}, nil)
	if _, ok := final[AttrEmissionType]; ok {
		t.Fatal("PUT não pode abrir a janela")
	}
}

func TestEmissionTypeOfReadsLegacyMode(t *testing.T) {
	if got := EmissionTypeOf(map[string]types.AttributeValue{}); got != EmissionNormal {
		t.Fatalf("absent: %q", got)
	}
	if got := EmissionTypeOf(map[string]types.AttributeValue{AttrContingencyMode: s("svc")}); got != EmissionSVC {
		t.Fatalf("legacy: %q", got)
	}
	if got := EmissionTypeOf(map[string]types.AttributeValue{AttrEmissionType: s("epec"), AttrContingencyMode: s("svc")}); got != EmissionEPEC {
		t.Fatalf("new wins: %q", got)
	}
}
