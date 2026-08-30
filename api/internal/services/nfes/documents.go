package nfes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
)

// GetDANFeURL validates tenant ownership and returns a direct cached PDF download.
func (s *NfeService) GetDANFeURL(ctx context.Context, orgPK, accessKey string) (*documents.Download, error) {
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
	return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeNFe, accessKey, xmlBytes, documents.CancelledWhen(documentAttr(doc, "status") == services.StatusCancelled))
}

// GetDANFCeURL validates tenant ownership and returns a direct cached PDF download.
func (s *NfceService) GetDANFCeURL(ctx context.Context, orgPK, accessKey string) (*documents.Download, error) {
	doc, err := s.GetNFCe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNFCeNotFound
	}
	xmlKey := documentAttr(doc, "xml_s3_key")
	if xmlKey == "" {
		return nil, problem.NotFound("XML da NFC-e ainda não disponível")
	}
	xmlBytes, err := downloadS3(ctx, s.clients, s.bucketDocs, xmlKey)
	if err != nil {
		return nil, err
	}
	return s.documentSvc.GetURL(ctx, orgPK, documents.DocTypeNFCe, accessKey, xmlBytes, documents.CancelledWhen(documentAttr(doc, "status") == services.StatusCancelled))
}

func documentAttr(item map[string]types.AttributeValue, key string) string {
	if value, ok := item[key].(*types.AttributeValueMemberS); ok {
		return value.Value
	}
	return ""
}
