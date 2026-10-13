//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestErasureRepository_PagesDeletesAndNeverCreates(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewErasureRepository(db, cfg)
	const table = "organization_products"
	org := "erasure-repo-org"

	for i := range 30 {
		putItem(t, table, map[string]any{"pk": org, "sk": fmt.Sprintf("PRODUCT_%02d", i)})
	}
	var keys []map[string]types.AttributeValue
	var start map[string]types.AttributeValue
	for {
		page, err := repo.QueryPartition(ctx, table, org, start)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range page.Items {
			keys = append(keys, map[string]types.AttributeValue{"pk": it["pk"], "sk": it["sk"]})
		}
		if page.Next == nil {
			break
		}
		start = page.Next
	}
	if len(keys) != 30 {
		t.Fatalf("paged %d rows, want 30", len(keys))
	}
	if err := repo.DeleteKeys(ctx, table, keys); err != nil { // > 25: two batches
		t.Fatal(err)
	}
	if page, _ := repo.QueryPartition(ctx, table, org, nil); len(page.Items) != 0 {
		t.Fatalf("%d rows left after DeleteKeys", len(page.Items))
	}

	missing := map[string]types.AttributeValue{"pk": repositories.SAV(org), "sk": repositories.SAV("PRODUCT_99")}
	if err := repo.SetAttrs(ctx, table, missing, map[string]types.AttributeValue{"name": repositories.SAV("x")}); err != nil {
		t.Fatal(err)
	}
	if item, _ := repo.GetItem(ctx, table, missing); item != nil {
		t.Fatal("SetAttrs created a row: an anonymization must never resurrect one")
	}
	if existed, err := repo.DeleteKey(ctx, table, missing); err != nil || existed {
		t.Fatalf("DeleteKey of an absent row = %v, %v; want false, nil", existed, err)
	}
}
