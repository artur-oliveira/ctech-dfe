package services

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/awsclient"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"software.sslmate.com/src/go-pkcs12"
)

// CertInfo holds parsed metadata from a PFX certificate.
type CertInfo struct {
	MD5       string
	CN        string
	CNPJ      string // extracted from CN (format: "COMPANY NAME:12345678000195")
	NotBefore time.Time
	NotAfter  time.Time
	IsExpired bool
}

// ParsePFX decodes and validates a PFX/P12 blob.
func ParsePFX(pfxData []byte, password string) (*x509.Certificate, *rsa.PrivateKey, *CertInfo, error) {
	privateKey, cert, _, err := pkcs12.DecodeChain(pfxData, password)
	if err != nil {
		return nil, nil, nil, problem.BadRequest("invalid certificate or password: " + err.Error())
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, nil, problem.BadRequest("certificate private key must be RSA")
	}

	sum := md5.Sum(pfxData)
	md5hex := fmt.Sprintf("%x", sum)

	cn := cert.Subject.CommonName
	cnpj := ""
	// Brazilian A1/A3 certs encode CNPJ after colon: "COMPANY NAME:12345678000195"
	if idx := strings.LastIndex(cn, ":"); idx != -1 {
		candidate := cn[idx+1:]
		candidate = strings.NewReplacer(".", "", "/", "", "-", "").Replace(candidate)
		if cnpjRe.MatchString(candidate) {
			cnpj = candidate
		}
	}

	info := &CertInfo{
		MD5:       md5hex,
		CN:        cn,
		CNPJ:      cnpj,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		IsExpired: time.Now().UTC().After(cert.NotAfter),
	}
	return cert, rsaKey, info, nil
}

// CertificateService mirrors api/app/services/certificates.py.
type CertificateService struct {
	repo       *repositories.CertificateRepository
	auditRepo  *repositories.AuditLogRepository
	awsClients *awsclient.Clients
	bucketName string
}

func NewCertificateService(
	repo *repositories.CertificateRepository,
	auditRepo *repositories.AuditLogRepository,
	clients *awsclient.Clients,
	bucketName string,
) *CertificateService {
	return &CertificateService{repo: repo, auditRepo: auditRepo, awsClients: clients, bucketName: bucketName}
}

// Upload parses the PFX, uploads to S3, then writes the certificate row and
// its CREATE audit row atomically. S3 upload happens first — an audit row for
// a certificate that failed to upload would be wrong.
// alias is an optional human-readable label (defaults to CN if empty).
func (s *CertificateService) Upload(ctx context.Context, orgPK string, pfxData []byte, password, alias, userID, userName string) (map[string]any, error) {
	_, _, info, err := ParsePFX(pfxData, password)
	if err != nil {
		return nil, err
	}
	if info.IsExpired {
		return nil, problem.BadRequest("certificate is expired")
	}

	if strings.HasPrefix(orgPK, "CNPJ_") {
		orgCNPJ := strings.TrimPrefix(orgPK, "CNPJ_")
		if info.CNPJ != "" && info.CNPJ != orgCNPJ {
			return nil, problem.BadRequest(fmt.Sprintf(
				"certificate CNPJ %s does not match organization CNPJ %s", info.CNPJ, orgCNPJ,
			))
		}
	}

	if alias == "" {
		alias = info.CN
	}

	s3Key := fmt.Sprintf("certs/%s/%s.pfx", orgPK, info.MD5)
	_, err = s.awsClients.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(s.bucketName),
		Key:                  aws.String(s3Key),
		Body:                 bytes.NewReader(pfxData),
		ContentType:          aws.String("application/x-pkcs12"),
		ServerSideEncryption: "aws:kms",
	})
	if err != nil {
		return nil, problem.InternalServer("failed to upload certificate to S3")
	}

	certTx, item := s.repo.BuildCreateTxItem(orgPK, alias, info.MD5, password, s3Key, info.NotAfter.Format(time.RFC3339))

	afterMap, err := attributeMapToPlain(item)
	if err != nil {
		return nil, err
	}
	delete(afterMap, "password") // never surfaced in an audit row, even as an "after" value
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceCertificate, info.MD5, repositories.AuditActionCreate,
		userID, userName, Diff(nil, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{certTx, auditTx}); err != nil {
		return nil, err
	}

	var out map[string]any
	if err := attributevalue.UnmarshalMap(item, &out); err != nil {
		return nil, problem.InternalServer("failed to unmarshal certificate")
	}
	delete(out, "password")
	return out, nil
}

func (s *CertificateService) List(ctx context.Context, orgPK string) ([]map[string]any, error) {
	items, err := s.repo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		var m map[string]any
		if err := attributevalue.UnmarshalMap(item, &m); err != nil {
			return nil, problem.InternalServer("failed to unmarshal certificate")
		}
		delete(m, "password")
		result = append(result, m)
	}
	return result, nil
}

func (s *CertificateService) Get(ctx context.Context, orgPK, md5hex string) (map[string]any, error) {
	item, err := s.repo.Get(ctx, orgPK, md5hex)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("certificate not found")
	}
	var out map[string]any
	if err := attributevalue.UnmarshalMap(item, &out); err != nil {
		return nil, problem.InternalServer("failed to unmarshal certificate")
	}
	delete(out, "password")
	return out, nil
}

// Delete removes the certificate and writes its DELETE audit row atomically.
// password is excluded from the diff — it's real data, but not something that
// should appear in an audit trail even as a "before" value.
func (s *CertificateService) Delete(ctx context.Context, orgPK, md5hex, userID, userName string) error {
	current, err := s.repo.Get(ctx, orgPK, md5hex)
	if err != nil {
		return err
	}
	if current == nil {
		return problem.NotFound("certificate not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return err
	}
	delete(beforeMap, "password")

	certTx := s.repo.BuildDeleteTxItem(orgPK, md5hex)
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceCertificate, md5hex, repositories.AuditActionDelete,
		userID, userName, Diff(beforeMap, nil),
	)
	if err != nil {
		return err
	}

	return s.repo.TransactWrite(ctx, []types.TransactWriteItem{certTx, auditTx})
}

// DownloadPFX fetches the raw PFX from S3 for SEFAZ Lambda invocation.
func (s *CertificateService) DownloadPFX(ctx context.Context, s3Key string) ([]byte, error) {
	out, err := s.awsClients.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return nil, problem.InternalServer("failed to fetch certificate from S3")
	}
	defer out.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(out.Body); err != nil {
		return nil, problem.InternalServer("failed to read certificate from S3")
	}
	return buf.Bytes(), nil
}
