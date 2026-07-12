//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
)

// seedCertForOrg writes a non-expired certificate row directly (no S3), so
// branch-certificate inheritance can be exercised without a real bucket.
func seedCertForOrg(t *testing.T, orgPK string) {
	t.Helper()
	if _, err := certRepo.Create(context.Background(), orgPK, "matriz", "abc123", "pw", "certs/x.pfx", "2999-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
}

func TestMembershipLifecycle(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	const owner, member = "owner-sub", "member-sub"

	if err := memberSvc.Create(ctx, orgPK, owner, repositories.RoleOwner, "", "Owner Name", nil); err != nil {
		t.Fatal(err)
	}
	if err := memberSvc.Create(ctx, orgPK, member, repositories.RoleViewer, owner, "Member Name", nil); err != nil {
		t.Fatal(err)
	}

	m, err := memberSvc.Get(ctx, orgPK, member)
	if err != nil || m == nil {
		t.Fatalf("Get member: %v %v", m, err)
	}
	if m.Role != repositories.RoleViewer {
		t.Errorf("role = %q, want VIEWER", m.Role)
	}

	members, err := memberSvc.ListByOrg(ctx, orgPK)
	if err != nil || len(members) != 2 {
		t.Fatalf("ListByOrg = %d members (%v), want 2", len(members), err)
	}
	// The members screen renders these — a missing name shows a bare UUID and a
	// missing created_at renders "Invalid Date".
	for _, mm := range members {
		if mm.Name == "" {
			t.Errorf("member %s has no name snapshot", mm.UserID)
		}
		if mm.CreatedAt == "" {
			t.Errorf("member %s has no created_at", mm.UserID)
		}
	}

	orgs, err := memberSvc.ListByUser(ctx, member)
	if err != nil || len(orgs) != 1 || orgs[0].OrgPK != orgPK {
		t.Fatalf("ListByUser = %+v (%v), want [%s]", orgs, err, orgPK)
	}

	// Promote then remove.
	if err := memberSvc.ChangeRole(ctx, orgPK, member, repositories.RoleUser); err != nil {
		t.Fatal(err)
	}
	if err := memberSvc.Remove(ctx, orgPK, member); err != nil {
		t.Fatal(err)
	}
	if m, _ := memberSvc.Get(ctx, orgPK, member); m != nil {
		t.Error("member should be gone after Remove")
	}
}

func TestCannotRemoveLastOwner(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	if err := memberSvc.Create(ctx, orgPK, "solo-owner", repositories.RoleOwner, "", "Solo", nil); err != nil {
		t.Fatal(err)
	}
	err := memberSvc.Remove(ctx, orgPK, "solo-owner")
	if err == nil {
		t.Fatal("expected error removing the last owner")
	}
	if p, ok := err.(*problem.Problem); !ok || p.Status != 409 {
		t.Errorf("want 409 Conflict, got %v", err)
	}
}

func TestCreateWithOwner_RequiresCertificate(t *testing.T) {
	ctx := context.Background()
	cnpj := randomCNPJ()
	fields := map[string]interface{}{} // minimal; validation happens at the route
	_ = fields
	_, err := orgSvc.CreateWithOwner(ctx, cnpj, "kyc-user", "KYC User", nil, nil, "")
	if err == nil {
		t.Fatal("expected certificate-required error")
	}
	if p, ok := err.(*problem.Problem); !ok || p.Status != 400 {
		t.Errorf("want 400 BadRequest, got %v", err)
	}
}

func TestCreateWithOwner_BranchInheritsCertificate(t *testing.T) {
	ctx := context.Background()
	root := randomCNPJ()[:8]
	matriz := "CNPJ_" + root + "000180"
	filial := root + "000261" // same root, different order

	// Matriz already exists with a cert and the user is its OWNER.
	if err := memberSvc.Create(ctx, matriz, "grp-user", repositories.RoleOwner, "", "Group User", nil); err != nil {
		t.Fatal(err)
	}
	seedCertForOrg(t, matriz)

	// Creating the filial without a certificate should succeed (inherits matriz).
	org, err := orgSvc.CreateWithOwner(ctx, filial, "grp-user", "Group User", nil, nil, "")
	if err != nil {
		t.Fatalf("filial creation should succeed: %v", err)
	}
	if org == nil {
		t.Fatal("expected filial org item")
	}
	// The filial must now be able to emit — i.e. it has an inherited cert row.
	certs, err := certRepo.List(ctx, "CNPJ_"+filial)
	if err != nil || len(certs) == 0 {
		t.Fatalf("filial should have an inherited certificate, got %d (%v)", len(certs), err)
	}
	// And the creator is its OWNER.
	if m, _ := memberSvc.Get(ctx, "CNPJ_"+filial, "grp-user"); m == nil || m.Role != repositories.RoleOwner {
		t.Error("creator should be OWNER of the filial")
	}
}

func TestInvitationLifecycle(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	if err := memberSvc.Create(ctx, orgPK, "inv-owner", repositories.RoleOwner, "", "Inv Owner", nil); err != nil {
		t.Fatal(err)
	}

	token, _, err := invSvc.Create(ctx, orgPK, repositories.RoleUser, "inv-owner", "Owner")
	if err != nil {
		t.Fatal(err)
	}

	preview, err := invSvc.Preview(ctx, token, "invitee")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Role != repositories.RoleUser || preview.Status != repositories.InvitationPending {
		t.Errorf("unexpected preview: %+v", preview)
	}
	if preview.AlreadyMember {
		t.Error("invitee should not yet be a member")
	}

	if _, err := invSvc.Accept(ctx, token, "invitee", "Invitee"); err != nil {
		t.Fatal(err)
	}
	if m, _ := memberSvc.Get(ctx, orgPK, "invitee"); m == nil || m.Role != repositories.RoleUser {
		t.Error("invitee should be a USER member after accept")
	}

	// Second accept of the same token must fail (single use).
	if _, err := invSvc.Accept(ctx, token, "other", "Other"); err == nil {
		t.Error("expected second accept to fail")
	}
}

func TestInvitation_RejectsOwnerRole(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	if _, _, err := invSvc.Create(ctx, orgPK, repositories.RoleOwner, "u", "U"); err == nil {
		t.Error("invitations must not grant OWNER")
	}
}

func TestInvitation_AlreadyMember(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	_ = memberSvc.Create(ctx, orgPK, "owner2", repositories.RoleOwner, "", "Owner2", nil)
	_ = memberSvc.Create(ctx, orgPK, "existing", repositories.RoleViewer, "owner2", "Existing", nil)

	token, _, err := invSvc.Create(ctx, orgPK, repositories.RoleUser, "owner2", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := invSvc.Accept(ctx, token, "existing", "Existing"); err == nil {
		t.Error("accepting when already a member should fail")
	}
}
