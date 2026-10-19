package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/ratelimit"
	"gopkg.aoctech.app/api-commons/ratelimit/fiberlimit"
)

// Per-user request budgets. Writes (every issuance and registry change) get the
// smaller one; reads the larger, since the UI polls and lists. The budgets are
// abuse guards, far above what a person or an integration normally does.
const (
	ReadLimitPerWindow  int64 = 1200
	WriteLimitPerWindow int64 = 300
	RateLimitWindow           = time.Minute

	rateLimitReadPrefix  = "dfe:rl:read"
	rateLimitWritePrefix = "dfe:rl:write"
)

// NewCounter picks the limiter's storage: the shared Valkey when the cache is
// Redis-backed (fleet-wide cap), otherwise a per-process counter.
func NewCounter(b cache.Backend) ratelimit.Counter {
	if rb, ok := b.(*cache.RedisBackend); ok {
		return ratelimit.NewValkeyCounter(rb.Client())
	}
	return ratelimit.NewMemoryCounter()
}

// RateLimit caps requests per authenticated user, with a smaller budget for
// writes. It must run after authentication (the key is the verified subject: the
// organization header is client-supplied and would let a caller mint buckets).
// Fail open: a counter outage lets requests through; availability of the
// product beats abuse protection here. Denials answer 429 with Retry-After.
func RateLimit(counter ratelimit.Counter) fiber.Handler {
	policy := func(prefix string, max int64) fiber.Handler {
		return fiberlimit.NewPolicy(fiberlimit.PolicyConfig{
			Limiter: &ratelimit.Limiter{Counter: counter, Prefix: prefix, Max: max, Window: RateLimitWindow, FailMode: ratelimit.FailOpen},
			Key:     func(c fiber.Ctx) string { return ratelimit.Key(GetUserID(c)) },
		})
	}
	read, write := policy(rateLimitReadPrefix, ReadLimitPerWindow), policy(rateLimitWritePrefix, WriteLimitPerWindow)
	return func(c fiber.Ctx) error {
		if isMutating(c.Method()) {
			return write(c)
		}
		return read(c)
	}
}
