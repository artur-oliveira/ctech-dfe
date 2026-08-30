package services

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestServiceSchemaVersionOfTreatsLegacyAsOne(t *testing.T) {
	if got := ServiceSchemaVersionOf(map[string]types.AttributeValue{}); got != ServiceSchemaVersionLegacy {
		t.Errorf("registro sem o atributo = %d, esperado %d", got, ServiceSchemaVersionLegacy)
	}
	stamped := map[string]types.AttributeValue{
		AttrServiceSchemaVersion: &types.AttributeValueMemberN{Value: "2"},
	}
	if got := ServiceSchemaVersionOf(stamped); got != 2 {
		t.Errorf("registro carimbado = %d, esperado 2", got)
	}
}

func TestServiceCompletenessReportsMissingByScenario(t *testing.T) {
	legacy := map[string]types.AttributeValue{
		"trib_nacional_code": &types.AttributeValueMemberS{Value: "010101"},
		"iss": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"trib_issqn": &types.AttributeValueMemberN{Value: "1"},
			"tax_rate":   &types.AttributeValueMemberS{Value: "5.00"},
		}},
	}
	report := ServiceCompleteness(legacy)

	// Registro legado continua legível: o cenário nacional só acusa o default
	// de local, que os subgrupos novos introduziram.
	if got := report[ScenarioNacional]; len(got) != 1 || got[0] != "location_defaults.c_loc_prestacao" {
		t.Errorf("cenário nacional = %v", got)
	}
	if len(report[ScenarioExterior]) != len(serviceScenarioFields[ScenarioExterior]) {
		t.Errorf("cenário exterior deveria acusar todos os campos: %v", report[ScenarioExterior])
	}

	// Cenário avaliado e completo sai com lista vazia, não ausente: o cliente
	// precisa distinguir "sem pendência" de "não avaliado".
	complete := map[string]types.AttributeValue{
		"tot_trib": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"p_tot_trib_sn": &types.AttributeValueMemberS{Value: "0.00"},
		}},
	}
	missing, ok := ServiceCompleteness(complete)[ScenarioTransparencia]
	if !ok || len(missing) != 0 {
		t.Errorf("transparência completa = %v (presente: %v)", missing, ok)
	}
}

func TestServiceCompletenessIgnoresEmptyAndNullValues(t *testing.T) {
	item := map[string]types.AttributeValue{
		"tot_trib": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"p_tot_trib_sn": &types.AttributeValueMemberNULL{Value: true},
		}},
	}
	if len(ServiceCompleteness(item)[ScenarioTransparencia]) != 1 {
		t.Error("NULL contou como valor preenchido")
	}
	item["tot_trib"] = &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
		"p_tot_trib_sn": &types.AttributeValueMemberS{Value: ""},
	}}
	if len(ServiceCompleteness(item)[ScenarioTransparencia]) != 1 {
		t.Error("string vazia contou como valor preenchido")
	}
}
