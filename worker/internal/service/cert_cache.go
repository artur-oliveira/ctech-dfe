package service

import (
	"sync"
	"time"
)

// certCacheTTL bounds how long a downloaded certificate blob is trusted
// before certCache forces a fresh S3 fetch — certificates are rotated rarely
// (~yearly validity), so this only guards against a stale blob outliving an
// org uploading a replacement, not against fast-changing data.
const certCacheTTL = 15 * time.Minute

// certCache holds already-downloaded PFX blobs (still base64-encoded, still
// password-encrypted — never the decrypted key) keyed by S3 object key, for
// the lifetime of this Lambda execution environment: a warm container
// reuses process memory across invocations (AWS Lambda execution
// environment reuse), so a burst of messages for the same org's certificate
// skips the S3 round trip after the first. The password is never cached —
// it keeps arriving per-message and decryption happens fresh on every call,
// exactly as before this cache existed; this only saves the download.
var certCache = newCertCacheStore(certCacheTTL)

type certCacheEntry struct {
	certB64   string
	fetchedAt time.Time
}

type certCacheStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]certCacheEntry
}

func newCertCacheStore(ttl time.Duration) *certCacheStore {
	return &certCacheStore{ttl: ttl, entries: make(map[string]certCacheEntry)}
}

func (c *certCacheStore) get(s3Key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[s3Key]
	if !ok || time.Since(entry.fetchedAt) > c.ttl {
		return "", false
	}
	return entry.certB64, true
}

func (c *certCacheStore) set(s3Key, certB64 string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[s3Key] = certCacheEntry{certB64: certB64, fetchedAt: time.Now()}
}
