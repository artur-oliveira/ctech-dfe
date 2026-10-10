package services

import (
	"context"
	"errors"
	"testing"

	"gopkg.aoctech.app/api-commons/accountorgs"
	"gopkg.aoctech.app/api-commons/cache"
)

type fakeWorkspaceSource struct {
	role, kind string
	member     bool
	err        error
	orgs       []accountorgs.Organization
	listErr    error
	calls      int
}

func (f *fakeWorkspaceSource) Membership(_ context.Context, _, _ string) (accountorgs.Membership, error) {
	f.calls++
	if f.err != nil {
		return accountorgs.Membership{}, f.err
	}
	return accountorgs.Membership{Member: f.member, Role: f.role, Kind: f.kind}, nil
}

func (f *fakeWorkspaceSource) Organizations(_ context.Context, _ string) ([]accountorgs.Organization, error) {
	return f.orgs, f.listErr
}

func TestRoleIsReadAndCached(t *testing.T) {
	src := &fakeWorkspaceSource{role: AccountRoleAdmin, kind: "organization", member: true}
	s := NewWorkspaceRoleService(src, cache.NewMemoryBackend(16))
	for range 2 {
		role, err := s.Role(context.Background(), "org_1", "usr_1")
		if err != nil || role != AccountRoleAdmin {
			t.Fatalf("role=%q err=%v", role, err)
		}
	}
	if src.calls != 1 {
		t.Fatalf("calls = %d, want 1 (cached)", src.calls)
	}
}

func TestANonMemberHasNoRole(t *testing.T) {
	s := NewWorkspaceRoleService(&fakeWorkspaceSource{}, cache.NewMemoryBackend(16))
	if role, err := s.Role(context.Background(), "org_1", "usr_1"); err != nil || role != "" {
		t.Fatalf("role=%q err=%v", role, err)
	}
}

// A personal space is never an organization whose plan the DF-e manages.
func TestARoleInASpaceIsNoRole(t *testing.T) {
	s := NewWorkspaceRoleService(&fakeWorkspaceSource{role: AccountRoleOwner, kind: "personal", member: true}, cache.NewMemoryBackend(16))
	if role, err := s.Role(context.Background(), "sp_1", "usr_1"); err != nil || role != "" {
		t.Fatalf("role=%q err=%v", role, err)
	}
}

func TestRoleOutageIsAnErrorAndNotCached(t *testing.T) {
	src := &fakeWorkspaceSource{err: errors.New("down")}
	s := NewWorkspaceRoleService(src, cache.NewMemoryBackend(16))
	if _, err := s.Role(context.Background(), "org_1", "usr_1"); err == nil {
		t.Fatal("an outage must be an error")
	}
	src.err, src.role, src.kind, src.member = nil, AccountRoleOwner, "", true
	if role, err := s.Role(context.Background(), "org_1", "usr_1"); err != nil || role != AccountRoleOwner {
		t.Fatalf("after recovery role=%q err=%v (an outage must not be cached)", role, err)
	}
}

func TestANilRoleServiceFailsClosed(t *testing.T) {
	var s *WorkspaceRoleService
	if _, err := s.Role(context.Background(), "org_1", "usr_1"); err == nil {
		t.Fatal("a nil service must refuse")
	}
	if s.OrganizationName(context.Background(), "org_1", "usr_1") != "" {
		t.Fatal("a nil service names nothing")
	}
}

func TestMayManageBilling(t *testing.T) {
	for role, want := range map[string]bool{AccountRoleOwner: true, AccountRoleAdmin: true, "member": false, "viewer": false, "": false} {
		if MayManageBilling(role) != want {
			t.Errorf("MayManageBilling(%q) = %v", role, !want)
		}
	}
}

func TestOrganizationNameIsBestEffort(t *testing.T) {
	s := NewWorkspaceRoleService(&fakeWorkspaceSource{orgs: []accountorgs.Organization{{ID: "org_1", DisplayName: "Escritório Silva"}}}, cache.NewMemoryBackend(16))
	if got := s.OrganizationName(context.Background(), "org_1", "usr_1"); got != "Escritório Silva" {
		t.Fatalf("name = %q", got)
	}
	failing := NewWorkspaceRoleService(&fakeWorkspaceSource{listErr: errors.New("down")}, cache.NewMemoryBackend(16))
	if got := failing.OrganizationName(context.Background(), "org_1", "usr_1"); got != "" {
		t.Fatalf("name on failure = %q", got)
	}
}
