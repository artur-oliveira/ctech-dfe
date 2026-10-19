// Package dfeproblem holds the DFe-specific RFC 7807 problems (no certificate,
// SEFAZ rejection, retired route, account pending deletion, billing refusals).
// The generic constructors, the Problem type and the sender live in
// gopkg.aoctech.app/api-commons/problem.
package dfeproblem

import (
	"net/http"

	"gopkg.aoctech.app/api-commons/problem"
)

// Problem type URIs owned by this product (RFC 7807 "type" member).
const (
	TypeNoCertificate  = "/problems/no-certificate"
	TypeRetired        = "/problems/retired"
	TypeSefazRejection = "/problems/sefaz-rejection"
	// TypeAccountPendingDeletion refuses a write of an account in the deletion
	// saga (locked during grace, or erased). Same name ctech-account uses.
	TypeAccountPendingDeletion = "/problems/account-pending-deletion"
)

// Extension members of the 402 body. The UI branches on these keys.
const (
	extraReason     = "reason"
	extraMeter      = "meter"
	extraPlan       = "plan"
	extraQuotaLimit = "quota_limit"
	extraQuotaUsed  = "quota_used"
)

// Reasons a request was refused for billing. Machine keys, stable across
// wording changes; the UI branches on these, never on the detail text.
const (
	ReasonSubscriptionMissing = "subscription_missing"
	ReasonSubscriptionPastDue = "subscription_past_due"
	// ReasonSubscriptionIncomplete is "chose the paid plan and never paid". It is
	// separate from past_due because the screens differ: one offers the first
	// checkout, the other offers to settle an overdue bill.
	ReasonSubscriptionIncomplete = "subscription_incomplete"
	ReasonSubscriptionCanceled   = "subscription_canceled"
	ReasonSubscriptionPaused     = "subscription_paused"
	ReasonQuotaExceeded          = "quota_exceeded"
)

func NoCertificate(detail string) *problem.Problem {
	return problem.New(http.StatusBadRequest, TypeNoCertificate, "No Certificate Found", detail)
}

func SefazRejection(detail string) *problem.Problem {
	return problem.New(http.StatusBadRequest, TypeSefazRejection, "Sefaz Rejection", detail)
}

// AccountPendingDeletion refuses a state-changing request of a locked account.
func AccountPendingDeletion(detail string) *problem.Problem {
	return problem.New(http.StatusForbidden, TypeAccountPendingDeletion, "Account Pending Deletion", detail)
}

// Retired answers a route that existed and is gone, naming where it moved.
//
// 410 rather than 404: a 404 sends an integrator looking for a typo in their own
// code; 410 says "this existed and is gone". The type is machine-readable so a
// client branches on "retired" rather than on prose.
func Retired(detail string) *problem.Problem {
	return problem.New(http.StatusGone, TypeRetired, "Endpoint Retired", detail)
}

// PaymentRequired refuses an action until the account pays for it.
//
// 402 rather than 403: the caller is allowed to do this; what is missing is a
// live subscription or headroom in it. The reason is a stable machine key.
func PaymentRequired(reason, detail string) *problem.Problem {
	return problem.PaymentRequired(detail).WithExtra(extraReason, reason)
}

// QuotaExceeded is PaymentRequired with the numbers the upgrade screen needs, so
// a 402 is actionable in one round trip. Zero values are omitted, as before.
func QuotaExceeded(meter, plan string, limit, used int64, detail string) *problem.Problem {
	p := PaymentRequired(ReasonQuotaExceeded, detail)
	if meter != "" {
		p = p.WithExtra(extraMeter, meter)
	}
	if plan != "" {
		p = p.WithExtra(extraPlan, plan)
	}
	if limit != 0 {
		p = p.WithExtra(extraQuotaLimit, limit)
	}
	if used != 0 {
		p = p.WithExtra(extraQuotaUsed, used)
	}
	return p
}
