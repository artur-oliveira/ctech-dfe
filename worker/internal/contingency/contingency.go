// Package contingency holds the names and decisions shared by the contingency
// retransmission dispatcher and worker (docs/specs/2026-11-08-contingencia.md,
// section 6). The attribute names mirror api/internal/repositories.
package contingency

import "time"

const (
	// IndexName is the sparse GSI of the document tables: only documents issued
	// in EPEC or offline contingency and not yet reconciled carry SweepAttr.
	IndexName    = "contingency-index"
	SweepAttr    = "contingency_sweep"
	SweepValue   = "1"
	StateAttr    = "contingency_state"
	DeadlineAttr = "contingency_deadline"
	// MsgAttr holds the WorkerMessage JSON (signed XML as body) the resend sends.
	MsgAttr = "contingency_msg"

	StatePending        = "pending"
	StateAuthorized     = "authorized"
	StateAuthorizedLate = "authorized_late"
	StateRejectedLate   = "rejected_late"
	StateOverdue        = "overdue"

	// cStat that ends a retransmission as a success. 150 is "autorizado fora de
	// prazo" and counts as success (owner decision 3). 100 is the normal
	// authorization. ponytail: 150 is confirmed by the CT-e MOC only (rule G103: offline CT-e sent after
	// 168 h returns 150). The MDF-e MOC states no late cStat, so a late MDF-e answers 100 (also a success
	// here, by the deadline comparison) or a rejection; 150 for the MDF-e stays UNVERIFIED.
	CStatAuthorized     = 100
	CStatAuthorizedLate = 150
	// Duplicity: the SEFAZ already holds the key; reconcile by a consulta.
	CStatDuplicityNFe = 204
	CStatDuplicityMOC = 539

	// NFCeDeadlineDays is the retransmission window of an offline NFC-e (owner
	// decision 4; varies by UF, hence a constant to be replaced by a UF table).
	NFCeDeadlineDays = 3
	// OfflineWindowHours is the MOC window of an offline MDF-e and CT-e (168 h); the API computes the
	// deadline (services.OfflineWindowMDFe/CTe), the worker only reads it.
	OfflineWindowHours = 168
)

// Tables are the document tables the sweep covers, without the table prefix.
var Tables = []string{"nfes", "nfces", "mdfes", "ctes"}

// Job is the payload the dispatcher sends to the retransmission worker.
type Job struct {
	Table string `json:"table"`
	PK    string `json:"pk"`
	SK    string `json:"sk"`
}

// Outcome is what a retransmission attempt ended as.
type Outcome int

const (
	OutcomeRetry     Outcome = iota // transient: leave pending, the next sweep retries
	OutcomeReconcile                // duplicity: consult by key before deciding
	OutcomeAuthorized
	OutcomeAuthorizedLate
	OutcomeRejected
	OutcomeOverdue
)

// Classify turns the SEFAZ answer of a resend into an outcome. A document whose
// deadline has passed without an authorization is overdue; one authorized after
// the deadline is a success (authorized_late), never an error.
func Classify(now, deadline time.Time, cStat int, answered bool) Outcome {
	late := !deadline.IsZero() && now.After(deadline)
	switch {
	case !answered:
		if late {
			return OutcomeOverdue
		}
		return OutcomeRetry
	case cStat == CStatAuthorizedLate:
		return OutcomeAuthorizedLate
	case cStat == CStatAuthorized && late:
		return OutcomeAuthorizedLate
	case cStat == CStatAuthorized:
		return OutcomeAuthorized
	case cStat == CStatDuplicityNFe || cStat == CStatDuplicityMOC:
		return OutcomeReconcile
	default:
		return OutcomeRejected
	}
}

// StateFor is the contingency_state an outcome persists ("" keeps the state).
func StateFor(o Outcome) string {
	switch o {
	case OutcomeAuthorized:
		return StateAuthorized
	case OutcomeAuthorizedLate:
		return StateAuthorizedLate
	case OutcomeRejected:
		return StateRejectedLate
	case OutcomeOverdue:
		return StateOverdue
	}
	return ""
}
