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

// implemented is the compiled set of (docType, service) pairs go-dfe handles
// in-process today. Every other (docType, service) falls back to the py-dfe
// Lambda (see worker/internal/service/dfe.go's invokePyDfe seam,
// distribution.go's variant, and api/internal/services/external.go).
//
// Unsigned operations (status/consulta/distribuição) are here per the normal
// plan gate (shadow-mode parity, see docs/plans/2026-07-17-go-dfe-migration.md).
//
// Signed operations (autorização, eventos, inutilização) — everything the
// worker's SNS-routed Lambdas actually process (nfe-emission/-event/
// -inutilization, cte-emission/-event, mdfe-emission/-event, see
// cdk/lib/worker-definitions.ts) — were added 2026-07-18 at explicit
// operator direction to fully cut worker over from py-dfe, WITHOUT the
// plan's byte-identical signature gate (no dedicated SEFAZ test certificate
// exists in this repo to run it against). Decision was made deliberately for
// a controlled zero-traffic window (no live users at the time), as an
// accepted, explicit exception to the gate documented elsewhere in this
// file/go-dfe/CLAUDE.md — not a silent skip. Re-tighten this (remove until
// the gate passes) before any real fiscal traffic depends on it if that
// tradeoff is ever reconsidered.
var implemented = map[string]map[string]bool{
	constants.DocTypeNFE: {
		"NfeStatusServico":     true,
		"NfeConsultaProtocolo": true,
		"NfeConsultaCadastro":  true,
		"NFeDistribuicaoDFe":   true,
		// NFeRetAutorizacao: async-batch-authorization poll, unsigned (not in
		// NFE_CONFIG.services_requiring_signature/validation, py-dfe/py_dfe/services/config.py).
		"NFeRetAutorizacao": true,
		// Signed — worker's nfe-emission/-event/-inutilization workers. See
		// doc comment above: promoted without the byte-identical gate.
		"NFeAutorizacao":  true,
		"RecepcaoEvento":  true,
		"NfeInutilizacao": true,
	},
	constants.DocTypeNFCE: {
		"NfeStatusServico": true,
		// nfce shares nfe's WSDL/config for these two (NfCeService enum,
		// py-dfe/py_dfe/constants/enums.py) — both unsigned.
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
	// NFS-e não migra do py-dfe: py-dfe nunca implementou NFS-e, então não
	// existe autoridade anterior contra a qual rodar shadow-mode nem corpus
	// para o portão de assinatura byte-idêntica. O portão aplicável é a
	// homologação em produção restrita (fase F6 do plano de NFS-e), não a
	// comparação de paridade descrita acima.
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

// Implements reports whether go-dfe handles (docType, service) in-process.
// Callers (worker/api) must check this before choosing between dfe.Call and
// the py-dfe Lambda invoke path.
func Implements(docType, service string) bool {
	return implemented[docType][service]
}

// Call executes req in-process against SEFAZ, mirroring py-dfe's Lambda
// handler contract exactly (see request.go's Request/Response/Problem) so a
// caller currently marshaling lambdaPayload/parsing lambdaResponse needs no
// change beyond swapping the Lambda Invoke for this call. Callers MUST check
// Implements(req.DocType, req.Service) first — Call returns an error for any
// operation not in the implemented set rather than silently guessing.
func Call(ctx context.Context, req Request) (Response, error) {
	if !Implements(req.DocType, req.Service) {
		return Response{}, fmt.Errorf("dfe: %s/%s not implemented in go-dfe — caller must use the py-dfe Lambda fallback", req.DocType, req.Service)
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
// matching py-dfe's error responses exactly (see py-dfe/py_dfe/exceptions.py
// to_problem / handler.py's DFeError branch) so callers parsing
// lambdaResponse.Body as a Problem see no difference between py-dfe and
// go-dfe error shapes.
func problemResponse(status int, code, detail string) (Response, error) {
	p := Problem{Type: "about:blank", Title: code, Detail: detail, Status: status}
	body, err := json.Marshal(p)
	if err != nil {
		return Response{}, fmt.Errorf("dfe: encode problem response: %w", err)
	}
	return Response{StatusCode: status, Body: string(body), Headers: map[string]string{}}, nil
}
