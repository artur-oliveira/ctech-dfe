package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const userCacheTTL = 300

// CurrentTermsAddendumVersion is the dfe-specific ToS/Privacy addendum version
// (see ui/src/app/terms-addendum). Bump it to re-gate every account on next
// login; a user's stored terms_addendum_version must match this exactly.
const CurrentTermsAddendumVersion = "2.0"

// CtechUserInfo mirrors the /v1.0/userinfo response from ctech-account.
type CtechUserInfo struct {
	Sub        string `json:"sub"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Picture    string `json:"picture,omitempty"`
	Verified   bool   `json:"email_verified"`
}

// UserService mirrors api/app/services/users.py.
type UserService struct {
	repo       *repositories.UserRepository
	cache      cache.Backend
	ctechURL   string
	httpClient *http.Client
	orgSvc     *OrganizationService
}

func NewUserService(
	repo *repositories.UserRepository,
	c cache.Backend,
	ctechURL string,
	orgSvc *OrganizationService,
) *UserService {
	return &UserService{
		repo:       repo,
		cache:      c,
		ctechURL:   ctechURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		orgSvc:     orgSvc,
	}
}

// GetOrCreate provisions a bare user row on first login (organizations: []).
// Profile fields are never written here — see CreateMinimal.
func (s *UserService) GetOrCreate(ctx context.Context, userID string) (map[string]types.AttributeValue, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}
	return s.repo.CreateMinimal(ctx, userID)
}

// AcceptTermsAddendum stamps the current dfe ToS/Privacy addendum as accepted now.
// This is dfe-owned data (not duplicated from ctech-account) — it records a fact
// about this product specifically, so it belongs on the local row.
func (s *UserService) AcceptTermsAddendum(ctx context.Context, userID string) error {
	if _, err := s.repo.Update(ctx, userID, map[string]any{
		"terms_addendum_version": CurrentTermsAddendumVersion,
		"terms_accepted_at":      time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	s.InvalidateCache(ctx, userID)
	return nil
}

// GetMe returns the user record, using cache.
func (s *UserService) GetMe(ctx context.Context, userID string) (map[string]types.AttributeValue, error) {
	cacheKey := fmt.Sprintf("user:%s", userID)
	if v, ok := cacheGetItem(ctx, s.cache, cacheKey); ok {
		return v, nil
	}
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, problem.NotFound("user not found")
	}
	cacheSetItem(ctx, s.cache, cacheKey, user, userCacheTTL)
	return user, nil
}

// GetMeData returns the enriched user response, using a dedicated cache key.
// Profile fields (email, name, verification) are fetched live from ctech-account's
// userinfo endpoint with the caller's own access token — not read from a local
// copy — so ctech-account remains the single place that owns them. A userinfo
// fetch failure degrades to a blank profile rather than failing the request.
func (s *UserService) GetMeData(ctx context.Context, userID, accessToken string) (map[string]any, error) {
	cacheKey := fmt.Sprintf("me:%s", userID)
	if v, ok := cacheGet[map[string]any](ctx, s.cache, cacheKey); ok {
		return *v, nil
	}

	user, err := s.GetMe(ctx, userID)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := attributevalue.UnmarshalMap(user, &m); err != nil {
		return nil, problem.InternalServer("failed to unmarshal user")
	}

	orgsRaw, _ := m["organizations"].([]any)
	enrichedOrgs := make([]map[string]any, 0, len(orgsRaw))
	for _, entry := range orgsRaw {
		ref, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		pk, _ := ref["pk"].(string)
		role, _ := ref["role"].(string)
		perms := ref["permissions"]
		if perms == nil {
			perms = []any{}
		}
		enriched := map[string]any{
			"pk":               pk,
			"role":             role,
			"permissions":      perms,
			"name":             pk,
			"description":      nil,
			"state_federation": nil,
		}
		if pk != "" && s.orgSvc != nil {
			if org, orgErr := s.orgSvc.Get(ctx, pk); orgErr == nil && org != nil {
				var orgMap map[string]any
				if attributevalue.UnmarshalMap(org, &orgMap) == nil {
					enriched["name"] = orgMap["name"]
					enriched["description"] = orgMap["description"]
					if person, ok := orgMap["person"].(map[string]any); ok {
						addrs, _ := person["addresses"].([]any)
						if len(addrs) == 0 {
							if addr := person["address"]; addr != nil {
								addrs = []any{addr}
							}
						}
						if len(addrs) > 0 {
							if addr, ok := addrs[0].(map[string]any); ok {
								enriched["state_federation"] = addr["state_federation"]
							}
						}
					}
				}
			}
		}
		enrichedOrgs = append(enrichedOrgs, enriched)
	}

	profile, err := s.GetUserInfo(ctx, accessToken)
	if err != nil {
		profile = &CtechUserInfo{}
	}
	username := profile.Email
	if idx := strings.Index(profile.Email, "@"); idx != -1 {
		username = profile.Email[:idx]
	}

	termsAddendumVersion, _ := m["terms_addendum_version"].(string)

	result := map[string]any{
		"username":                username,
		"email":                   profile.Email,
		"first_name":              profile.GivenName,
		"last_name":               profile.FamilyName,
		"email_verified":          profile.Verified,
		"is_enabled":              true, // dfe has no independent enable/disable of its own — ctech-account owns that
		"last_login_at":           m["last_login_at"],
		"organizations":           enrichedOrgs,
		"terms_addendum_accepted": termsAddendumVersion == CurrentTermsAddendumVersion,
	}

	cacheSet(ctx, s.cache, cacheKey, result, userCacheTTL)
	return result, nil
}

// ResolveActor returns (userID, userName) for audit attribution: the cache
// populated by GetMeData is checked first (cheap, already warm for any user who
// has hit GET /auth/me — which the frontend calls on every mount). On a miss it
// falls back to a live userinfo fetch, and on total failure to userID itself.
// Never blocks or errors the caller — audit attribution degrades, it never fails
// the underlying mutation.
func (s *UserService) ResolveActor(ctx context.Context, userID, accessToken string) (string, string) {
	if v, ok := cacheGet[map[string]any](ctx, s.cache, fmt.Sprintf("me:%s", userID)); ok {
		if name := actorNameFromMeCache(*v); name != "" {
			return userID, name
		}
	}

	profile, err := s.GetUserInfo(ctx, accessToken)
	if err == nil {
		if name := actorNameFromProfile(profile); name != "" {
			return userID, name
		}
	}

	return userID, userID
}

// actorNameFromMeCache extracts a display name from the map cached by
// GetMeData ("me:{userID}"): "first last", or the username, in that order.
func actorNameFromMeCache(m map[string]any) string {
	first, _ := m["first_name"].(string)
	last, _ := m["last_name"].(string)
	if full := strings.TrimSpace(first + " " + last); full != "" {
		return full
	}
	username, _ := m["username"].(string)
	return username
}

// actorNameFromProfile extracts a display name from a live ctech-account
// userinfo response: Name, then "given family", then the email local-part.
func actorNameFromProfile(p *CtechUserInfo) string {
	if p.Name != "" {
		return p.Name
	}
	if full := strings.TrimSpace(p.GivenName + " " + p.FamilyName); full != "" {
		return full
	}
	if idx := strings.Index(p.Email, "@"); idx != -1 {
		return p.Email[:idx]
	}
	return ""
}

// AttachToOrg appends an org membership entry to the user record.
// Mirrors UserService.attach_to_organization from api.
func (s *UserService) AttachToOrg(ctx context.Context, userID, orgPK, role string, permissions []string) error {
	return s.repo.AddOrgMembership(ctx, userID, orgPK, role, permissions)
}

// GetUserInfo fetches the OIDC userinfo endpoint from ctech-account.
func (s *UserService) GetUserInfo(ctx context.Context, accessToken string) (*CtechUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.ctechURL+"/v1.0/userinfo", nil)
	if err != nil {
		return nil, problem.InternalServer("failed to build userinfo request")
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, problem.InternalServer("userinfo request failed")
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, problem.Unauthorized("ctech userinfo error: " + string(body))
	}

	var info CtechUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, problem.InternalServer("invalid userinfo response")
	}
	return &info, nil
}

// InvalidateCache drops cached user and me entries.
func (s *UserService) InvalidateCache(ctx context.Context, userID string) {
	_ = s.cache.Delete(ctx, fmt.Sprintf("user:%s", userID))
	_ = s.cache.Delete(ctx, fmt.Sprintf("me:%s", userID))
}
