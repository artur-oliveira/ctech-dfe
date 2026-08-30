package documents

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
)

const (
	ContentTypeXML        = "application/xml"
	fileExtensionXML      = ".xml"
	dispositionAttachment = "attachment"
)

// NewFileService creates the signing-only form used by services which do not
// render auxiliary documents.
func NewFileService(clients *awsclient.Clients, bucket string) (*Service, error) {
	if clients == nil || clients.S3 == nil {
		return nil, fmt.Errorf("signed files: S3 client is required")
	}
	if bucket == "" {
		return nil, fmt.Errorf("signed files: documents bucket is required")
	}
	return &Service{bucket: bucket, presigner: s3.NewPresignClient(clients.S3), now: time.Now}, nil
}

// SignFile returns a direct S3 URL for an object whose tenant ownership and
// availability were already validated by the calling service.
func (s *Service) SignFile(ctx context.Context, objectKey, filename, contentType string) (*SignedFileDownload, error) {
	if strings.TrimSpace(objectKey) == "" {
		return nil, problem.InternalServer("referência do arquivo não informada")
	}
	filename = safeDownloadFilename(filename)
	if filename == "" {
		return nil, problem.InternalServer("nome do arquivo não informado")
	}
	if contentType == "" {
		return nil, problem.InternalServer("tipo do arquivo não informado")
	}
	disposition := mime.FormatMediaType(dispositionAttachment, map[string]string{"filename": filename})
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(objectKey),
		ResponseContentType:        aws.String(contentType),
		ResponseContentDisposition: aws.String(disposition),
	}, func(options *s3.PresignOptions) { options.Expires = presignedURLTTL })
	if err != nil {
		return nil, problem.InternalServer("falha ao criar URL de download").WithCause(err)
	}
	return &SignedFileDownload{
		URL:         request.URL,
		ExpiresAt:   s.now().UTC().Add(presignedURLTTL),
		Filename:    filename,
		ContentType: contentType,
	}, nil
}

// XMLFilename appends the XML extension after reducing user-controlled route
// values to a safe attachment filename.
func XMLFilename(base string) string {
	return safeDownloadFilename(base) + fileExtensionXML
}

func safeDownloadFilename(value string) string {
	value = filepath.Base(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	return strings.TrimSpace(value)
}
