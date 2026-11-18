package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// RequireScope guards the personal routes (notifications): a scoped token needs the scope, a token without any dfe:*
// scope (an identity-only session) passes like everywhere else in the API.
func TestRequireScope(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(ScopesKey, strings.Fields(c.Get("X-Scope")))
		return c.Next()
	})
	app.Get("/read", RequireScope("get.notifications"), func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Put("/write", RequireScope("update.notifications"), func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	cases := []struct {
		name, method, path, scope string
		want                      int
	}{
		{"read scope reads", http.MethodGet, "/read", "openid dfe:notifications:read", http.StatusNoContent},
		{"write scope writes", http.MethodPut, "/write", "openid dfe:notifications:write", http.StatusNoContent},
		{"read scope cannot write", http.MethodPut, "/write", "openid dfe:notifications:read", http.StatusForbidden},
		{"a document scope is not a notification scope", http.MethodGet, "/read", "openid dfe:nfes:read dfe:nfes:write", http.StatusForbidden},
		{"a scoped token that lacks it is refused (a session issued before the scope existed)", http.MethodGet, "/read", "openid dfe:bpes:read", http.StatusForbidden},
		{"an identity-only session passes", http.MethodGet, "/read", "openid profile", http.StatusNoContent},
		{"no scope claim passes", http.MethodPut, "/write", "", http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Scope", tc.scope)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}
