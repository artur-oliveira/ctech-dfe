//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services/nfcoms"
)

func marshal(t *testing.T, m map[string]any) map[string]types.AttributeValue {
	t.Helper()
	av, err := repositories.MarshalMapOmitNull(m)
	if err != nil {
		t.Fatal(err)
	}
	return av
}

// A emissão da NFCom resolve assinante, pessoa e plano dos cadastros: o corpo da requisição leva só ids e valores.
func TestNfcom_ResolveBodyFromRegistries(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	otherOrg := "CNPJ_" + randomCNPJ()
	subRepo := repositories.NewNfcomSubscriberRepository(db, cfg)
	planRepo := repositories.NewNfcomPlanRepository(db, cfg)

	if _, err := personSvc.Create(ctx, orgPK, "33000167000101", map[string]any{
		"name": "Cliente Exemplo Ltda",
		"addresses": []any{map[string]any{
			"street": "Rua B", "number": "20", "neighborhood": "Bairro", "city_ibge_code": "3550308",
			"city": "Sao Paulo", "postal_code": "01002000", "state_federation": "SP",
		}},
		"state_registrations": []any{map[string]any{"uf": "SP", "state_registration": "110042490114"}},
	}, testUserID, testUserName); err != nil {
		t.Fatalf("pessoa: %v", err)
	}
	sub, err := subRepo.Create(ctx, orgPK, marshal(t, map[string]any{
		"name": "Assinante 1", "person_id": "CNPJ_33000167000101", "code": "ASS-001", "type": "1", "service_used": "4",
		"primary_terminal": "1130000000",
	}))
	if err != nil {
		t.Fatalf("assinante: %v", err)
	}
	plan, err := planRepo.Create(ctx, orgPK, marshal(t, map[string]any{
		"name": "Internet 100", "code": "PLANO100", "description": "Internet 100 Mbps", "class_code": "0100301",
		"cfop": "5307", "unit": "4", "unit_value": "99.90",
		"icms": map[string]any{"cst": "00", "rate": "18"},
		"pis":  map[string]any{"cst": "01", "rate": "0.65"}, "cofins": map[string]any{"cst": "01", "rate": "3"},
	}))
	if err != nil {
		t.Fatalf("plano: %v", err)
	}
	subID, planID := sub["sk"].(*types.AttributeValueMemberS).Value, plan["sk"].(*types.AttributeValueMemberS).Value

	svc := nfcoms.NewNfcomService(nil, nil, nfcomConfigRepo, nfcomRepo, nil, personRepo, subRepo, planRepo, nil, nil, nil, nil, "", nfcoms.TechData{}, nil)
	req := nfcoms.NfcomEmitRequest{
		SubscriberID: subID,
		Items:        []nfcoms.NfcomItemRequest{{PlanID: planID, Quantity: "2"}},
		Prepaid:      true,
	}
	body, err := svc.Resolve(ctx, orgPK, req)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if body.Customer.Name != "Cliente Exemplo Ltda" || body.Customer.CNPJ == nil || *body.Customer.CNPJ != "33000167000101" ||
		body.Customer.IEIndicator != "1" || body.Customer.Address.IBGECode != "3550308" {
		t.Errorf("cliente = %+v", body.Customer)
	}
	if body.Subscriber.Code != "ASS-001" || body.Subscriber.Type != "1" || body.Subscriber.PrimaryTerminal == nil {
		t.Errorf("assinante = %+v", body.Subscriber)
	}
	if len(body.Items) != 1 || body.Items[0].UnitValue != "99.90" || body.Items[0].ICMS.CST != "00" || body.Items[0].ClassCode != "0100301" {
		t.Errorf("itens = %+v", body.Items)
	}

	// Cadastro de outra organização não resolve (isolamento por tenant), nem plano inexistente.
	if _, err := svc.Resolve(ctx, otherOrg, req); problemStatus(err) != 404 {
		t.Errorf("outra organização: status %d, queria 404 (%v)", problemStatus(err), err)
	}
	req.Items[0].PlanID = "NFCOMPLAN_inexistente"
	if _, err := svc.Resolve(ctx, orgPK, req); problemStatus(err) != 404 {
		t.Errorf("plano inexistente: status %d, queria 404 (%v)", problemStatus(err), err)
	}
}
