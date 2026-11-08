package ctes

import (
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"
	dfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

func s(v string) *string { return &v }

var osNow = time.Date(2026, 11, 5, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))

func osBase(service string) CteOsEmitBody {
	b := CteOsEmitBody{
		ServiceType: service,
		Taker:       CteOsTaker{PersonID: "x"},
		Service:     CteOsService{Description: "TRANSPORTE", Quantity: s("12")},
		Origin:      &CteOsPlace{CityIBGE: "3550308", City: "São Paulo", UF: "SP"},
		Destination: &CteOsPlace{CityIBGE: "3106200", City: "Belo Horizonte", UF: "MG"},
		RouteUFs:    []string{"SP", "MG"},
		CteChargeBody: CteChargeBody{
			Freight: CteFreight{Total: "500.00"}, CFOP: s("6353"),
		},
	}
	switch service {
	case OsServicePeople:
		b.Road = &CteOsRoad{TAF: s("123456789012"), Vehicle: &CteOsVehicle{Plate: "ABC1D23"},
			Charter: &CteOsCharter{Type: 1, TripAt: s("2026-12-01T08:00:00-03:00")}}
		b.FederalTaxes = &CteOsFederalTaxes{INSS: s("10.00")}
	case OsServiceBaggage:
		b.Road = &CteOsRoad{TAF: s("123456789012")}
		b.DocRefs = []CteOsDocRef{{Number: s("123"), IssuedOn: s("2026-11-01"), DocAmount: s("10.00")}}
	case OsServiceValues:
		b.GTVe = []CteOsGTVe{{Key: services.GenerateAccessKey("SP", cnpjA, "64", 1, 1, osNow, services.TpEmisNormal), Components: []CteOsGTVeComp{{Type: 1, Value: "300.00"}, {Type: 6, Value: "200.00", Name: s("TAXA")}}}}
	}
	return b
}

