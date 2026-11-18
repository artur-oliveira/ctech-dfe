//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services/bpes"
)

func lineSK(t *testing.T, repo *repositories.BpeLineRepository, orgPK string, line map[string]any) string {
	t.Helper()
	item, err := repo.Create(context.Background(), orgPK, marshal(t, line))
	if err != nil {
		t.Fatalf("linha: %v", err)
	}
	return item["sk"].(*types.AttributeValueMemberS).Value
}

// A venda do BP-e aponta a linha por id: o servidor monta o corpo completo (percurso, serviço, ICMS e locais) a partir do
// cadastro, e o passageiro avulso segue direto no corpo.
func TestBpe_ResolveTicketFromLine(t *testing.T) {
	ctx := context.Background()
	orgPK, other := "CNPJ_"+randomCNPJ(), "CNPJ_"+randomCNPJ()
	lineRepo := repositories.NewBpeLineRepository(db, cfg)
	svc := bpes.NewBpeService(nil, nil, bpeConfigRepo, bpeRepo, nil, personRepo, lineRepo, nil, nil, nil, nil, "", bpes.TechData{}, nil)

	normal := lineSK(t, lineRepo, orgPK, map[string]any{
		"name": "SP x RJ", "variant": "normal", "modal": "rodoviario",
		"origin":      map[string]any{"uf": "SP", "ibge_code": "3550308", "city": "Sao Paulo"},
		"destination": map[string]any{"uf": "RJ", "ibge_code": "3304557", "city": "Rio de Janeiro"},
		"route_code":  "L100", "route_name": "SAO PAULO X RIO", "trip_type": "00", "service_type": "6", "accommodation": "1",
		"prefix": "1234", "icms": map[string]any{"cst": "00", "rate": "12"},
	})
	req := bpes.BpeTicketRequest{
		Presence: "1", Boarding: "2026-11-20T08:00:00-03:00", Validity: "2026-12-20T08:00:00-03:00",
		Trips:      []bpes.BpeTripRequest{{LineID: normal, LegType: "1", Departure: "2026-11-20T08:00:00-03:00", Seat: strp("12")}},
		Passenger:  &bpes.BpePassenger{Name: "  Maria   Souza ", DocType: "1", DocNumber: "123456789"},
		Components: []bpes.BpeComponent{{Type: "01", Value: "100.00"}},
		Payments:   []bpes.BpePayment{{Type: "01", Value: "100.00"}},
	}
	variant, body, _, err := svc.ResolveTicket(ctx, orgPK, req)
	if err != nil {
		t.Fatalf("ResolveTicket: %v", err)
	}
	if variant != bpes.VariantNormal || body.Modal != "rodoviario" || body.Origin.IBGECode != "3550308" || body.Destination.UF != "RJ" ||
		body.Passage.FromName != "Sao Paulo" || body.Trips[0].RouteCode != "L100" || body.Trips[0].ServiceType != "6" ||
		body.ICMS.CST != "00" || body.Passage.Passenger.Name != "Maria Souza" {
		t.Errorf("corpo = %+v", body)
	}

	// Outra organização não enxerga a linha; linha de outra variante é recusada.
	if _, _, _, err := svc.ResolveTicket(ctx, other, req); problemStatus(err) != 404 {
		t.Errorf("outra organização: status %d, queria 404 (%v)", problemStatus(err), err)
	}
	req.Variant = bpes.VariantTA
	if _, _, _, err := svc.ResolveTicket(ctx, orgPK, req); problemStatus(err) != 400 {
		t.Errorf("linha comum numa venda TA: status %d, queria 400 (%v)", problemStatus(err), err)
	}
}

