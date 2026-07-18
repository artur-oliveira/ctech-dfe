package services

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

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
	CPF       string // extracted from CN of an e-CPF (format: "PERSON NAME:12345678901")
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
	cnpj, cpf := "", ""
	// Brazilian ICP-Brasil certs encode the holder's document after a colon:
	// e-CNPJ → "COMPANY NAME:12345678000195" (14 digits),
	// e-CPF  → "PERSON NAME:12345678901"     (11 digits).
	if idx := strings.LastIndex(cn, ":"); idx != -1 {
		candidate := strings.NewReplacer(".", "", "/", "", "-", "").Replace(cn[idx+1:])
		switch {
		case cnpjRe.MatchString(candidate):
			cnpj = candidate
		case cpfRe.MatchString(candidate):
			cpf = candidate
		}
	}

	info := &CertInfo{
		MD5:       md5hex,
		CN:        cn,
		CNPJ:      cnpj,
		CPF:       cpf,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		IsExpired: time.Now().UTC().After(cert.NotAfter),
	}
	return cert, rsaKey, info, nil
}

// MatchOrgDocument verifies that the certificate's holder document matches the
// organization's PK. A CNPJ org (PK "CNPJ_…") must present an e-CNPJ for the
// same CNPJ; a CPF org (PK "CPF_…") an e-CPF for the same CPF. When the CN
// carries no recognizable document (some legacy certs) the check is skipped —
// possession of the PFX + password is still proven by ParsePFX.
func MatchOrgDocument(orgPK string, info *CertInfo) error {
	if cnpj, ok := strings.CutPrefix(orgPK, "CNPJ_"); ok {
		if info.CNPJ != "" && info.CNPJ != cnpj {
			return problem.BadRequest(fmt.Sprintf(
				"o CNPJ do certificado (%s) não corresponde ao CNPJ da organização (%s)", info.CNPJ, cnpj))
		}
		return nil
	}
	if cpf, ok := strings.CutPrefix(orgPK, "CPF_"); ok {
		if info.CPF != "" && info.CPF != cpf {
			return problem.BadRequest(fmt.Sprintf(
				"o CPF do certificado (%s) não corresponde ao CPF da organização (%s)", info.CPF, cpf))
		}
		return nil
	}
	return nil
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

	if err := MatchOrgDocument(orgPK, info); err != nil {
		return nil, err
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

// StageUpload validates the PFX (password, expiry, document match) and uploads
// it to S3, returning the parsed info and S3 key so the caller can compose the
// certificate row into a larger transaction (e.g. atomic org creation). It does
// NOT write to DynamoDB. The S3 object is keyed by content MD5, so a stray
// upload from a later-failed transaction is harmless.
func (s *CertificateService) StageUpload(ctx context.Context, orgPK string, pfxData []byte, password string) (*CertInfo, string, error) {
	_, _, info, err := ParsePFX(pfxData, password)
	if err != nil {
		return nil, "", err
	}
	if info.IsExpired {
		return nil, "", problem.BadRequest("certificate is expired")
	}
	if err := MatchOrgDocument(orgPK, info); err != nil {
		return nil, "", err
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
		return nil, "", problem.InternalServer("failed to upload certificate to S3")
	}
	return info, s3Key, nil
}

// BuildCertTxItem builds the certificate create tx item (and item) for
// composing into a transaction. alias defaults to the CN when empty.
func (s *CertificateService) BuildCertTxItem(orgPK, alias, md5, password, s3Key, cn, expiresAt string) (types.TransactWriteItem, map[string]types.AttributeValue) {
	if alias == "" {
		alias = cn
	}
	return s.repo.BuildCreateTxItem(orgPK, alias, md5, password, s3Key, expiresAt)
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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(out.Body)
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(out.Body); err != nil {
		return nil, problem.InternalServer("failed to read certificate from S3")
	}
	return buf.Bytes(), nil
}
