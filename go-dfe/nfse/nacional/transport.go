package nacional

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/internal/xmlops"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// XPaths de assinatura, no formato Clark que xmlops.Sign espera — o mesmo
// que internal/services/client.go monta para os demais doc types.
const (
	signXPathInfDPS    = ".//{" + nfse.Namespace + "}infDPS"
	signXPathInfPedReg = ".//{" + nfse.Namespace + "}infPedReg"
)

// Política de retry idêntica à de internal/services/client.go: só
// infraestrutura, nunca rejeição de negócio.
const backoffBase = 1 * time.Second

var retryableHTTPStatus = map[int]bool{500: true, 502: true, 503: true, 504: true}

// SignDPS assina infDPS com o XML-DSig já usado pelos demais documentos
// (enveloped, RSA-SHA1, digest SHA-1, C14N 1.0).
func SignDPS(xmlBytes []byte, cert *x509.Certificate, key *rsa.PrivateKey) ([]byte, error) {
	return xmlops.Sign(xmlBytes, signXPathInfDPS, cert, key)
}

// SignPedRegEvento assina infPedReg.
func SignPedRegEvento(xmlBytes []byte, cert *x509.Certificate, key *rsa.PrivateKey) ([]byte, error) {
	return xmlops.Sign(xmlBytes, signXPathInfPedReg, cert, key)
}

// withUTF8Declaration garante o prólogo exigido pelo Sefin Nacional. Ele é
// aplicado depois da assinatura porque xmlops.Sign reserializa somente o
// documento XML, sem preservar a declaração.
func withUTF8Declaration(raw []byte) []byte {
	if bytes.HasPrefix(raw, []byte(xml.Header)) {
		return raw
	}
	out := make([]byte, 0, len(xml.Header)+len(raw))
	out = append(out, xml.Header...)
	return append(out, raw...)
}

// GzipB64 comprime e codifica o XML no formato que a API nacional exige
// (dpsXmlGZipB64, pedidoRegistroEventoXmlGZipB64, nfseXmlGZipB64).
func GzipB64(raw []byte) (string, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	defer func(zw *gzip.Writer) {
		err := zw.Close()
		if err != nil {

		}
	}(zw)
	if _, err := zw.Write(raw); err != nil {
		return "", fmt.Errorf("nacional: gzip: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("nacional: gzip close: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// UngzipB64 é o inverso de GzipB64.
func UngzipB64(s string) ([]byte, error) {
	blob, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("nacional: base64: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		return nil, fmt.Errorf("nacional: gzip reader: %w", err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("nacional: gunzip: %w", err)
	}
	return out, nil
}

// errorEnvelope cobre as duas formas de erro do Sefin: "erro" singular
// (MensagemProcessamento) e "erros" plural (NFSePostResponseErro).
type errorEnvelope struct {
	Erro  *nfse.Message  `json:"erro"`
	Erros []nfse.Message `json:"erros"`
}

// httpDo executa a requisição com retry apenas em falha de infraestrutura e
// converte qualquer resposta não-2xx em *nfse.FiscalError com o código e a
// descrição do fisco preservados. out pode ser nil (resposta binária).
func httpDo(ctx context.Context, client *http.Client, method, url string, body, out any, maxRetries int) (int, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("nacional: encode request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(attempt) * backoffBase):
			}
		}

		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return 0, fmt.Errorf("nacional: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("nacional: %s %s: %w", method, url, err)
			continue
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("nacional: ler resposta: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(respBody) > 0 {
				if err := json.Unmarshal(respBody, out); err != nil {
					return resp.StatusCode, fmt.Errorf("nacional: decode resposta: %w", err)
				}
			}
			return resp.StatusCode, nil
		}

		if retryableHTTPStatus[resp.StatusCode] {
			lastErr = fmt.Errorf("nacional: HTTP %d", resp.StatusCode)
			continue
		}
		return resp.StatusCode, toFiscalError(resp.StatusCode, respBody)
	}
	return 0, lastErr
}

func toFiscalError(status int, body []byte) error {
	var env errorEnvelope
	_ = json.Unmarshal(body, &env)
	msgs := env.Erros
	if env.Erro != nil {
		msgs = append(msgs, *env.Erro)
	}
	if len(msgs) == 0 {
		msgs = []nfse.Message{{Descricao: string(body)}}
	}
	return &nfse.FiscalError{Status: status, Messages: msgs}
}
