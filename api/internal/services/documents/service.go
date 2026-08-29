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

// Download describes the short-lived direct S3 download returned by the API.
type Download struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	Cached    bool      `json:"cached"`
}

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
func (s *Service) GetURL(ctx context.Context, orgPK, docType, accessKey string, xmlBytes []byte, canceled bool) (*Download, error) {
	if _, ok := templateByDocType[docType]; !ok {
		return nil, problem.BadRequest("tipo de documento auxiliar não suportado")
	}
	if len(digits(accessKey)) != 44 || digits(accessKey) != accessKey {
		return nil, problem.BadRequest("chave de acesso inválida")
	}
	objectKey := cacheKey(orgPK, docType, accessKey, canceled)
	if exists, err := s.exists(ctx, objectKey); err != nil {
		return nil, problem.InternalServer("falha ao consultar cache do PDF").WithCause(err)
	} else if exists {
		return s.presign(ctx, objectKey, accessKey, true)
	}

	value, err, _ := s.requests.Do(objectKey, func() (any, error) {
		generationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), generationTimeout)
		defer cancel()
		if exists, err := s.exists(generationCtx, objectKey); err != nil {
			return nil, err
		} else if exists {
			return true, nil
		}
		pdf, err := s.renderer.Render(generationCtx, docType, xmlBytes, canceled)
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
	return s.presign(ctx, objectKey, accessKey, wasCached)
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

func (s *Service) presign(ctx context.Context, objectKey, accessKey string, cached bool) (*Download, error) {
	filename := accessKey + fileExtensionPDF
	disposition := fmt.Sprintf(`attachment; filename="%s"`, filename)
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey), ResponseContentType: aws.String(contentTypePDF),
		ResponseContentDisposition: aws.String(disposition),
	}, func(options *s3.PresignOptions) { options.Expires = presignedURLTTL })
	if err != nil {
		return nil, problem.InternalServer("falha ao criar URL de download do PDF").WithCause(err)
	}
	return &Download{URL: request.URL, ExpiresAt: s.now().UTC().Add(presignedURLTTL), Cached: cached}, nil
}

func cacheKey(orgPK, docType, accessKey string, canceled bool) string {
	state := cacheStateActive
	if canceled {
		state = cacheStateCanceled
	}
	return path.Join("pdfs", docType, orgPK, cacheSchemaVersion, accessKey+"-"+state+fileExtensionPDF)
}

func hasS3Code(err error, code string) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == code
}
