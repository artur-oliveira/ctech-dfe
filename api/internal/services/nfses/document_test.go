package nfses

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func attrs(m map[string]string) map[string]types.AttributeValue {
	out := map[string]types.AttributeValue{}
	for k, v := range m {
		out[k] = &types.AttributeValueMemberS{Value: v}
	}
	return out
}

// minimalInput builds a valid documentInput: organização com CNPJ e grupo nfse
// (op_simp_nac=1), config nacional (c_loc_emi=2211001, serie=1), um item de
// catálogo (trib_nacional_code=10101, value=1000.00, iss.tax_rate=2.00) e um
// body de emissão do prestador (tp_emit=1) referenciando o catálogo.
func minimalInput() documentInput {
	// Mesma forma que organizations/organization_persons gravam: identidade na
	// raiz, endereços/contatos/grupo nfse dentro de `person`.
	org := map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: "CNPJ_11222333000181"},
		"cpf_or_cnpj": &types.AttributeValueMemberS{Value: "11222333000181"},
		"name":        &types.AttributeValueMemberS{Value: "Prestador LTDA"},
		"person": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"nfse": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"reg_trib": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"op_simp_nac":  &types.AttributeValueMemberN{Value: "1"},
					"reg_esp_trib": &types.AttributeValueMemberN{Value: "0"},
				}},
			}},
			"contacts": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"emails": &types.AttributeValueMemberL{Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "fiscal@prestador.com.br"},
				}},
				"phones": &types.AttributeValueMemberL{Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "9832001000"},
				}},
			}},
			"addresses": &types.AttributeValueMemberL{Value: []types.AttributeValue{
				&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"street":         &types.AttributeValueMemberS{Value: "Rua Um"},
					"number":         &types.AttributeValueMemberS{Value: "100"},
					"neighborhood":   &types.AttributeValueMemberS{Value: "Centro"},
					"city_ibge_code": &types.AttributeValueMemberS{Value: "2211001"},
					"postal_code":    &types.AttributeValueMemberS{Value: "65000000"},
				}},
			}},
		}},
	}

	config := attrs(map[string]string{
		"provider":  "nacional",
		"c_loc_emi": "2211001",
		"serie":     "1",
	})

	service := map[string]types.AttributeValue{
		"trib_nacional_code":  &types.AttributeValueMemberS{Value: "10101"},
		"trib_municipal_code": &types.AttributeValueMemberS{Value: "415"},
		"description":         &types.AttributeValueMemberS{Value: "Análise de sistemas"},
		"code":                &types.AttributeValueMemberS{Value: "SVC-001"},
		"value":               &types.AttributeValueMemberS{Value: "1000.00"},
		"iss": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"trib_issqn":   &types.AttributeValueMemberN{Value: "1"},
			"tax_rate":     &types.AttributeValueMemberS{Value: "2.00"},
			"tp_ret_issqn": &types.AttributeValueMemberN{Value: "1"},
		}},
		"ibs_cbs": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"c_ind_op":     &types.AttributeValueMemberS{Value: "100301"},
			"cst":          &types.AttributeValueMemberS{Value: "000"},
			"c_class_trib": &types.AttributeValueMemberS{Value: "000001"},
			"ind_dest":     &types.AttributeValueMemberN{Value: "0"},
			"fin_nfse":     &types.AttributeValueMemberN{Value: "0"},
		}},
	}

	return documentInput{
		Org:         org,
		Config:      config,
		Prestador:   org,
		Service:     service,
		Environment: 2,
		Serie:       "1",
		Numero:      1,
		DhEmi:       "2026-08-08T10:30:00-03:00",
		Body: NfseEmitBody{
			TpEmit:     1,
			Competence: "2026-08-01",
			Service: NfseServiceItem{
				ServiceID: "SERVICE_x",
			},
		},
	}
}

func TestBuildDocument_RejectsServiceWithoutIBSCBS(t *testing.T) {
	in := minimalInput()
	delete(in.Service, "ibs_cbs")
	if _, err := buildDocument(in); err == nil {
		t.Fatal("serviço sem IBS/CBS aceito")
	}
}

func TestBuildDocument_MapsISOCompetenceAndDhEmi(t *testing.T) {
	doc, err := buildDocument(minimalInput())
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	if doc.Competencia != "2026-08-01" {
		t.Errorf("Competencia = %q", doc.Competencia)
	}
	if doc.DhEmi != "2026-08-08T10:30:00-03:00" {
		t.Errorf("DhEmi = %q", doc.DhEmi)
	}
}

func TestNfseEmissionTime_UsesConfiguredTimezoneAndFallback(t *testing.T) {
	now := time.Date(2026, 8, 8, 13, 30, 0, 0, time.UTC)
	fortaleza, err := nfseEmissionTime(now, "America/Fortaleza")
	if err != nil {
		t.Fatalf("nfseEmissionTime Fortaleza: %v", err)
	}
	if got := fortaleza.Format(dfeDateTimeLayout); got != "2026-08-08T10:30:00-03:00" {
		t.Errorf("Fortaleza = %q", got)
	}
	fallback, err := nfseEmissionTime(now, "")
	if err != nil {
		t.Fatalf("nfseEmissionTime fallback: %v", err)
	}
	if fallback.Location().String() != defaultTimezone {
		t.Errorf("fallback timezone = %q", fallback.Location())
	}
	if _, err := nfseEmissionTime(now, "America/Invalid"); err == nil {
		t.Fatal("timezone inválido aceito")
	}
}

