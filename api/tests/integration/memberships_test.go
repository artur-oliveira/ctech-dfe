//go:build integration

package integration_test

import (
	"context"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// seedOwner establishes an organization's owner the way the product does: by
// writing the membership row directly, which is what OrganizationService does
// inside the transaction that creates the organization.
//
// It cannot go through MembershipService, and that is the point rather than an
// inconvenience — member management refuses to write an OWNER, because an
// organization has exactly one and it is whoever created it. A fixture that
// could hand out ownership would be a fixture testing a path the product does
// not have.
func seedOwner(t *testing.T, orgPK, userID, name string) {
	t.Helper()
	if err := orgUserRepo.Create(context.Background(), orgPK, userID, repositories.RoleOwner, "", name, nil); err != nil {
		t.Fatal(err)
	}
}

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

	seedOwner(t, orgPK, owner, "Owner Name")
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
	if err := memberSvc.Remove(ctx, orgPK, member, "test-actor", "Test Actor"); err != nil {
		t.Fatal(err)
	}
	if m, _ := memberSvc.Get(ctx, orgPK, member); m != nil {
		t.Error("member should be gone after Remove")
	}
}

func TestCannotRemoveLastOwner(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	seedOwner(t, orgPK, "solo-owner", "Solo")
	err := memberSvc.Remove(ctx, orgPK, "solo-owner", "test-actor", "Test Actor")
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
	seedOwner(t, matriz, "grp-user", "Group User")
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
	seedOwner(t, orgPK, "inv-owner", "Inv Owner")

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

// TestAnOrganizationHasExactlyOneOwner walks the three ways a second OWNER could
// appear and closes all of them.
//
// The promotion case is the one that was open: the route's payload happened to
// reject OWNER, but the service accepted it, so the invariant held only for
// callers that went through that one DTO — and ownership transfer will arrive as
// a second caller.
func TestAnOrganizationHasExactlyOneOwner(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	seedOwner(t, orgPK, "the-owner", "The Owner")
	if err := memberSvc.Create(ctx, orgPK, "second", repositories.RoleAdmin, "the-owner", "Second", nil); err != nil {
		t.Fatal(err)
	}

	// 1. Promotion.
	err := memberSvc.ChangeRole(ctx, orgPK, "second", repositories.RoleOwner)
	if err == nil {
		t.Fatal("promoting a member to OWNER must be refused")
	}
	if p, ok := err.(*problem.Problem); !ok || p.Status != 409 {
		t.Errorf("want 409 Conflict, got %v", err)
	}

	// 2. A direct membership write.
	if err := memberSvc.Create(ctx, orgPK, "third", repositories.RoleOwner, "the-owner", "Third", nil); err == nil {
		t.Error("member management must not write an OWNER")
	}

	// 3. An invitation — covered above, and asserted here too because these are
	// three doors into one room and a test per door is what keeps them shut.
	if _, _, err := invSvc.Create(ctx, orgPK, repositories.RoleOwner, "the-owner", "The Owner"); err == nil {
		t.Error("invitations must not grant OWNER")
	}

	// The org still has exactly one owner, and it is the one who created it.
	owners, err := orgUserRepo.CountOwners(ctx, orgPK)
	if err != nil {
		t.Fatal(err)
	}
	if owners != 1 {
		t.Fatalf("owners = %d, want exactly 1", owners)
	}
	if m, _ := memberSvc.Get(ctx, orgPK, "the-owner"); m == nil || m.Role != repositories.RoleOwner {
		t.Errorf("the creator must still be the owner, got %+v", m)
	}
	// And the refused promotion left no trace: "second" is still ADMIN.
	if m, _ := memberSvc.Get(ctx, orgPK, "second"); m == nil || m.Role != repositories.RoleAdmin {
		t.Errorf("the refused promotion must not have changed anything, got %+v", m)
	}
}

func TestInvitation_AlreadyMember(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	seedOwner(t, orgPK, "owner2", "Owner2")
	_ = memberSvc.Create(ctx, orgPK, "existing", repositories.RoleViewer, "owner2", "Existing", nil)

	token, _, err := invSvc.Create(ctx, orgPK, repositories.RoleUser, "owner2", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := invSvc.Accept(ctx, token, "existing", "Existing"); err == nil {
		t.Error("accepting when already a member should fail")
	}
}
