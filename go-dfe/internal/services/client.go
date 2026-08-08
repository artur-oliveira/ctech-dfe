package services

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
	"gopkg.aoctech.app/dfe/go-dfe/internal/endpoints"
	"gopkg.aoctech.app/dfe/go-dfe/internal/soap"
	"gopkg.aoctech.app/dfe/go-dfe/internal/textutil"
	"gopkg.aoctech.app/dfe/go-dfe/internal/xmlops"
)

// Retry/timeout defaults, mirroring py-dfe's SefazClient class attributes
// (py-dfe/py_dfe/services/base.py: MAX_RETRIES, TIMEOUT_CONNECT, TIMEOUT_READ,
// BACKOFF_BASE).
const (
	defaultTimeoutConnect = 3 * time.Second
	defaultTimeoutRead    = 15 * time.Second
	backoffBase           = 1 * time.Second
	ufMatoGrosso          = "MT"
)

// retryableHTTPStatus mirrors py-dfe's _RETRYABLE_HTTP: only infrastructure
// failures are retried, never a 4xx business rejection.
var retryableHTTPStatus = map[int]bool{500: true, 502: true, 503: true, 504: true}

// gzipEndpoints mirrors py-dfe's _GZIP_ENDPOINTS: services whose SOAP body
// must be gzip-compressed + base64-encoded rather than embedded as raw XML.
var gzipEndpoints = map[string]bool{
	"CTeRecepcaoSinc": true, "CTeRecepcaoOS": true, "CTeRecepcaoGTVe": true,
	"CTeRecepcaoSimp": true, "MDFeRecepcaoSinc": true,
}

// Client is a generic SEFAZ web service client for one (docType, uf,
// environment), mirroring py-dfe's SefazClient (py-dfe/py_dfe/services/base.py):
// JSON payload -> XML -> optional sign -> SOAP envelope -> mTLS POST with
// retry -> parsed response.
type Client struct {
	docType        string
	uf             string
	environment    string
	httpClient     *http.Client
	cert           *x509.Certificate
	key            *rsa.PrivateKey
	config         Config
	validateSchema bool
	maxRetries     int
}

// NewClient builds a Client. httpClient/cert/key come from
// certificate.NewClient + the same PFX decode (see go-dfe/internal/certificate) —
// this package only consumes an already-decoded certificate, it does not
// parse PFX itself. maxRetries mirrors py-dfe's max_retries (0-10, default
// constants.DefaultMaxRetries); validateSchema, if true, and the service
// requires validation, is a hard error (see Config.RequiresValidation) since
// go-dfe does not implement XSD validation (CGO_ENABLED=0 rules out the only
// mature Go option — see docs/plans/2026-07-17-go-dfe-migration.md).
func NewClient(
	docType, uf, environment string,
	httpClient *http.Client,
	cert *x509.Certificate,
	key *rsa.PrivateKey,
	validateSchema bool,
	maxRetries int,
) (*Client, error) {
	cfg, err := GetConfig(docType)
	if err != nil {
		return nil, err
	}
	return &Client{
		docType: docType, uf: uf, environment: environment,
		httpClient: httpClient, cert: cert, key: key,
		config: cfg, validateSchema: validateSchema, maxRetries: maxRetries,
	}, nil
}

// Call executes a SEFAZ service call: payload (a single-key map, e.g.
// {"consStatServ": {...}}, matching Request.Body's existing shape) becomes
// XML, is signed if the service requires it, wrapped in a SOAP envelope,
// POSTed with retry, and the response parsed back into the shape py-dfe's
// facade layer produces (py-dfe/py_dfe/services/_nf.py, cte.py, mdfe.py):
// unwrapped per the per-(authorizer,service) response node path
// (unwrapResponseNode, response.go), any known repeated-but-possibly-single
// element normalized to a list (ensureList, response.go), and — for signed
// emission/event services — the "@xml" processed document attached
// (xmlops.BuildProcessedXML; a no-op for every currently unsigned
// implemented service).
func (c *Client) Call(ctx context.Context, service string, payload map[string]any) (map[string]any, error) {
	rootTag, body, err := singleRootElement(payload)
	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}

	xmlBytes, err := xmlops.BuildXML(body, rootTag, constants.DocNamespace[c.docType])
	if err != nil {
		return nil, fmt.Errorf("services: build xml: %w", err)
	}
	xmlBytes = normalizeFiscalText(c.docType, c.uf, xmlBytes)

	if c.config.RequiresSignature(service) {
		xpath := ""
		if local := c.config.SignXPath(service); local != "" {
			xpath = fmt.Sprintf(".//{%s}%s", constants.DocNamespace[c.docType], local)
		}
		xmlBytes, err = xmlops.Sign(xmlBytes, xpath, c.cert, c.key)
		if err != nil {
			return nil, fmt.Errorf("services: sign: %w", err)
		}
	}

	slog.Info("sending xml", "xml", string(xmlBytes))

	if c.validateSchema && c.config.RequiresValidation(service) {
		return nil, fmt.Errorf("services: %q requires XSD validation, which go-dfe does not implement (see docs/plans/2026-07-17-go-dfe-migration.md)", service)
	}

	builder, err := soap.NewBuilder(c.docType, c.uf, service)
	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}
	soapBody, err := builder.Build(xmlBytes, gzipEndpoints[service], false)
	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}

	slog.Info("soap body", "body", string(soapBody))

	url, err := endpoints.Resolve(c.docType, c.uf, c.environment, service)
	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}

	raw, err := c.postWithRetry(ctx, url, soapBody, builder.ContentType())
	if err != nil {
		return nil, err
	}

	resultXML, err := soap.ParseResult(c.docType, raw)

	slog.Info("soap response", "response", string(resultXML))

	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}
	parsed, err := xmlops.ParseXML(resultXML)
	if err != nil {
		return nil, fmt.Errorf("services: parse response: %w", err)
	}

	result, err := unwrapResponseNode(c.docType, c.uf, service, parsed)
	if err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}

	logCStat(service, result)

	for _, path := range ensureListPathsFor(c.docType, service) {
		ensureList(result, path)
	}

	if processedXML, ok := xmlops.BuildProcessedXML(c.docType, service, xmlBytes, resultXML); ok {
		result["@xml"] = processedXML
	}

	return result, nil
}

