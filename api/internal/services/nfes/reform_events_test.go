package nfes

import (
	"strings"
	"testing"
)

func TestReformEventDescLengthsMatchMOC(t *testing.T) {
	// "Tam." column of the MOC (8.x P19): the literal descEvento length.
	want := map[string]int{
		TpEventoPagamentoIntegral: 85, TpEventoNaoFornecido: 51, TpEventoPrevEntrega: 42, TpEventoCredPresumido: 47,
		TpEventoPerecimentoAdq: 82, TpEventoCredCombustivel: 52, TpEventoCredAdquirente: 98,
		TpEventoSucessaoIBS: 85, TpEventoSucessaoCBS: 82,
	}
	for code, n := range want {
		if got := len([]rune(reformEvents[code].desc)); got != n {
			t.Errorf("%s: descEvento tem %d caracteres, o MOC diz %d", code, got, n)
		}
	}
	if len([]rune(descVinculoPagamento)) != 20 {
		t.Error("110300 descEvento deve ter 20 caracteres")
	}
}

func TestStockItemsBuildsGroup(t *testing.T) {
	r := ReformEventRequest{Items: []ReformItem{{ItemNumber: 1, IBS: "1.00", CBS: "2.00", Quantity: "3", Unit: "UN"}}}
	det, err := reformEvents[TpEventoCredCombustivel].build(r)
	if err != nil {
		t.Fatal(err)
	}
	g := det["gConsumoComb"].([]map[string]any)[0]
	est := g["gControleEstoque"].(map[string]any)
	if g["@nItem"] != "1" || est["qComb"] != "3" || est["uComb"] != "UN" {
		t.Fatalf("%v", g)
	}
	r.Items = append(r.Items, r.Items[0]) // nItem duplicado
	if _, err := reformEvents[TpEventoCredCombustivel].build(r); err == nil || !strings.Contains(err.Error(), "repetido") {
		t.Fatalf("item repetido deve ser recusado: %v", err)
	}
	r.Items = []ReformItem{{ItemNumber: 1, IBS: "1", CBS: "2.00", Quantity: "3", Unit: "UN"}}
	if _, err := reformEvents[TpEventoCredCombustivel].build(r); err == nil {
		t.Fatal("ibs sem 2 casas deve ser recusado")
	}
}

func TestIndicatorQuitacaoMustBeOne(t *testing.T) {
	zero, one := 0, 1
	if _, err := reformEvents[TpEventoPagamentoIntegral].build(ReformEventRequest{Indicator: &zero}); err == nil {
		t.Fatal("indQuitacao 0 deve ser recusado")
	}
	det, err := reformEvents[TpEventoPagamentoIntegral].build(ReformEventRequest{Indicator: &one})
	if err != nil || det["indQuitacao"] != "1" {
		t.Fatalf("%v %v", det, err)
	}
	if det, err := reformEvents[TpEventoAceiteDebito].build(ReformEventRequest{Indicator: &zero}); err != nil || det["indAceitacao"] != "0" {
		t.Fatalf("indAceitacao 0 e valido: %v %v", det, err)
	}
}

func TestPaymentLinkDet(t *testing.T) {
	p := &PaymentLink{TransactionID: "abc123", PaymentMethod: "17", ReceiverCNPJ: "11222333000181", PSPBaseCNPJ: "11222333"}
	det, err := paymentLinkDet(p, "135240000000001")
	if err != nil || det["nProt"] != "135240000000001" || det["gPgto"].(map[string]any)["@idTransacao"] != "abc123" {
		t.Fatalf("%v %v", det, err)
	}
	p.PSPBaseCNPJ = "123"
	if _, err := paymentLinkDet(p, "1"); err == nil {
		t.Fatal("cnpj base invalido")
	}
}

func TestCredPresumidoGroups(t *testing.T) {
	r := ReformEventRequest{Items: []ReformItem{{
		ItemNumber: 2, BaseValue: "100.00", PresumedCode: "01",
		IBSPresumed: &PresumedCredit{Code: "01", Percent: "10.00", Value: "10.00"},
	}}}
	det, err := reformEvents[TpEventoCredPresumido].build(r)
	if err != nil {
		t.Fatal(err)
	}
	g := det["gCredPresOper"].([]map[string]any)[0]
	if _, ok := g["gCBSCredPres"]; ok || g["gIBSCredPres"] == nil {
		t.Fatalf("%v", g)
	}
}

func TestAuthorUFCodeAndKeyUF(t *testing.T) {
	const key = "35240611222333000181550010000000014000000017"
	if authorUFCode(key, "RJ", TpAutorEmitente) != "35" || authorUFCode(key, "RJ", TpAutorDestino) != "33" {
		t.Fatal("cOrgaoAutor")
	}
	if ufOfKey(key) != "SP" {
		t.Fatal("ufOfKey")
	}
}

func TestDeliveryEventsGoToAN(t *testing.T) {
	body := withEventOrgao(buildDetEventoBody(epecKey, &nfeEventContext{docTag: "CNPJ", cnpj: "1", environment: 2}, TpEventoComprovanteEntrega, 1, map[string]any{}), cOrgaoAN)
	inf := body["envEvento"].(map[string]any)["evento"].(map[string]any)["infEvento"].(map[string]any)
	if inf["cOrgao"] != "91" {
		t.Fatal(inf)
	}
}