func osBuilt(t *testing.T, service string) (buildParams, map[string]any) {
	t.Helper()
	req := osBase(service)
	emit := baseParams(t, RoleRemetente).emit
	taker := actor{CNPJ: cnpjB, IE: "ISENTO", Name: "TOMADOR", Addr: addr("MG", "3106200", "Belo Horizonte")}
	p, err := osParams(req, emit, taker, 1, osNow)
	if err != nil {
		t.Fatalf("%s: %v", service, err)
	}
	ch, err := resolveCharge(req.CteChargeBody, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch.apply(&p, req.CteChargeBody)
	b := baseParams(t, RoleRemetente)
	p.accessKey = services.GenerateAccessKey("SP", cnpjA, ModelCTeOS, 1, 9, osNow, services.TpEmisNormal)
	p.serie, p.number, p.environment, p.now, p.tpEmis, p.tech = 1, 9, 2, osNow, services.TpEmisNormal, b.tech
	return p, BuildCTe(p)
}

func TestCTeOSParityWithXSD(t *testing.T) {
	for _, svc := range []string{OsServicePeople, OsServiceBaggage, OsServiceValues} {
		p, body := osBuilt(t, svc)
		cte, rodo := dfecheck.CTeOSModal(body)
		findings := dfecheck.Shadow(dfecheck.SpecCTeOS, "cteos", p.accessKey, cte)
		if rodo != nil {
			findings = append(findings, dfecheck.Shadow(dfecheck.SpecCTeRodoOS, "cte_rodoos", p.accessKey, rodo)...)
		}
		for _, v := range findings {
			if v.Code == "rule" && v.Severity != validate.SevError {
				continue
			}
			t.Errorf("%s %s %s: %s", svc, v.Path, v.Code, v.Message)
		}
		if err := gateCTe(body); err != nil {
			t.Errorf("%s portão: %v", svc, err)
		}
		ide := cte["infCte"].(map[string]any)["ide"].(map[string]any)
		if ide["mod"] != "67" || ide["tpServ"] != osTpServ[svc] || ide["modal"] != "01" {
			t.Errorf("%s ide = %+v", svc, ide)
		}
	}
}

func TestCTeOSXMLOrder(t *testing.T) {
	_, body := osBuilt(t, OsServicePeople)
	raw, err := dfe.BuildXMLFragment(body["CTeOS"].(map[string]any), "CTeOS", cteXMLNS)
	if err != nil {
		t.Fatal(err)
	}
	xml, last := string(raw), -1
	for _, tag := range []string{"<ide>", "<UFFim>", "<infPercurso>", "<emit>", "<toma>", "<vPrest>", "<imp>", "<infTribFed>",
		"<infCTeNorm>", "<infServico>", "<infQ>", "<infModal", "<rodoOS>", "<TAF>", "<veic>", "<infFretamento>", "<infRespTec>", "<infCTeSupl>"} {
		i := strings.Index(xml[last+1:], tag)
		if i < 0 {
			t.Fatalf("%s fora de ordem ou ausente em\n%s", tag, xml)
		}
		last += i + 1
	}
	if _, err := nfexml.ParseCTe(raw); err != nil {
		t.Fatal(err)
	}
}

func TestOsParamsRules(t *testing.T) {
	emit := baseParams(t, RoleRemetente).emit
	pj := actor{CNPJ: cnpjB, Name: "T", Addr: addr("MG", "3106200", "Belo Horizonte")}
	cases := []struct {
		name string
		mut  func(*CteOsEmitBody)
		want string
	}{
		{"pessoas sem origem", func(b *CteOsEmitBody) { b.Origin = nil }, CodeOsPlaceRequired},
		{"município fora da UF", func(b *CteOsEmitBody) { b.Origin.UF = "RJ" }, CodeOsPlaceUFMismatch},
		{"pessoas sem modal", func(b *CteOsEmitBody) { b.Road = nil }, CodeOsRoadRequired},
		{"interestadual sem TAF", func(b *CteOsEmitBody) { b.Road.TAF = nil; b.Road.StateRegistration = s(strings.Repeat("1", 25)) }, CodeOsTAFNeeded},
		{"sem fretamento", func(b *CteOsEmitBody) { b.Road.Charter = nil }, CodeOsCharterRequired},
		{"eventual sem viagem", func(b *CteOsEmitBody) { b.Road.Charter.TripAt = nil }, CodeOsTripAtRequired},
		{"viagem no passado", func(b *CteOsEmitBody) { b.Road.Charter.TripAt = s("2026-01-01T08:00:00-03:00") }, CodeOsTripAtPast},
		{"sem INSS para PJ", func(b *CteOsEmitBody) { b.FederalTaxes = nil }, CodeOsINSSRequired},
		{"GTV-e em pessoas", func(b *CteOsEmitBody) { b.GTVe = []CteOsGTVe{{}} }, CodeOsGTVeNotAllowed},
	}
	for _, c := range cases {
		b := osBase(OsServicePeople)
		c.mut(&b)
		if _, err := osParams(b, emit, pj, 1, osNow); code(err) != c.want {
			t.Errorf("%s: code = %q (%v), want %q", c.name, code(err), err, c.want)
		}
	}
	bag := osBase(OsServiceBaggage)
	bag.DocRefs = nil
	if _, err := osParams(bag, emit, pj, 1, osNow); code(err) != CodeOsDocRefsRequired {
		t.Errorf("bagagem sem documentos: %v", err)
	}
	val := osBase(OsServiceValues)
	val.Road = &CteOsRoad{TAF: s("123456789012")}
	if _, err := osParams(val, emit, pj, 1, osNow); code(err) != CodeOsRoadNotAllowed {
		t.Errorf("valores com modal: %v", err)
	}
	val = osBase(OsServiceValues)
	val.GTVe = nil
	if _, err := osParams(val, emit, pj, 1, osNow); code(err) != CodeOsGTVeRequired {
		t.Errorf("valores sem GTV-e: %v", err)
	}
	val = osBase(OsServiceValues)
	if err := checkGTVeTotal(val, nfexml.Dec("499.95")); err != nil {
		t.Errorf("dentro da tolerância: %v", err)
	}
	if code(checkGTVeTotal(val, nfexml.Dec("400.00"))) != CodeOsGTVeTotalMismatch {
		t.Error("H053: soma diferente do frete")
	}
}
