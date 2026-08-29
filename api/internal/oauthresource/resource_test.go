package oauthresource

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestProtectedResourceMetadata(t *testing.T) {
	app := fiber.New()
	Register(app, "https://api.example.test", "https://accounts.example.test")
	resp, err := app.Test(httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		Scopes               []string `json:"scopes_supported"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Resource != "https://api.example.test" || len(body.AuthorizationServers) != 1 || body.AuthorizationServers[0] != "https://accounts.example.test" {
		t.Fatalf("unexpected metadata: %#v", body)
	}
	// Contado a partir do manifesto, não fixo: um escopo novo não pode quebrar
	// este teste, mas um escopo que some do metadata tem que quebrar.
	manifest, err := ManifestDocument()
	if err != nil {
		t.Fatal(err)
	}
	if len(body.Scopes) != len(manifest.Scopes) {
		t.Fatalf("scopes_supported = %d, want %d", len(body.Scopes), len(manifest.Scopes))
	}
}
