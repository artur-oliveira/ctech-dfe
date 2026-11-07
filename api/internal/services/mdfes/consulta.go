package mdfes

// consulta.go: consulta de situação (MDFeConsulta) e de MDF-e não encerrados (MDFeConsNaoEnc). As duas são
// leituras síncronas: a API chama o go-dfe-egress em sa-east-1 (ExternalService.CallDfe), como a consulta de
// cadastro e as consultas do ADN da NFS-e; não há fila nem worker no caminho. A reconciliação do worker
// (worker/internal/service/reconcile.go) usa o mesmo MDFeConsulta depois de uma duplicidade 204.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/dfeproblem"
	"gopkg.aoctech.app/dfe/api/internal/services"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
)

const (
	sefazServiceConsulta = "MDFeConsulta"
	sefazServiceNaoEnc   = "MDFeConsNaoEnc"

	xServConsultar = "CONSULTAR"
	xServNaoEnc    = "CONSULTAR NÃO ENCERRADOS"

	tpAmbProd = "1"
	tpAmbHom  = "2"

	// cStat de MDFeConsulta que são resposta (a situação do documento), não rejeição da consulta.
	cStatAutorizado = "100"
	cStatCancelado  = "101"
	cStatEncerrado  = "132"
	cStatNaoConsta  = "217"
	// cStat de MDFeConsNaoEnc: 111 há não encerrados, 112 não há.
	cStatNaoEncLocalizados = "111"
	cStatNaoEncNenhum      = "112"

	// StatusNotFound é a situação de um MDF-e que a SEFAZ diz não constar na base (217).
	StatusNotFound = "not_found"

	consultaMaxRetries = 2
)

// situationByCStat traduz o cStat da consulta na situação do MDF-e, no vocabulário dos status da API.
var situationByCStat = map[string]string{
	cStatAutorizado: StatusAuthorized,
	cStatCancelado:  StatusCancelled,
	cStatEncerrado:  StatusClosed,
	cStatNaoConsta:  StatusNotFound,
}

// dfeCaller é o que a consulta usa do ExternalService: o certificado e a chamada ao go-dfe-egress. Os testes
// simulam o egress por aqui.
type dfeCaller interface {
	CertificateB64(ctx context.Context, orgPK string) (string, string, error)
	CallDfe(ctx context.Context, req godfe.Request) (godfe.Response, error)
}

// MdfeSituation é a resposta de GET /mdfes/{access_key}/sefaz-status.
type MdfeSituation struct {
	AccessKey string `json:"access_key"`
	CStat     string `json:"c_stat"`
	XMotivo   string `json:"x_motivo"`
	// Status: authorized (100), cancelled (101), closed (132) ou not_found (217).
	Status   string               `json:"status"`
	Protocol *MdfeSituationProt   `json:"protocol"`
	Events   []MdfeSituationEvent `json:"events"`
}

// MdfeSituationProt é o protocolo de autorização (protMDFe/infProt).
type MdfeSituationProt struct {
	NProt    string `json:"n_prot"`
	DhRecbto string `json:"dh_recbto"`
	CStat    string `json:"c_stat"`
	XMotivo  string `json:"x_motivo"`
}

// MdfeSituationEvent é um evento registrado (procEventoMDFe): tipo e sequência do pedido, retorno e protocolo.
type MdfeSituationEvent struct {
	TpEvento    string `json:"tp_evento"`
	NSeqEvento  string `json:"n_seq_evento"`
	CStat       string `json:"c_stat"`
	XMotivo     string `json:"x_motivo"`
	NProt       string `json:"n_prot"`
	DhRegEvento string `json:"dh_reg_evento"`
}

// MdfeUnclosed é a resposta de GET /mdfes/unclosed: os MDF-e autorizados e ainda não encerrados do emitente.
type MdfeUnclosed struct {
	CStat   string             `json:"c_stat"`
	XMotivo string             `json:"x_motivo"`
	Items   []MdfeUnclosedItem `json:"items"`
}

// MdfeUnclosedItem é um MDF-e não encerrado: chave e protocolo de autorização.
type MdfeUnclosedItem struct {
	AccessKey string `json:"access_key"`
	NProt     string `json:"n_prot"`
}

// ConsultSituation consulta na SEFAZ a situação atual de um MDF-e da organização (MDFeConsulta), com o
// protocolo e os eventos registrados. Só lê: o registro local não muda.
func (s *MdfeService) ConsultSituation(ctx context.Context, orgPK, accessKey string) (*MdfeSituation, error) {
	record, err := s.GetMDFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrMDFeNotFound
	}
	env := envPrefixFromPK(strAttr(record, "pk"))
	body := map[string]any{"consSitMDFe": map[string]any{
		"@versao": mdfeVersao, "@xmlns": mdfeXMLNS,
		"tpAmb": tpAmbFor(env), "xServ": xServConsultar, "chMDFe": accessKey,
	}}
	ret, err := s.callConsulta(ctx, orgPK, strAttr(record, "emit_cpf_cnpj"), emitUFFromAccessKey(accessKey), env,
		sefazServiceConsulta, body, "retConsSitMDFe")
	if err != nil {
		return nil, err
	}
	return parseSituation(accessKey, ret)
}

