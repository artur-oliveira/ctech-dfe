package ciots

import (
	"context"
	"encoding/json"
	"strings"

	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/api/internal/services"
	godfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/ciot"
)

// CarrierResult is the synchronous answer of the carrier queries. Result is the egress body (ciot.Result).
type CarrierResult struct {
	Cached bool            `json:"cached"`
	Result json.RawMessage `json:"result"`
}

// CarrierStatus is ConsultarSituacaoTransportador with a 1 h DynamoDB TTL cache keyed by environment and
// CPF/CNPJ only (public RNTRC data).
func (s *Service) CarrierStatus(ctx context.Context, orgPK, env, doc string) (*CarrierResult, error) {
	doc = ciot.NormalizeDoc(doc)
	if cached, err := s.repo.GetCarrierStatus(ctx, env, doc, s.now()); err == nil && cached != "" {
		return &CarrierResult{Cached: true, Result: json.RawMessage(cached)}, nil
	}
	body, err := s.query(ctx, orgPK, env, ciot.ServiceSituacaoTransportador, func(self string) map[string]any {
		return map[string]any{"CpfCnpjInteressado": self, "CpfCnpjTransportador": doc}
	})
	if err != nil {
		return nil, err
	}
	// A failed cache write only costs the next call a live query.
	_ = s.repo.PutCarrierStatus(ctx, env, doc, body, s.now(), CarrierStatusTTL)
	return &CarrierResult{Result: json.RawMessage(body)}, nil
}

// CarrierFleet is ConsultarFrotaTransportador, always live.
func (s *Service) CarrierFleet(ctx context.Context, orgPK, env, doc string, plates []string) (*CarrierResult, error) {
	if len(plates) == 0 || len(plates) > maxFleetPlates {
		return nil, problem.UnprocessableEntity("informe de 1 a 7 placas").WithCode(CodeValidation)
	}
	norm := make([]string, 0, len(plates))
	for _, p := range plates {
		p = ciot.NormalizePlate(p)
		if !ciot.ValidPlate(p) {
			return nil, problem.UnprocessableEntity("placa inválida: " + p).WithCode(CodeValidation)
		}
		norm = append(norm, p)
	}
	doc = ciot.NormalizeDoc(doc)
	body, err := s.query(ctx, orgPK, env, ciot.ServiceFrotaTransportador, func(self string) map[string]any {
		return map[string]any{"CpfCnpjInteressado": self, "CpfCnpjTransportador": doc, "Placas": norm}
	})
	if err != nil {
		return nil, err
	}
	return &CarrierResult{Result: json.RawMessage(body)}, nil
}

func (s *Service) query(ctx context.Context, orgPK, env, service string, build func(self string) map[string]any) (string, error) {
	oc, err := s.orgContext(ctx, orgPK)
	if err != nil {
		return "", err
	}
	b64, pwd, err := s.egress.CertificateB64(ctx, orgPK)
	if err != nil {
		return "", err
	}
	payload := build(oc.doc)
	payload["carrier_cpf_cnpj"] = oc.doc // the certificate belongs to the interested party
	resp, err := s.egress.CallDfe(ctx, godfe.Request{
		CNPJ: oc.doc, CertificateB64: b64, CertificatePassword: pwd, Environment: env,
		DocType: DocType, Service: service, Body: payload, MaxRetries: queryRetries,
	})
	if err != nil {
		return "", problem.ServiceUnavailable("serviço da ANTT indisponível; tente novamente").WithCode("ciot_unavailable")
	}
	return unwrapEgress(resp)
}

// unwrapEgress turns a non-200 egress answer into a Problem: 422 is a business rejection (stable internal code
// plus the ANTT code and message as reference), 400 a configuration problem, anything else unavailability.
func unwrapEgress(resp godfe.Response) (string, error) {
	if resp.StatusCode == 200 {
		return resp.Body, nil
	}
	var p struct {
		Detail       string `json:"detail"`
		InternalCode string `json:"internal_code"`
		ANTTCode     int    `json:"antt_code"`
		ANTTMessage  string `json:"antt_message"`
	}
	_ = json.Unmarshal([]byte(resp.Body), &p)
	switch resp.StatusCode {
	case 422:
		return "", problem.UnprocessableEntity(strings.TrimSpace(p.Detail)).WithCode(p.InternalCode).
			WithExtra("antt_code", p.ANTTCode).WithExtra("antt_message", p.ANTTMessage)
	case 400:
		return "", services.ProblemFromDfeBody(400, resp.Body)
	}
	return "", problem.ServiceUnavailable("serviço da ANTT indisponível; tente novamente").WithCode("ciot_unavailable")
}
