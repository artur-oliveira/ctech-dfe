package repositories

import (
	"context"
	"fmt"
	"sort"
	"time"

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
		AttrSweep:    SweepValue,
	}
}

func (r *CertificateRepository) Create(ctx context.Context, orgPK, alias, md5, password, s3Key, expiresAt string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Create(ctx, orgPK, certSK(md5), certFields(alias, md5, password, s3Key, expiresAt))
}

func (r *CertificateRepository) Get(ctx context.Context, orgPK, md5 string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, certSK(md5))
}

// List devolve os certificados da organização com o melhor primeiro: os não
// vencidos antes dos vencidos, e dentro de cada grupo o de maior vencimento. A
// chave de ordenação do banco é o MD5 do arquivo, sem relação com a validade, e
// todo ponto de emissão usa certs[0]; sem esta ordem, quem tem um certificado
// vencido e um válido emite com o vencido ao acaso (Sefin responde 403).
func (r *CertificateRepository) List(ctx context.Context, orgPK string) ([]map[string]types.AttributeValue, error) {
	res, err := r.Query(ctx, QueryOpts{PK: orgPK, SKPrefix: "CERTIFICATE_"})
	if err != nil {
		return nil, err
	}
	SortCertificates(res.Items, time.Now())
	return res.Items, nil
}

// SortCertificates ordena in place: válidos (expires_at depois de now) primeiro,
// maior vencimento primeiro; expires_at ausente ou ilegível conta como vencido.
func SortCertificates(items []map[string]types.AttributeValue, now time.Time) {
	exp := func(it map[string]types.AttributeValue) time.Time {
		if v, ok := it["expires_at"].(*types.AttributeValueMemberS); ok {
			if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
				return t
			}
		}
		return time.Time{}
	}
	sort.SliceStable(items, func(i, j int) bool {
		ei, ej := exp(items[i]), exp(items[j])
		vi, vj := ei.After(now), ej.After(now)
		if vi != vj {
			return vi
		}
		return ei.After(ej)
	})
}

func (r *CertificateRepository) Delete(ctx context.Context, orgPK, md5 string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, certSK(md5))
}

// BuildCreateTxItem returns a TransactWriteItem for a new certificate,
// mirroring Create's field construction, without writing.
func (r *CertificateRepository) BuildCreateTxItem(orgPK, alias, md5, password, s3Key, expiresAt string) (types.TransactWriteItem, map[string]types.AttributeValue) {
	tx, item, err := r.CRUDRepository.BuildCreateTxItem(orgPK, certSK(md5), certFields(alias, md5, password, s3Key, expiresAt))
	logUnexpectedError("build certificate create transaction", err)
	return tx, item
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a certificate, without writing.
func (r *CertificateRepository) BuildDeleteTxItem(orgPK, md5 string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, certSK(md5))
}
