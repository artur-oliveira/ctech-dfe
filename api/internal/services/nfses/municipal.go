package nfses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// municipalParamsTTL é 6 horas, em segundos. Parâmetros municipais mudam por
// competência, não por hora — cachear evita rate-limit do ADN e latência no
// formulário de emissão (spec §5.4).
const municipalParamsTTL = 6 * 60 * 60

const cacheKeyMunicipalPrefix = "dfe:nfse:munparams:"

// cacheKeyMunicipalParams NÃO inclui a organização: os parâmetros são públicos
// por município/competência. Incluir o tenant faria cada organização pagar a
// mesma consulta ao ADN.
func cacheKeyMunicipalParams(kind string, args []string) string {
	return cacheKeyMunicipalPrefix + kind + ":" + strings.Join(args, ":")
}

// validateParamKind usa nacional.ParamArity, a mesma tabela que o provider
// consulta ao montar o path — validar aqui evita uma ida ao ADN para receber
// 404, sem uma segunda cópia da aridade.
func validateParamKind(kind string, args []string) error {
	want, ok := nacional.ParamArity[kind]
	if !ok {
		return problem.BadRequest("tipo de parâmetro municipal desconhecido: " + kind)
	}
	if len(args) != want {
		return problem.BadRequest(fmt.Sprintf("o parâmetro %s exige %d argumentos, recebeu %d", kind, want, len(args)))
	}
	return nil
}

// danfseSupported rejeita o provider que não tem DANFSE. O leiaute ABRASF 2.04
// não define PDF padrão (spec §11): isso é 501, não erro nosso.
func danfseSupported(provider string) *problem.Problem {
	if provider == nfse.ProviderAbrasf204 {
		return problem.NotImplemented("o município ABRASF 2.04 não expõe DANFSE padronizada; use o portal do município")
	}
	return nil
}

// MunicipalParameters consulta a parametrização do município, com cache. A
// chamada ao go-dfe é síncrona e feita direto do serviço: é leitura pública,
// sem escrita e sem risco de estourar o timeout da requisição.
func (s *NfseService) MunicipalParameters(ctx context.Context, orgPK, kind string, args []string) (map[string]any, error) {
	if err := validateParamKind(kind, args); err != nil {
		return nil, err
	}
	key := cacheKeyMunicipalParams(kind, args)
	if cached, ok := services.CacheGet[map[string]any](ctx, s.cacheBackend, key); ok {
		return *cached, nil
	}

	result, err := s.callGoDfe(ctx, orgPK, nfse.ServiceParametrosMunicipais, map[string]any{
		nfse.BodyKeyParamKind: kind,
		nfse.BodyKeyParamArgs: args,
	})
	if err != nil {
		return nil, err
	}
	services.CacheSet(ctx, s.cacheBackend, key, result.Parametros, municipalParamsTTL)
	return result.Parametros, nil
}

// callGoDfe executa uma operação NFS-e de leitura in-process. body recebe as
// chaves específicas da operação; provider e certificado saem da config e do
// cadastro da organização.
func (s *NfseService) callGoDfe(ctx context.Context, orgPK, service string, body map[string]any) (nfse.Result, error) {
	configItem, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nfse.Result{}, err
	}
	if configItem == nil {
		return nfse.Result{}, ErrNfseNoConfig
	}
	provider := strAttr(configItem, "provider")

	// The issuer's document comes off the record: the key is a company id since
	// ADR 0022 and carries none.
	org, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nfse.Result{}, err
	}
	issuerDoc, _ := services.IssuerDocAV(org, orgPK)

	certB64, certPassword, err := s.extSvc.CertificateB64(ctx, orgPK)
	if err != nil {
		return nfse.Result{}, err
	}

	full := map[string]any{
		nfse.BodyKeyProvider:     provider,
		nfse.BodyKeyMunicipality: strAttr(configItem, "c_loc_emi"),
	}
	for k, v := range body {
		full[k] = v
	}

	resp, err := godfe.Call(ctx, godfe.Request{
		CNPJ:                issuerDoc,
		CertificateB64:      certB64,
		CertificatePassword: certPassword,
		UF:                  "", // competência municipal: não há UF autorizadora
		Environment:         services.EnvToPrefix(intAttr(configItem, "environment", 2)),
		DocType:             DocTypeNfse,
		Service:             service,
		Body:                full,
	})
	if err != nil {
		return nfse.Result{}, problem.InternalServer("falha na consulta ao fisco: " + err.Error())
	}
	if resp.StatusCode != 200 {
		return nfse.Result{}, problemFromDfeBody(resp.StatusCode, resp.Body)
	}

	var result nfse.Result
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		return nfse.Result{}, problem.InternalServer("resposta do fisco em formato inesperado")
	}
	return result, nil
}

// problemFromDfeBody traduz o Problem RFC 7807 que o go-dfe devolve no corpo
// para o Problem da api, preservando status e detalhe — a rejeição do fisco
// chega ao usuário, não vira 500 genérico.
func problemFromDfeBody(status int, body string) *problem.Problem {
	var p godfe.Problem
	if err := json.Unmarshal([]byte(body), &p); err != nil || p.Detail == "" {
		return problem.New(status, problem.TypeSefazRejection, "Sefaz Rejection", body)
	}
	return problem.New(status, problem.TypeSefazRejection, "Sefaz Rejection", p.Detail)
}
