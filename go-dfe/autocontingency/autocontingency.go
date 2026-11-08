// Package autocontingency is the decision logic of the automatic contingency
// (docs/specs/2026-11-08-contingencia.md section 4.1): which answers count as
// "authorizer unavailable", the circuit breaker with hysteresis, the proof that
// the original (tpEmis 1) emission was not authorized, and the orchestration of
// the re-issue. It is pure (no AWS, no clock but the one passed in) so the
// table tests cover flapping and the duplication proof.
//
// The actual switch is OFF: SwitchEnabled is false. The breaker store, the probe,
// the API pre-check and the return to normal are wired (spec 4.1.1) and all gated by it;
// still missing before it is enabled: owner sign-off and a run against a real authorizer.
package autocontingency

import (
	"context"
	"slices"
	"time"
)

const (
	// SwitchEnabled is the master switch of the automatic switch of the emission type (feature flag). It stays
	// false by default; auto_contingency on a configuration only stores the owner's choice meanwhile. Consumers
	// (api services, worker service, dispatcher) read it through a local variable so tests can turn it on.
	SwitchEnabled = false

	// FailureThreshold consecutive unavailable answers within FailureWindow open the breaker.
	FailureThreshold = 3
	FailureWindow    = 2 * time.Minute
	// CoolDown is the minimum time the breaker stays open; RecoveryProbes consecutive healthy probes
	// after it close the breaker. Together they are the hysteresis that prevents flapping.
	CoolDown       = 5 * time.Minute
	RecoveryProbes = 3
	// InFlightQuarantine is how long after the last send of the original a "not found" consulta is still
	// not a proof: a request stuck in a timeout can reach the SEFAZ late. It is longer than the egress
	// call timeout plus the SEFAZ processing time.
	InFlightQuarantine = 10 * time.Minute

	// cStat of the MOC used by the signals and the proof.
	CStatServiceHalted         = "108" // Serviço Paralisado Momentaneamente
	CStatServiceHaltedNoETA    = "109" // Serviço Paralisado sem Previsão
	CStatServiceInOperation    = "107"
	CStatAuthorized            = "100"
	CStatAuthorizedLate        = "150"
	CStatNotFound              = "217" // NF-e não consta na base de dados da SEFAZ
	CStatDuplicityDifferentKey = "539"
	// Authorized and then cancelled/denied: the number is consumed, the original cannot be re-issued.
	cStatCancelled  = "101"
	cStatCancelledB = "151"
	cStatCancelledC = "155"
	cStatDenied     = "110"
)

// Verdict is what one answer of the authorizer says about its availability.
type Verdict int

const (
	Neutral Verdict = iota
	Healthy
	Unavailable
)

// Observation is one answer (or the lack of it) from the authorizer, as the egress reports it.
type Observation struct {
	// TransportErr: the egress call failed or timed out before an answer.
	TransportErr bool
	// SOAPFault: the authority answered with a SOAP fault.
	SOAPFault bool
	// HTTPStatus of the authority (0 when none).
	HTTPStatus int
	// CStat of the answer ("" when there is none).
	CStat string
}

// Classify applies the signals of spec 4.1. A business answer (any other cStat, including a rejection of the
// document) proves the authorizer is up: it is Healthy and never counts as a failure.
func Classify(o Observation) Verdict {
	switch {
	case o.TransportErr, o.SOAPFault, o.HTTPStatus == 408, o.HTTPStatus >= 500:
		return Unavailable
	case o.CStat == CStatServiceHalted || o.CStat == CStatServiceHaltedNoETA:
		return Unavailable
	case o.CStat != "":
		return Healthy
	}
	return Neutral
}

// State of the breaker.
type State string

const (
	Closed State = "closed"
	Open   State = "open"
)

// Transition is what an observation changed.
type Transition int

const (
	NoChange Transition = iota
	Opened
	Closing
)

// Breaker is the per-authorizer (document type, UF, environment) circuit breaker.
type Breaker struct {
	State    State
	Failures []time.Time
	OpenedAt time.Time
	Streak   int // consecutive healthy probes while open
}

// Observe feeds one verdict. Only probes count while open (documents issued in contingency say nothing about
// the normal authorizer); while closed, any answer counts, and a healthy one resets the consecutive failures.
func (b *Breaker) Observe(now time.Time, v Verdict, probe bool) Transition {
	if b.State == "" {
		b.State = Closed
	}
	if b.State == Open {
		if !probe || v == Neutral {
			return NoChange
		}
		if v == Unavailable || now.Sub(b.OpenedAt) < CoolDown {
			b.Streak = 0
			return NoChange
		}
		b.Streak++
		if b.Streak < RecoveryProbes {
			return NoChange
		}
		*b = Breaker{State: Closed}
		return Closing
	}
	switch v {
	case Healthy:
		b.Failures = nil
	case Unavailable:
		b.Failures = append(slices.DeleteFunc(b.Failures, func(t time.Time) bool { return now.Sub(t) > FailureWindow }), now)
		if len(b.Failures) >= FailureThreshold {
			*b = Breaker{State: Open, OpenedAt: now}
			return Opened
		}
	}
	return NoChange
}

