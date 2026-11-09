package ciots

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/ciot"
)

func asProblem(t *testing.T, err error) *problem.Problem {
	t.Helper()
	p, ok := errors.AsType[*problem.Problem](err)
	if !ok {
		t.Fatalf("not a problem: %v", err)
	}
	return p
}

// An empty declaration is refused with 422 and a stable code before the certificate, the database or the
// outbox are touched (the Service has no dependencies here, so touching any of them would panic).
func TestDeclareEmptyBodyIsValidationProblem(t *testing.T) {
	s := &Service{now: time.Now}
	_, err := s.Declare(t.Context(), "ORG", DeclareBody{Environment: "hom"}, "u", "n")
	p := asProblem(t, err)
	if p.Status != 422 || p.Code != CodeValidation || len(p.Errors) == 0 {
		t.Fatalf("problem = %+v", p)
	}
}

func TestFreightFloorComputesFromVerifiedTable(t *testing.T) {
	res, err := FreightFloor(ciot.FloorInput{Composition: true, HighPerformance: true, CargoType: 5, Axles: 2, DistanceKm: 100, EmptyReturn: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Cents != 82242 || res.Value != "822.42" || res.OutboundValue != "511.00" || res.EmptyReturnValue != "311.42" ||
		res.AxlesUsed != 2 || res.TableVersion != "2026-07-17" || res.ValidFrom != "2026-07-17" {
		t.Fatalf("%+v", res)
	}
}

func TestFreightFloorUnverifiedIs503NeverANumber(t *testing.T) {
	tab := ciot.DefaultFloorTable()
	tab.Verified = false
	res, err := freightFloor(tab, ciot.FloorInput{CargoType: 1, Axles: 2, DistanceKm: 100})
	if res != nil {
		t.Fatalf("returned a number: %+v", res)
	}
	if p := asProblem(t, err); p.Status != 503 || p.Code != CodeFloor {
		t.Fatalf("problem = %+v", p)
	}
}

func TestFreightFloorBadInputIs422(t *testing.T) {
	for _, in := range []ciot.FloorInput{{CargoType: 0, Axles: 2, DistanceKm: 1}, {CargoType: 13, Axles: 2, DistanceKm: 1}, {CargoType: 1, Axles: 0, DistanceKm: 1}, {CargoType: 1, Axles: 2}} {
		if p := asProblem(t, func() error { _, err := FreightFloor(in); return err }()); p.Status != 422 || p.Code != CodeValidation {
			t.Fatalf("%+v: %+v", in, p)
		}
	}
}

func TestUnwrapEgress(t *testing.T) {
	if body, err := unwrapEgress(godfe.Response{StatusCode: 200, Body: `{"codigo":111}`}); err != nil || body != `{"codigo":111}` {
		t.Fatalf("200: %q %v", body, err)
	}
	_, err := unwrapEgress(godfe.Response{StatusCode: 422, Body: `{"detail":"x","internal_code":"CIOT_PAYMENT_233","antt_code":233,"antt_message":"m"}`})
	if p := asProblem(t, err); p.Status != 422 || p.Code != "CIOT_PAYMENT_233" {
		t.Fatalf("422: %+v", p)
	}
	_, err = unwrapEgress(godfe.Response{StatusCode: 502, Body: `{}`})
	if p := asProblem(t, err); p.Status != 503 || p.Code != "ciot_unavailable" {
		t.Fatalf("502: %+v", p)
	}
}

func TestToDeclaracaoNormalizes(t *testing.T) {
	now := time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)
	d := Operation{OperationType: 1, CarrierRNTRC: "12345678", FreightValue: "1500.5",
		Vehicles: []Vehicle{{Plate: "abc-1234"}}}.toDeclaracao("", now)
	if d.ValorFrete != "1500.50" || d.RNTRCContratado != "012345678" || d.Veiculos[0].Placa != "ABC1234" {
		t.Fatalf("declaration = %+v", d)
	}
	if d.DataDeclaracao != "2026-11-08T12:00:00" { // Brasilia is UTC-3
		t.Fatalf("DataDeclaracao = %s", d.DataDeclaracao)
	}
}

func TestLifecycleContextUsesDeclarationDateForAgregado(t *testing.T) {
	row := map[string]types.AttributeValue{
		"operation_type": &types.AttributeValueMemberN{Value: "3"},
		"declared_at":    &types.AttributeValueMemberS{Value: "2026-11-01T10:00:00"},
	}
	lc := lifecycleContext(row, ciot.OpClose, time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC))
	if lc.Start.IsZero() || lc.Tipo != 3 {
		t.Fatalf("context = %+v", lc)
	}
	if vs := ciot.ValidateLifecycle(lc); len(vs) == 0 { // closing 19 days after the start breaks B116
		t.Fatal("expected B116")
	}
}