// ListUnclosed consulta na SEFAZ os MDF-e autorizados e não encerrados do emitente (MDFeConsNaoEnc), no
// ambiente da configuração do MDF-e. Base das regras F85 a F88 (MDF-e não encerrado na mesma placa e UF).
func (s *MdfeService) ListUnclosed(ctx context.Context, orgPK string) (*MdfeUnclosed, error) {
	environment, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	org, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, problem.NotFound("organização não encontrada")
	}
	doc, _ := services.IssuerDocAV(org, orgPK)
	uf := extractEmitUF(org)
	if doc == "" || uf == "" {
		return nil, problem.BadRequest("documento ou UF do emitente não configurados")
	}
	env := envToPrefix(environment)
	cons := map[string]any{"@versao": mdfeVersao, "@xmlns": mdfeXMLNS, "tpAmb": tpAmbFor(env), "xServ": xServNaoEnc}
	if len(doc) == cnpjLen {
		cons["CNPJ"] = doc
	} else {
		cons["CPF"] = doc
	}
	ret, err := s.callConsulta(ctx, orgPK, doc, uf, env, sefazServiceNaoEnc,
		map[string]any{"consMDFeNaoEnc": cons}, "retConsMDFeNaoEnc")
	if err != nil {
		return nil, err
	}
	return parseUnclosed(ret)
}

// callConsulta chama o egress e devolve o nó de retorno (retTag) da resposta. Falha de transporte é 500;
// resposta não 200 do egress vira o Problem do go-dfe (rejeição do fisco).
func (s *MdfeService) callConsulta(ctx context.Context, orgPK, doc, uf, env, service string, body map[string]any, retTag string) (map[string]any, error) {
	if s.egress == nil {
		return nil, problem.InternalServer("consulta à SEFAZ não configurada")
	}
	certB64, certPassword, err := s.egress.CertificateB64(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	resp, err := s.egress.CallDfe(ctx, godfe.Request{
		CNPJ: doc, CertificateB64: certB64, CertificatePassword: certPassword,
		UF: uf, Environment: env, DocType: s3PrefixMdfe, Service: service, Body: body,
		MaxRetries: consultaMaxRetries,
	})
	if err != nil {
		return nil, problem.InternalServer("falha na consulta à SEFAZ: " + err.Error())
	}
	if resp.StatusCode != 200 {
		return nil, services.ProblemFromDfeBody(resp.StatusCode, resp.Body)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		return nil, problem.InternalServer("resposta da SEFAZ em formato inesperado")
	}
	if ret, ok := out[retTag].(map[string]any); ok {
		return ret, nil
	}
	return out, nil
}

// parseSituation lê o retConsSitMDFe. cStat fora de 100, 101, 132 e 217 é rejeição da consulta (chave inválida,
// muito antiga, diferente da base): vira 400 sefaz-rejection com o cStat e o motivo.
func parseSituation(accessKey string, ret map[string]any) (*MdfeSituation, error) {
	cStat, xMotivo := anyStr(ret, "cStat"), anyStr(ret, "xMotivo")
	status, ok := situationByCStat[cStat]
	if !ok {
		return nil, sefazRejection(cStat, xMotivo)
	}
	out := &MdfeSituation{AccessKey: accessKey, CStat: cStat, XMotivo: xMotivo, Status: status, Events: []MdfeSituationEvent{}}
	if prot, ok := ret["protMDFe"].(map[string]any); ok {
		inf, _ := prot["infProt"].(map[string]any)
		out.Protocol = &MdfeSituationProt{
			NProt: anyStr(inf, "nProt"), DhRecbto: anyStr(inf, "dhRecbto"),
			CStat: anyStr(inf, "cStat"), XMotivo: anyStr(inf, "xMotivo"),
		}
	}
	for _, proc := range services.AsSliceOfMaps(ret, "procEventoMDFe") {
		ev := subMap(proc, "eventoMDFe", "infEvento")
		rt := subMap(proc, "retEventoMDFe", "infEvento")
		out.Events = append(out.Events, MdfeSituationEvent{
			TpEvento: anyStr(ev, "tpEvento"), NSeqEvento: anyStr(ev, "nSeqEvento"),
			CStat: anyStr(rt, "cStat"), XMotivo: anyStr(rt, "xMotivo"),
			NProt: anyStr(rt, "nProt"), DhRegEvento: anyStr(rt, "dhRegEvento"),
		})
	}
	return out, nil
}

// parseUnclosed lê o retConsMDFeNaoEnc: 111 traz a lista, 112 é lista vazia, o resto é rejeição.
func parseUnclosed(ret map[string]any) (*MdfeUnclosed, error) {
	cStat, xMotivo := anyStr(ret, "cStat"), anyStr(ret, "xMotivo")
	if cStat != cStatNaoEncLocalizados && cStat != cStatNaoEncNenhum {
		return nil, sefazRejection(cStat, xMotivo)
	}
	out := &MdfeUnclosed{CStat: cStat, XMotivo: xMotivo, Items: []MdfeUnclosedItem{}}
	for _, inf := range services.AsSliceOfMaps(ret, "infMDFe") {
		out.Items = append(out.Items, MdfeUnclosedItem{AccessKey: anyStr(inf, "chMDFe"), NProt: anyStr(inf, "nProt")})
	}
	return out, nil
}

func sefazRejection(cStat, xMotivo string) *problem.Problem {
	return dfeproblem.SefazRejection(strings.TrimSpace(fmt.Sprintf("%s - %s", cStat, xMotivo))).WithExtra("cstat", cStat)
}

// tpAmbFor traduz o prefixo de ambiente (prod, hom) no tpAmb do leiaute.
func tpAmbFor(env string) string {
	if env == EnvProd {
		return tpAmbProd
	}
	return tpAmbHom
}

// subMap desce pelas chaves e devolve o nó, ou nil.
func subMap(m map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		next, _ := m[k].(map[string]any)
		if next == nil {
			return nil
		}
		m = next
	}
	return m
}
