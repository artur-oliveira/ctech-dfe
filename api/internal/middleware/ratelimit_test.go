package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/ratelimit"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
)

func limitedApp(user string) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if user != "" {
			c.Locals(middleware.UserIDKey, user)
		}
		return c.Next()
	})
	app.Use(middleware.RateLimit(ratelimit.NewMemoryCounter()))
	app.All("/x", func(c fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
	return app
}

func hit(t *testing.T, app *fiber.App, method string) *http.Response {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, "/x", nil))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestRateLimit_WriteBudgetIsSmallerThanRead(t *testing.T) {
	app := limitedApp("u1")
	for i := int64(0); i < middleware.WriteLimitPerWindow; i++ {
		if got := hit(t, app, http.MethodPost).StatusCode; got != http.StatusNoContent {
			t.Fatalf("write %d: status %d", i, got)
		}
	}
	denied := hit(t, app, http.MethodPost)
	if denied.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", denied.StatusCode)
	}
	if denied.Header.Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
	// The write budget being spent leaves reads untouched.
	if got := hit(t, app, http.MethodGet).StatusCode; got != http.StatusNoContent {
		t.Errorf("read after write limit: status %d", got)
	}
}

func TestRateLimit_PerUserAndAnonymousUnlimited(t *testing.T) {
	app := limitedApp("u1")
	for i := int64(0); i <= middleware.ReadLimitPerWindow; i++ {
		hit(t, app, http.MethodGet)
	}
	if got := hit(t, app, http.MethodGet).StatusCode; got != http.StatusTooManyRequests {
		t.Fatalf("read over budget: status %d, want 429", got)
	}
	anon := limitedApp("")
	for i := int64(0); i <= middleware.ReadLimitPerWindow; i++ {
		if got := hit(t, anon, http.MethodGet).StatusCode; got != http.StatusNoContent {
			t.Fatalf("no identity must not be limited, got %d", got)
		}
	}
}

func TestAfterAuth_RunsBothGuardsAndStopsOnRateLimit(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.UserIDKey, "u1")
		return c.Next()
	})
	guardCalls := 0
	guard := func(c fiber.Ctx) error { // stands in for the idempotency guard: continues the request itself
		guardCalls++
		return c.Next()
	}
	app.Use(middleware.AfterAuth(middleware.NewRateLimitCheck(ratelimit.NewMemoryCounter()), guard))
	app.Post("/x", func(c fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })

	for i := int64(0); i < middleware.WriteLimitPerWindow; i++ {
		if got := hit(t, app, http.MethodPost).StatusCode; got != http.StatusNoContent {
			t.Fatalf("write %d: status %d", i, got)
		}
	}
	if guardCalls != int(middleware.WriteLimitPerWindow) {
		t.Fatalf("second guard ran %d times, want %d (it must run for every allowed request)", guardCalls, middleware.WriteLimitPerWindow)
	}
	resp := hit(t, app, http.MethodPost)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("over budget: status %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" || resp.Header.Get("Content-Type") != "application/problem+json" {
		t.Fatalf("429 must carry Retry-After and problem+json, got %v", resp.Header)
	}
	if guardCalls != int(middleware.WriteLimitPerWindow) {
		t.Fatalf("a rate-limited request reached the second guard (calls=%d)", guardCalls)
	}
}
