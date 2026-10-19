package app

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

func slowApp(t *testing.T, timeout time.Duration) *fiber.App {
	t.Helper()
	app := newFiberApp(&config.Config{RequestTimeout: timeout, CorsAllowedOrigins: []string{"https://dfe.example"}})
	app.Get("/v1.0/slow", func(c fiber.Ctx) error {
		select {
		case <-c.Context().Done():
			return c.Context().Err()
		case <-time.After(300 * time.Millisecond):
			return c.SendString("finished")
		}
	})
	return app
}

// A call that outlives the request budget ends with it and answers a retryable 504 problem.
func TestARouteIsBoundedByTheRequestTimeout(t *testing.T) {
	res, err := slowApp(t, 40*time.Millisecond).Test(httptest.NewRequest("GET", "/v1.0/slow", nil), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != fiber.StatusGatewayTimeout || !strings.Contains(string(body), "/problems/timeout") {
		t.Fatalf("status %d body %s, want a 504 problem of type /problems/timeout", res.StatusCode, body)
	}
}

// Zero turns it off.
func TestZeroRequestTimeoutDisablesIt(t *testing.T) {
	res, err := slowApp(t, 0).Test(httptest.NewRequest("GET", "/v1.0/slow", nil), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("status %v err %v, want 200", res, err)
	}
}
