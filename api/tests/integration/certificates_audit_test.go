//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
)

// TestCertificateService_Delete_WritesAuditLog exercises the transactional
// Delete path (real DynamoDB, no S3 involved — Delete never touches S3).
//
// Upload's transactional wiring is NOT covered by an integration test here:
// Upload calls the real S3 client (s.awsClients.S3.PutObject) before writing
// to DynamoDB, and this codebase has no S3 test double (no localstack/minio
// client, no PFX test fixture) — see task-10-report.md for the accepted scope
// boundary. Upload's tx-building logic is covered by the repository-level
// unit test (TestCertificateRepository_BuildCreateTxItem in
// internal/repositories/certificates_test.go) plus code review.
//
// This test seeds the certificate row directly via certRepo.Create (bypassing
// Upload/S3 entirely, exactly like a cert that's already been uploaded) and
// then exercises certSvc.Delete, which is pure DynamoDB Get+TransactWrite.
func TestCertificateService_Delete_WritesAuditLog(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	const (
		alias     = "Audited Cert"
		md5       = "deadbeefdeadbeefdeadbeefdeadbeef"
		password  = "s3cr3t-password"
		s3Key     = "certs/test/deadbeef.pfx"
		expiresAt = "2030-01-01T00:00:00Z"
	)

	if _, err := certRepo.Create(ctx, orgPK, alias, md5, password, s3Key, expiresAt); err != nil {
		t.Fatalf("seed certRepo.Create: %v", err)
	}

	if err := certSvc.Delete(ctx, orgPK, md5, "user-1", "Jane Doe"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// The certificate row must be gone.
	item, err := certRepo.Get(ctx, orgPK, md5)
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if item != nil {
		t.Errorf("certificate should be gone after Delete, got %v", item)
	}

	// Exactly one audit_logs row, action=DELETE.
	logs, err := auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if len(logs.Items) != 1 {
		t.Fatalf("audit_logs rows after Delete = %d, want 1", len(logs.Items))
	}

	row := logs.Items[0]
	if av, ok := row["action"].(*types.AttributeValueMemberS); !ok || av.Value != repositories.AuditActionDelete {
		t.Errorf("action = %v, want %q", row["action"], repositories.AuditActionDelete)
	}
	if av, ok := row["resource_type"].(*types.AttributeValueMemberS); !ok || av.Value != repositories.AuditResourceCertificate {
		t.Errorf("resource_type = %v, want %q", row["resource_type"], repositories.AuditResourceCertificate)
	}
	if av, ok := row["resource_id"].(*types.AttributeValueMemberS); !ok || av.Value != md5 {
		t.Errorf("resource_id = %v, want %q", row["resource_id"], md5)
	}
	if av, ok := row["user_id"].(*types.AttributeValueMemberS); !ok || av.Value != "user-1" {
		t.Errorf("user_id = %v, want %q", row["user_id"], "user-1")
	}

	modsAV, ok := row["modifications"].(*types.AttributeValueMemberL)
	if !ok {
		t.Fatalf("modifications attribute missing or wrong type: %v", row["modifications"])
	}
	var mods []repositories.Modification
	if err := attributevalue.UnmarshalList(modsAV.Value, &mods); err != nil {
		t.Fatalf("unmarshal modifications: %v", err)
	}

	// password must never appear in the audit row, not even redacted as a
	// "before" value.
	for _, m := range mods {
		if m.Name == "password" {
			t.Fatalf("password field leaked into audit modifications: %+v", m)
		}
	}
	// alias must appear, since it's real data that changed (removed) on delete.
	found := false
	for _, m := range mods {
		if m.Name == "alias" {
			found = true
			if m.After != nil {
				t.Errorf("DELETE modification %q after = %v, want nil", m.Name, m.After)
			}
			if m.Before != alias {
				t.Errorf("DELETE modification %q before = %v, want %q", m.Name, m.Before, alias)
			}
		}
	}
	if !found {
		t.Error("expected an 'alias' modification in the DELETE audit row")
	}
}
