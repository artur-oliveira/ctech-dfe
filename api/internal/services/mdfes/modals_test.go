package mdfes

import "testing"

// TestValidateModalPayload cobre o contrato de cada modal habilitado: escolher
// o modal sem mandar os dados dele é 400 com mensagem explícita, não um nó
// vazio que a SEFAZ rejeita depois.
func TestValidateModalPayload(t *testing.T) {
	if err := validateModalPayload(ModalAereo, MdfeEmitBody{}); err == nil {
		t.Fatal("modal aéreo sem payload deveria falhar")
	}
	if err := validateModalPayload(ModalFerroviario, MdfeEmitBody{}); err == nil {
		t.Fatal("modal ferroviário sem payload deveria falhar")
	}
	if err := validateModalPayload(ModalAereo, MdfeEmitBody{Air: &MdfeAirModal{}}); err != nil {
		t.Fatalf("modal aéreo com payload não deveria falhar: %v", err)
	}
	// Rodoviário monta o modal do cadastro de veículos: não há payload a exigir.
	if err := validateModalPayload(ModalRodoviario, MdfeEmitBody{}); err != nil {
		t.Fatalf("rodoviário não exige payload de modal: %v", err)
	}
}

func TestEnabledModals(t *testing.T) {
	for _, modal := range []string{ModalRodoviario, ModalAereo, ModalFerroviario} {
		if !enabledModals[modal] {
			t.Errorf("modal %s deveria estar habilitado", modal)
		}
	}
	// O aquaviário só entra quando buildAquav cobrir infEmbComb, as unidades
	// vazias e o MMSI (Task 36).
	if enabledModals[ModalAquaviario] {
		t.Error("modal aquaviário ainda não está completo")
	}
}

// TestBuildAereo confirma os seis campos obrigatórios do mdfeModalAereo.
func TestBuildAereo(t *testing.T) {
	got := buildAereo(&MdfeAirModal{
		Nac: "PP", Matr: "ABC123", NVoo: "JJ1234",
		CAerEmb: "GRU", CAerDes: "SDU", DVoo: "2026-09-01",
	})
	for k, want := range map[string]string{
		"nac": "PP", "matr": "ABC123", "nVoo": "JJ1234",
		"cAerEmb": "GRU", "cAerDes": "SDU", "dVoo": "2026-09-01",
	} {
		if got[k] != want {
			t.Errorf("%s = %v, want %s", k, got[k], want)
		}
	}
}

// TestBuildFerrov confirma que qVag é derivado da lista de vagões — o emissor
// nunca digita uma contagem que pode divergir dela.
func TestBuildFerrov(t *testing.T) {
	got := buildFerrov(&MdfeRailModal{
		XPref: "TR1", XOri: "PORTO", XDest: "TERMINAL",
		Wagons: []MdfeRailWagon{
			{PesoBC: "1000.000", PesoR: "1100.000", Serie: "A", NVag: "1", TU: "10.000"},
			{PesoBC: "2000.000", PesoR: "2100.000", Serie: "B", NVag: "2", TU: "20.000", TpVag: "GDT", NSeq: "02"},
		},
	})
	trem := got["trem"].(map[string]any)
	if trem["qVag"] != "2" {
		t.Fatalf("qVag = %v, want 2", trem["qVag"])
	}
	if _, has := trem["dhTrem"]; has {
		t.Errorf("dhTrem ausente no request não deve sair no XML: %v", trem)
	}
	vags := got["vag"].([]map[string]any)
	if len(vags) != 2 || vags[1]["tpVag"] != "GDT" || vags[1]["nSeq"] != "02" {
		t.Fatalf("vag = %v", vags)
	}
	if _, has := vags[0]["tpVag"]; has {
		t.Errorf("tpVag opcional não informado não deve sair: %v", vags[0])
	}
}

// TestBuildInfModal_DespachaPorModal garante que o dispatch entrega o nó certo
// e não cai no rodoviário por engano.
func TestBuildInfModal_DespachaPorModal(t *testing.T) {
	p := baseParams(nil)
	p.modal = ModalAereo
	p.air = &MdfeAirModal{Nac: "PP", Matr: "ABC123", NVoo: "1", CAerEmb: "GRU", CAerDes: "SDU", DVoo: "2026-09-01"}
	infModal := p.buildInfModal()
	if _, has := infModal["aereo"]; !has {
		t.Fatalf("infModal sem aereo: %v", infModal)
	}
	if _, has := infModal["rodo"]; has {
		t.Fatalf("infModal aéreo não deve trazer rodo: %v", infModal)
	}

	p.modal = ModalFerroviario
	p.rail = &MdfeRailModal{XPref: "TR1", XOri: "A", XDest: "B", Wagons: []MdfeRailWagon{{PesoBC: "1", PesoR: "1", Serie: "A", NVag: "1", TU: "1"}}}
	if _, has := p.buildInfModal()["ferrov"]; !has {
		t.Fatal("infModal sem ferrov")
	}
}
