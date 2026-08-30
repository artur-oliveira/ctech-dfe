package documents

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"golang.org/x/sync/singleflight"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
)

const (
	s3ErrorNotFound           = "NotFound"
	s3ErrorNoSuchKey          = "NoSuchKey"
	s3ErrorPreconditionFailed = "PreconditionFailed"
)

type objectStore interface {
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type objectPresigner interface {
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// SignedFileDownload describes a short-lived direct S3 download returned by
// the API for both source XMLs and generated auxiliary documents.
type SignedFileDownload struct {
	URL         string    `json:"url"`
	ExpiresAt   time.Time `json:"expires_at"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Cached      *bool     `json:"cached,omitempty"`
}

// Download is kept as an alias while auxiliary-document callers migrate to
// the generic signed-file name.
type Download = SignedFileDownload

// Service renders and caches auxiliary fiscal documents in the documents bucket.
type Service struct {
	bucket    string
	store     objectStore
	presigner objectPresigner
	renderer  pdfRenderer
	requests  singleflight.Group
	now       func() time.Time
}

func NewService(clients *awsclient.Clients, bucket string) (*Service, error) {
	if clients == nil || clients.S3 == nil {
		return nil, fmt.Errorf("auxiliary documents: S3 client is required")
	}
	if bucket == "" {
		return nil, fmt.Errorf("auxiliary documents: documents bucket is required")
	}
	renderer, err := newFolioRenderer()
	if err != nil {
		return nil, err
	}
	return newService(clients.S3, s3.NewPresignClient(clients.S3), renderer, bucket), nil
}

func newService(store objectStore, presigner objectPresigner, renderer pdfRenderer, bucket string) *Service {
	return &Service{bucket: bucket, store: store, presigner: presigner, renderer: renderer, now: time.Now}
}

// GetURL returns a presigned S3 URL, generating the PDF only on a cache miss.
func (s *Service) GetURL(ctx context.Context, orgPK, docType, accessKey string, xmlBytes []byte, state DocumentState) (*Download, error) {
	if _, ok := templateByDocType[docType]; !ok {
		return nil, problem.BadRequest("tipo de documento auxiliar não suportado")
	}
	if !documentStates[state] {
		return nil, problem.BadRequest("estado do documento auxiliar não suportado")
	}
	if digits(accessKey) != accessKey || len(accessKey) != accessKeyLengthByDocType[docType] {
		return nil, problem.BadRequest("chave de acesso inválida")
	}
	objectKey := cacheKey(orgPK, docType, accessKey, state)
	if exists, err := s.exists(ctx, objectKey); err != nil {
		return nil, problem.InternalServer("falha ao consultar cache do PDF").WithCause(err)
	} else if exists {
		return s.presignPDF(ctx, objectKey, accessKey, true)
	}

	value, err, _ := s.requests.Do(objectKey, func() (any, error) {
		generationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), generationTimeout)
		defer cancel()
		if exists, err := s.exists(generationCtx, objectKey); err != nil {
			return nil, err
		} else if exists {
			return true, nil
		}
		pdf, err := s.renderer.Render(generationCtx, docType, xmlBytes, state)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", docType, err)
		}
		_, err = s.store.PutObject(generationCtx, &s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(objectKey),
			Body:        bytes.NewReader(pdf),
			ContentType: aws.String(contentTypePDF),
			IfNoneMatch: aws.String(putIfAbsent),
			Tagging:     aws.String(cacheTagging),
		})
		if err != nil && !hasS3Code(err, s3ErrorPreconditionFailed) {
			return nil, fmt.Errorf("store generated PDF: %w", err)
		}
		return false, nil
	})
	if err != nil {
		return nil, problem.InternalServer("falha ao gerar documento auxiliar").WithCause(err)
	}
	wasCached, _ := value.(bool)
	return s.presignPDF(ctx, objectKey, accessKey, wasCached)
}

func (s *Service) exists(ctx context.Context, objectKey string) (bool, error) {
	_, err := s.store.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey)})
	if err == nil {
		return true, nil
	}
	if hasS3Code(err, s3ErrorNotFound) || hasS3Code(err, s3ErrorNoSuchKey) {
		return false, nil
	}
	return false, err
}

func (s *Service) presignPDF(ctx context.Context, objectKey, accessKey string, cached bool) (*Download, error) {
	filename := accessKey + fileExtensionPDF
	download, err := s.SignFile(ctx, objectKey, filename, contentTypePDF)
	if err != nil {
		return nil, err
	}
	download.Cached = aws.Bool(cached)
	return download, nil
}

func cacheKey(orgPK, docType, accessKey string, state DocumentState) string {
	return path.Join("pdfs", docType, orgPK, cacheSchemaVersion, accessKey+"-"+string(state)+fileExtensionPDF)
}

func hasS3Code(err error, code string) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == code
}
