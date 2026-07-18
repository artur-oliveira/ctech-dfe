package service

import (
	"testing"
	"time"
)

func TestCertCacheStore_HitAndMiss(t *testing.T) {
	c := newCertCacheStore(time.Minute)

	if _, ok := c.get("k1"); ok {
		t.Fatal("expected miss on empty cache")
	}

	c.set("k1", "cert-bytes")
	got, ok := c.get("k1")
	if !ok || got != "cert-bytes" {
		t.Errorf("get(k1) = (%q, %v), want (cert-bytes, true)", got, ok)
	}

	if _, ok := c.get("k2"); ok {
		t.Error("expected miss for a different key")
	}
}

func TestCertCacheStore_ExpiresAfterTTL(t *testing.T) {
	c := newCertCacheStore(-time.Second) // already-expired TTL
	c.set("k1", "cert-bytes")
	if _, ok := c.get("k1"); ok {
		t.Error("expected miss once TTL has elapsed")
	}
}
