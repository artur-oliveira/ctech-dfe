package mdfes

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func ownerItem(fields map[string]string) map[string]types.AttributeValue {
	m := map[string]types.AttributeValue{}
	for k, v := range fields {
		m[k] = &types.AttributeValueMemberS{Value: v}
	}
	return m
}

func TestOwnerFromRegistry(t *testing.T) {
	pj := ownerFromRegistry(ownerItem(map[string]string{
		"cpf_cnpj": "11.222.333/0001-81", "rntrc": "12345678", "name": "Transportes Acme", "type": "ETC",
	}))
	if pj == nil {
		t.Fatal("esperado proprietário")
	}
	if pj.CNPJ != "11222333000181" || pj.CPF != "" {
		t.Errorf("documento = cnpj %q cpf %q", pj.CNPJ, pj.CPF)
	}
	if pj.TpProp != tpPropOutros {
		t.Errorf("tp_prop = %q, esperado %q para ETC", pj.TpProp, tpPropOutros)
	}

	pf := ownerFromRegistry(ownerItem(map[string]string{
		"cpf_cnpj": "52998224725", "rntrc": "12345678", "name": "João Motorista", "type": "TAC",
	}))
	if pf == nil || pf.CPF != "52998224725" || pf.CNPJ != "" {
		t.Fatalf("PF = %+v", pf)
	}
	if pf.TpProp != tpPropTACIndependente {
		t.Errorf("tp_prop = %q, esperado %q para TAC", pf.TpProp, tpPropTACIndependente)
	}
}

// Um prop pela metade seria rejeitado pela SEFAZ — melhor não gerar nenhum.
func TestOwnerFromRegistry_IncompleteRegistryYieldsNoOwner(t *testing.T) {
	cases := []map[string]string{
		{"rntrc": "12345678", "name": "Sem documento"},
		{"cpf_cnpj": "11222333000181", "name": "Sem RNTRC"},
		{"cpf_cnpj": "11222333000181", "rntrc": "12345678"},
		{},
	}
	for _, fields := range cases {
		if got := ownerFromRegistry(ownerItem(fields)); got != nil {
			t.Errorf("cadastro incompleto %v gerou proprietário %+v", fields, got)
		}
	}
}

func TestFirstOwner(t *testing.T) {
	const orgPK = "CNPJ_11222333000181"
	fromRequest := &MdfeOwner{CNPJ: "99888777000166", Name: "Do request"}
	fromRegistry := &MdfeOwner{CNPJ: "55444333000122", Name: "Do cadastro"}

	if got := firstOwner(fromRequest, fromRegistry, orgPK); got != fromRequest {
		t.Error("o proprietário do request tem que vencer o do cadastro")
	}
	if got := firstOwner(nil, fromRegistry, orgPK); got != fromRegistry {
		t.Error("sem proprietário no request, vale o do cadastro")
	}
	if got := firstOwner(nil, nil, orgPK); got != nil {
		t.Error("sem os dois, não há proprietário")
	}
}

// O ponto de maior risco de regressão: um veículo próprio cadastrado com o
// emitente como proprietário não pode passar a gerar grupo prop, porque isso
// mudaria ide/tpTransp (regras SEFAZ F18/F19/F25).
func TestFirstOwner_RegisteredOwnerEqualToIssuerMeansOwnFleet(t *testing.T) {
	const orgPK = "CNPJ_11222333000181"
	if got := firstOwner(nil, &MdfeOwner{CNPJ: "11222333000181", Name: "A própria"}, orgPK); got != nil {
		t.Errorf("frota própria não pode virar proprietário terceiro: %+v", got)
	}
	const cpfOrg = "CPF_52998224725"
	if got := firstOwner(nil, &MdfeOwner{CPF: "52998224725", Name: "A própria"}, cpfOrg); got != nil {
		t.Errorf("frota própria (PF) não pode virar proprietário terceiro: %+v", got)
	}
}

func TestStringList(t *testing.T) {
	got := stringList([]any{"VEHICLE_1", "", "VEHICLE_2", 42})
	if len(got) != 2 || got[0] != "VEHICLE_1" || got[1] != "VEHICLE_2" {
		t.Errorf("stringList = %v", got)
	}
	if stringList(nil) != nil {
		t.Error("stringList(nil) tem que ser nil")
	}
}
