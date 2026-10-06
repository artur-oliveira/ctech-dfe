package dfe

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"gopkg.aoctech.app/dfe/go-dfe/internal/certificate"
	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
	"gopkg.aoctech.app/dfe/go-dfe/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"
)

// implemented is the allowlist of (docType, service) pairs Call accepts; any
// other pair is rejected rather than guessed at.
//
// Unsigned operations (status/consulta/distribuição) were validated in
// shadow mode against the previous SEFAZ client before it was retired.
//
// Signed operations (autorização, eventos, inutilização) — everything the
// worker's SNS-routed Lambdas process (nfe-emission/-event/-inutilization,
// cte-emission/-event, mdfe-emission/-event, see cdk/lib/worker-definitions.ts)
// — were added 2026-07-18 WITHOUT the byte-identical signature gate (no
// dedicated SEFAZ test certificate exists in this repo to run it against).
// That was a deliberate, explicit exception made while no live users existed,
// not a silent skip. Re-tighten it before real fiscal traffic depends on it if
// that tradeoff is ever reconsidered.
var implemented = map[string]map[string]bool{
	constants.DocTypeNFE: {
		"NfeStatusServico":     true,
		"NfeConsultaProtocolo": true,
		"NfeConsultaCadastro":  true,
		"NFeDistribuicaoDFe":   true,
		// NFeRetAutorizacao: async-batch-authorization poll, unsigned.
		"NFeRetAutorizacao": true,
		// Signed — worker's nfe-emission/-event/-inutilization workers. See
		// doc comment above: promoted without the byte-identical gate.
		"NFeAutorizacao":  true,
		"RecepcaoEvento":  true,
		"NfeInutilizacao": true,
	},
	constants.DocTypeNFCE: {
		"NfeStatusServico": true,
		// nfce shares nfe's WSDL/config for these two — both unsigned.
		"NfeConsultaProtocolo": true,
		"NFeRetAutorizacao":    true,
		// Signed — nfce shares nfe's emission/event/inutilization workers
		// (same sefaz_service names route both doc types to the same SNS
		// filter, see cdk/lib/worker-definitions.ts). Same exception as nfe.
		"NFeAutorizacao":  true,
		"RecepcaoEvento":  true,
		"NfeInutilizacao": true,
	},
	constants.DocTypeCTE: {
		"CTeStatusServico":   true,
		"CTeConsulta":        true,
		"CTeDistribuicaoDFe": true,
		// Signed — worker's cte-emission/-event workers. Same exception as nfe.
		"CTeRecepcaoSinc":   true,
		"CTeRecepcaoOS":     true,
		"CTeRecepcaoGTVe":   true,
		"CTeRecepcaoSimp":   true,
		"CTeRecepcaoEvento": true,
	},
	constants.DocTypeMDFE: {
		"MDFeStatusServico":   true,
		"MDFeConsulta":        true,
		"MDFeConsNaoEnc":      true,
		"MDFeDistribuicaoDFe": true,
		// Signed — worker's mdfe-emission/-event workers. Same exception as nfe.
		"MDFeRecepcaoSinc":   true,
		"MDFeRecepcaoEvento": true,
	},
	// NFS-e nunca teve cliente anterior, então não existe autoridade contra a
	// qual rodar shadow-mode nem corpus para o portão de assinatura
	// byte-idêntica. O portão aplicável é a homologação em produção restrita
	// (fase F6 do plano de NFS-e).
	constants.DocTypeNFSE: {
		constants.ServiceNFSeRecepcao:             true,
		constants.ServiceNFSeConsulta:             true,
		constants.ServiceNFSeConsultaDPS:          true,
		constants.ServiceNFSeEvento:               true,
		constants.ServiceNFSeConsultaEvento:       true,
		constants.ServiceNFSeDistribuicao:         true,
		constants.ServiceNFSeDANFSE:               true,
		constants.ServiceNFSeParametrosMunicipais: true,
	},
}

// Implements reports whether (docType, service) is a supported operation.
func Implements(docType, service string) bool {
	return implemented[docType][service]
}

