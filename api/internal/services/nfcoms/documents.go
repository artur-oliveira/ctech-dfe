package nfcoms

import (
	"context"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
)

// GetDANFEComURL validates tenant ownership and returns a direct cached PDF download of the DANFE-COM. The DANFE-COM
// has a single layout, so any format is refused (400).
func (s *NfcomService) GetDANFEComURL(ctx context.Context, orgPK, accessKey, format string) (*documents.Download, error) {
	if p := documents.ValidateFormat(documents.DocTypeNFCom, format); p != nil {
		return nil, p
	}
	doc, err := s.GetNFCom(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNFComNotFound
	}
	xmlKey := strAttr(doc, "xml_s3_key")
	if xmlKey == "" {
		// Offline NFCom not yet authorized: print from the stored signed XML ("EMITIDO EM CONTINGÊNCIA").
		if signed := services.ContingencySignedXML(doc); signed != "" {
			return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeNFCom, accessKey, []byte(signed), documents.StateContingencyPending, format)
		}
		return nil, problem.NotFound("XML da NFCom ainda não disponível")
	}
	xmlBytes, err := services.DownloadS3(ctx, s.clients, s.bucketDocs, xmlKey)
	if err != nil {
		return nil, err
	}
	return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeNFCom, accessKey, xmlBytes, documents.CancelledWhen(strAttr(doc, "status") == services.StatusCancelled), format)
}
