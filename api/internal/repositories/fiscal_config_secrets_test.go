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
