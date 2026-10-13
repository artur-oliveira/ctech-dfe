package services

import (
	"context"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/testsupport"
)

func TestPurgeObjects_EveryVersionOfOnePrefix(t *testing.T) {
	ctx := context.Background()
	s := testsupport.NewObjectStore()
	s.Put("b", "certs/C1/a.pfx", "v1")
	s.Put("b", "certs/C1/a.pfx", "v2")
	s.Put("b", "certs/C1/b.pfx", "v1")
	s.Put("b", "certs/C10/a.pfx", "other company")

	n, err := purgeObjects(ctx, s, "b", "certs/C1/", false)
	if err != nil || n != 3 {
		t.Fatalf("purged %d (%v), want 3", n, err)
	}
	if got := s.Keys("b"); len(got) != 1 || got[0] != "certs/C10/a.pfx" {
		t.Fatalf("remaining %v, want only certs/C10/a.pfx", got)
	}
}

func TestPurgeObjects_ExactKeyLeavesSiblings(t *testing.T) {
	ctx := context.Background()
	s := testsupport.NewObjectStore()
	s.Put("b", "nfe/prod/CNPJ_1/K1.xml", "x")
	s.Put("b", "nfe/prod/CNPJ_1/K1.xml.bak", "sibling")

	if _, err := purgeObjects(ctx, s, "b", "nfe/prod/CNPJ_1/K1.xml", true); err != nil {
		t.Fatal(err)
	}
	if s.Versions("b", "nfe/prod/CNPJ_1/K1.xml") != 0 || s.Versions("b", "nfe/prod/CNPJ_1/K1.xml.bak") != 1 {
		t.Fatalf("exact purge touched the wrong keys: %v", s.Keys("b"))
	}
}

func TestPurgeObjects_EmptyPrefixIsANoop(t *testing.T) {
	s := testsupport.NewObjectStore()
	s.Put("b", "anything", "x")
	if n, err := purgeObjects(context.Background(), s, "b", "", false); err != nil || n != 0 || s.Versions("b", "anything") != 1 {
		t.Fatal("an empty prefix must never sweep the bucket")
	}
}
