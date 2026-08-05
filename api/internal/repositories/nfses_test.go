package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func TestNfseRepository_TableAndIndexNames(t *testing.T) {
	r := &NfseRepository{DocumentRepository: DocumentRepository{db: &dynamodb.Client{}}}
	if r.db == nil {
		t.Errorf("db is nil")
	}
	if accessKeyIndexName != "access-key-index" {
		t.Errorf("accessKeyIndexName = %q, esperado access-key-index", accessKeyIndexName)
	}
}

func TestNfseListOpts_DefaultSort(t *testing.T) {
	opts := NfseListOpts{}
	if normalizeSort(opts.Sort) != "asc" {
		t.Errorf("sort default = %q, esperado asc", normalizeSort(opts.Sort))
	}
	if normalizeSort("desc") != "desc" {
		t.Error("sort desc não preservado")
	}
}

func TestDistributionCursorSK(t *testing.T) {
	if distributionCursorSK != "CURSOR" {
		t.Errorf("distributionCursorSK = %q", distributionCursorSK)
	}
}
