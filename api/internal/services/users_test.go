package services

import (
	"context"
	"testing"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestUserService_ResolveActor_CacheHit(t *testing.T) {
	c := cache.NewMemoryBackend(10)
	repo := &repositories.UserRepository{} // not touched on a cache hit
	svc := NewUserService(repo, c, "http://ctech.invalid", nil, nil)

	CacheSet(context.Background(), c, "dfe:me:user-1", map[string]any{
		"username": "jane", "email": "jane@example.com", "first_name": "Jane", "last_name": "Doe",
	}, userCacheTTL)

	_, name := svc.ResolveActor(context.Background(), "user-1", "token-doesnt-matter")
	if name != "Jane Doe" {
		t.Errorf("name = %q, want %q", name, "Jane Doe")
	}
}

func TestUserService_ResolveActor_NoCacheNoNetwork_FallsBackToUserID(t *testing.T) {
	c := cache.NewMemoryBackend(10)
	repo := &repositories.UserRepository{}
	svc := NewUserService(repo, c, "http://127.0.0.1:1", nil, nil) // unroutable — GetUserInfo fails fast

	_, name := svc.ResolveActor(context.Background(), "user-2", "token")
	if name != "user-2" {
		t.Errorf("name = %q, want fallback to userID %q", name, "user-2")
	}
}

// A company linked from the ctech-account handoff has to appear in /auth/me at
// once. GetMeData rebuilds the response from ListByUser, which caches the org
// list under its own key for five minutes — so clearing "me:" alone rebuilt the
// same stale list, the browser saw no new organization, and it kept sending the
// previously selected organization's pk on every request.
func TestUserService_InvalidateCache_DropsTheOrgList(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryBackend(10)
	svc := NewUserService(&repositories.UserRepository{}, c, "http://ctech.invalid", nil, nil)

	CacheSet(ctx, c, userMeCacheKey("user-1"), map[string]any{"username": "jane"}, userCacheTTL)
	CacheSet(ctx, c, userOrgsCacheKey("user-1"), []Membership{{OrgPK: "old-company"}}, userOrgsCacheTTL)

	svc.InvalidateCache(ctx, "user-1")

	for _, key := range []string{userMeCacheKey("user-1"), userOrgsCacheKey("user-1"), userItemCacheKey("user-1")} {
		if _, ok := CacheGet[any](ctx, c, key); ok {
			t.Errorf("%s survived InvalidateCache", key)
		}
	}
}
