package services

import (
	"context"
	"testing"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
)

func TestUserService_ResolveActor_CacheHit(t *testing.T) {
	c := cache.NewMemoryBackend(10)
	repo := &repositories.UserRepository{} // not touched on a cache hit
	svc := NewUserService(repo, c, "http://ctech.invalid", nil, nil)

	cacheSet(context.Background(), c, "me:user-1", map[string]any{
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
