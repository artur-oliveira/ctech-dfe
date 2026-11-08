package contingency

import (
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	now := time.Now()
	before, after := now.Add(time.Hour), now.Add(-time.Hour)
	cases := []struct {
		name     string
		deadline time.Time
		cStat    int
		answered bool
		want     Outcome
	}{
		{"authorized in time", before, 100, true, OutcomeAuthorized},
		{"authorized after deadline is success", after, 100, true, OutcomeAuthorizedLate},
		{"cStat 150 is success", before, 150, true, OutcomeAuthorizedLate},
		{"duplicity reconciles", before, 204, true, OutcomeReconcile},
		{"rejection", before, 539 + 1, true, OutcomeRejected},
		{"no answer in time retries", before, 0, false, OutcomeRetry},
		{"no answer past deadline is overdue", after, 0, false, OutcomeOverdue},
	}
	for _, c := range cases {
		if got := Classify(now, c.deadline, c.cStat, c.answered); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	if StateFor(OutcomeAuthorizedLate) != StateAuthorizedLate || StateFor(OutcomeRetry) != "" {
		t.Error("StateFor")
	}
}

// Offline MDF-e / CT-e (168 h window): an authorization after the deadline, with 100 or CT-e's 150,
// is a success; only a missing answer past the deadline is overdue.
func TestClassifyOfflineLateSuccess(t *testing.T) {
	now := time.Now()
	deadline := now.Add(-time.Minute)
	if got := Classify(now, deadline, 100, true); got != OutcomeAuthorizedLate {
		t.Errorf("100 late = %v", got)
	}
	if got := Classify(now, deadline, 150, true); got != OutcomeAuthorizedLate {
		t.Errorf("150 late = %v", got)
	}
	if StateFor(OutcomeAuthorizedLate) != StateAuthorizedLate {
		t.Error("state")
	}
}
