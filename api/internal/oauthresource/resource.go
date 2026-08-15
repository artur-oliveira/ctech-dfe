// Package oauthresource exposes the OAuth protected-resource metadata and the
// versioned scope manifest owned by this service.
package oauthresource

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/gofiber/fiber/v3"
)

//go:embed scope-manifest.json
var manifestJSON []byte

type Scope struct {
	Name         string            `json:"name"`
	Descriptions map[string]string `json:"descriptions"`
	Visibility   string            `json:"visibility"`
	Status       string            `json:"status"`
}

type Manifest struct {
	SchemaVersion    int     `json:"schema_version"`
	ResourceServerID string  `json:"resource_server_id"`
	DisplayName      string  `json:"display_name"`
	Scopes           []Scope `json:"scopes"`
}

// ManifestDocument returns a fresh copy of the embedded manifest.
func ManifestDocument() (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode embedded OAuth scope manifest: %w", err)
	}
	return m, nil
}

// ManifestBytes returns a copy suitable for tooling and contract tests.
func ManifestBytes() []byte { return append([]byte(nil), manifestJSON...) }

// PublicActiveScopes returns the scopes clients may request interactively.
func PublicActiveScopes() ([]string, error) {
	m, err := ManifestDocument()
	if err != nil {
		return nil, err
	}
	var scopes []string
	for _, scope := range m.Scopes {
		if scope.Visibility == "public" && scope.Status == "active" {
			scopes = append(scopes, scope.Name)
		}
	}
	sort.Strings(scopes)
	return scopes, nil
}

// Register mounts RFC 9728 OAuth Protected Resource Metadata.
func Register(app *fiber.App, resource, authorizationServer string) {
	app.Get("/.well-known/oauth-protected-resource", func(c fiber.Ctx) error {
		scopes, err := PublicActiveScopes()
		if err != nil {
			return fiber.ErrInternalServerError
		}
		return c.JSON(fiber.Map{
			"resource":              resource,
			"authorization_servers": []string{authorizationServer},
			"scopes_supported":      scopes,
		})
	})
}