// Proof is what the consulta by key of the original emission established.
type Proof int

const (
	// ProofImpossible: the consulta failed, did not answer clearly, or answered too soon after the send.
	ProofImpossible Proof = iota
	// ProofNotAuthorized: the SEFAZ does not know the key and the quarantine has passed.
	ProofNotAuthorized
	// ProofAuthorized: the original was authorized; it is the document, nothing is re-issued.
	ProofAuthorized
	// ProofNumberConsumed: authorized and then cancelled or denied; the number cannot be used again.
	ProofNumberConsumed
)

// Consulta is the answer of the consulta by key (NfeConsultaProtocolo and equivalents).
type Consulta struct {
	Answered bool
	CStat    string
}

// ProveOriginal turns the consulta of the original key into a proof. sentAt is the last time the original was
// sent to the authorizer. Only "not found" after the quarantine proves it was not authorized, and even that is
// backed by the SEFAZ rule 539 (same serie and number with another key are refused): see the spec.
func ProveOriginal(c Consulta, sentAt, now time.Time) Proof {
	if !c.Answered {
		return ProofImpossible
	}
	switch c.CStat {
	case CStatAuthorized, CStatAuthorizedLate:
		return ProofAuthorized
	case cStatCancelled, cStatCancelledB, cStatCancelledC, cStatDenied:
		return ProofNumberConsumed
	case CStatNotFound:
		if now.Sub(sentAt) >= InFlightQuarantine {
			return ProofNotAuthorized
		}
	}
	return ProofImpossible
}

// Outcome of one re-issue attempt.
type Outcome int

const (
	// OutcomeDisabled: the switch is off or the configuration does not allow it; nothing was touched.
	OutcomeDisabled Outcome = iota
	OutcomeRegenerated
	OutcomeAdoptedOriginal
	// OutcomeKeptPending: the proof was not possible; the original stays pending and an alert is raised.
	OutcomeKeptPending
)

// Config is what the re-issue needs from the fiscal configuration of the document type.
type Config struct {
	Enabled bool     // SwitchEnabled (tests pass true)
	Auto    bool     // auto_contingency
	Type    string   // auto_contingency_type
	Options []string // emission_options of the document type
	Current string   // emission_type now
}

// Target is the emission type to switch to, or "" when the configuration does not allow switching.
func (c Config) Target() string {
	if !c.Enabled || !c.Auto || c.Type == "" || c.Type == "normal" || !slices.Contains(c.Options, c.Type) {
		return ""
	}
	return c.Type
}

// Deps are the effects of the re-issue; the API wires them, tests fake them.
type Deps interface {
	// Consult asks the authorizer (through the egress) for the original key.
	Consult(ctx context.Context, accessKey string) (Consulta, error)
	// Regenerate re-issues the document from the original request with the target emission type: new tpEmis,
	// access key, cNF, dhCont/xJust and signature, same serie and number. Never patches the stored XML.
	Regenerate(ctx context.Context, target string) error
	// Adopt keeps the original as the document (it was authorized) and discards the attempt to re-issue.
	Adopt(ctx context.Context) error
	// Alert raises the operator alert (go-common alerts) with the reason.
	Alert(ctx context.Context, reason string)
}

// Reissue runs the decision for one document whose normal emission failed for unavailability. The original is
// only left behind when the consulta proves it was not authorized; if the consulta cannot prove anything the
// safe choice is to NOT regenerate: two documents with the same serie and number but different keys could both
// reach the SEFAZ, and a duplicated fiscal document is worse than a delayed one.
func Reissue(ctx context.Context, cfg Config, originalKey string, sentAt, now time.Time, d Deps) (Outcome, error) {
	target := cfg.Target()
	if target == "" {
		return OutcomeDisabled, nil
	}
	c, err := d.Consult(ctx, originalKey)
	if err != nil {
		c = Consulta{}
	}
	switch ProveOriginal(c, sentAt, now) {
	case ProofAuthorized:
		return OutcomeAdoptedOriginal, d.Adopt(ctx)
	case ProofNotAuthorized:
		return OutcomeRegenerated, d.Regenerate(ctx, target)
	case ProofNumberConsumed:
		d.Alert(ctx, "numero ja consumido pelo original (cancelado ou denegado): nao regerado")
	default:
		d.Alert(ctx, "nao foi possivel provar que o original nao foi autorizado: nao regerado")
	}
	return OutcomeKeptPending, nil
}

// ShouldSwitch: the breaker is open, the owner enabled the automatic contingency and chose a type, and the
// document type is still in normal emission.
func ShouldSwitch(cfg Config, b Breaker) (string, bool) {
	t := cfg.Target()
	return t, t != "" && cfg.Current == "normal" && b.State == Open
}

// ShouldReturn: back to normal only when the contingency was entered by the automatic switch (autoEntered;
// a manual choice of the owner is never undone) and the breaker closed.
func ShouldReturn(cfg Config, b Breaker, autoEntered bool) bool {
	return cfg.Enabled && cfg.Auto && autoEntered && cfg.Current != "normal" && b.State == Closed
}
