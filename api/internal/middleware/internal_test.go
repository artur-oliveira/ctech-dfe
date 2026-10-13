package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRequireInternalScope(t *testing.T) {
	const scope = "internal:dfe:erasure-eligibility"
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error {
		c.Locals(SessionIDKey, c.Get("X-Sid"))
		c.Locals(ScopesKey, strings.Fields(c.Get("X-Scope")))
		return c.Next()
	}, RequireInternalScope(scope), func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	cases := []struct {
		name, sid, scope string
		want             int
	}{
		{"service token with the scope", "", scope, fiber.StatusOK},
		{"service token without it", "", "internal:dfe:other", fiber.StatusForbidden},
		{"user session carrying the scope", "sid-1", scope, fiber.StatusForbidden},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/x", nil)
		req.Header.Set("X-Sid", tc.sid)
		req.Header.Set("X-Scope", tc.scope)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, resp.StatusCode, tc.want)
		}
	}
}
