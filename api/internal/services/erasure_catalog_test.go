package services

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

// TestErasureCatalogCoversEveryCDKTable fails when a table is added to
// cdk/lib/dynamodb-stack.ts without deciding how the account-deletion purge
// reaches it (inventory §5: "erase everything keyed by the org").
func TestErasureCatalogCoversEveryCDKTable(t *testing.T) {
	src, err := os.ReadFile("../../../cdk/lib/dynamodb-stack.ts")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)export type TableName = \((.*?)\)`).FindSubmatch(src)
	if block == nil {
		t.Fatal("TableName union not found in dynamodb-stack.ts")
	}
	cdk := map[string]bool{}
	for _, m := range regexp.MustCompile(`'([a-z_]+)'`).FindAllSubmatch(block[1], -1) {
		cdk[string(m[1])] = true
	}

	catalog := map[string]ErasureTable{}
	for _, tb := range ErasureCatalog() {
		if _, dup := catalog[tb.CDK]; dup {
			t.Fatalf("catalog lists %s twice", tb.CDK)
		}
		catalog[tb.CDK] = tb
	}
	var missing, stale []string
	for name := range cdk {
		if _, ok := catalog[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range catalog {
		if !cdk[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 || len(stale) > 0 {
		t.Fatalf("catalog drift: missing from catalog %v; not in CDK %v", missing, stale)
	}

	byName := map[string]ErasureTable{}
	for _, tb := range catalog {
		byName[tb.Name] = tb
	}
	for _, tb := range catalog {
		if tb.Scope == ScopeEnvDocs {
			if ev, ok := byName[tb.Events]; !ok || ev.Scope != ScopeDocEvents {
				t.Errorf("%s: events table %q is not a ScopeDocEvents entry", tb.Name, tb.Events)
			}
		}
	}
}

func TestInutEventPK(t *testing.T) {
	if got := InutEventPK(EnvProd, "C1"); got != "INUT#prod#C1" {
		t.Fatalf("InutEventPK = %q", got)
	}
}
