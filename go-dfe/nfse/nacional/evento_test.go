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
