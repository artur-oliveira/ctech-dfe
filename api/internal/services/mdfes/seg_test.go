package mdfes

import "testing"

func TestBuildSegComResponsavelESeguradora(t *testing.T) {
	got := buildSeg([]resolvedPolicy{{
		RespSeg: "1", CNPJ: "11111111111111", XSeg: "Seguradora X", CNPJSeg: "22222222222222",
		NApol: "AP-1", NAver: []string{"AV-1", "AV-2"},
	}})
	s := got[0]
	if s["infResp"].(map[string]any)["respSeg"] != "1" {
		t.Fatalf("infResp errado: %v", s)
	}
	if s["infSeg"].(map[string]any)["xSeg"] != "Seguradora X" {
		t.Fatalf("infSeg errado: %v", s)
	}
	if len(s["nAver"].([]string)) != 2 {
		t.Fatalf("averbações ausentes: %v", s)
	}
}

// Emitente responsável não se identifica: o XSD só aceita CNPJ/CPF em infResp
// quando o responsável não é o emitente do MDF-e.
func TestBuildSegSemDocumentoNemSeguradora(t *testing.T) {
	s := buildSeg([]resolvedPolicy{{RespSeg: "1", NApol: "AP-9"}})[0]
	resp := s["infResp"].(map[string]any)
	if _, ok := resp["CNPJ"]; ok {
		t.Fatalf("CNPJ não deveria estar presente: %v", resp)
	}
	if _, ok := s["infSeg"]; ok {
		t.Fatalf("infSeg não deveria estar presente: %v", s)
	}
	if s["nApol"] != "AP-9" {
		t.Fatalf("nApol errado: %v", s)
	}
}

func TestBuildSegVazio(t *testing.T) {
	if buildSeg(nil) != nil {
		t.Fatal("sem apólice, seg não deve existir")
	}
}