// Call executes req against SEFAZ / the municipal authority. Request, Response
// and Problem (request.go) are the go-dfe-egress Lambda's wire contract. Call
// returns an error for any (docType, service) outside the implemented set
// rather than silently guessing.
func Call(ctx context.Context, req Request) (Response, error) {
	if !Implements(req.DocType, req.Service) {
		return Response{}, fmt.Errorf("dfe: %s/%s is not a supported operation", req.DocType, req.Service)
	}

	maxRetries := req.MaxRetries
	if maxRetries == 0 {
		maxRetries = constants.DefaultMaxRetries
	}

	// Every service in the implemented set requires mTLS to SEFAZ (status
	// queries included — SEFAZ requires client-certificate auth even for
	// StatusServico), and worker/api always populate CertificateB64 in the
	// request today (see WorkerMessage.CertS3Key in worker/internal/service/dfe.go).
	// Auxiliary-document rendering is handled by the API and is outside this
	// SEFAZ client package; every operation dispatched here requires mTLS.
	if req.CertificateB64 == "" {
		return problemResponse(400, constants.ErrCodeCertRequired, fmt.Sprintf("service %q requires a certificate", req.Service))
	}
	httpClient, cert, key, err := certificate.Load(req.CertificateB64, req.CertificatePassword)
	if err != nil {
		return problemResponse(400, constants.ErrCodeCertificate, err.Error())
	}

	if req.DocType == constants.DocTypeNFSE {
		return callNFSe(ctx, req, httpClient, cert, key, maxRetries)
	}

	client, err := services.NewClient(req.DocType, req.UF, req.Environment, httpClient, cert, key, req.ValidateSchema, maxRetries)
	if err != nil {
		return problemResponse(400, constants.ErrCodeValidation, err.Error())
	}

	result, err := client.Call(ctx, req.Service, req.Body)
	if err != nil {
		return problemResponse(400, constants.ErrCodeSOAPRequest, err.Error())
	}

	bodyJSON, err := json.Marshal(result)
	if err != nil {
		return problemResponse(500, constants.ErrCodeUnexpected, "failed to encode response")
	}
	return Response{StatusCode: 200, Body: string(bodyJSON), Headers: map[string]string{}}, nil
}

// callNFSe é o caminho NFS-e: REST + JSON, sem SOAP e sem endpoints.Resolve.
// Mantém o mesmo contrato de Response (Body como JSON string) que o caminho
// SOAP, para que worker/api não distingam os dois.
func callNFSe(ctx context.Context, req Request, httpClient *http.Client,
	cert *x509.Certificate, key *rsa.PrivateKey, maxRetries int) (Response, error) {
	providerName, _ := req.Body[nfse.BodyKeyProvider].(string)
	municipalityCode, _ := req.Body[nfse.BodyKeyMunicipality].(string)
	provider, err := newNFSeProvider(providerName, req.Environment, municipalityCode,
		httpClient, cert, key, maxRetries, req.CNPJ)
	if err != nil {
		return problemResponse(400, constants.ErrCodeValidation, err.Error())
	}

	result, err := nfse.Dispatch(ctx, provider, req.Service, req.Body)
	if err != nil {
		if fe, ok := errors.AsType[*nfse.FiscalError](err); ok {
			return problemResponse(fe.Status, constants.ErrCodeSOAPRequest, fe.Error())
		}
		return problemResponse(400, constants.ErrCodeValidation, err.Error())
	}

	bodyJSON, err := json.Marshal(result)
	if err != nil {
		return problemResponse(500, constants.ErrCodeUnexpected, "failed to encode response")
	}
	return Response{StatusCode: 200, Body: string(bodyJSON), Headers: map[string]string{}}, nil
}

// newNFSeProvider constrói o provider nacional/ABRASF a partir do nome vindo
// do Body. Vive em dfe.go (não em nfse/dispatch.go) porque nfse/nacional já
// importa nfse — nfse não pode importar nfse/nacional de volta sem criar um
// ciclo; dfe é o único ponto que legitimamente conhece os dois.
func newNFSeProvider(name, environment, municipalityCode string, httpClient *http.Client,
	cert *x509.Certificate, key *rsa.PrivateKey, maxRetries int, cnpj string) (nfse.Provider, error) {
	switch name {
	case nfse.ProviderNacional:
		return nacional.New(nacional.Config{
			Environment: environment, MunicipalityCode: municipalityCode,
			HTTPClient: httpClient, Cert: cert,
			Key: key, MaxRetries: maxRetries, CNPJ: cnpj,
		})
	case nfse.ProviderAbrasf204:
		return nil, fmt.Errorf("nfse: provider %q chega na fase F5", name)
	default:
		return nil, fmt.Errorf("nfse: provider desconhecido %q", name)
	}
}

// problemResponse builds a Response carrying an RFC7807-shaped Problem body,
// so callers can parse Response.Body as a Problem for every error path.
func problemResponse(status int, code, detail string) (Response, error) {
	p := Problem{Type: "about:blank", Title: code, Detail: detail, Status: status}
	body, err := json.Marshal(p)
	if err != nil {
		return Response{}, fmt.Errorf("dfe: encode problem response: %w", err)
	}
	return Response{StatusCode: status, Body: string(body), Headers: map[string]string{}}, nil
}