func TestBpe_ResolveTMFromLine(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	lineRepo := repositories.NewBpeLineRepository(db, cfg)
	svc := bpes.NewBpeService(nil, nil, bpeConfigRepo, bpeRepo, nil, personRepo, lineRepo, nil, nil, nil, nil, "", bpes.TechData{}, nil)

	tm := lineSK(t, lineRepo, orgPK, map[string]any{
		"name": "Linha 1", "variant": "tm", "modal": "ferroviario", "cfop": "5353", "origin_uf": "SP", "prefix": "L1",
		"legs":            []any{map[string]any{"origin_ibge_code": "3550308", "icms": map[string]any{"cst": "40"}}},
		"passenger_types": []any{"Inteira", "Meia"},
	})
	req := bpes.BpeTMRequest{
		Variant: "tm", Competence: "2026-11-01",
		Groups: []bpes.BpeTMGroupRequest{{LineID: tm, Items: []bpes.BpeTMItemRequest{{
			Leg: 1, Passengers: "10", Value: "40.00",
			Components: []bpes.BpeTMComponent{{Name: "Inteira", Quantity: 6}, {Name: "Meia", Quantity: 4}},
		}}}},
	}
	body, err := svc.ResolveTM(ctx, orgPK, req)
	if err != nil {
		t.Fatalf("ResolveTM: %v", err)
	}
	g := body.Groups[0]
	if body.Modal != "ferroviario" || body.CFOP != "5353" || g.OriginUF != "SP" || g.Items[0].OriginIBGE != "3550308" || g.Items[0].ICMS.CST != "40" {
		t.Errorf("corpo = %+v", body)
	}
	// Tipo de passageiro que a linha não tem, e trecho inexistente, são 400.
	req.Groups[0].Items[0].Components[0].Name = "Estudante"
	if _, err := svc.ResolveTM(ctx, orgPK, req); problemStatus(err) != 400 {
		t.Errorf("tipo de passageiro desconhecido: status %d (%v)", problemStatus(err), err)
	}
	req.Groups[0].Items[0].Components[0].Name = "Inteira"
	req.Groups[0].Items[0].Leg = 2
	if _, err := svc.ResolveTM(ctx, orgPK, req); problemStatus(err) != 400 {
		t.Errorf("trecho inexistente: status %d (%v)", problemStatus(err), err)
	}
}

// Pessoa de outra organização nunca entra no bilhete: o cadastro é lido pela partição da organização que emite.
func TestBpe_PersonOfOtherOrganizationIsNotResolved(t *testing.T) {
	ctx := context.Background()
	orgA, orgB := "CNPJ_"+randomCNPJ(), "CNPJ_"+randomCNPJ()
	lineRepo := repositories.NewBpeLineRepository(db, cfg)
	svc := bpes.NewBpeService(nil, nil, bpeConfigRepo, bpeRepo, nil, personRepo, lineRepo, nil, nil, nil, nil, "", bpes.TechData{}, nil)
	line := map[string]any{
		"name": "SP x RJ", "variant": "normal", "modal": "rodoviario",
		"origin":      map[string]any{"uf": "SP", "ibge_code": "3550308", "city": "Sao Paulo"},
		"destination": map[string]any{"uf": "RJ", "ibge_code": "3304557", "city": "Rio de Janeiro"},
		"route_code":  "L100", "route_name": "SAO PAULO X RIO", "trip_type": "00", "service_type": "6", "accommodation": "1",
		"icms": map[string]any{"cst": "40"},
	}
	lineB := lineSK(t, lineRepo, orgB, line)
	if _, err := personSvc.Create(ctx, orgA, "33000167000101", map[string]any{
		"name": "Empresa da A",
		"addresses": []any{map[string]any{
			"street": "Rua B", "number": "20", "neighborhood": "Bairro", "city_ibge_code": "3550308",
			"city": "Sao Paulo", "postal_code": "01002000", "state_federation": "SP",
		}},
	}, testUserID, testUserName); err != nil {
		t.Fatal(err)
	}
	other := "CNPJ_33000167000101" // existe só na organização A
	req := bpes.BpeTicketRequest{
		Presence: "1", Boarding: "2026-11-20T08:00:00-03:00", Validity: "2026-12-20T08:00:00-03:00",
		Trips:         []bpes.BpeTripRequest{{LineID: lineB, LegType: "1", Departure: "2026-11-20T08:00:00-03:00"}},
		Passenger:     &bpes.BpePassenger{Name: "Maria Souza", DocType: "1", DocNumber: "1"},
		BuyerPersonID: &other,
		Components:    []bpes.BpeComponent{{Type: "01", Value: "10.00"}},
		Payments:      []bpes.BpePayment{{Type: "01", Value: "10.00"}},
	}
	if _, _, _, err := svc.ResolveTicket(ctx, orgB, req); problemStatus(err) != 404 {
		t.Fatalf("comprador de outra organização: status %d, queria 404 (%v)", problemStatus(err), err)
	}
	req.BuyerPersonID, req.AgencyPersonID = nil, &other
	if _, _, _, err := svc.ResolveTicket(ctx, orgB, req); problemStatus(err) != 404 {
		t.Fatalf("agência de outra organização: status %d, queria 404 (%v)", problemStatus(err), err)
	}
	cpf := "CPF_52998224725"
	req.AgencyPersonID, req.Passenger, req.PassengerPersonID = nil, nil, &cpf
	if _, _, _, err := svc.ResolveTicket(ctx, orgB, req); problemStatus(err) != 404 {
		t.Fatalf("passageiro inexistente na organização: status %d, queria 404 (%v)", problemStatus(err), err)
	}
}

func strp(s string) *string { return &s }
