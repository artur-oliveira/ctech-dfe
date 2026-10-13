package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"gopkg.aoctech.app/api-commons/observability"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// xmlExportPageSize bounds one zip: each document costs one or two S3 reads
// plus its events, and the whole page must fit the 10 s write timeout.
const xmlExportPageSize = 100

// XMLExportService zips an organization's production XMLs a page at a time,
// so an organization about to be erased with its only member can keep its
// fiscal documents (overview D4, plan ruling R12).
type XMLExportService struct {
	docs    map[string]*repositories.DocumentRepository
	events  map[string]*repositories.DocumentEventRepository
	objects ObjectStore
	bucket  string
}

func NewXMLExportService(
	docs map[string]*repositories.DocumentRepository,
	events map[string]*repositories.DocumentEventRepository,
	objects ObjectStore,
	bucket string,
) *XMLExportService {
	return &XMLExportService{docs: docs, events: events, objects: objects, bucket: bucket}
}

// Page returns a zip of up to xmlExportPageSize production documents of
// docType (document XML, DPS XML for NFS-e, and every event XML) and the key
// to continue from (nil on the last page).
func (s *XMLExportService) Page(ctx context.Context, orgPK, docType string, start map[string]types.AttributeValue) ([]byte, map[string]types.AttributeValue, error) {
	docs, events := s.docs[docType], s.events[docType]
	if docs == nil || events == nil {
		return nil, nil, problem.BadRequest("tipo de documento inválido; use nfe, nfce, cte, mdfe ou nfse")
	}
	res, err := docs.List(ctx, EnvProd+"#"+orgPK, repositories.DocumentListOpts{Limit: xmlExportPageSize, StartKey: start})
	if err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, doc := range res.Items {
		keys := []string{avAttr(doc, "xml_s3_key"), avAttr(doc, "dps_xml_s3_key")}
		evs, err := events.GetDocumentEvents(ctx, avAttr(doc, "sk"), xmlExportPageSize, nil)
		if err != nil {
			return nil, nil, err
		}
		for _, ev := range evs.Items {
			keys = append(keys, avAttr(ev, "xml_s3_key"))
		}
		for _, key := range keys {
			if key == "" {
				continue
			}
			if err := s.addObject(ctx, zw, key); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := zw.Close(); err != nil {
		return nil, nil, problem.InternalServer("falha ao gerar o zip").WithCause(err)
	}
	return buf.Bytes(), res.LastEvaluatedKey, nil
}

func (s *XMLExportService) addObject(ctx context.Context, zw *zip.Writer, key string) error {
	out, err := s.objects.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var missing *s3types.NoSuchKey
		if errors.As(err, &missing) {
			observability.Warn(ctx, "xml export: object missing", err, "s3_key", key)
			return nil
		}
		return problem.InternalServer("falha ao ler XML do armazenamento").WithCause(err)
	}
	defer closeReadCloser(ctx, out.Body, "xml export")
	w, err := zw.Create(path.Base(key))
	if err != nil {
		return problem.InternalServer("falha ao gerar o zip").WithCause(err)
	}
	if _, err := io.Copy(w, out.Body); err != nil {
		return problem.InternalServer("falha ao ler XML do armazenamento").WithCause(err)
	}
	return nil
}
