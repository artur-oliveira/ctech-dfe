package nacional

import (
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func baseEvent(tipo string) nfse.EventRequest {
	return nfse.EventRequest{
		ChaveAcesso: strings.Repeat("1", 50), TipoEvento: tipo, NSeqEvento: 1,
		TpAmb: 2, VerAplic: "ctech-1.0", CNPJAutor: "11222333000181",
		DhEvento: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
	}
}

func TestBuildPedRegEvento_Cancelamento(t *testing.T) {
	ev := baseEvent(nfse.EventCancelamento)
	ev.Motivo = &nfse.EventMotivo{Codigo: "1", Descricao: "Erro na emissão"}
	out, id, err := BuildPedRegEvento(ev)
	if err != nil {
		t.Fatalf("BuildPedRegEvento: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<e101101>") {
		t.Error("elemento e101101 ausente")
	}
	if !strings.Contains(s, "<cMotivo>1</cMotivo>") {
		t.Error("cMotivo ausente")
	}
	if !strings.Contains(s, `Id="`+id+`"`) {
		t.Errorf("infPedReg sem Id=%q", id)
	}
	if !strings.HasPrefix(id, "PRE") || len(id) != 3+50+6+3 {
		t.Errorf("Id malformado: %q (len %d)", id, len(id))
	}
}

func TestBuildPedRegEvento_ConfirmacaoTomadorSemCorpo(t *testing.T) {
	out, _, err := BuildPedRegEvento(baseEvent(nfse.EventConfirmacaoTomador))
	if err != nil {
		t.Fatalf("BuildPedRegEvento: %v", err)
	}
	if !strings.Contains(string(out), "<e203202>") {
		t.Error("elemento e203202 ausente")
	}
}

func TestBuildPedRegEvento_Substituicao(t *testing.T) {
	ev := baseEvent(nfse.EventCancelamentoPorSubst)
	ev.Motivo = &nfse.EventMotivo{Codigo: "1"}
	ev.ChSubstituta = strings.Repeat("2", 50)
	out, _, err := BuildPedRegEvento(ev)
	if err != nil {
		t.Fatalf("BuildPedRegEvento: %v", err)
	}
	if !strings.Contains(string(out), "<chSubstituta>"+ev.ChSubstituta+"</chSubstituta>") {
		t.Error("chSubstituta ausente")
	}
	if !strings.Contains(string(out), "<xDesc>Cancelamento de NFS-e por Substituição</xDesc>") {
		t.Error("xDesc (TE105102, valor fixo do XSD) ausente")
	}
}

func TestBuildPedRegEvento_RejectsFiscoOnlyEvent(t *testing.T) {
	// 305101 é privativo do município/fisco — só chega pela distribuição.
	if _, _, err := BuildPedRegEvento(baseEvent("305101")); err == nil {
		t.Fatal("esperado erro ao tentar emitir evento privativo do fisco")
	}
}

func TestBuildPedRegEvento_RequiresMotivoWhenTypeDemandsIt(t *testing.T) {
	if _, _, err := BuildPedRegEvento(baseEvent(nfse.EventCancelamento)); err == nil {
		t.Fatal("esperado erro: cancelamento exige motivo")
	}
}

func TestBuildPedRegEvento_RequiresChSubstitutaForSubstituicao(t *testing.T) {
	ev := baseEvent(nfse.EventCancelamentoPorSubst)
	ev.Motivo = &nfse.EventMotivo{Codigo: "1"}
	if _, _, err := BuildPedRegEvento(ev); err == nil {
		t.Fatal("esperado erro: cancelamento por substituição exige chSubstituta")
	}
}

func TestBuildPedRegEvento_AnulacaoRejeicao(t *testing.T) {
	ev := baseEvent(nfse.EventAnulacaoRejeicao)
	ev.CPFAgTrib = "12345678909"
	ev.IDEvManifRej = "PRE" + strings.Repeat("1", 50) + "203206" + "001"
	ev.Motivo = &nfse.EventMotivo{Descricao: "Manifestação equivocada"}
	out, id, err := BuildPedRegEvento(ev)
	if err != nil {
		t.Fatalf("BuildPedRegEvento: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<e205208>") {
		t.Error("elemento e205208 ausente")
	}
	if !strings.Contains(s, "<CPFAgTrib>"+ev.CPFAgTrib+"</CPFAgTrib>") {
		t.Error("CPFAgTrib ausente")
	}
	if !strings.Contains(s, "<idEvManifRej>"+ev.IDEvManifRej+"</idEvManifRej>") {
		t.Error("idEvManifRej ausente")
	}
	if !strings.Contains(s, "<xMotivo>"+ev.Motivo.Descricao+"</xMotivo>") {
		t.Error("xMotivo ausente")
	}
	if !strings.Contains(s, "<xDesc>Manifestação de NFS-e - Anulação da Rejeição</xDesc>") {
		t.Error("xDesc (TE205208, valor fixo do XSD) ausente")
	}
	if !strings.Contains(s, `Id="`+id+`"`) {
		t.Errorf("infPedReg sem Id=%q", id)
	}
}

func TestBuildPedRegEvento_AnulacaoRejeicaoRequiresAllFields(t *testing.T) {
	if _, _, err := BuildPedRegEvento(baseEvent(nfse.EventAnulacaoRejeicao)); err == nil {
		t.Fatal("esperado erro: anulação de rejeição exige CPFAgTrib, idEvManifRej e xMotivo")
	}
}

// TestBuildPedRegEvento_RemainingEventTypes cobre os tipos que ainda não
// tinham nenhum teste dedicado (regression test para um copy/paste errado
// no switch de evento.go trocar o elemento de um tipo pelo de outro).
func TestBuildPedRegEvento_RemainingEventTypes(t *testing.T) {
	cases := []struct {
		tipo, elemento string
		needsMotivo    bool
	}{
		{nfse.EventSolicAnaliseFiscalCanc, "e101103", true},
		{nfse.EventConfirmacaoPrestador, "e202201", false},
		{nfse.EventConfirmacaoIntermediario, "e204203", false},
		{nfse.EventRejeicaoPrestador, "e202205", true},
		{nfse.EventRejeicaoTomador, "e203206", true},
		{nfse.EventRejeicaoIntermediario, "e204207", true},
	}
	for _, c := range cases {
		t.Run(c.tipo, func(t *testing.T) {
			ev := baseEvent(c.tipo)
			if c.needsMotivo {
				ev.Motivo = &nfse.EventMotivo{Codigo: "1"}
			}
			out, _, err := BuildPedRegEvento(ev)
			if err != nil {
				t.Fatalf("BuildPedRegEvento: %v", err)
			}
			if !strings.Contains(string(out), "<"+c.elemento+">") {
				t.Errorf("elemento %s ausente: %s", c.elemento, out)
			}
		})
	}
}
