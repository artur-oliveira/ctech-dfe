package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/idempotency"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
)

func idemApp(calls *int) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.UserIDKey, "u1")
		return c.Next()
	})
	app.Use(middleware.Idempotency(&idempotency.MemoryStore{}))
	h := func(c fiber.Ctx) error { *calls++; return c.Status(http.StatusCreated).SendString("ok") }
	app.Post("/v1.0/nfes", h)
	app.Post("/v1.0/auth/terms-addendum/accept", h)
	app.Get("/v1.0/nfes", h)
	return app
}

func idemPost(t *testing.T, app *fiber.App, path, key string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if key != "" {
		req.Header.Set(idempotency.HeaderKey, key)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestIdempotency_PostNeedsKeyAndReplays(t *testing.T) {
	calls := 0
	app := idemApp(&calls)
	if got := idemPost(t, app, "/v1.0/nfes", "").StatusCode; got != http.StatusBadRequest {
		t.Fatalf("no key: status %d, want 400", got)
	}
	idemPost(t, app, "/v1.0/nfes", "k1")
	replay := idemPost(t, app, "/v1.0/nfes", "k1")
	if replay.StatusCode != http.StatusCreated || replay.Header.Get(idempotency.HeaderReplay) != "true" || calls != 1 {
		t.Fatalf("replay: status %d, header %q, handler calls %d", replay.StatusCode, replay.Header.Get(idempotency.HeaderReplay), calls)
	}
}

func TestIdempotency_OptionalSurfaceAndReads(t *testing.T) {
	calls := 0
	app := idemApp(&calls)
	if got := idemPost(t, app, "/v1.0/auth/terms-addendum/accept", "").StatusCode; got != http.StatusCreated {
		t.Errorf("optional surface without key: status %d, want 201", got)
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1.0/nfes", nil))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Errorf("GET must pass untouched: %v %v", resp, err)
	}
}
