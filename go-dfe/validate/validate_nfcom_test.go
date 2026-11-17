package validate

import "testing"

func TestNFComSpecsLoad(t *testing.T) {
	for _, n := range []string{
		"nfcom",
		"evento_nfcom_evCancNFCom", "evento_nfcom_evVincPgto", "evento_nfcom_evCancVincPgto", "evento_nfcom_eventoNFCom",
	} {
		if _, err := Load(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}

// Um documento quase vazio deve acusar os grupos obrigatórios do leiaute e a marca de modelo errado.
func TestNFComSchemaFindings(t *testing.T) {
	s, err := Load("nfcom")
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"infNFCom": map[string]any{
			"@versao": "1.00",
			"@Id":     "NFCom123",
			"ide":     map[string]any{"cUF": "35", "mod": "99", "bogus": "x"},
		},
	}
	got := codes(s.Validate(doc, Options{Skip: []string{"Signature"}}))
	for _, want := range []string{
		"NFCom/infNFCom/@Id|pattern",
		"NFCom/infNFCom/ide/mod|enum",
		"NFCom/infNFCom/ide/bogus|unknown_tag",
		"NFCom/infNFCom/emit|required",
	} {
		if !got[want] {
			t.Errorf("faltou %s; got %v", want, got)
		}
	}
	if got["NFCom/Signature|required"] {
		t.Error("Signature deveria ser ignorada")
	}
}
