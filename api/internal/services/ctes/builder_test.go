package ctes

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"
	dfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/validate"

	"github.com/shopspring/decimal"
)

func addr(uf, cMun, xMun string) nfexml.Address {
	return nfexml.Address{Street: "Rua A", Number: "10", District: "Centro", CityIBGE: cMun, City: xMun, UF: uf, CEP: "01001-000"}
}

func baseParams(t *testing.T, takerRole string) buildParams {
	t.Helper()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	comps, _ := mustComponents(t, CteFreight{Total: "1850.00"})
	icms, err := resolveICMS(nil, 1, decimal.RequireFromString("1850.00"))
	if err != nil {
		t.Fatal(err)
	}
	p := buildParams{
		accessKey:   services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 7, now, services.TpEmisNormal),
		serie:       1,
		number:      7,
		environment: 2,
		now:         now,
		tpEmis:      services.TpEmisNormal,
		cfop:        "6353",
		emit:        actor{CNPJ: cnpjA, IE: "123456789012", Name: "TRANSPORTES TESTE", Phone: "1133334444", Addr: addr("SP", "3550308", "São Paulo")},
		crt:         1,
		rem:         actor{CNPJ: cnpjA, IE: "123456789012", Name: "REMETENTE", Addr: addr("SP", "3550308", "São Paulo")},
		dest:        actor{CNPJ: cnpjB, IE: "ISENTO", Name: "DESTINATARIO", Addr: addr("MG", "3106200", "Belo Horizonte")},
		takerRole:   takerRole,
		origin:      Place{UF: "SP", CityIBGE: "3550308", City: "São Paulo"},
		destination: Place{UF: "MG", CityIBGE: "3106200", City: "Belo Horizonte"},
		vTPrest:     decimal.RequireFromString("1850.00"),
		comps:       comps,
		icms:        icms,
		vCarga:      decimal.RequireFromString("4500.00"),
		proPred:     "CAIXA",
		weight:      decimal.RequireFromString("960"),
		volumes:     36,
		nfeKeys:     []string{nfeKey(1), nfeKey(2)},
		rntrc:       "12345678",
		tech:        TechData{CNPJ: cnpjB, Name: "Resp", Email: "a@b.com", Phone: "11999990000"},
	}
	if takerRole == RoleOutro {
		p.toma4 = &actor{CPF: cpfC, Name: "PAGADOR", Addr: addr("RJ", "3304557", "Rio de Janeiro")}
	}
	return p
}

func mustComponents(t *testing.T, f CteFreight) ([]map[string]any, decimal.Decimal) {
	t.Helper()
	total, comps, err := resolveComponents(f)
	if err != nil {
		t.Fatal(err)
	}
	return comps, total
}

func infCte(body map[string]any) map[string]any {
	return body["CTe"].(map[string]any)["infCte"].(map[string]any)
}

func TestBuildCTe_Toma3Remetente(t *testing.T) {
	inf := infCte(BuildCTe(baseParams(t, RoleRemetente)))
	ide := inf["ide"].(map[string]any)
	if ide["toma3"].(map[string]any)["toma"] != toma3Remetente || ide["toma4"] != nil {
		t.Fatalf("tomador = %+v / %+v", ide["toma3"], ide["toma4"])
	}
	if ide["indIEToma"] != indIETomaContrib || ide["mod"] != "57" || ide["modal"] != "01" || ide["CFOP"] != "6353" {
		t.Fatalf("ide = %+v", ide)
	}
	if ide["UFIni"] != "SP" || ide["cMunFim"] != "3106200" {
		t.Fatalf("percurso = %v %v", ide["UFIni"], ide["cMunFim"])
	}
	// Homologação: xNome literal do MOC nos atores.
	if inf["rem"].(map[string]any)["xNome"] != homologXNome {
		t.Fatal("xNome de homologação ausente no remetente")
	}
	rodo := inf["infCTeNorm"].(map[string]any)["infModal"].(map[string]any)["rodo"].(map[string]any)
	if rodo["RNTRC"] != "12345678" {
		t.Fatalf("rodo = %+v", rodo)
	}
}

func TestBuildCTe_Toma4OutroAndDestinatario(t *testing.T) {
	inf := infCte(BuildCTe(baseParams(t, RoleOutro)))
	t4 := inf["ide"].(map[string]any)["toma4"].(map[string]any)
	if t4["toma"] != toma4Outros || t4["CPF"] != cpfC || t4["enderToma"].(map[string]any)["UF"] != "RJ" {
		t.Fatalf("toma4 = %+v", t4)
	}
	if inf["ide"].(map[string]any)["indIEToma"] != indIETomaNaoContr {
		t.Fatal("tomador sem IE é não contribuinte (9)")
	}
	dest := infCte(BuildCTe(baseParams(t, RoleDestinatario)))["ide"].(map[string]any)
	if dest["toma3"].(map[string]any)["toma"] != toma3Destinatario || dest["indIEToma"] != indIETomaIsento {
		t.Fatalf("ide = %+v", dest)
	}
}

func TestResolveComponents_SumsToTotal(t *testing.T) {
	comps, total := mustComponents(t, CteFreight{Total: "1850.00", Components: []CteComponent{{Name: "Pedágio", Value: "120.00"}, {Name: "GRIS", Value: "30.50"}}})
	if total.String() != "1850" || len(comps) != 3 || comps[0]["xNome"] != compFreteValor || comps[0]["vComp"] != "1699.50" {
		t.Fatalf("comps = %+v", comps)
	}
	sum := decimal.Zero
	for _, c := range comps {
		sum = sum.Add(decimal.RequireFromString(c["vComp"].(string)))
	}
	if !sum.Equal(total) {
		t.Fatalf("soma dos Comp %s != vTPrest %s", sum, total)
	}
	single, _ := mustComponents(t, CteFreight{Total: "100"})
	if len(single) != 1 || single[0]["vComp"] != "100.00" {
		t.Fatalf("sem componentes = %+v", single)
	}
	if _, _, err := resolveComponents(CteFreight{Total: "10", Components: []CteComponent{{Name: "X", Value: "11"}}}); code(err) != CodeComponentsExceedTotal {
		t.Fatalf("err = %v", err)
	}
}

