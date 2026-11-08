package ciot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/internal/services"
)

// ErrServer is an HTTP 5xx from the ANTT: not retried by the client (the request may have been processed); the
// generator uses the status to decide on the fallback.
type ErrServer struct{ Status int }

func (e *ErrServer) Error() string { return fmt.Sprintf("ciot: server error HTTP %d", e.Status) }

// HostFor returns the host of an environment; anything else is a configuration error (envs never mix).
func HostFor(env string) (string, error) {
	if h, ok := testHosts[env]; ok {
		return h, nil
	}
	// Guarda contra o host real em teste: um binário de teste nunca chega à ANTT. Um teste que esqueceu de fixar
	// o host de um ambiente (um `defer SetTestHosts` dentro de laço, por exemplo) falha aqui em vez de sair da máquina.
	if testing.Testing() {
		return "", fmt.Errorf("ciot: teste sem host falso para o ambiente %q: o host real da ANTT é recusado em teste", env)
	}
	return realHostFor(env)
}

// realHostFor é só o mapa ambiente para host real (sem a guarda de teste); os testes o usam para conferir que
// produção e homologação nunca se misturam, sem abrir conexão.
func realHostFor(env string) (string, error) {
	switch env {
	case EnvProd:
		return HostProducao, nil
	case EnvHom:
		return HostHomologacao, nil
	}
	return "", fmt.Errorf("ciot: unknown environment %q (want %q or %q)", env, EnvProd, EnvHom)
}

// Client calls the PEF services over an mTLS http.Client.
type Client struct {
	Host string
	HTTP *http.Client
	// Sleep waits between retries; tests replace it. Nil uses services.BackoffDuration.
	Sleep func(ctx context.Context, attempt int) error
}

// NewClient builds a client for env; host is HostFor(env) unless hostOverride is set (tests only).
func NewClient(env string, httpClient *http.Client, hostOverride string) (*Client, error) {
	host := hostOverride
	if host == "" {
		h, err := HostFor(env)
		if err != nil {
			return nil, err
		}
		host = h
	}
	if httpClient == nil {
		return nil, errors.New("ciot: nil http client")
	}
	return &Client{Host: strings.TrimRight(host, "/"), HTTP: httpClient}, nil
}

func (c *Client) wait(ctx context.Context, attempt int) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, attempt)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(services.BackoffDuration(attempt)):
		return nil
	}
}

// Call posts (or GETs, for ConsultarExcecao) a service and returns the parsed Result. A non-success code is
// returned as *Rejection and is never retried; only transport failures are retried (QueryRetries times).
func (c *Client) Call(ctx context.Context, service string, in any) (Result, error) {
	method, timeout := http.MethodPost, DeclarationTimeout
	switch service {
	case ServiceExcecao:
		method, timeout = http.MethodGet, QueryTimeout
	case ServiceSituacaoTransportador, ServiceFrotaTransportador, ServiceCIOTGerado:
		timeout = QueryTimeout
	case ServiceDeclaracao, ServiceCancelamento, ServiceRetificacao, ServiceEncerramento:
	default:
		return Result{}, fmt.Errorf("ciot: unknown service %q", service)
	}
	target := c.Host + apiPathPrefix + service
	var payload []byte
	if method == http.MethodGet {
		var q struct {
			Doc string `json:"CPFCNPJTransportador"`
		}
		raw, _ := json.Marshal(in)
		_ = json.Unmarshal(raw, &q)
		target += "?" + url.Values{queryCarrier: {q.Doc}}.Encode()
	} else {
		var err error
		if payload, err = json.Marshal(in); err != nil {
			return Result{}, fmt.Errorf("ciot: encode request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= QueryRetries; attempt++ {
		if attempt > 0 {
			if err := c.wait(ctx, attempt); err != nil {
				return Result{}, err
			}
		}
		status, body, err := c.do(ctx, method, target, payload, timeout, nil)
		if err != nil {
			lastErr = err
			continue // transport failure only
		}
		return interpret(status, body)
	}
	return Result{}, lastErr
}

// do runs one attempt; extra sets additional headers. Errors are transport errors only.
func (c *Client) do(ctx context.Context, method, target string, payload []byte, timeout time.Duration,
	headers map[string]string) (int, []byte, error) {
	actx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var rd io.Reader
	if payload != nil {
		rd = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(actx, method, target, rd)
	if err != nil {
		return 0, nil, fmt.Errorf("ciot: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("ciot: %s %s: %w", method, target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("ciot: read response: %w", err)
	}
	return resp.StatusCode, body, nil
}

// interpret maps an HTTP response to Result, *Rejection or *ErrServer.
func interpret(status int, body []byte) (Result, error) {
	if status >= 500 {
		return Result{}, &ErrServer{Status: status}
	}
	res, err := ParseResult(body)
	if err != nil {
		return Result{}, fmt.Errorf("%w (HTTP %d)", err, status)
	}
	if !res.Success() {
		return res, newRejection(res)
	}
	return res, nil
}

// Declarar normalizes and declares; on success Result.CIOT holds the 16 characters.
func (c *Client) Declarar(ctx context.Context, d Declaracao) (Result, error) {
	d.Normalize()
	res, err := c.Call(ctx, ServiceDeclaracao, d)
	if err == nil && res.CIOT == "" && res.CodigoVerificador != "" {
		res.CIOT = d.IdOperacaoTransporte + res.CodigoVerificador
	}
	return res, err
}

// testHosts redirects HostFor to fake servers; set only through SetTestHosts (tests of this module and of
// go-dfe-egress). Production code never sets it.
var testHosts map[string]string

// SetTestHosts points environments at fake servers and returns the function that restores the real hosts.
func SetTestHosts(h map[string]string) (restore func()) {
	prev := testHosts
	testHosts = h
	return func() { testHosts = prev }
}
