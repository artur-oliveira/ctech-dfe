package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/idempotency"
)

// IdempotencyTable is the DynamoDB table (after "{prefix}_") holding claims and
// replayable answers; TTL on `ttl`, created in cdk/lib/dynamodb-stack.ts.
const IdempotencyTable = "idempotency"

// idempotencyScopeSep joins the user and the organization header into one key
// namespace. The header is client-supplied, which is harmless here: it can only
// narrow the caller's own namespace, never reach another user's.
const idempotencyScopeSep = "#"

// optionalIdempotencyPrefixes are POST surfaces where the key is honored when
// sent but not demanded: account-level actions that are idempotent by nature or
// carry their own key towards billing.
var optionalIdempotencyPrefixes = []string{
	"/v1.0/billing/",
	"/v1.0/auth/",
	"/v1.0/invitations/",
}

// optionalIdempotencySuffixes are POSTs that write nothing.
var optionalIdempotencySuffixes = []string{"/cargo-preview"}

func idempotencyOptional(path string) bool {
	for _, p := range optionalIdempotencyPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	for _, s := range optionalIdempotencySuffixes {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}

// Idempotency makes every authenticated POST safe to retry: Idempotency-Key is
// required (400 without it) except on the small optional list above. Like the
// subscription gate it is default-deny by shape, so a POST added tomorrow is
// covered without anybody remembering. It must run after authentication (the
// scope is the verified user). Fail closed: if the store is down the request is
// refused with a retryable 503 rather than issuing a fiscal document unprotected.
func Idempotency(store idempotency.Store) fiber.Handler {
	scope := func(c fiber.Ctx) (string, bool) {
		user := GetUserID(c)
		return user + idempotencyScopeSep + c.Get(OrgHeader), user != ""
	}
	cfg := idempotency.Config{Store: store, Scope: scope, Methods: []string{fiber.MethodPost}}
	cfg.Required = true
	required := idempotency.New(cfg)
	cfg.Required = false
	optional := idempotency.New(cfg)
	return func(c fiber.Ctx) error {
		if idempotencyOptional(c.Path()) {
			return optional(c)
		}
		return required(c)
	}
}
