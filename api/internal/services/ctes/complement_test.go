package ctes

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"
	dfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// originalOf serializa o CT-e normal do builder e o lê de volta como o XML autorizado do S3.
func originalOf(t *testing.T, role string) *original {
	t.Helper()
	p := baseParams(t, role)
	raw, err := dfe.BuildXMLFragment(BuildCTe(p)["CTe"].(map[string]any), "CTe", cteXMLNS)
	if err != nil {
		t.Fatal(err)
	}
	c, err := nfexml.ParseCTe(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &original{key: p.accessKey, cte: c}
}

// derived completa os dados que persist() coloca (chave nova, série, hora) e o valor complementar.
func derived(t *testing.T, p buildParams) buildParams {
	t.Helper()
	b := baseParams(t, RoleRemetente)
	p.accessKey = services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 8, b.now, services.TpEmisNormal)
	p.serie, p.number, p.environment, p.now, p.tpEmis = 1, 8, 2, b.now, services.TpEmisNormal
	p.cfop, p.vTPrest, p.comps, p.icms = b.cfop, b.vTPrest, b.comps, b.icms
	p.rntrc, p.tech = b.rntrc, b.tech
	return p
}

func parity(t *testing.T, p buildParams) map[string]any {
	t.Helper()
	body := BuildCTe(p)
	cte := body["CTe"].(map[string]any)
	for _, v := range dfecheck.Shadow(dfecheck.SpecCTe, "cte", p.accessKey, cte) {
		if v.Code == "rule" && v.Severity != validate.SevError {
			continue
		}
		t.Errorf("%s %s: %s %s", v.Path, v.Code, v.Rule, v.Message)
	}
	if err := gateCTe(body); err != nil {
		t.Errorf("portão: %v", err)
	}
	return cte["infCte"].(map[string]any)
}

func TestComplementParityWithXSD(t *testing.T) {
	for _, role := range []string{RoleRemetente, RoleDestinatario, RoleOutro} {
		o := originalOf(t, role)
		o2 := originalOf(t, role)
		o2.key = services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 3, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), services.TpEmisNormal)
		p, err := complementParams([]*original{o, o2}, baseParams(t, role).emit, 1)
		if err != nil {
			t.Fatal(err)
		}
		inf := parity(t, derived(t, p))
		ide := inf["ide"].(map[string]any)
		if ide["tpCTe"] != "1" || inf["infCTeNorm"] != nil {
			t.Fatalf("%s: ide.tpCTe=%v infCTeNorm=%v", role, ide["tpCTe"], inf["infCTeNorm"])
		}
		if comp := inf["infCteComp"].([]map[string]any); len(comp) != 2 || comp[0]["chCTe"] != o.key {
			t.Fatalf("%s: infCteComp = %+v", role, inf["infCteComp"])
		}
		if role == RoleOutro && ide["toma4"] == nil {
			t.Fatal("toma4 do original não foi repetido")
		}
		if ide["UFIni"] != "SP" || ide["UFFim"] != "MG" || ide["tpServ"] != "0" {
			t.Fatalf("percurso/serviço do original perdidos: %+v", ide)
		}
	}
}

func TestSubstituteParityWithXSD(t *testing.T) {
	o := originalOf(t, RoleRemetente)
	p, err := substituteParams(o, baseParams(t, RoleRemetente).emit, 1)
	if err != nil {
		t.Fatal(err)
	}
	inf := parity(t, derived(t, p))
	norm := inf["infCTeNorm"].(map[string]any)
	sub := norm["infCteSub"].(map[string]any)
	if sub["chCte"] != o.key || sub["indAlteraToma"] != nil {
		t.Fatalf("infCteSub = %+v", sub)
	}
	if inf["ide"].(map[string]any)["tpCTe"] != "3" || len(norm["infDoc"].(map[string]any)["infNFe"].([]map[string]any)) != 2 {
		t.Fatal("substituto deve repetir as NF-e do original")
	}

	// Troca de tomador: indAlteraToma=1 e toma3 diferente.
	p.sub.alterTakerFlag, p.takerRole = true, RoleDestinatario
	sub = parity(t, derived(t, p))["infCTeNorm"].(map[string]any)["infCteSub"].(map[string]any)
	if sub["indAlteraToma"] != "1" {
		t.Fatalf("indAlteraToma ausente: %+v", sub)
	}
}

func TestXMLOrderComplement(t *testing.T) {
	p, _ := complementParams([]*original{originalOf(t, RoleRemetente)}, baseParams(t, RoleRemetente).emit, 1)
	raw, err := dfe.BuildXMLFragment(BuildCTe(derived(t, p))["CTe"].(map[string]any), "CTe", cteXMLNS)
	if err != nil {
		t.Fatal(err)
	}
	xml, last := string(raw), -1
	for _, tag := range []string{"<ide>", "<emit>", "<rem>", "<dest>", "<vPrest>", "<imp>", "<infCteComp>", "<chCTe>", "<infRespTec>"} {
		i := indexFrom(xml, tag, last+1)
		if i < 0 {
			t.Fatalf("%s fora de ordem ou ausente em\n%s", tag, xml)
		}
		last = i
	}
}

