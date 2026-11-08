package dfe

import "testing"

func TestCiotPayloadDropsControlKeys(t *testing.T) {
	got := ciotPayload(map[string]any{"carrier_cpf_cnpj": "x", "payment_institution": true, "generator_api_key": "k",
		"generator_fallback_url": "u", "CpfCnpjTransportador": "1"})
	if len(got) != 1 || got["CpfCnpjTransportador"] != "1" {
		t.Fatalf("payload = %v", got)
	}
}
