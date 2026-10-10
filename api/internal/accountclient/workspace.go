package accountclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gopkg.aoctech.app/api-commons/oauth2client"
)

// MemberScope lets this client ask whether a person belongs to an organization
// and with which ladder role. The DF-e uses the answer to decide who may manage
// the organization's subscription (docs/specs/2026-10-10-organization-subscription.md § 2).
const MemberScope = "internal:account:org-member"

// ListScope lets this client list a person's organizations. Used for the
// organization's display name on the plan screen and by cmd/migrate-billing-org;
// never for an authorization decision.
const ListScope = "internal:account:user-organizations"

// WorkspaceClient asks ctech-account about organization membership.
//
// A separate type and a separate credential from Client (reach): the two hold
// different scopes, and one being wrong must not disable the other. One token
// manager per scope so a grant missing for the list never breaks the role check.
type WorkspaceClient struct {
	http         *http.Client
	memberTokens *oauth2client.TokenManager
	listTokens   *oauth2client.TokenManager
	baseURL      string
}

// NewWorkspace builds the client, or nil when the credential is not configured.
// Callers treat nil as a refusal, like Client.
func NewWorkspace(cfg Config) *WorkspaceClient {
	if cfg.BaseURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	hc := &http.Client{Timeout: requestTimeout}
	return &WorkspaceClient{
		http:         hc,
		memberTokens: oauth2client.New(hc, cfg.Cache, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, MemberScope),
		listTokens:   oauth2client.New(hc, cfg.Cache, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, ListScope),
		baseURL:      strings.TrimSuffix(cfg.BaseURL, "/"),
	}
}

type membershipResponse struct {
	Member bool   `json:"member"`
	Role   string `json:"role"`
	Kind   string `json:"kind"`
}

// Membership answers whether userID belongs to organizationID and with which
// role. ("", "", false, nil) is a refusal; any error is an outage the caller
// must refuse on.
func (c *WorkspaceClient) Membership(ctx context.Context, organizationID, userID string) (string, string, bool, error) {
	if c == nil {
		return "", "", false, fmt.Errorf("ctech-account workspace client is not configured")
	}
	token, err := c.memberTokens.Get(ctx)
	if err != nil {
		return "", "", false, fmt.Errorf("minting a service token: %w", err)
	}
	return c.membershipWithToken(ctx, token, organizationID, userID)
}

func (c *WorkspaceClient) membershipWithToken(ctx context.Context, token, organizationID, userID string) (string, string, bool, error) {
	path := fmt.Sprintf("%s/v1.0/internal/organizations/%s/members/%s",
		c.baseURL, url.PathEscape(organizationID), url.PathEscape(userID))
	var out membershipResponse
	if err := c.getJSON(ctx, token, path, maxBody, &out); err != nil {
		return "", "", false, err
	}
	if !out.Member {
		return "", "", false, nil
	}
	return out.Role, out.Kind, true, nil
}

// Workspace is one organization a person belongs to.
type Workspace struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Kind        string `json:"kind"`
}

// IsOrganization reports a workspace this product bills (absent kind included).
func (w Workspace) IsOrganization() bool { return isOrganization(w.Kind) }

// Organizations lists the person's workspaces. Information only.
func (c *WorkspaceClient) Organizations(ctx context.Context, userID string) ([]Workspace, error) {
	if c == nil {
		return nil, fmt.Errorf("ctech-account workspace client is not configured")
	}
	token, err := c.listTokens.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("minting a service token: %w", err)
	}
	return c.organizationsWithToken(ctx, token, userID)
}

func (c *WorkspaceClient) organizationsWithToken(ctx context.Context, token, userID string) ([]Workspace, error) {
	path := fmt.Sprintf("%s/v1.0/internal/users/%s/organizations", c.baseURL, url.PathEscape(userID))
	var out struct {
		Organizations []Workspace `json:"organizations"`
	}
	if err := c.getJSON(ctx, token, path, maxListBody, &out); err != nil {
		return nil, err
	}
	return out.Organizations, nil
}

// maxListBody caps the organization list: a person in many organizations
// outgrows the 8 KiB that suits a single membership answer.
const maxListBody = 256 << 10

// getJSON is the request both calls make: bearer token, 200 or error, capped body.
func (c *WorkspaceClient) getJSON(ctx context.Context, token, path string, limit int64, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling ctech-account: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ctech-account answered %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, limit)).Decode(out); err != nil {
		return fmt.Errorf("decoding the answer: %w", err)
	}
	return nil
}