func indexFrom(s, sub string, from int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestComplementGateBlocksRepeatedAndBadKeys(t *testing.T) {
	o := originalOf(t, RoleRemetente)
	p, _ := complementParams([]*original{o, o}, baseParams(t, RoleRemetente).emit, 1)
	if err := gateCTe(BuildCTe(derived(t, p))); err == nil {
		t.Fatal("G177: chave repetida deveria recusar")
	}
	p.complementKeys = []string{"123"}
	if err := gateCTe(BuildCTe(derived(t, p))); err == nil {
		t.Fatal("G161: chave inválida deveria recusar")
	}
}

func TestCheckOriginal(t *testing.T) {
	ie := "123456789012"
	if _, err := checkOriginal(originalOf(t, RoleRemetente), ie); err != nil {
		t.Fatal(err)
	}
	if _, err := checkOriginal(originalOf(t, RoleRemetente), "999"); code(err) != CodeEmitterIEChanged {
		t.Fatalf("IE trocada: %v", err)
	}
	o := originalOf(t, RoleRemetente)
	o.cte.TpCTe = "1"
	if _, err := checkOriginal(o, ie); code(err) != CodeOriginalTypeInvalid {
		t.Fatalf("original complementar não pode ser complementado: %v", err)
	}
	o.cte.TpCTe, o.cte.Modal = "0", "02"
	if _, err := checkOriginal(o, ie); code(err) != CodeOriginalUnsupported {
		t.Fatalf("modal: %v", err)
	}
}

func TestSameShape(t *testing.T) {
	a, b := originalOf(t, RoleRemetente), originalOf(t, RoleRemetente)
	if w := sameShape(a.cte, b.cte); w != "" {
		t.Fatal(w)
	}
	b.cte.Dest.CNPJ = cnpjA
	if sameShape(a.cte, b.cte) != "destinatário" {
		t.Fatal("destinatário diferente deveria ser apontado")
	}
}

func TestCheckKeyAge(t *testing.T) {
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	key := func(y, m int) string {
		return services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 1, time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC), services.TpEmisNormal)
	}
	if err := checkKeyAge(key(2025, 11), now); err != nil {
		t.Fatalf("exatamente 12 meses é aceito: %v", err)
	}
	if err := checkKeyAge(key(2025, 10), now); code(err) != CodeOriginalTooOld {
		t.Fatalf("13 meses: %v", err)
	}
}

func TestCheckEmissionForType(t *testing.T) {
	normal, off := emission{tpEmis: services.TpEmisNormal}, emission{tpEmis: tpEmisOffline, offline: true}
	svc := emission{tpEmis: tpEmisSVCRS, svc: "SVC-RS"}
	for _, c := range []struct {
		tp   string
		em   emission
		want string
	}{
		{"", svc, ""}, {tpCTeComplement, normal, ""}, {tpCTeComplement, off, ""}, {tpCTeComplement, svc, CodeContingencyNotAllowed},
		{tpCTeSubstitute, normal, ""}, {tpCTeSubstitute, off, CodeContingencyNotAllowed}, {tpCTeSubstitute, svc, CodeContingencyNotAllowed},
	} {
		if got := code(checkEmissionForType(c.tp, c.em)); got != c.want {
			t.Errorf("tp %q %+v: code = %q, want %q", c.tp, c.em, got, c.want)
		}
	}
}

func TestSubstitutionWindow(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *original {
		return &original{key: "K", item: map[string]types.AttributeValue{"dh_emi": &types.AttributeValueMemberS{Value: now.Add(-d).Format(layoutDateTimeTZ)}}}
	}
	if err := checkSubstitutionWindow(at(66*24*time.Hour), now); err != nil {
		t.Fatalf("dentro da folga da autorização: %v", err)
	}
	if err := checkSubstitutionWindow(at(68*24*time.Hour), now); code(err) != CodeSubstitutionWindowEnded {
		t.Fatalf("fora do prazo: %v", err)
	}
}

func TestCheckNewTaker(t *testing.T) {
	o := originalOf(t, RoleRemetente)
	_, old := takerOf(o.cte)
	same := actor{CNPJ: o.cte.Dest.CNPJ, Addr: old.Addr}
	if err := checkNewTaker(o.cte, same, old); err != nil {
		t.Fatalf("destinatário do original com a UF do tomador: %v", err)
	}
	other := same
	other.Addr.UF = "RJ"
	if code(checkNewTaker(o.cte, other, old)) != CodeTakerUFMismatch {
		t.Fatal("G100: UF diferente")
	}
	stranger := actor{CNPJ: "45723174000110", Addr: old.Addr}
	if code(checkNewTaker(o.cte, stranger, old)) != CodeTakerBaseNotInOriginal {
		t.Fatal("G099: CNPJ-base fora dos papéis")
	}
}
