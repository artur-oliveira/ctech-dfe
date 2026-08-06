package nfses

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"
)

const chaveTeste = "11111111111111111111111111111111111111111111111111"

func TestValidateEventType_RejectsFiscoOnly(t *testing.T) {
	for _, tipo := range []string{"105104", "105105", "205204", "305101", "305102", "305103"} {
		if err := validateEventType(tipo); err == nil {
			t.Errorf("evento %s privativo do fisco foi aceito", tipo)
		}
	}
}

func TestValidateEventType_AcceptsContribuinte(t *testing.T) {
	for tipo := range nfse.ContribuinteEvents {
		if err := validateEventType(tipo); err != nil {
			t.Errorf("evento %s do contribuinte foi rejeitado: %v", tipo, err)
		}
	}
}

func TestBuildEventRequest_CancelamentoExigeMotivo(t *testing.T) {
	_, err := buildEventRequest(chaveTeste,
		NfseEventBody{EventType: nfse.EventCancelamento}, "11222333000181", 2)
	if err == nil || !strings.Contains(err.Error(), "reason_code") {
		t.Fatalf("esperado erro de motivo obrigatório, veio: %v", err)
	}
}

func TestBuildEventRequest_CancelamentoExigeDescricao(t *testing.T) {
	_, err := buildEventRequest(chaveTeste,
		NfseEventBody{EventType: nfse.EventCancelamento, ReasonCode: "1"}, "11222333000181", 2)
	if err == nil || !strings.Contains(err.Error(), "reason_description") {
		t.Fatalf("esperado erro de descrição obrigatória, veio: %v", err)
	}
}

func TestBuildEventRequest_SubstituicaoNaoEhEvento(t *testing.T) {
	// 105102 é gerado pelo fisco a partir de uma nova DPS com grupo subst.
	// A api nunca deve enfileirá-lo como pedido de registro de evento.
	_, err := buildEventRequest(chaveTeste,
		NfseEventBody{EventType: nfse.EventCancelamentoPorSubst}, "11222333000181", 2)
	if err == nil || !strings.Contains(err.Error(), "substitute") {
		t.Fatalf("esperado erro direcionando para /substitute, veio: %v", err)
	}
}

func TestBuildEventRequest_DefaultsSequenceToOne(t *testing.T) {
	ev, err := buildEventRequest(chaveTeste, NfseEventBody{
		EventType: nfse.EventConfirmacaoTomador,
	}, "11222333000181", 2)
	if err != nil {
		t.Fatalf("buildEventRequest: %v", err)
	}
	if ev.NSeqEvento != 1 {
		t.Errorf("NSeqEvento = %d, esperado 1", ev.NSeqEvento)
	}
	if ev.CNPJAutor != "11222333000181" {
		t.Errorf("CNPJAutor = %q", ev.CNPJAutor)
	}
	if ev.VerAplic != appVersion {
		t.Errorf("VerAplic = %q, esperado %q", ev.VerAplic, appVersion)
	}
}

func TestBuildEventRequest_CPFAutorQuandoPrestadorPF(t *testing.T) {
	ev, err := buildEventRequest(chaveTeste, NfseEventBody{
		EventType: nfse.EventConfirmacaoPrestador,
	}, "12345678909", 2)
	if err != nil {
		t.Fatalf("buildEventRequest: %v", err)
	}
	// TCInfPedReg exige escolha única entre CNPJ e CPF do autor.
	if ev.CPFAutor != "12345678909" || ev.CNPJAutor != "" {
		t.Errorf("autor PF errado: CPF=%q CNPJ=%q", ev.CPFAutor, ev.CNPJAutor)
	}
}

// O pedido montado pela api tem que atravessar a serialização do go-dfe: se um
// campo obrigatório do leiaute faltar, o erro aparece aqui e não no worker.
func TestBuildEventRequest_SerializaNoGoDfe(t *testing.T) {
	ev, err := buildEventRequest(chaveTeste, NfseEventBody{
		EventType:         nfse.EventCancelamento,
		ReasonCode:        "1",
		ReasonDescription: "erro na emissão",
	}, "11222333000181", 2)
	if err != nil {
		t.Fatalf("buildEventRequest: %v", err)
	}
	if _, _, err := nacional.BuildPedRegEvento(ev); err != nil {
		t.Fatalf("BuildPedRegEvento rejeitou o pedido da api: %v", err)
	}
}

// O Body do comando do worker é decodificado por nfse.Dispatch com política de
// campo desconhecido: um drift de chave entre api e go-dfe falha aqui.
func TestBuildEventBody_DecodesBackInGoDfe(t *testing.T) {
	ev, err := buildEventRequest(chaveTeste, NfseEventBody{
		EventType:         nfse.EventCancelamento,
		ReasonCode:        "1",
		ReasonDescription: "erro na emissão",
	}, "11222333000181", 2)
	if err != nil {
		t.Fatalf("buildEventRequest: %v", err)
	}
	body, err := buildEventWorkerBody(nfse.ProviderNacional, ev)
	if err != nil {
		t.Fatalf("buildEventWorkerBody: %v", err)
	}
	sub, ok := body[nfse.BodyKeyEvent].(map[string]any)
	if !ok {
		t.Fatalf("body[%q] não é um mapa: %T", nfse.BodyKeyEvent, body[nfse.BodyKeyEvent])
	}
	decoded, err := nfse.DecodeEventRequest(sub)
	if err != nil {
		t.Fatalf("DecodeEventRequest: %v", err)
	}
	if decoded.TipoEvento != ev.TipoEvento || decoded.ChaveAcesso != ev.ChaveAcesso {
		t.Errorf("round-trip perdeu campos: %+v", decoded)
	}
	if decoded.Motivo == nil || decoded.Motivo.Codigo != "1" {
		t.Errorf("motivo não sobreviveu ao round-trip: %+v", decoded.Motivo)
	}
}
