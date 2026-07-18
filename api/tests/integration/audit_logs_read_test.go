//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

func TestAuditLogService_List_ByOrgTimeIndex(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	svc := services.NewAuditLogService(auditRepo)

	txItem, err := auditRepo.BuildLogTxItem(orgPK, repositories.AuditResourceProduct, "PRODUCT_1", repositories.AuditActionCreate, "user-1", "Jane", nil)
	if err != nil {
		t.Fatalf("BuildLogTxItem: %v", err)
	}
	if err := auditRepo.TransactWrite(ctx, []types.TransactWriteItem{txItem}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := svc.List(ctx, orgPK, services.AuditLogQueryOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(res.Items))
	}
}

func TestAuditLogService_List_ByResourceHistory(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	svc := services.NewAuditLogService(auditRepo)

	tx1, _ := auditRepo.BuildLogTxItem(orgPK, repositories.AuditResourceProduct, "PRODUCT_1", repositories.AuditActionCreate, "user-1", "Jane", nil)
	tx2, _ := auditRepo.BuildLogTxItem(orgPK, repositories.AuditResourceProduct, "PRODUCT_1", repositories.AuditActionUpdate, "user-1", "Jane", nil)
	tx3, _ := auditRepo.BuildLogTxItem(orgPK, repositories.AuditResourceProduct, "PRODUCT_2", repositories.AuditActionCreate, "user-1", "Jane", nil)
	if err := auditRepo.TransactWrite(ctx, []types.TransactWriteItem{tx1}); err != nil {
		t.Fatalf("seed tx1: %v", err)
	}
	if err := auditRepo.TransactWrite(ctx, []types.TransactWriteItem{tx2}); err != nil {
		t.Fatalf("seed tx2: %v", err)
	}
	if err := auditRepo.TransactWrite(ctx, []types.TransactWriteItem{tx3}); err != nil {
		t.Fatalf("seed tx3: %v", err)
	}

	res, err := svc.List(ctx, orgPK, services.AuditLogQueryOpts{ResourceType: repositories.AuditResourceProduct, ResourceID: "PRODUCT_1"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2 (only PRODUCT_1's history)", len(res.Items))
	}
}

func TestAuditLogService_List_ByUserID_ScopedToOrg(t *testing.T) {
	ctx := context.Background()
	orgA := "CNPJ_" + randomCNPJ()
	orgB := "CNPJ_" + randomCNPJ()
	svc := services.NewAuditLogService(auditRepo)

	txA, _ := auditRepo.BuildLogTxItem(orgA, repositories.AuditResourceProduct, "PRODUCT_1", repositories.AuditActionCreate, "user-shared", "Jane", nil)
	txB, _ := auditRepo.BuildLogTxItem(orgB, repositories.AuditResourceProduct, "PRODUCT_1", repositories.AuditActionCreate, "user-shared", "Jane", nil)
	if err := auditRepo.TransactWrite(ctx, []types.TransactWriteItem{txA}); err != nil {
		t.Fatalf("seed txA: %v", err)
	}
	if err := auditRepo.TransactWrite(ctx, []types.TransactWriteItem{txB}); err != nil {
		t.Fatalf("seed txB: %v", err)
	}

	res, err := svc.List(ctx, orgA, services.AuditLogQueryOpts{UserID: "user-shared"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1 (only orgA's row for this user, orgB's must be filtered out)", len(res.Items))
	}
}
