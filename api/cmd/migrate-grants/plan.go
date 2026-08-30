package main

import (
	"fmt"
	"sort"
	"strings"
)

// overlay is one authorization row this product holds: who, where, what role,
// and any explicit grants on top of it.
//
// After the membership unification (ctech-billing ADR 0023) this row grants
// nothing on its own — the edge in ctech-account is what grants reach, and this
// says what the person may do once they have it.
type overlay struct {
	CompanyID   string
	UserID      string
	Role        string
	Permissions []string
}

// HasGrants reports an explicit grant beyond whatever the role carries.
func (o overlay) HasGrants() bool { return len(o.Permissions) > 0 }

const (
	actionReady  = "ready"
	actionReview = "review"
)

// decision is what this tool proposes for one overlay row.
type decision struct {
	Overlay overlay
	// GrantEdge is the edge to write: this person is authorized here and
	// ctech-account has no record of them being able to reach it.
	GrantEdge bool
	Action    string
	Review    string
	Notes     []string
}

func (d decision) NeedsHuman() bool { return d.Action == actionReview }

// reach is what ctech-account says about one (company, user).
type reach struct {
	HasEdge bool
	// OrganizationID is empty when there is no edge, and the edge is what knows
	// it: a company id alone cannot be scoped without one.
	OrganizationID string
}

// plan decides what happens to each overlay row.
//
// **It writes nothing to ctech-dfe.** The grants are already where ADR 0023 says
// they belong — this product owns the verbs — so there is nothing to move. What
// it writes, on -apply, is the missing EDGES: the platform's record that this
// person may reach the company they are already authorized in.
//
// The refusal that matters: a row carrying explicit grants with no edge and no
// invitation to derive one from. That person is exercising permissions the
// platform has no record of them being allowed to reach, and inventing the edge
// is inventing access. A role-only row is different — it is the ordinary state
// of every member, and its edge is the one thing the migration is for.
func plan(overlays []overlay, reaches map[string]reach, companyOrg map[string]string) []decision {
	out := make([]decision, 0, len(overlays))
	for _, o := range overlays {
		d := decision{Overlay: o, Action: actionReady}
		r := reaches[edgeKey(o.CompanyID, o.UserID)]

		switch {
		case r.HasEdge:
			d.Notes = append(d.Notes, "reach already recorded")
		case o.HasGrants():
			// The one case a human decides. Granting reach to somebody because
			// they hold permissions is circular: the permissions were granted
			// on the assumption they could reach it, and that assumption is
			// what this migration is supposed to verify.
			d.Action = actionReview
			d.Review = fmt.Sprintf("carries %d explicit grant(s) and has no edge in ctech-account: %s",
				len(o.Permissions), strings.Join(o.Permissions, ", "))
		case companyOrg[o.CompanyID] == "":
			// Without the organization the edge cannot be keyed, and guessing it
			// would write a row nothing can read.
			d.Action = actionReview
			d.Review = "no organization known for this company; the edge cannot be keyed"
		default:
			d.GrantEdge = true
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Overlay.CompanyID != out[j].Overlay.CompanyID {
			return out[i].Overlay.CompanyID < out[j].Overlay.CompanyID
		}
		return out[i].Overlay.UserID < out[j].Overlay.UserID
	})
	return out
}

func edgeKey(companyID, userID string) string { return companyID + "|" + userID }

// report renders the decisions. It is the whole output of a dry run, and what
// somebody reads before allowing writes.
func report(decisions []decision) string {
	var b strings.Builder
	var grant, already, review int
	for _, d := range decisions {
		switch {
		case d.NeedsHuman():
			review++
			fmt.Fprintf(&b, "  REVIEW  %s / %s — %s\n", d.Overlay.CompanyID, d.Overlay.UserID, d.Review)
		case d.GrantEdge:
			grant++
			fmt.Fprintf(&b, "  grant   %s / %s (%s)\n", d.Overlay.CompanyID, d.Overlay.UserID, d.Overlay.Role)
		default:
			already++
			fmt.Fprintf(&b, "  ok      %s / %s (%s) — reach already recorded\n",
				d.Overlay.CompanyID, d.Overlay.UserID, d.Overlay.Role)
		}
	}
	fmt.Fprintf(&b, "\n%d edge(s) to grant, %d already recorded, %d need a human\n", grant, already, review)
	return b.String()
}

// countReviews is how the command decides its exit code.
func countReviews(decisions []decision) int {
	n := 0
	for _, d := range decisions {
		if d.NeedsHuman() {
			n++
		}
	}
	return n
}
