package nfes

import (
	"testing"
	"time"
)

const epecKey = "35240611222333000181550010000000014000000017" // dígito 35 = 4

func epecOK() EPECSummary {
	return EPECSummary{AccessKey: epecKey, IssuedAt: time.Now(), TpNF: "1", IE: "111111111111", DestUF: "SP",
		DestTag: "CNPJ", DestDoc: "11222333000181", VNF: "10.00", VICMS: "1.80", VST: "0.00", VerAplic: "ctech-1"}
}

func TestEPECValidate(t *testing.T) {
	if err := epecOK().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := epecOK()
	bad.AccessKey = "35240611222333000181550010000000011000000017" // tpEmis 1
	if bad.Validate() == nil {
		t.Fatal("chave normal deve ser recusada")
	}
	bad = epecOK()
	bad.VST = ""
	if bad.Validate() == nil {
		t.Fatal("resumo incompleto deve ser recusado")
	}
}

func TestBuildEPECBodyShape(t *testing.T) {
	body := buildEPECBody("CNPJ", "11222333000181", 2, epecOK(), 1)
	inf := body["envEvento"].(map[string]any)["evento"].(map[string]any)["infEvento"].(map[string]any)
	if inf["cOrgao"] != "91" || inf["tpEvento"] != TpEventoEPEC {
		t.Fatalf("%v", inf)
	}
	det := inf["detEvento"].(map[string]any)
	dest := det["dest"].(map[string]any)
	if dest["vNF"] != "10.00" || dest["vST"] != "0.00" || det["vNF"] != "10.00" || det["vICMS"] != "1.80" {
		t.Fatalf("e110140 (010e): vNF, vICMS e vST dentro de dest; vNF e vICMS também em detEvento: %v", det)
	}
	if _, ok := det["vST"]; ok {
		t.Fatalf("vST só existe dentro de dest: %v", det)
	}
	if det["tpAutor"] != "1" || dest["CNPJ"] != "11222333000181" {
		t.Fatalf("%v", det)
	}
	if got := EPECDeadline(time.Unix(0, 0)); got.Sub(time.Unix(0, 0)) != 168*time.Hour {
		t.Fatal("prazo")
	}
}
