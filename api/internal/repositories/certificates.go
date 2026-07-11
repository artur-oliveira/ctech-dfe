package repositories

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

type CertificateRepository struct {
	Base
}

func NewCertificateRepository(db *dynamodb.Client, cfg *config.Config) *CertificateRepository {
	return &CertificateRepository{Base: NewBase(db, cfg, "organization_certificates")}
}

func (r *CertificateRepository) Create(ctx context.Context, orgPK, alias, md5, password, s3Key, expiresAt string) (map[string]types.AttributeValue, error) {
	item := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: orgPK},
		"sk":         &types.AttributeValueMemberS{Value: fmt.Sprintf("CERTIFICATE_%s", md5)},
		"alias":      &types.AttributeValueMemberS{Value: alias},
		"md5":        &types.AttributeValueMemberS{Value: md5},
		"password":   &types.AttributeValueMemberS{Value: password},
		"s3_key":     &types.AttributeValueMemberS{Value: s3Key},
		"expires_at": &types.AttributeValueMemberS{Value: expiresAt},
		"created_at": &types.AttributeValueMemberS{Value: NowStr()},
	}
	return item, r.PutItem(ctx, item)
}

func (r *CertificateRepository) Get(ctx context.Context, orgPK, md5 string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, orgPK, fmt.Sprintf("CERTIFICATE_%s", md5))
}

func (r *CertificateRepository) List(ctx context.Context, orgPK string) ([]map[string]types.AttributeValue, error) {
	res, err := r.Query(ctx, QueryOpts{PK: orgPK, SKPrefix: "CERTIFICATE_"})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func (r *CertificateRepository) Delete(ctx context.Context, orgPK, md5 string) (bool, error) {
	return r.DeleteItem(ctx, orgPK, fmt.Sprintf("CERTIFICATE_%s", md5))
}

// BuildCreateTxItem returns a TransactWriteItem for a new certificate,
// mirroring Create's field construction, without writing.
func (r *CertificateRepository) BuildCreateTxItem(orgPK, alias, md5, password, s3Key, expiresAt string) (types.TransactWriteItem, map[string]types.AttributeValue) {
	item := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: orgPK},
		"sk":         &types.AttributeValueMemberS{Value: fmt.Sprintf("CERTIFICATE_%s", md5)},
		"alias":      &types.AttributeValueMemberS{Value: alias},
		"md5":        &types.AttributeValueMemberS{Value: md5},
		"password":   &types.AttributeValueMemberS{Value: password},
		"s3_key":     &types.AttributeValueMemberS{Value: s3Key},
		"expires_at": &types.AttributeValueMemberS{Value: expiresAt},
		"created_at": &types.AttributeValueMemberS{Value: NowStr()},
	}
	return r.BuildPutTxItem(item), item
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a certificate, without writing.
func (r *CertificateRepository) BuildDeleteTxItem(orgPK, md5 string) types.TransactWriteItem {
	return r.Base.BuildDeleteTxItem(orgPK, fmt.Sprintf("CERTIFICATE_%s", md5))
}
