package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	dfe "gopkg.aoctech.app/dfe/go-dfe"
)

func TestNormalizeEnvironment(t *testing.T) {
	cases := map[string]string{
		"producao": "prod", "homologacao": "hom", "prod": "prod", "hom": "hom", "": "",
	}
	for in, want := range cases {
		if got := normalizeEnvironment(in); got != want {
			t.Errorf("normalizeEnvironment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHandlerNormalizesEnvironmentAndKeepsEmptyUF(t *testing.T) {
	var got dfe.Request
	h := newHandler(func(_ context.Context, req dfe.Request) (dfe.Response, error) {
		got = req
		return dfe.Response{StatusCode: 200, Body: "{}"}, nil
	})

	resp, err := h(context.Background(), dfe.Request{
		Environment: "homologacao", DocType: "nfse", Service: "NFSeRecepcao", UF: "",
	})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if got.Environment != "hom" {
		t.Errorf("environment = %q, want hom", got.Environment)
	}
	if got.UF != "" {
		t.Errorf("uf = %q, want empty (NFS-e is municipal)", got.UF)
	}
}

func TestHandlerPropagatesCallError(t *testing.T) {
	want := errors.New("boom")
	h := newHandler(func(context.Context, dfe.Request) (dfe.Response, error) { return dfe.Response{}, want })
	if _, err := h(context.Background(), dfe.Request{DocType: "nfe", Service: "x"}); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestHandlerNeverLogsPayload(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	h := newHandler(func(context.Context, dfe.Request) (dfe.Response, error) {
		return dfe.Response{}, errors.New("falhou")
	})
	_, _ = h(context.Background(), dfe.Request{
		CertificateB64: "SEGREDO-PFX", CertificatePassword: "SEGREDO-SENHA",
		DocType: "nfe", Service: "NfeStatusServico", UF: "SP", Environment: "prod",
	})
	if out := buf.String(); strings.Contains(out, "SEGREDO") {
		t.Fatalf("log contains secret material: %s", out)
	}
}
