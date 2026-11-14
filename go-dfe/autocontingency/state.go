package autocontingency

import (
	"encoding/json"
	"time"
)

// State persisted on the fiscal configuration row of a document type (attribute AttrBreaker, one JSON string,
// written only by the worker with a compare-and-swap on the previous string; spec 4.1).
type stored struct {
	State    State       `json:"s,omitempty"`
	Failures []time.Time `json:"f,omitempty"`
	OpenedAt time.Time   `json:"o,omitempty"`
	Streak   int         `json:"k,omitempty"`
}

// Encode serializes the breaker for storage. A closed breaker with no failures encodes as "" (nothing to store).
func Encode(b Breaker) string {
	if (b.State == "" || b.State == Closed) && len(b.Failures) == 0 {
		return ""
	}
	raw, _ := json.Marshal(stored(b))
	return string(raw)
}

// Decode reads a stored breaker. An empty or unreadable value is a closed breaker (fail safe: a corrupt state
// must never force the emission into contingency).
func Decode(raw string) Breaker {
	var s stored
	if raw == "" || json.Unmarshal([]byte(raw), &s) != nil {
		return Breaker{State: Closed}
	}
	if s.State == "" {
		s.State = Closed
	}
	return Breaker(s)
}

const (
	// AutoReason is the xJust (15 to 256 characters) the automatic switch writes.
	AutoReason = "Contingencia automatica: autorizador normal indisponivel"
	// ProbeGap separates the probes of one burst. The probe runs in the 15-minute dispatcher, so a burst of up to
	// RecoveryProbes status calls stands in for the spec's "one probe per minute" (see spec 4.1, decisions).
	ProbeGap = 5 * time.Second
)
