package nfes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
)

// GetDANFeURL validates tenant ownership and returns a direct cached PDF download. format "" prints the
// format the XML asks for (tpImp); otherwise one of documents.AllowedFormats(DocTypeNFe).
func (s *NfeService) GetDANFeURL(ctx context.Context, orgPK, accessKey, format string) (*documents.Download, error) {
	if p := documents.ValidateFormat(documents.DocTypeNFe, format); p != nil {
		return nil, p
	}
	doc, err := s.GetNFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNFeNotFound
	}
	xmlKey := documentAttr(doc, "xml_s3_key")
	if xmlKey == "" {
		return nil, problem.NotFound("XML da NF-e ainda não disponível")
	}
	xmlBytes, err := downloadS3(ctx, s.clients, s.bucketDocs, xmlKey)
	if err != nil {
		return nil, err
	}
	return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeNFe, accessKey, xmlBytes, documents.CancelledWhen(documentAttr(doc, "status") == services.StatusCancelled), format)
}

// GetDANFCeURL validates tenant ownership and returns a direct cached PDF download. format "" derives it from
// tpImp (5 = mensagem, else completo); see documents.AllowedFormats(DocTypeNFCe).
func (s *NfceService) GetDANFCeURL(ctx context.Context, orgPK, accessKey, format string) (*documents.Download, error) {
	if p := documents.ValidateFormat(documents.DocTypeNFCe, format); p != nil {
		return nil, p
	}
	doc, err := s.GetNFCe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNFCeNotFound
	}
	xmlKey := documentAttr(doc, "xml_s3_key")
	if xmlKey == "" {
		// Offline NFC-e not yet authorized: print from the stored signed XML.
		if signed := offlineSignedXML(doc); signed != "" {
			return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeNFCe, accessKey, []byte(signed), documents.StateContingencyPending, format)
		}
		return nil, problem.NotFound("XML da NFC-e ainda não disponível")
	}
	xmlBytes, err := downloadS3(ctx, s.clients, s.bucketDocs, xmlKey)
	if err != nil {
		return nil, err
	}
	return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeNFCe, accessKey, xmlBytes, documents.CancelledWhen(documentAttr(doc, "status") == services.StatusCancelled), format)
}

func documentAttr(item map[string]types.AttributeValue, key string) string {
	if value, ok := item[key].(*types.AttributeValueMemberS); ok {
		return value.Value
	}
	return ""
}
