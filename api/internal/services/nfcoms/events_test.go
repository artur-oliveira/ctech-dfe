package nfcoms

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

// O envelope de cancelamento que o service monta passa no schema do evento (NT 2026.005).
func TestCancelEnvelopeMatchesEventSchema(t *testing.T) {
	p := testParams(t)
	ec := &eventContext{environment: 2, cnpj: testCNPJ, emitUF: "SP"}
	body := buildEventEnvelope(ec, p.accessKey, TpEventoCancelamento, 1, cancelDet("1352600000000011", "Erro de digitacao no valor"))
	if found := dfecheck.EventFindings("nfcom", body); len(found) != 0 {
		t.Fatalf("achados: %+v", found)
	}
	inf := body["eventoNFCom"].(map[string]any)["infEvento"].(map[string]any)
	if inf["@Id"] != "ID110111"+p.accessKey+"001" || inf["cOrgao"] != "35" || inf["chNFCom"] != p.accessKey {
		t.Errorf("infEvento = %v", inf)
	}
}

func TestEmitUFFromAccessKey(t *testing.T) {
	if got := emitUFFromAccessKey("35261100000000000000620010000000101012345671"); got != "SP" {
		t.Errorf("UF = %q", got)
	}
	if got := emitUFFromAccessKey("3"); got != "" {
		t.Errorf("chave curta devolveu %q", got)
	}
}

func TestOfflineResolution(t *testing.T) {
	if tp, off := resolveNFComEmission(nil); tp != tpEmisNormal || off {
		t.Errorf("sem configuração: %s %v", tp, off)
	}
}
