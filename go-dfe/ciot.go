package dfe

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/ciot"
	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
)

// Request.Body keys of the ciot doc_type (the egress reads no AWS service: the worker resolves the SSM parameters
// /ctech-dfe/{env}/ciot/generator-api-key and generator-fallback-url and passes them here; never logged).
const (
	bodyKeyCarrierDoc   = "carrier_cpf_cnpj"    // certificate must belong to it (B112) unless payment_institution
	bodyKeyPaymentInst  = "payment_institution" // the certificate is an Instituicao de Pagamento's
	bodyKeyGenAPIKey    = "generator_api_key"
	bodyKeyGenFallback  = "generator_fallback_url"
	bodyKeyGenDoc       = "cpf_cnpj" // GerarIdentificador: the contratado
	respKeyIdentifier   = "id"
	problemTypeRejected = "urn:ctech-dfe:ciot:rejection"
)

func ciotServiceSet() map[string]bool {
	m := make(map[string]bool, len(ciot.Services))
	for _, s := range ciot.Services {
		m[s] = true
	}
	return m
}

// ciotProblem extends Problem with the ANTT reference fields; parsers of Problem keep working.
type ciotProblem struct {
	Problem
	ANTTCode     int    `json:"antt_code,omitempty"`
	ANTTMessage  string `json:"antt_message,omitempty"`
	Protocol     string `json:"protocol,omitempty"`
	InternalCode string `json:"internal_code,omitempty"`
	Group        string `json:"group,omitempty"`
}

const (
	statusRejection   = 422 // ANTT business rejection: never retried
	statusUnavailable = 502 // transport/5xx/generator down: the caller may retry
)

// callCIOT runs one ciot operation. Certificate and host come from the same environment (ciot.HostFor); the
// certificate preflight (OID 2.16.76.1.3.3, client EKU, carrier match) runs before any network call.
func callCIOT(ctx context.Context, req Request, httpClient *http.Client, cert *x509.Certificate) (Response, error) {
	env := req.Environment
	carrier, _ := req.Body[bodyKeyCarrierDoc].(string)
	isIP, _ := req.Body[bodyKeyPaymentInst].(bool)
	if err := ciot.CheckCertificate(cert, carrier, isIP, time.Now()); err != nil {
		return problemResponse(400, constants.ErrCodeCertificate, err.Error())
	}
	client, err := ciot.NewClient(env, httpClient, "")
	if err != nil {
		return problemResponse(400, constants.ErrCodeValidation, err.Error())
	}

	var out any
	if req.Service == ciot.ServiceGerarIdentificador {
		out, err = ciotGenerate(ctx, req, env, client)
	} else {
		out, err = ciotService(ctx, req, client)
	}
	if err != nil {
		return ciotError(err)
	}
	body, merr := json.Marshal(out)
	if merr != nil {
		return problemResponse(500, constants.ErrCodeUnexpected, "failed to encode response")
	}
	return Response{StatusCode: 200, Body: string(body), Headers: map[string]string{}}, nil
}

func ciotGenerate(ctx context.Context, req Request, env string, client *ciot.Client) (any, error) {
	key, _ := req.Body[bodyKeyGenAPIKey].(string)
	fallback, _ := req.Body[bodyKeyGenFallback].(string)
	doc, _ := req.Body[bodyKeyGenDoc].(string)
	gen, err := ciot.NewGenerator(ctx, env, client, nil, ciot.StaticSecrets{ciot.ParamAPIKey: key, ciot.ParamFallbackURL: fallback})
	if err != nil {
		return nil, &ciotConfigError{err}
	}
	id, err := gen.Generate(ctx, doc)
	if err != nil {
		return nil, err
	}
	return map[string]string{respKeyIdentifier: id}, nil
}

func ciotService(ctx context.Context, req Request, client *ciot.Client) (any, error) {
	in := ciotPayload(req.Body)
	if req.Service == ciot.ServiceDeclaracao {
		raw, _ := json.Marshal(in)
		var d ciot.Declaracao
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, &ciotConfigError{err}
		}
		res, err := client.Declarar(ctx, d)
		return res, err
	}
	return client.Call(ctx, req.Service, in)
}

// ciotConfigError marks a configuration/validation failure (HTTP 400), as opposed to an availability one.
type ciotConfigError struct{ error }

func ciotError(err error) (Response, error) {
	if rj, ok := errors.AsType[*ciot.Rejection](err); ok {
		p := ciotProblem{
			Problem:  Problem{Type: problemTypeRejected, Title: rj.Internal, Detail: rj.Mensagem, Status: statusRejection},
			ANTTCode: rj.Codigo, ANTTMessage: rj.Mensagem, Protocol: rj.Protocolo, InternalCode: rj.Internal, Group: rj.Group,
		}
		body, _ := json.Marshal(p)
		return Response{StatusCode: statusRejection, Body: string(body), Headers: map[string]string{}}, nil
	}
	if ce, ok := errors.AsType[*ciotConfigError](err); ok {
		return problemResponse(400, constants.ErrCodeValidation, ce.Error())
	}
	return problemResponse(statusUnavailable, constants.ErrCodeSOAPRequest, err.Error())
}

// ciotPayload is the Body without the keys that steer the egress (certificate preflight, generator secrets): they
// must never reach the ANTT.
func ciotPayload(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))
	for k, v := range body {
		switch k {
		case bodyKeyCarrierDoc, bodyKeyPaymentInst, bodyKeyGenAPIKey, bodyKeyGenFallback, bodyKeyGenDoc:
		default:
			out[k] = v
		}
	}
	return out
}
