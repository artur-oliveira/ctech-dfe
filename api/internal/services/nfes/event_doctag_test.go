package nfes

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// A natural-person issuer (produtor rural, MEI pessoa física) must have its
// document emitted as CPF — infEvento is a choice CNPJ|CPF and SEFAZ rejects
// a CPF sent under the CNPJ element.
func TestEventBuildersUseIssuerDocTag(t *testing.T) {
	const (
		key = "35240611222333000181550010000000011000000017"
		doc = "12345678901"
	)
	if got := services.IssuerDocTag("CPF_" + doc); got != services.TagCPF {
		t.Fatalf("IssuerDocTag(CPF_) = %q, want CPF", got)
	}
	if got := services.IssuerDocTag("CNPJ_11222333000181"); got != services.TagCNPJ {
		t.Fatalf("IssuerDocTag(CNPJ_) = %q, want CNPJ", got)
	}

	bodies := map[string]map[string]any{
		"cancel":     buildCancelBody(key, services.TagCPF, doc, 2, "PROT1", "justificativa de teste", 1),
		"cce":        buildCCeBody(key, services.TagCPF, doc, 2, "correcao", 1),
		"manifest":   buildManifestBody(key, services.TagCPF, doc, 2, TpEventoCienciaOperacao, 1, nil),
		"substitute": buildSubstituteBody(key, services.TagCPF, doc, 2, "PROT1", key, "just", 1, "ver"),
	}
	for name, body := range bodies {
		infEvento := body["envEvento"].(map[string]any)["evento"].(map[string]any)["infEvento"].(map[string]any)
		if infEvento[services.TagCPF] != doc {
			t.Errorf("%s: CPF not set in infEvento: %#v", name, infEvento)
		}
		if _, ok := infEvento[services.TagCNPJ]; ok {
			t.Errorf("%s: CNPJ present alongside CPF (choice violation)", name)
		}
	}
}

// The prorrogação / cancel-event builders share buildDetEventoBody; the item
// list carries the item number as an XML attribute, not an element.
func TestBuildDetEventoBodyProrrogacao(t *testing.T) {
	const key = "35240611222333000181550010000000011000000017"
	ectx := &nfeEventContext{environment: 2, cnpj: "11222333000181", docTag: services.TagCNPJ}

	body := buildDetEventoBody(key, ectx, TpEventoProrrogacao1, 1, map[string]any{
		"descEvento": descProrrogacao,
		"nProt":      "135240000000001",
		"itemPedido": []map[string]any{{"@numItem": "1", "qtdeItem": "10.0000"}},
	})

	infEvento := body["envEvento"].(map[string]any)["evento"].(map[string]any)["infEvento"].(map[string]any)
	if infEvento["@Id"] != "ID"+TpEventoProrrogacao1+key+"01" {
		t.Errorf("@Id = %v", infEvento["@Id"])
	}
	if infEvento["tpEvento"] != TpEventoProrrogacao1 || infEvento["cOrgao"] != "35" {
		t.Errorf("unexpected infEvento: %#v", infEvento)
	}
	det := infEvento["detEvento"].(map[string]any)
	if det["@versao"] != eventVersao || det["@xmlns"] != nfeXMLNS {
		t.Errorf("detEvento missing namespace/version: %#v", det)
	}
	item := det["itemPedido"].([]map[string]any)[0]
	if item["@numItem"] != "1" || item["qtdeItem"] != "10.0000" {
		t.Errorf("unexpected itemPedido: %#v", item)
	}
}
