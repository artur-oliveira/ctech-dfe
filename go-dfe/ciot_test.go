package dfe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/ciot"
	"gopkg.aoctech.app/dfe/go-dfe/ciot/ciottest"
	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
)

func ciotReq(t *testing.T, srv *ciottest.Server, pfx, env, service string, body map[string]any) Request {
	t.Helper()
	return Request{
		DocType: constants.DocTypeCIOT, Service: service, Environment: env,
		CertificateB64: pfx, CertificatePassword: ciottest.PFXPassword, Body: body,
	}
}

func TestImplementsEveryCIOTService(t *testing.T) {
	for _, s := range ciot.Services {
		if !Implements(constants.DocTypeCIOT, s) {
			t.Errorf("ciot/%s missing from the allowlist", s)
		}
	}
	if Implements(constants.DocTypeCIOT, "Other") || len(ciot.Services) != 9 {
		t.Error("allowlist must hold exactly the 8 services plus the generator")
	}
}

func TestCIOTContractThroughCall(t *testing.T) {
	srv := ciottest.NewServer(t)
	defer ciot.SetTestHosts(map[string]string{ciot.EnvHom: srv.URL, ciot.EnvProd: srv.URL})()
	pfx := ciottest.PFX(t, ciottest.CarrierCNPJ, true)
	ciot.ResetTokenCache()

	// generator -> declaration -> success carries the 16-character CIOT
	resp, err := Call(context.Background(), ciotReq(t, srv, pfx, ciot.EnvHom, ciot.ServiceGerarIdentificador, map[string]any{
		"cpf_cnpj": "52998224725", "generator_api_key": ciottest.GeneratorKey, "carrier_cpf_cnpj": ciottest.CarrierCNPJ,
	}))
	var gen map[string]string
	if err != nil || resp.StatusCode != 200 || json.Unmarshal([]byte(resp.Body), &gen) != nil || gen["id"] != ciottest.GeneratedID {
		t.Fatalf("generator: %v %+v", err, resp)
	}
	resp, _ = Call(context.Background(), ciotReq(t, srv, pfx, ciot.EnvHom, ciot.ServiceDeclaracao, map[string]any{
		"IdOperacaoTransporte": gen["id"], "RNTRCContratado": "12345678",
	}))
	var res ciot.Result
	_ = json.Unmarshal([]byte(resp.Body), &res)
	if resp.StatusCode != 200 || res.CIOT != ciottest.GeneratedID+ciottest.Verifier || res.AvisoTransportador == "" {
		t.Fatalf("declaration: %+v", resp)
	}

	// each remaining service answers success through the same path
	for _, s := range []string{ciot.ServiceSituacaoTransportador, ciot.ServiceFrotaTransportador, ciot.ServiceCancelamento,
		ciot.ServiceRetificacao, ciot.ServiceEncerramento, ciot.ServiceExcecao, ciot.ServiceCIOTGerado} {
		resp, err = Call(context.Background(), ciotReq(t, srv, pfx, ciot.EnvHom, s, map[string]any{"CPFCNPJTransportador": "x"}))
		if err != nil || resp.StatusCode != 200 {
			t.Errorf("%s: %v %+v", s, err, resp)
		}
	}
	if srv.ClientCertSeen != len(srv.Paths) || srv.ClientCertSeen == 0 {
		t.Errorf("mTLS: %d of %d requests carried the client certificate", srv.ClientCertSeen, len(srv.Paths))
	}
	var sawGet bool
	for _, p := range srv.Paths {
		sawGet = sawGet || p == "/api/ConsultarExcecao"
	}
	if !sawGet {
		t.Error("ConsultarExcecao not reached")
	}

	// business rejections come back as 422 with the stable internal code and the ANTT reference
	for id, want := range map[string]string{ciottest.DuplicateID: "CIOT_STATE_219", ciottest.RejectFloorID: "CIOT_FLOOR_291"} {
		resp, _ = Call(context.Background(), ciotReq(t, srv, pfx, ciot.EnvHom, ciot.ServiceDeclaracao, map[string]any{"IdOperacaoTransporte": id}))
		var p ciotProblem
		_ = json.Unmarshal([]byte(resp.Body), &p)
		if resp.StatusCode != 422 || p.InternalCode != want || p.ANTTCode == 0 || p.Protocol == "" {
			t.Errorf("%s: %+v", id, resp)
		}
	}
}

func TestCIOTConfigurationFailures(t *testing.T) {
	srv := ciottest.NewServer(t)
	defer ciot.SetTestHosts(map[string]string{ciot.EnvHom: srv.URL})()
	good := ciottest.PFX(t, ciottest.CarrierCNPJ, true)
	call := func(pfx, env string, body map[string]any) Response {
		r, err := Call(context.Background(), ciotReq(t, srv, pfx, env, ciot.ServiceSituacaoTransportador, body))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	for name, resp := range map[string]Response{
		"no OID":          call(ciottest.PFX(t, "", true), ciot.EnvHom, nil),
		"no client EKU":   call(ciottest.PFX(t, ciottest.CarrierCNPJ, false), ciot.EnvHom, nil),
		"not the carrier": call(good, ciot.EnvHom, map[string]any{"carrier_cpf_cnpj": "52998224725"}),
		"unknown env":     call(good, "staging", nil),
	} {
		if resp.StatusCode != 400 {
			t.Errorf("%s: status %d", name, resp.StatusCode)
		}
	}
	if len(srv.Paths) != 0 {
		t.Errorf("configuration failures must not reach the network: %v", srv.Paths)
	}
	// the payment institution may call for another carrier
	if r := call(good, ciot.EnvHom, map[string]any{"carrier_cpf_cnpj": "52998224725", "payment_institution": true}); r.StatusCode != 200 {
		t.Errorf("IP: %+v", r)
	}
	// placeholder key is a configuration failure of the generator operation
	r, _ := Call(context.Background(), ciotReq(t, srv, good, ciot.EnvHom, ciot.ServiceGerarIdentificador,
		map[string]any{"cpf_cnpj": "52998224725", "generator_api_key": ciot.PlaceholderAPIKey}))
	if r.StatusCode != 400 || !strings.Contains(r.Body, "CHANGE_ME") {
		t.Errorf("placeholder key: %+v", r)
	}
}

func TestCIOTGeneratorDownIsUnavailable(t *testing.T) {
	srv := ciottest.NewServer(t)
	srv.GeneratorStatus = 500
	defer ciot.SetTestHosts(map[string]string{ciot.EnvHom: srv.URL})()
	ciot.ResetTokenCache()
	r, _ := Call(context.Background(), ciotReq(t, srv, ciottest.PFX(t, ciottest.CarrierCNPJ, true), ciot.EnvHom,
		ciot.ServiceGerarIdentificador, map[string]any{"cpf_cnpj": "52998224725", "generator_api_key": ciottest.GeneratorKey}))
	if r.StatusCode != 502 || !strings.Contains(r.Body, "PROVIDER_UNAVAILABLE") {
		t.Errorf("%+v", r)
	}
}
