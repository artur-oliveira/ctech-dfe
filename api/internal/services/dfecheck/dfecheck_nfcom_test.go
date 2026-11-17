package dfecheck

import (
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// GateNFCom lê o <NFCom> do corpo do builder; sem ele (corpo vazio) não recusa nada.
func TestGateNFCom(t *testing.T) {
	if got := GateNFCom(map[string]any{}, nil); got != nil {
		t.Fatalf("corpo sem NFCom: %v", got)
	}
	// Em block, um Id fora do padrão do XSD recusa; em shadow (padrão) só tamanho recusa.
	t.Setenv("DFE_GATE_SCHEMA_NFCOM", "block")
	body := map[string]any{"NFCom": map[string]any{"infNFCom": map[string]any{"@versao": "1.00", "@Id": "x"}}}
	found := GateNFCom(body, nil)
	seen := false
	for _, v := range found {
		seen = seen || (v.Path == "NFCom/infNFCom/@Id" && v.Code == validate.CodePattern)
	}
	if !seen {
		t.Fatalf("faltou o achado de padrão do Id: %v", found)
	}
}

func TestGateNFComDefaultsToShadow(t *testing.T) {
	if SchemaMode(KindNFCom) != validate.ModeShadow || SchemaMode(KindNFComEvent) != validate.ModeShadow {
		t.Fatal("NFCom entra em shadow no primeiro rollout, como os demais documentos")
	}
}

func TestEventFindingsNFCom(t *testing.T) {
	good := map[string]any{"eventoNFCom": map[string]any{
		"@versao": "1.00",
		"infEvento": map[string]any{
			"@Id": "ID1101113526111122233300018162001000000010101234567100", "cOrgao": "35", "tpAmb": "2",
			"CNPJ": "11222333000181", "chNFCom": "35261111222333000181620010000000101012345671",
			"dhEvento": "2026-11-05T10:00:00-03:00", "tpEvento": "110111", "nSeqEvento": "1",
			"detEvento": map[string]any{"@versaoEvento": "1.00", "evCancNFCom": map[string]any{
				"descEvento": "Cancelamento", "nProt": "1352600000000011", "xJust": "Erro de digitacao no valor",
			}},
		},
	}}
	if got := EventFindings("nfcom", good); len(got) != 0 {
		t.Fatalf("evento válido com achados: %v", got)
	}
	bad := map[string]any{"eventoNFCom": map[string]any{
		"@versao": "1.00",
		"infEvento": map[string]any{
			"detEvento": map[string]any{"evCancNFCom": map[string]any{"descEvento": "Cancelamento", "nProt": "1", "xJust": ""}},
		},
	}}
	if got := EventFindings("nfcom", bad); len(got) == 0 {
		t.Fatal("evento de cancelamento sem justificativa deveria ter achados")
	}
}
