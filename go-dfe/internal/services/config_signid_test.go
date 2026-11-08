package services

import "testing"

// A assinatura do CT-e OS cobre o infCte (o Id "CTe"+chave está em infCte também no cteOS_v4.00.xsd); o
// infCTeOS que a config trazia antes não existe no esquema.
func TestCTeOSSignsInfCte(t *testing.T) {
	if got := CTeConfig.SignIDXPath["CTeRecepcaoOS"]; got != "infCte" {
		t.Fatalf("SignIDXPath[CTeRecepcaoOS] = %q, want infCte", got)
	}
}
