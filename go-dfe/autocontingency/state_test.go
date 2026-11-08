package autocontingency

import (
	"testing"
	"time"
)

func TestEncodeDecodeRoundTripAndFailSafe(t *testing.T) {
	b := Breaker{State: Open, OpenedAt: t0, Streak: 2, Failures: []time.Time{t0}}
	got := Decode(Encode(b))
	if got.State != Open || !got.OpenedAt.Equal(t0) || got.Streak != 2 || len(got.Failures) != 1 {
		t.Fatalf("round trip: %+v", got)
	}
	if Encode(Breaker{State: Closed}) != "" || Encode(Breaker{}) != "" {
		t.Error("a clean closed breaker stores nothing")
	}
	for _, raw := range []string{"", "not json", "{"} {
		if Decode(raw).State != Closed {
			t.Errorf("%q must decode to closed (never force a contingency from a corrupt state)", raw)
		}
	}
	if n := len([]rune(AutoReason)); n < 15 || n > 256 {
		t.Errorf("AutoReason length %d outside xJust bounds", n)
	}
}
