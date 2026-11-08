package mdfes

import (
	"context"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
)

// GetDAMDFEURL validates tenant ownership and returns a direct cached PDF download.
func (s *MdfeService) GetDAMDFEURL(ctx context.Context, orgPK, accessKey string) (*documents.Download, error) {
	doc, err := s.GetMDFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrMDFeNotFound
	}
	xmlKey := avStr(doc, "xml_s3_key")
	if xmlKey == "" {
		// Offline MDF-e not yet authorized: print from the stored signed XML (EMISSÃO EM CONTINGÊNCIA).
		if signed := services.ContingencySignedXML(doc); signed != "" {
			return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeMDFe, accessKey, []byte(signed), documents.StateContingencyPending)
		}
		return nil, problem.NotFound("XML do MDF-e ainda não disponível")
	}
	xmlBytes, err := downloadS3(ctx, s.clients, s.bucketDocs, xmlKey)
	if err != nil {
		return nil, err
	}
	return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeMDFe, accessKey, xmlBytes, documents.CancelledWhen(avStr(doc, "status") == services.StatusCancelled))
}
