package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
)

// authDuplicatedStat é o cStat 204 do MOC (NF-e, NFC-e, CT-e e MDF-e), "Duplicidade
// de documento" para a MESMA chave: a SEFAZ já autorizou. Acontece quando a
// primeira chamada autorizou e a resposta se perdeu; o reenvio volta 204. O 539
// (chave diferente para a mesma numeração) é rejeição real e não se reconcilia.
const authDuplicatedStat = "204"

// consultaSpec descreve a consulta de situação de cada documento: serviço do
// egress, raiz do corpo, namespace, tag da chave, protocolo e retorno.
type consultaSpec struct {
	service, rootTag, versao, ns, keyTag, protTag, retTag string
}

var consultaSpecs = map[string]consultaSpec{
	docTypeNFe:  {"NfeConsultaProtocolo", "consSitNFe", "4.00", nsNFe, "chNFe", "protNFe", "retConsSitNFe"},
	docTypeNFCe: {"NfeConsultaProtocolo", "consSitNFe", "4.00", nsNFe, "chNFe", "protNFe", "retConsSitNFe"},
	docTypeMDFe: {"MDFeConsulta", "consSitMDFe", "3.00", nsMDFe, "chMDFe", "protMDFe", "retConsSitMDFe"},
	docTypeCTe:  {"CTeConsulta", "consSitCTe", "4.00", nsCTe, "chCTe", "protCTe", "retConsSitCTe"},
}

// reconciled é o resultado de uma duplicidade confirmada como o mesmo documento.
type reconciled struct {
	respBody map[string]any
	cStat    string
	xMotivo  string
	nProt    string
}

// reconcileDuplicateAuthorization consulta a situação do documento na SEFAZ
// depois de um 204 e, se ele consta autorizado com o mesmo digest, monta o
// processado (documento assinado + protocolo vigente) a partir do XML que o
// egress devolveu junto do 204. Qualquer divergência devolve ok=false e o
// documento segue o caminho normal de rejeição. Erro só para falha de
// transporte, que o chamador trata como retentável.
func (s *DfeService) reconcileDuplicateAuthorization(ctx context.Context, msg WorkerMessage, respBody map[string]any) (reconciled, bool, error) {
	spec, ok := consultaSpecs[msg.DocType]
	signedProc, _ := respBody["@xml"].(string)
	if !ok || signedProc == "" {
		return reconciled{}, false, nil
	}

	certB64, err := s.getCertB64(ctx, msg.CertS3Key)
	if err != nil {
		return reconciled{}, false, fmt.Errorf("getCertB64: %w", err)
	}
	tpAmb := "2"
	if msg.SefazEnvironment == sefazEnvProd {
		tpAmb = "1"
	}
	lambdaResp, err := s.invokeEgress(ctx, lambdaPayload{
		CNPJ: msg.CNPJ, CertificateB64: certB64, CertificatePassword: msg.CertPassword,
		UF: msg.UF, Environment: msg.SefazEnvironment, DocType: msg.DocType,
		Service: spec.service,
		Body: map[string]any{spec.rootTag: map[string]any{
			"@versao": spec.versao, "@xmlns": spec.ns,
			"tpAmb": tpAmb, "xServ": "CONSULTAR", spec.keyTag: msg.AccessKey,
		}},
	})
	if err != nil {
		return reconciled{}, false, fmt.Errorf("consulta de situação: %w", err)
	}
	if lambdaResp.StatusCode != 200 {
		if isRetryableEngineStatus(lambdaResp.StatusCode) {
			return reconciled{}, false, fmt.Errorf("consulta de situação: egress %d", lambdaResp.StatusCode)
		}
		return reconciled{}, false, nil
	}
	var consulta map[string]any
	if err := json.Unmarshal([]byte(lambdaResp.Body), &consulta); err != nil {
		return reconciled{}, false, fmt.Errorf("consulta de situação: %w", err)
	}

	ret := asMap(consulta, spec.retTag)
	if ret == nil {
		ret = consulta
	}
	cStat := mapStr(ret, "cStat", "")
	protDict := asMap(ret, spec.protTag)
	infProt := asMap(protDict, "infProt")
	nProt := mapStr(infProt, "nProt", "")
	if (cStat != "100" && cStat != "150") || nProt == "" {
		return reconciled{}, false, nil
	}
	// Mesma chave não basta: o digest do protocolo tem que ser o do documento
	// assinado que enviamos, senão é outro documento ocupando a chave.
	if digVal := mapStr(infProt, "digVal", ""); digVal == "" || !strings.Contains(signedProc, "<DigestValue>"+digVal+"</DigestValue>") {
		slog.Warn("duplicidade 204 com digest diferente; mantida como rejeição", "access_key", msg.AccessKey)
		return reconciled{}, false, nil
	}

	fragment, err := godfe.BuildXMLFragment(protDict, spec.protTag, spec.ns)
	if err != nil {
		return reconciled{}, false, fmt.Errorf("build %s: %w", spec.protTag, err)
	}
	proc, ok := swapProtocol(signedProc, spec.protTag, string(fragment))
	if !ok {
		return reconciled{}, false, nil
	}

	out := make(map[string]any, len(respBody))
	for k, v := range respBody {
		out[k] = v
	}
	out["@xml"] = proc
	return reconciled{respBody: out, cStat: cStat, xMotivo: mapStr(infProt, "xMotivo", mapStr(ret, "xMotivo", "")), nProt: nProt}, true, nil
}

// swapProtocol troca o elemento de protocolo do processado, byte a byte fora
// dele: a assinatura do documento depende do conteúdo exato e nunca é reserializada.
func swapProtocol(proc, protTag, fragment string) (string, bool) {
	start := strings.Index(proc, "<"+protTag)
	closeTag := "</" + protTag + ">"
	end := strings.Index(proc, closeTag)
	if start < 0 || end < start {
		return "", false
	}
	return proc[:start] + fragment + proc[end+len(closeTag):], true
}
