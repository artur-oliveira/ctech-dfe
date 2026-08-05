package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

func TestNfseRepository_TableAndIndexNames(t *testing.T) {
	db := &dynamodb.Client{}
	cfg := &config.Config{TablePrefix: "dev_dfe"}
	r := NewNfseRepository(db, cfg)
	if r.TableName != "dev_dfe_nfses" {
		t.Errorf("TableName = %q, esperado dev_dfe_nfses", r.TableName)
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
