package repositories

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

type CertificateRepository struct {
	CRUDRepository[map[string]any]
}

func NewCertificateRepository(db *dynamodb.Client, cfg *config.Config) *CertificateRepository {
	return &CertificateRepository{
		CRUDRepository: NewCRUDRepository[map[string]any](db, cfg, "organization_certificates"),
	}
}

// certSK builds the certificate sort key from the content MD5.
func certSK(md5 string) string {
	return fmt.Sprintf("CERTIFICATE_%s", md5)
}

// certFields is the stored attribute set for a certificate row (excluding the
// pk/sk/created_at/updated_at keys the generic CRUDRepository adds).
func certFields(alias, md5, password, s3Key, expiresAt string) map[string]any {
	return map[string]any{
		"alias":      alias,
		"md5":        md5,
		"password":   password,
		"s3_key":     s3Key,
		"expires_at": expiresAt,
	}
}

func (r *CertificateRepository) Create(ctx context.Context, orgPK, alias, md5, password, s3Key, expiresAt string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Create(ctx, orgPK, certSK(md5), certFields(alias, md5, password, s3Key, expiresAt))
}

func (r *CertificateRepository) Get(ctx context.Context, orgPK, md5 string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, certSK(md5))
}

func (r *CertificateRepository) List(ctx context.Context, orgPK string) ([]map[string]types.AttributeValue, error) {
	res, err := r.Query(ctx, QueryOpts{PK: orgPK, SKPrefix: "CERTIFICATE_"})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func (r *CertificateRepository) Delete(ctx context.Context, orgPK, md5 string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, certSK(md5))
}

// BuildCreateTxItem returns a TransactWriteItem for a new certificate,
// mirroring Create's field construction, without writing.
func (r *CertificateRepository) BuildCreateTxItem(orgPK, alias, md5, password, s3Key, expiresAt string) (types.TransactWriteItem, map[string]types.AttributeValue) {
	tx, item, _ := r.CRUDRepository.BuildCreateTxItem(orgPK, certSK(md5), certFields(alias, md5, password, s3Key, expiresAt))
	return tx, item
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a certificate, without writing.
func (r *CertificateRepository) BuildDeleteTxItem(orgPK, md5 string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, certSK(md5))
}
