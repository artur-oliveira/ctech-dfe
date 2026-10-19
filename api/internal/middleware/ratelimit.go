package middleware

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/api-commons/ratelimit"
)

// Per-user request budgets. Writes (every issuance and registry change) get the
// smaller one; reads the larger, since the UI polls and lists. The budgets are
// abuse guards, far above what a person or an integration normally does.
const (
	ReadLimitPerWindow  int64 = 1200
	WriteLimitPerWindow int64 = 300
	RateLimitWindow           = time.Minute

	rateLimitDetail = "Too many requests. Please slow down and try again later."

	// IETF draft-ietf-httpapi-ratelimit-headers, as the shared fiberlimit sets them.
	rateLimitHeaderLimit     = "RateLimit-Limit"
	rateLimitHeaderRemaining = "RateLimit-Remaining"
	rateLimitHeaderReset     = "RateLimit-Reset"

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

// RateLimitCheck decides one request: stop reports that the response was
// already written (429), so the caller must not continue. It never calls
// c.Next(), which is what lets AfterAuth chain it with another guard.
type RateLimitCheck func(c fiber.Ctx) (stop bool, err error)

// NewRateLimitCheck caps requests per authenticated user, with a smaller budget
// for writes. It must run after authentication (the key is the verified subject:
// the organization header is client-supplied and would let a caller mint
// buckets). Fail open: a counter outage lets requests through; availability of
// the product beats abuse protection here. Denials answer 429 with Retry-After.
func NewRateLimitCheck(counter ratelimit.Counter) RateLimitCheck {
	limiter := func(prefix string, max int64) *ratelimit.Limiter {
		return &ratelimit.Limiter{Counter: counter, Prefix: prefix, Max: max, Window: RateLimitWindow, FailMode: ratelimit.FailOpen}
	}
	read, write := limiter(rateLimitReadPrefix, ReadLimitPerWindow), limiter(rateLimitWritePrefix, WriteLimitPerWindow)
	return func(c fiber.Ctx) (bool, error) {
		id := ratelimit.Key(GetUserID(c))
		if id == "" {
			return false, nil
		}
		lim := read
		if isMutating(c.Method()) {
			lim = write
		}
		d, err := lim.Decide(c.Context(), id)
		if err != nil {
			slog.WarnContext(c.Context(), "rate limiter backend error", "prefix", lim.Prefix, "err", err)
		}
		if d.Outcome == ratelimit.Deny {
			setRateLimitHeaders(c, d)
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(d.RetryAfterSeconds()))
			return true, problem.TooManyRequestsAfter(rateLimitDetail, d.RetryAfterSeconds()).Send(c)
		}
		if err == nil {
			setRateLimitHeaders(c, d)
		}
		return false, nil
	}
}

// RateLimit is NewRateLimitCheck as a standalone middleware.
func RateLimit(counter ratelimit.Counter) fiber.Handler {
	check := NewRateLimitCheck(counter)
	return func(c fiber.Ctx) error {
		if stop, err := check(c); stop || err != nil {
			return err
		}
		return c.Next()
	}
}

// AfterAuth chains the rate limit check and then the guard that continues the
// request itself (idempotency). Verifier.WithAfterAuth holds a single handler, so
// two guards would silently replace one another without this.
func AfterAuth(rl RateLimitCheck, next fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		if stop, err := rl(c); stop || err != nil {
			return err
		}
		return next(c)
	}
}

func setRateLimitHeaders(c fiber.Ctx, d ratelimit.Decision) {
	c.Set(rateLimitHeaderLimit, strconv.FormatInt(d.Limit, 10))
	c.Set(rateLimitHeaderRemaining, strconv.FormatInt(d.Remaining, 10))
	c.Set(rateLimitHeaderReset, strconv.Itoa(d.RetryAfterSeconds()))
}
