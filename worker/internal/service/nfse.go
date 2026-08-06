package service

import (
	"context"
	"fmt"
)

// docTypeNfse é o valor de WorkerMessage.DocType para NFS-e.
const docTypeNfse = "nfse"

// nfseCancellationEvent é o TE101101 (cancelamento). Não é o 110111 da NF-e.
const nfseCancellationEvent = "101101"

// Chaves do nfse.Result serializado pelo go-dfe (go-dfe/nfse/result.go).
const (
	fieldChaveAcesso = "chave_acesso"
	fieldIDDPS       = "id_dps"
	fieldNfseXML     = "nfse_xml"
	fieldDpsXML      = "dps_xml"
	fieldEventoXML   = "evento_xml"
	fieldErros       = "erros"
	fieldCodigo      = "codigo"
	fieldDescricao   = "descricao"
)

// suffixDPS distingue o XML da DPS que enviamos do XML da NFS-e devolvida pelo
// fisco: os dois ficam sob o mesmo prefixo do documento.
const suffixDPS = "_dps"

func isNfse(docType string) bool { return docType == docTypeNfse }

// nfseOutcome é o resultado normalizado de uma chamada NFS-e.
type nfseOutcome struct {
	AccessKey string
	IDDPS     string
	NFSeXML   string
	DPSXML    string
	EventoXML string
	Status    string
	Motivo    string
}

// parseNfseResponse traduz o nfse.Result do go-dfe. Não há cStat/xMotivo em
// NFS-e: a rejeição vem na lista "erros", e é sempre terminal — o fisco já
// avaliou as regras de negócio, repetir a chamada só produz a mesma recusa.
func parseNfseResponse(respBody map[string]any) nfseOutcome {
	out := nfseOutcome{
		AccessKey: strFromAny(respBody[fieldChaveAcesso]),
		IDDPS:     strFromAny(respBody[fieldIDDPS]),
		NFSeXML:   strFromAny(respBody[fieldNfseXML]),
		DPSXML:    strFromAny(respBody[fieldDpsXML]),
		EventoXML: strFromAny(respBody[fieldEventoXML]),
		Status:    StatusAuthorized,
	}

	if errs, ok := respBody[fieldErros].([]any); ok && len(errs) > 0 {
		out.Status = StatusRejected
		if m, ok := errs[0].(map[string]any); ok {
			out.Motivo = fmt.Sprintf("%s - %s", strFromAny(m[fieldCodigo]), strFromAny(m[fieldDescricao]))
		}
	}
	return out
}

func strFromAny(v any) string {
	s, _ := v.(string)
	return s
}

// handleNfseResponse persiste o resultado de uma chamada NFS-e. Substitui
// handleSefazResponse, que procura cStat/xMotivo/nProt — campos que não
// existem no layout nacional.
//
// A SK da linha em `nfses` é o id_dps (msg.AccessKey), não a chave de acesso:
// a chave só nasce na resposta do fisco e entra como atributo `access_key`.
func (s *DfeService) handleNfseResponse(ctx context.Context, msg WorkerMessage, respBody map[string]any) error {
	out := parseNfseResponse(respBody)
	if out.Status == StatusRejected {
		return s.failTerminal(ctx, msg, out.Motivo)
	}

	if msg.EventsTableName != nil && msg.EventSK != nil {
		return s.persistNfseEvent(ctx, msg, out)
	}

	xmlKey, err := s.putXML(ctx, msg, "", out.NFSeXML)
	if err != nil {
		return err
	}
	dpsKey, err := s.putXML(ctx, msg, suffixDPS, out.DPSXML)
	if err != nil {
		return err
	}

	attrs := updateAttrs{XMLS3Key: xmlKey, DPSXMLS3Key: dpsKey}
	if out.AccessKey != "" {
		attrs.AccessKey = &out.AccessKey
	}
	return s.updateClaimedDocument(ctx, msg, StatusAuthorized, attrs, true)
}

// persistNfseEvent grava o evento aceito e, quando ele cancela o documento,
// reverte o status da NFS-e — o mesmo desenho de handleSefazResponse, onde a
// notificação do usuário sai de publishEventResult, não do documento.
func (s *DfeService) persistNfseEvent(ctx context.Context, msg WorkerMessage, out nfseOutcome) error {
	xmlKey, err := s.putXML(ctx, msg, "", out.EventoXML)
	if err != nil {
		return err
	}

	if isCancellationEvent(msg.DocType, msg.EventType) {
		if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusCancelled, updateAttrs{}, false); err != nil {
			return err
		}
	}

	attrs := updateAttrs{XMLS3Key: xmlKey}
	if err := s.updateClaimedEvent(ctx, msg, EventStatusSuccess, attrs); err != nil {
		return err
	}
	s.publishEventResult(ctx, msg, EventStatusSuccess, attrs)
	return nil
}

// putXML grava um XML sob o prefixo do documento e devolve a chave S3. Devolve
// nil sem gravar quando o XML não veio — nem toda resposta traz os dois
// (consulta não traz DPS, evento não traz NFS-e).
func (s *DfeService) putXML(ctx context.Context, msg WorkerMessage, suffix, xml string) (*string, error) {
	if xml == "" {
		return nil, nil
	}
	key, err := s.putObject(ctx,
		s.documentS3Key(msg.DocPK, msg.S3Prefix, msg.ExpectedFileName+suffix, extXML),
		[]byte(xml), contentTypeXML)
	if err != nil {
		return nil, fmt.Errorf("putXML %s: %w", suffix, err)
	}
	return &key, nil
}