// normalizeFiscalText aplica somente compatibilidades comprovadamente
// específicas do autorizador. A transformação ocorre antes da assinatura.
func normalizeFiscalText(docType, uf string, xmlBytes []byte) []byte {
	if docType != constants.DocTypeNFE || uf != ufMatoGrosso {
		return xmlBytes
	}
	return []byte(textutil.RemoveDiacritics(string(xmlBytes)))
}

// singleRootElement extracts payload's one top-level key/value pair, which
// becomes the XML document's root tag/content — mirroring py-dfe's
// to_xml_bytes(payload), which infers the root element from the dict's
// single key.
func singleRootElement(payload map[string]any) (tag string, body map[string]any, err error) {
	if len(payload) != 1 {
		return "", nil, fmt.Errorf("payload must have exactly one root key, got %d", len(payload))
	}
	for k, v := range payload {
		inner, ok := v.(map[string]any)
		if !ok {
			return "", nil, fmt.Errorf("payload root %q must be an object", k)
		}
		return k, inner, nil
	}
	return "", nil, fmt.Errorf("unreachable")
}

// logCStat logs SEFAZ's response status, if present, mirroring py-dfe's
// diagnostic log in _parse_response.
func logCStat(service string, result map[string]any) {
	cStat, _ := result["cStat"].(string)
	if cStat == "" {
		return
	}
	xMotivo, _ := result["xMotivo"].(string)
	slog.Info("sefaz response", "service", service, "cStat", cStat, "xMotivo", xMotivo)
}

// postWithRetry POSTs body to url with retries, mirroring py-dfe's
// _post_with_retry: retry only on retryableHTTPStatus or network/timeout
// errors, exponential backoff (backoffBase * 2^attempt), never retry a 4xx
// business rejection.
func (c *Client) postWithRetry(ctx context.Context, url string, body []byte, contentType string) ([]byte, error) {
	client := c.httpClient
	if client.Timeout == 0 {
		// Caller-supplied httpClient (certificate.NewClient) sets no overall
		// timeout; apply py-dfe's read timeout as this client's request
		// deadline (connect timeout is not separately controllable via
		// net/http's high-level Client without a custom DialContext, and
		// TIMEOUT_READ is the larger of the two in py-dfe anyway).
		clientCopy := *client
		clientCopy.Timeout = defaultTimeoutRead
		client = &clientCopy
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("services: build request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.maxRetries {
				slog.Warn("sefaz request failed, retrying", "attempt", attempt+1, "max_retries", c.maxRetries, "err", err)
				sleepFn(attempt)
				continue
			}
			break
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("services: read response: %w", readErr)
		}

		if retryableHTTPStatus[resp.StatusCode] && attempt < c.maxRetries {
			slog.Warn("sefaz returned retryable status, retrying", "attempt", attempt+1, "max_retries", c.maxRetries, "status", resp.StatusCode)
			sleepFn(attempt)
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("services: http %d from %s: %s", resp.StatusCode, url, string(respBody))
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("services: all %d retries exhausted for %s: %w", c.maxRetries, url, lastErr)
}

// sleepFn is a package var (not a plain function) so tests can stub out the
// exponential backoff and run retry scenarios in milliseconds instead of
// seconds — see client_test.go.
var sleepFn = func(attempt int) {
	time.Sleep(backoffBase * time.Duration(1<<uint(attempt)))
}
