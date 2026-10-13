//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// TestGetOrCreate_DoesNotResurrectALockedUser: GET /auth/me provisions the
// users row on first sight. For a locked or erased sub it must refuse, or a
// late token would bring back the row the purge deleted.
func TestGetOrCreate_DoesNotResurrectALockedUser(t *testing.T) {
	ctx := context.Background()
	svc := services.NewUserService(userRepo, memCache, "", nil, nil).
		WithErasureLock(func(context.Context, string) (bool, error) { return true, nil })

	_, err := svc.GetOrCreate(ctx, "locked-user-1")
	var p *problem.Problem
	if !errors.As(err, &p) || p.Type != problem.TypeAccountPendingDeletion {
		t.Fatalf("err = %v, want account-pending-deletion", err)
	}
	if item, _ := userRepo.GetByID(ctx, "locked-user-1"); item != nil {
		t.Fatal("a users row was created for a locked sub")
	}
}