func TestBuildIDDPS_MatchesLayout(t *testing.T) {
	got := BuildIDDPS("2211001", "2", "11222333000181", "1", 42)
	if len(got) != 45 {
		t.Fatalf("len = %d, esperado 45 (TSIdDPS)", len(got))
	}
	if !strings.HasPrefix(got, "DPS2211001211222333000181") {
		t.Errorf("prefixo incorreto: %q", got)
	}
	if !strings.HasSuffix(got, "00001000000000000042") {
		t.Errorf("serie/nDPS mal preenchidos: %q", got)
	}
}

func TestBuildDocument_RequiresMotivoWhenTomadorEmits(t *testing.T) {
	in := minimalInput()
	in.Body.TpEmit = 2
	if _, err := buildDocument(in); err == nil {
		t.Fatal("esperado erro: c_motivo_emis_ti obrigatório com tp_emit=2")
	}
}

func TestBuildDocument_RequiresRegTribOnPrestador(t *testing.T) {
	in := minimalInput()
	in.Body.TpEmit = 2
	in.Body.MotivoEmisTI = 1
	in.Prestador = attrs(map[string]string{"name": "Prestador Terceiro"}) // sem grupo nfse
	if _, err := buildDocument(in); err == nil {
		t.Fatal("esperado erro: prestador sem reg_trib no cadastro")
	}
}

// Regressão: identidade, endereço e contatos vivem em `person` no item de
// cadastro. Ler a raiz do item produzia um DPS sem CNPJ e sem endereço.
func TestBuildDocument_ReadsIdentityFromPersonGroup(t *testing.T) {
	doc, err := buildDocument(minimalInput())
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	p := doc.Prestador.Pessoa
	if p.CNPJ != "11222333000181" {
		t.Errorf("CNPJ = %q, esperado 11222333000181", p.CNPJ)
	}
	if p.Email != "fiscal@prestador.com.br" || p.Fone != "9832001000" {
		t.Errorf("contatos = %q/%q, esperado vir de person.contacts", p.Email, p.Fone)
	}
	if p.End == nil || p.End.CMun != "2211001" || p.End.XLgr != "Rua Um" {
		t.Fatalf("endereço = %+v, esperado vir de person.addresses[0]", p.End)
	}
}

func TestBuildDocument_UsesServiceCatalogDefaults(t *testing.T) {
	doc, err := buildDocument(minimalInput())
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	if doc.Servico.CServ.CTribNac != "10101" {
		t.Errorf("cTribNac = %q, esperado vir do catálogo", doc.Servico.CServ.CTribNac)
	}
	if doc.Servico.CServ.CTribMun != "415" {
		t.Errorf("cTribMun = %q, esperado vir do catálogo", doc.Servico.CServ.CTribMun)
	}
	if doc.Valores.VServPrest.VServ != "1000.00" {
		t.Errorf("vServ = %q, esperado vir do catálogo", doc.Valores.VServPrest.VServ)
	}
}

func TestBuildDocument_ItemOverridesCatalogMunicipalTaxCode(t *testing.T) {
	in := minimalInput()
	override := "416"
	in.Body.Service.CTribMun = &override
	doc, err := buildDocument(in)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	if doc.Servico.CServ.CTribMun != override {
		t.Errorf("cTribMun = %q, esperado o override %q", doc.Servico.CServ.CTribMun, override)
	}
}

func TestBuildDocument_ItemOverridesCatalogValue(t *testing.T) {
	in := minimalInput()
	override := "2500.00"
	in.Body.Service.Value = &override
	doc, err := buildDocument(in)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	if doc.Valores.VServPrest.VServ != override {
		t.Errorf("vServ = %q, esperado o override %q", doc.Valores.VServPrest.VServ, override)
	}
}

// TestBuildDocument_AdditionalInfoMapsToInfoCompl is a regression test:
// req.AdditionalInfo was parsed and validated but never copied into the
// neutral document, so anything the caller wrote there was silently dropped
// from the DPS. Confirms it now lands in Servico.InfoCompl.XInfComp.
func TestBuildDocument_AdditionalInfoMapsToInfoCompl(t *testing.T) {
	in := minimalInput()
	info := "informação complementar de teste"
	in.Body.AdditionalInfo = &info

	doc, err := buildDocument(in)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	if doc.Servico.InfoCompl == nil || doc.Servico.InfoCompl.XInfComp != info {
		t.Errorf("InfoCompl.XInfComp = %+v, esperado %q", doc.Servico.InfoCompl, info)
	}
}

// TestBuildDocument_NoAdditionalInfo_OmitsInfoCompl: sem additional_info, o
// grupo opcional infoCompl não deve ser criado.
func TestBuildDocument_NoAdditionalInfo_OmitsInfoCompl(t *testing.T) {
	doc, err := buildDocument(minimalInput())
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	if doc.Servico.InfoCompl != nil {
		t.Errorf("InfoCompl = %+v, esperado nil sem additional_info", doc.Servico.InfoCompl)
	}
}

func TestBuildDocument_RequiresRegApTribSNWhenOpSimpNacIs3(t *testing.T) {
	in := minimalInput()
	in.Prestador = map[string]types.AttributeValue{
		"nfse": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"reg_trib": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"op_simp_nac":  &types.AttributeValueMemberN{Value: "3"},
				"reg_esp_trib": &types.AttributeValueMemberN{Value: "0"},
			}},
		}},
		"cpf_cnpj": &types.AttributeValueMemberS{Value: "11222333000181"},
	}
	if _, err := buildDocument(in); err == nil {
		t.Fatal("esperado erro: op_simp_nac=3 exige reg_ap_trib_sn")
	}
}
