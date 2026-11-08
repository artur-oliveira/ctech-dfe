package main

import (
	"context"
	"encoding/json"
	"testing"

	dfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/ciot"
	"gopkg.aoctech.app/dfe/go-dfe/ciot/ciottest"
)

// Contract test of the ciot doc_type through the real handler and dfe.Call, against the fake ANTT only.
func TestHandlerRoutesCIOTWithLongEnvironment(t *testing.T) {
	srv := ciottest.NewServer(t)
	defer ciot.SetTestHosts(map[string]string{ciot.EnvHom: srv.URL, ciot.EnvProd: srv.URL})()
	h := newHandler(dfe.Call)
	resp, err := h(context.Background(), dfe.Request{
		DocType: "ciot", Service: ciot.ServiceDeclaracao, Environment: "homologacao",
		CertificateB64: ciottest.PFX(t, ciottest.CarrierCNPJ, true), CertificatePassword: ciottest.PFXPassword,
		Body: map[string]any{"IdOperacaoTransporte": ciottest.GeneratedID},
	})
	var res ciot.Result
	if err != nil || resp.StatusCode != 200 || json.Unmarshal([]byte(resp.Body), &res) != nil ||
		res.CIOT != ciottest.GeneratedID+ciottest.Verifier {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if srv.ClientCertSeen == 0 {
		t.Error("mTLS client certificate not presented")
	}
}
