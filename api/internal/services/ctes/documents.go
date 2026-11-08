package ctes

import (
	"context"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
)

// GetDACTEURL validates tenant ownership and returns a direct cached PDF download.
func (s *CteService) GetDACTEURL(ctx context.Context, orgPK, accessKey string) (*documents.Download, error) {
	doc, err := s.GetCTe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrCTeNotFound
	}
	xmlKey := strAttr(doc, "xml_s3_key")
	if xmlKey == "" {
		// Offline CT-e not yet authorized: print from the stored signed XML (DACTE em contingência).
		if signed := services.ContingencySignedXML(doc); signed != "" {
			return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeCTe, accessKey, []byte(signed), documents.StateContingencyPending, "")
		}
		return nil, problem.NotFound("XML do CT-e ainda não disponível")
	}
	xmlBytes, err := services.DownloadS3(ctx, s.clients, s.bucketDocs, xmlKey)
	if err != nil {
		return nil, err
	}
	return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeCTe, accessKey, xmlBytes, documents.CancelledWhen(strAttr(doc, "status") == services.StatusCancelled), "")
}
