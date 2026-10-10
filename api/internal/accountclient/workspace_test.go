package accountclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func workspaceServing(t *testing.T, status int, body string, gotPath *string) *WorkspaceClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &WorkspaceClient{http: srv.Client(), baseURL: srv.URL}
}

func TestMembershipReadsTheRole(t *testing.T) {
	var path string
	c := workspaceServing(t, http.StatusOK, `{"member":true,"role":"admin","kind":"organization"}`, &path)
	role, kind, member, err := c.membershipWithToken(context.Background(), "tok", "org_1", "usr_1")
	if err != nil || !member || role != "admin" || kind != "organization" {
		t.Fatalf("got %q %q %v %v", role, kind, member, err)
	}
	if path != "/v1.0/internal/organizations/org_1/members/usr_1" {
		t.Fatalf("path = %q", path)
	}
}

// "Not a member" is an answer, not an error.
func TestMembershipNonMemberIsARefusal(t *testing.T) {
	c := workspaceServing(t, http.StatusOK, `{"member":false}`, nil)
	role, _, member, err := c.membershipWithToken(context.Background(), "tok", "org_1", "usr_1")
	if err != nil || member || role != "" {
		t.Fatalf("got %q %v %v", role, member, err)
	}
}

// Any non-200 (a 403 means our own credential is wrong) is an outage.
func TestMembershipNon200IsAnError(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		c := workspaceServing(t, status, `{}`, nil)
		if _, _, _, err := c.membershipWithToken(context.Background(), "tok", "org_1", "usr_1"); err == nil {
			t.Fatalf("status %d: want an error", status)
		}
	}
}

func TestANilWorkspaceClientRefuses(t *testing.T) {
	var c *WorkspaceClient
	if _, _, member, err := c.Membership(context.Background(), "org_1", "usr_1"); err == nil || member {
		t.Fatalf("member=%v err=%v", member, err)
	}
	if _, err := c.Organizations(context.Background(), "usr_1"); err == nil {
		t.Fatal("want an error")
	}
	if NewWorkspace(Config{}) != nil {
		t.Fatal("an incomplete config must build no client")
	}
}

func TestOrganizationsListsWorkspaces(t *testing.T) {
	var path string
	c := workspaceServing(t, http.StatusOK,
		`{"organizations":[{"id":"org_1","display_name":"Escritório","role":"owner","kind":"organization"},{"id":"sp_1","display_name":"Pessoal","role":"owner","kind":"personal"}]}`, &path)
	got, err := c.organizationsWithToken(context.Background(), "tok", "usr_1")
	if err != nil || len(got) != 2 || got[0].ID != "org_1" || got[0].DisplayName != "Escritório" || got[1].Kind != "personal" {
		t.Fatalf("got %+v %v", got, err)
	}
	if path != "/v1.0/internal/users/usr_1/organizations" {
		t.Fatalf("path = %q", path)
	}
}
