package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TestCertificateRepository_BuildCreateTxItem is a pure unit test — no
// DynamoDB client, no S3 — mirroring TestBase_BuildPutTxItem's pattern of
// constructing a Base literal directly (base_test.go). It confirms
// BuildCreateTxItem assembles the same pk/sk/alias/md5/password/s3_key/
// expires_at/created_at fields as Create, without writing anything.
func TestCertificateRepository_BuildCreateTxItem(t *testing.T) {
	r := &CertificateRepository{Base: Base{TableName: "test_organization_certificates"}}

	txItem, item := r.BuildCreateTxItem("CNPJ_12345678000195", "My Cert", "abc123md5", "s3cr3t", "certs/CNPJ_12345678000195/abc123md5.pfx", "2030-01-01T00:00:00Z")

	if txItem.Put == nil {
		t.Fatal("expected Put transact item, got nil")
	}
	if *txItem.Put.TableName != r.TableName {
		t.Errorf("table name = %q, want %q", *txItem.Put.TableName, r.TableName)
	}

	wantSK := "CERTIFICATE_abc123md5"
	if got := item["sk"].(*types.AttributeValueMemberS).Value; got != wantSK {
		t.Errorf("sk = %q, want %q", got, wantSK)
	}
	if got := item["pk"].(*types.AttributeValueMemberS).Value; got != "CNPJ_12345678000195" {
		t.Errorf("pk = %q, want %q", got, "CNPJ_12345678000195")
	}
	if got := item["alias"].(*types.AttributeValueMemberS).Value; got != "My Cert" {
		t.Errorf("alias = %q, want %q", got, "My Cert")
	}
	if got := item["md5"].(*types.AttributeValueMemberS).Value; got != "abc123md5" {
		t.Errorf("md5 = %q, want %q", got, "abc123md5")
	}
	// password IS stored in the item by design (never returned in API
	// responses — see the delete(out, "password") pattern in the service —
	// but it is persisted, so it must be present here).
	if got := item["password"].(*types.AttributeValueMemberS).Value; got != "s3cr3t" {
		t.Errorf("password = %q, want %q", got, "s3cr3t")
	}
	if got := item["s3_key"].(*types.AttributeValueMemberS).Value; got != "certs/CNPJ_12345678000195/abc123md5.pfx" {
		t.Errorf("s3_key = %q, want %q", got, "certs/CNPJ_12345678000195/abc123md5.pfx")
	}
	if got := item["expires_at"].(*types.AttributeValueMemberS).Value; got != "2030-01-01T00:00:00Z" {
		t.Errorf("expires_at = %q, want %q", got, "2030-01-01T00:00:00Z")
	}
	if _, ok := item["created_at"]; !ok {
		t.Error("expected created_at to be set")
	}

	// The item returned from Put must match what's carried in the tx item.
	if txItem.Put.Item["md5"].(*types.AttributeValueMemberS).Value != "abc123md5" {
		t.Error("tx item's Put.Item does not match returned item")
	}
}

func TestCertificateRepository_BuildDeleteTxItem(t *testing.T) {
	r := &CertificateRepository{Base: Base{TableName: "test_organization_certificates"}}

	txItem := r.BuildDeleteTxItem("CNPJ_12345678000195", "abc123md5")

	if txItem.Delete == nil {
		t.Fatal("expected Delete transact item, got nil")
	}
	if *txItem.Delete.TableName != r.TableName {
		t.Errorf("table name = %q, want %q", *txItem.Delete.TableName, r.TableName)
	}
	wantSK := "CERTIFICATE_abc123md5"
	if got := txItem.Delete.Key["sk"].(*types.AttributeValueMemberS).Value; got != wantSK {
		t.Errorf("sk = %q, want %q", got, wantSK)
	}
	if got := txItem.Delete.Key["pk"].(*types.AttributeValueMemberS).Value; got != "CNPJ_12345678000195" {
		t.Errorf("pk = %q, want %q", got, "CNPJ_12345678000195")
	}
}
