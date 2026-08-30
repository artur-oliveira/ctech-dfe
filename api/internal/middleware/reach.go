package middleware

import (
	"context"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// reachChecker is the one question this middleware asks ctech-account. An
// interface so the authorization decision is testable without a network.
type reachChecker interface {
	MayAct(ctx context.Context, companyID, userID string) (string, bool, error)
}

// accessDenied is the single answer to every refusal on this path.
//
// One message, deliberately. "No edge", "no role" and "ctech-account is down"
// are three different facts, and telling them apart from outside would make the
// API a probe: which company ids are real, and who is in them. The distinction
// belongs in the log, where the person debugging can see it and the person
// guessing cannot.
const accessDenied = "Acesso negado"

// authorize decides whether a request may proceed, given the reach ctech-account
// reports and the authorization row this product holds.
//
// Reach from the edge, verbs from the row (ctech-billing ADR 0023). The row
// alone stops being enough, which is the whole point: a row that survived a
// revoked edge must grant nothing.
//
// A legacy CNPJ_/CPF_ key skips the edge entirely. Nothing has been migrated for
// it, there is no company id to ask about, and asking would refuse every
// existing customer on the day this ships. That branch disappears with the
// legacy partitions.
func authorize(
	ctx context.Context,
	reach reachChecker,
	orgPK, userID string,
	row *services.Membership,
) (*services.Membership, *problem.Problem) {
	if !repositories.IsCompanyKey(orgPK) {
		// Pre-migration: the product's own row is still the access record.
		return row, nil
	}

	_, mayAct, err := reach.MayAct(ctx, orgPK, userID)
	if err != nil {
		// Fail closed. An authorization dependency we cannot reach must never
		// read as permission — the opposite of the billing snapshot, which
		// degrades open so an outage does not stop issuance.
		return nil, problem.Forbidden(accessDenied)
	}
	if !mayAct {
		return nil, problem.Forbidden(accessDenied)
	}

	// Reach granted. The row is what says WHAT they may do, and its absence is
	// not an error: somebody invited to a company nobody has given a role in
	// yet reaches it and can do nothing, which the permission check below
	// refuses on its own.
	return row, nil
}
