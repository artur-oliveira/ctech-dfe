package nfse

import (
	"encoding/json"
	"testing"
)

// A api monta o Document como JSON dentro de dfe.Request.Body["document"];
// DecodeDocument tem que reconstruir todos os grupos sem perda.
func TestDecodeDocument_RoundTrip(t *testing.T) {
	src := Document{
		Ambiente: 2, TpEmit: 1, Serie: "00001", Numero: 42,
		Competencia: "2026-08-01", CLocEmi: "2211001", VerAplic: "ctech-1.0",
		Prestador: Prestador{
			Pessoa:  Pessoa{CNPJ: "11222333000181", XNome: "Prestador Teste"},
			RegTrib: RegTrib{OpSimpNac: 1, RegEspTrib: 0},
		},
		Tomador: &Pessoa{CPF: "12345678909", XNome: "Tomador Teste"},
		Servico: Servico{
			LocPrest: LocPrest{CLocPrestacao: "2211001"},
			CServ:    CServ{CTribNac: "010101", XDescServ: "Análise de sistemas"},
		},
		Valores: Valores{VServPrest: VServPrest{VServ: "1000.00"},
			Trib: Tributacao{TribMun: TribMunicipal{TribISSQN: 1, TpRetISSQN: 1, PAliq: "2.00"}}},
	}

	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	got, err := DecodeDocument(body)
	if err != nil {
		t.Fatalf("DecodeDocument: %v", err)
	}
	if got.Prestador.Pessoa.CNPJ != src.Prestador.Pessoa.CNPJ {
		t.Errorf("CNPJ = %q, esperado %q", got.Prestador.Pessoa.CNPJ, src.Prestador.Pessoa.CNPJ)
	}
	if got.Tomador == nil || got.Tomador.CPF != "12345678909" {
		t.Errorf("tomador perdido no round-trip: %+v", got.Tomador)
	}
	if got.Servico.CServ.CTribNac != "010101" {
		t.Errorf("cTribNac = %q, esperado 010101", got.Servico.CServ.CTribNac)
	}
	if got.Valores.Trib.TribMun.PAliq != "2.00" {
		t.Errorf("pAliq = %q, esperado 2.00", got.Valores.Trib.TribMun.PAliq)
	}
}

func TestDecodeDocument_RejectsUnknownField(t *testing.T) {
	// Campo desconhecido é erro, não silêncio: um typo na api tem que
	// estourar aqui, não virar DPS incompleta aceita pelo fisco.
	_, err := DecodeDocument(map[string]any{"tp_emit": 1, "campo_inexistente": "x"})
	if err == nil {
		t.Fatal("esperado erro para campo desconhecido")
	}
}

func TestFieldNotSupportedError_Message(t *testing.T) {
	err := &FieldNotSupportedError{Provider: "abrasf204", Field: "IBSCBS"}
	want := `nfse: provider "abrasf204" não suporta o campo "IBSCBS"`
	if err.Error() != want {
		t.Errorf("Error() = %q, esperado %q", err.Error(), want)
	}
}