func code(err error) string {
	var p *problem.Problem
	if errors.As(err, &p) {
		return p.Code
	}
	return ""
}

func TestResolveICMS(t *testing.T) {
	base := decimal.RequireFromString("1850.00")
	sn, _ := resolveICMS(nil, 2, base)
	if g := sn["ICMSSN"].(map[string]any); g["CST"] != "90" || g["indSN"] != "1" {
		t.Fatalf("SN = %+v", sn)
	}
	p12 := "12.00"
	n00, _ := resolveICMS(&CteICMS{CST: "00", PICMS: &p12}, 3, base)
	if g := n00["ICMS00"].(map[string]any); g["vBC"] != "1850.00" || g["pICMS"] != "12.00" || g["vICMS"] != "222.00" {
		t.Fatalf("00 = %+v", n00)
	}
	// Arredondamento bancário: 0,125 vai para 0,12.
	p125, red := "12.50", "0.00"
	n90, _ := resolveICMS(&CteICMS{CST: "90", PICMS: &p125, PRedBC: &red}, 3, decimal.RequireFromString("1.00"))
	if g := n90["ICMS90"].(map[string]any); g["vICMS"] != "0.12" {
		t.Fatalf("90 = %+v", n90)
	}
	if _, err := resolveICMS(nil, 3, base); code(err) != CodeICMSRequired {
		t.Fatalf("regime normal sem ICMS: %v", err)
	}
	if _, err := resolveICMS(&CteICMS{CST: "00"}, 3, base); code(err) != CodeICMSRateRequired {
		t.Fatalf("CST 00 sem alíquota: %v", err)
	}
	if _, err := resolveICMS(&CteICMS{CST: "SN"}, 3, base); code(err) != CodeICMSSNNotAllowed {
		t.Fatalf("SN no regime normal: %v", err)
	}
	if g, _ := resolveICMS(&CteICMS{CST: "41"}, 3, base); g["ICMS45"].(map[string]any)["CST"] != "41" {
		t.Fatalf("41 = %+v", g)
	}
}

// Paridade builder x XSD do CT-e e do modal rodoviário: nenhuma violação de schema (o builder preenche o
// documento inteiro) e nenhuma regra block do MOC.
func TestBuildCTe_ParidadeComXSD(t *testing.T) {
	for _, role := range []string{RoleRemetente, RoleOutro} {
		p := baseParams(t, role)
		p12 := "12.00"
		icms, _ := resolveICMS(&CteICMS{CST: "00", PICMS: &p12}, 3, p.vTPrest)
		for _, ic := range []map[string]any{p.icms, icms} {
			p.icms = ic
			cte := BuildCTe(p)["CTe"].(map[string]any)
			findings := dfecheck.Shadow(dfecheck.SpecCTe, "cte", p.accessKey, cte)
			rodo := cte["infCte"].(map[string]any)["infCTeNorm"].(map[string]any)["infModal"].(map[string]any)["rodo"].(map[string]any)
			findings = append(findings, dfecheck.Shadow("cte_rodo", "cte_rodo", p.accessKey, rodo)...)
			for _, v := range findings {
				if v.Code == "rule" && v.Severity != validate.SevError {
					continue
				}
				t.Errorf("%s %s %s: %s %s", role, v.Path, v.Code, v.Rule, v.Message)
			}
		}
	}
}

func TestQRCodeURL(t *testing.T) {
	if u := qrCodeURL("SP", "K", 1, ""); !strings.HasPrefix(u, qrCodeBaseByUF["SP"]+"?chCTe=K&tpAmb=1") {
		t.Fatal(u)
	}
	if u := qrCodeURL("MT", "K", 2, ""); !strings.HasPrefix(u, "https://homologacao.sefaz.mt.gov.br/cte/qrcode?chCTe=K&tpAmb=2") {
		t.Errorf("MT em homologação usa o endereço próprio: %s", u)
	}
	if u := qrCodeURL("BA", "K", 1, ""); !strings.HasPrefix(u, qrCodeBaseSVRS) {
		t.Fatal(u)
	}
}

// O XML sai na ordem do XSD (tabela xsdorder do go-dfe), não na ordem alfabética do mapa.
func TestBuildCTe_XMLOrder(t *testing.T) {
	body := BuildCTe(baseParams(t, RoleRemetente))
	raw, err := dfe.BuildXMLFragment(body["CTe"].(map[string]any), "CTe", cteXMLNS)
	if err != nil {
		t.Fatal(err)
	}
	xml := string(raw)
	last := -1
	for _, tag := range []string{"<ide>", "<cUF>", "<CFOP>", "<UFFim>", "<retira>", "<indIEToma>", "<toma3>", "</ide>",
		"<emit>", "<CRT>", "<rem>", "<CEP>", "<dest>", "<vPrest>", "<vRec>", "<Comp>", "<imp>", "<infCTeNorm>",
		"<infCarga>", "<infDoc>", "<infModal", "<rodo>", "<infRespTec>", "<infCTeSupl>"} {
		i := strings.Index(xml[last+1:], tag)
		if i < 0 {
			t.Fatalf("%s fora de ordem ou ausente em\n%s", tag, xml)
		}
		last += i + 1
	}
}
