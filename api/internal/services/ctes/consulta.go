package ctes

// consulta.go: consulta de situação do CT-e (CTeConsulta). Leitura síncrona: a API chama o go-dfe-egress em
// sa-east-1 (ExternalService.CallDfe), sem fila nem worker, como mdfes/consulta.go.

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
	sefazServiceConsulta = "CTeConsulta"
	xServConsultar       = "CONSULTAR"

	tpAmbProd = "1"
	tpAmbHom  = "2"

	// cStat de CTeConsulta que são resposta (a situação do documento), não rejeição da consulta. O MOC
	// descreve 100 e 101 como retorno de documento localizado; 217 é "não consta na base".
	cStatAutorizado = "100"
	cStatCancelado  = "101"
	cStatNaoConsta  = "217"

	// StatusNotFound é a situação de um CT-e que a SEFAZ diz não constar na base (217).
	StatusNotFound = "not_found"
	// StatusCancelled é o CT-e com cancelamento homologado (101).
	StatusCancelled = "cancelled"

	consultaMaxRetries = 2
)

// situationByCStat traduz o cStat da consulta na situação do CT-e, no vocabulário dos status da API.
var situationByCStat = map[string]string{
	cStatAutorizado: statusAuthorized,
	cStatCancelado:  StatusCancelled,
	cStatNaoConsta:  StatusNotFound,
}

// dfeCaller é o que a consulta usa do ExternalService: o certificado e a chamada ao go-dfe-egress.
type dfeCaller interface {
	CertificateB64(ctx context.Context, orgPK string) (string, string, error)
	CallDfe(ctx context.Context, req godfe.Request) (godfe.Response, error)
}

// CteSituation é a resposta de GET /ctes/{access_key}/sefaz-status.
type CteSituation struct {
	AccessKey string `json:"access_key"`
	CStat     string `json:"c_stat"`
	XMotivo   string `json:"x_motivo"`
	// Status: authorized (100), cancelled (101) ou not_found (217).
	Status   string              `json:"status"`
	Protocol *CteSituationProt   `json:"protocol"`
	Events   []CteSituationEvent `json:"events"`
}

// CteSituationProt é o protocolo de autorização (protCTe/infProt).
type CteSituationProt struct {
	NProt    string `json:"n_prot"`
	DhRecbto string `json:"dh_recbto"`
	CStat    string `json:"c_stat"`
	XMotivo  string `json:"x_motivo"`
}

// CteSituationEvent é um evento registrado (procEventoCTe): tipo e sequência do pedido, retorno e protocolo.
type CteSituationEvent struct {
	TpEvento    string `json:"tp_evento"`
	NSeqEvento  string `json:"n_seq_evento"`
	CStat       string `json:"c_stat"`
	XMotivo     string `json:"x_motivo"`
	NProt       string `json:"n_prot"`
	DhRegEvento string `json:"dh_reg_evento"`
}

// ConsultSituation consulta na SEFAZ a situação atual de um CT-e da organização, com o protocolo e os eventos
// registrados. Só lê: o registro local não muda.
func (s *CteService) ConsultSituation(ctx context.Context, orgPK, accessKey string) (*CteSituation, error) {
	record, err := s.GetCTe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrCTeNotFound
	}
	env := envPrefixFromPK(strAttr(record, "pk"))
	body := map[string]any{"consSitCTe": map[string]any{
		"@versao": cteVersao, "@xmlns": cteXMLNS,
		"tpAmb": tpAmbFor(env), "xServ": xServConsultar, "chCTe": accessKey,
	}}
	ret, err := s.callConsulta(ctx, orgPK, strAttr(record, "emit_cpf_cnpj"), ufFromAccessKey(accessKey), env,
		sefazServiceConsulta, body, "retConsSitCTe")
	if err != nil {
		return nil, err
	}
	return parseSituation(accessKey, ret)
}

// callConsulta chama o egress e devolve o nó de retorno (retTag) da resposta. Falha de transporte é 500;
// resposta não 200 do egress vira o Problem do go-dfe (rejeição do fisco).
func (s *CteService) callConsulta(ctx context.Context, orgPK, doc, uf, env, service string, body map[string]any, retTag string) (map[string]any, error) {
	if s.egress == nil {
		return nil, problem.InternalServer("consulta à SEFAZ não configurada")
	}
	certB64, certPassword, err := s.egress.CertificateB64(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	resp, err := s.egress.CallDfe(ctx, godfe.Request{
		CNPJ: doc, CertificateB64: certB64, CertificatePassword: certPassword,
		UF: uf, Environment: env, DocType: s3PrefixCte, Service: service, Body: body,
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

// parseSituation lê o retConsSitCTe. cStat fora de 100, 101 e 217 é rejeição da consulta (chave inválida,
// muito antiga, diferente da base): vira 400 sefaz-rejection com o cStat e o motivo.
func parseSituation(accessKey string, ret map[string]any) (*CteSituation, error) {
	cStat, xMotivo := anyStr(ret, "cStat"), anyStr(ret, "xMotivo")
	status, ok := situationByCStat[cStat]
	if !ok {
		return nil, sefazRejection(cStat, xMotivo)
	}
	out := &CteSituation{AccessKey: accessKey, CStat: cStat, XMotivo: xMotivo, Status: status, Events: []CteSituationEvent{}}
	if prot, ok := ret["protCTe"].(map[string]any); ok {
		inf, _ := prot["infProt"].(map[string]any)
		out.Protocol = &CteSituationProt{
			NProt: anyStr(inf, "nProt"), DhRecbto: anyStr(inf, "dhRecbto"),
			CStat: anyStr(inf, "cStat"), XMotivo: anyStr(inf, "xMotivo"),
		}
	}
	for _, proc := range services.AsSliceOfMaps(ret, "procEventoCTe") {
		ev := subMap(proc, "eventoCTe", "infEvento")
		rt := subMap(proc, "retEventoCTe", "infEvento")
		out.Events = append(out.Events, CteSituationEvent{
			TpEvento: anyStr(ev, "tpEvento"), NSeqEvento: anyStr(ev, "nSeqEvento"),
			CStat: anyStr(rt, "cStat"), XMotivo: anyStr(rt, "xMotivo"),
			NProt: anyStr(rt, "nProt"), DhRegEvento: anyStr(rt, "dhRegEvento"),
		})
	}
	return out, nil
}

func sefazRejection(cStat, xMotivo string) *problem.Problem {
	return dfeproblem.SefazRejection(strings.TrimSpace(fmt.Sprintf("%s - %s", cStat, xMotivo))).WithExtra("cstat", cStat)
}

// tpAmbFor traduz o prefixo de ambiente (prod, hom) no tpAmb do leiaute.
func tpAmbFor(env string) string {
	if env == services.EnvProd {
		return tpAmbProd
	}
	return tpAmbHom
}

// envPrefixFromPK extrai o prefixo de ambiente ("prod"/"hom") do PK do documento.
func envPrefixFromPK(pk string) string { return strings.SplitN(pk, "#", 2)[0] }

// ufFromAccessKey devolve a sigla da UF do emitente (cUF nas posições 0-1 da chave).
func ufFromAccessKey(accessKey string) string {
	if len(accessKey) < 2 {
		return ""
	}
	return services.UFFromCode[accessKey[0:2]]
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
