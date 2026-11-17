package bpes

import (
	"context"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
)

// GetDABPEURL valida o tenant e devolve o PDF do DABPE em cache. O DABPE tem leiaute único (qualquer format é 400) e não se
// aplica ao TM, que é apuração por viagem e não tem bilhete individual.
func (s *BpeService) GetDABPEURL(ctx context.Context, orgPK, accessKey, format string) (*documents.Download, error) {
	if p := documents.ValidateFormat(documents.DocTypeBPe, format); p != nil {
		return nil, p
	}
	doc, err := s.GetBPe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrBPeNotFound
	}
	if strAttr(doc, "variant") == VariantTM {
		return nil, problem.BadRequest("o BP-e TM não tem DABPE")
	}
	xmlKey := strAttr(doc, "xml_s3_key")
	if xmlKey == "" {
		// BP-e offline ainda não autorizado: imprime do XML assinado guardado ("EMITIDO EM CONTINGÊNCIA").
		if signed := services.ContingencySignedXML(doc); signed != "" {
			return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeBPe, accessKey, []byte(signed), documents.StateContingencyPending, format)
		}
		return nil, problem.NotFound("XML do BP-e ainda não disponível")
	}
	xmlBytes, err := services.DownloadS3(ctx, s.clients, s.bucketDocs, xmlKey)
	if err != nil {
		return nil, err
	}
	return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeBPe, accessKey, xmlBytes, documents.CancelledWhen(strAttr(doc, "status") == StatusCancelled), format)
}
