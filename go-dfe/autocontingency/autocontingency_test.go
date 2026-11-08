package autocontingency

import (
	"context"
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		o    Observation
		want Verdict
	}{
		{"timeout", Observation{TransportErr: true}, Unavailable},
		{"http 503", Observation{HTTPStatus: 503}, Unavailable},
		{"http 408", Observation{HTTPStatus: 408}, Unavailable},
		{"soap fault", Observation{SOAPFault: true}, Unavailable},
		{"cStat 108", Observation{CStat: "108"}, Unavailable},
		{"cStat 109", Observation{CStat: "109"}, Unavailable},
		{"status in operation", Observation{CStat: "107"}, Healthy},
		{"business rejection is not a failure", Observation{HTTPStatus: 200, CStat: "225"}, Healthy},
		{"authorized", Observation{CStat: "100"}, Healthy},
		{"nothing", Observation{}, Neutral},
	}
	for _, c := range cases {
		if got := Classify(c.o); got != c.want {
			t.Errorf("%s: %v want %v", c.name, got, c.want)
		}
	}
}

// Flapping: a healthy answer resets the consecutive failures; failures outside the window do not add up;
// closing needs the cool-down AND RecoveryProbes consecutive healthy probes, and one bad probe restarts them.
func TestBreakerFlapping(t *testing.T) {
	var b Breaker
	at := t0
	step := func(d time.Duration, v Verdict, probe bool) Transition {
		at = at.Add(d)
		return b.Observe(at, v, probe)
	}

	step(0, Unavailable, false)
	step(10*time.Second, Unavailable, false)
	step(10*time.Second, Healthy, false) // resets
	step(10*time.Second, Unavailable, false)
	if step(10*time.Second, Unavailable, false) != NoChange || b.State != Closed {
		t.Fatal("two failures after a reset must not open")
	}
	step(FailureWindow+time.Second, Unavailable, false) // earlier ones fell out of the window
	if b.State != Closed {
		t.Fatal("failures spread over more than the window must not open")
	}
	step(time.Second, Unavailable, false)
	if step(time.Second, Unavailable, false) != Opened || b.State != Open {
		t.Fatal("three consecutive failures in the window open")
	}
	if step(time.Second, Healthy, false) != NoChange || b.State != Open {
		t.Fatal("a document answer does not close the breaker")
	}
	// healthy probes inside the cool-down count for nothing
	for range RecoveryProbes + 1 {
		step(10*time.Second, Healthy, true)
	}
	if b.State != Open {
		t.Fatal("cool-down not elapsed")
	}
	step(CoolDown, Healthy, true)
	step(time.Minute, Healthy, true)
	step(time.Minute, Unavailable, true) // flap: streak restarts
	step(time.Minute, Healthy, true)
	step(time.Minute, Healthy, true)
	if b.State != Open {
		t.Fatal("a bad probe must restart the streak")
	}
	if step(time.Minute, Healthy, true) != Closing || b.State != Closed || len(b.Failures) != 0 {
		t.Fatal("RecoveryProbes consecutive healthy probes after the cool-down close it")
	}
}

func TestProveOriginal(t *testing.T) {
	sent := t0
	late, soon := t0.Add(InFlightQuarantine), t0.Add(time.Minute)
	cases := []struct {
		name string
		c    Consulta
		now  time.Time
		want Proof
	}{
		{"consulta failed", Consulta{}, late, ProofImpossible},
		{"authorized", Consulta{true, "100"}, soon, ProofAuthorized},
		{"authorized late", Consulta{true, "150"}, soon, ProofAuthorized},
		{"cancelled", Consulta{true, "101"}, soon, ProofNumberConsumed},
		{"denied", Consulta{true, "110"}, soon, ProofNumberConsumed},
		{"not found after the quarantine", Consulta{true, "217"}, late, ProofNotAuthorized},
		{"not found too soon (request may be in flight)", Consulta{true, "217"}, soon, ProofImpossible},
		{"consulta service halted", Consulta{true, "108"}, late, ProofImpossible},
		{"unknown answer", Consulta{true, "999"}, late, ProofImpossible},
	}
	for _, c := range cases {
		if got := ProveOriginal(c.c, sent, c.now); got != c.want {
			t.Errorf("%s: %v want %v", c.name, got, c.want)
		}
	}
}

type fakeDeps struct {
	consulta        Consulta
	consultaErr     error
	regenerated     []string
	adopted, alerts int
	regenErr        error
}

func (f *fakeDeps) Consult(context.Context, string) (Consulta, error) {
	return f.consulta, f.consultaErr
}
func (f *fakeDeps) Regenerate(_ context.Context, target string) error {
	f.regenerated = append(f.regenerated, target)
	return f.regenErr
}
func (f *fakeDeps) Adopt(context.Context) error   { f.adopted++; return nil }
func (f *fakeDeps) Alert(context.Context, string) { f.alerts++ }

func cfg() Config {
	return Config{Enabled: true, Auto: true, Type: "svc", Options: []string{"normal", "svc", "epec"}, Current: "normal"}
}

// The full re-issue path with a fake egress/consulta: regenerate only with the proof; with no proof, never.
func TestReissue(t *testing.T) {
	sent, late := t0, t0.Add(InFlightQuarantine+time.Second)
	cases := []struct {
		name    string
		cfg     func(*Config)
		d       fakeDeps
		now     time.Time
		want    Outcome
		regen   int
		adopted int
		alerts  int
	}{
		{"proof: not authorized -> regenerates once in the chosen type", func(*Config) {}, fakeDeps{consulta: Consulta{true, "217"}}, late, OutcomeRegenerated, 1, 0, 0},
		{"original authorized -> adopts, regenerates nothing", func(*Config) {}, fakeDeps{consulta: Consulta{true, "100"}}, late, OutcomeAdoptedOriginal, 0, 1, 0},
		{"consulta fails -> safe option: keep pending and alert", func(*Config) {}, fakeDeps{consultaErr: errors.New("timeout")}, late, OutcomeKeptPending, 0, 0, 1},
		{"consulta says not found too soon -> keep pending and alert", func(*Config) {}, fakeDeps{consulta: Consulta{true, "217"}}, sent.Add(time.Minute), OutcomeKeptPending, 0, 0, 1},
		{"number consumed -> alert", func(*Config) {}, fakeDeps{consulta: Consulta{true, "101"}}, late, OutcomeKeptPending, 0, 0, 1},
		{"switch disabled (the shipped constant)", func(c *Config) { c.Enabled = SwitchEnabled }, fakeDeps{consulta: Consulta{true, "217"}}, late, OutcomeDisabled, 0, 0, 0},
		{"auto off", func(c *Config) { c.Auto = false }, fakeDeps{consulta: Consulta{true, "217"}}, late, OutcomeDisabled, 0, 0, 0},
		{"type not in the options", func(c *Config) { c.Type = "offline" }, fakeDeps{consulta: Consulta{true, "217"}}, late, OutcomeDisabled, 0, 0, 0},
	}
	for _, c := range cases {
		cf := cfg()
		c.cfg(&cf)
		d := c.d
		got, err := Reissue(context.Background(), cf, "KEY", sent, c.now, &d)
		if err != nil || got != c.want || len(d.regenerated) != c.regen || d.adopted != c.adopted || d.alerts != c.alerts {
			t.Errorf("%s: outcome=%v err=%v regen=%v adopted=%d alerts=%d", c.name, got, err, d.regenerated, d.adopted, d.alerts)
		}
		if c.regen == 1 && d.regenerated[0] != "svc" {
			t.Errorf("%s: target %v", c.name, d.regenerated)
		}
	}
	if SwitchEnabled {
		t.Fatal("the switch must ship disabled")
	}
}

func TestShouldSwitchAndReturn(t *testing.T) {
	open, closed := Breaker{State: Open}, Breaker{State: Closed}
	if _, ok := ShouldSwitch(cfg(), closed); ok {
		t.Error("closed breaker must not switch")
	}
	if tgt, ok := ShouldSwitch(cfg(), open); !ok || tgt != "svc" {
		t.Errorf("open breaker + auto: %q %v", tgt, ok)
	}
	c := cfg()
	c.Current = "svc"
	if _, ok := ShouldSwitch(c, open); ok {
		t.Error("already in contingency: no second switch")
	}
	if !ShouldReturn(c, closed, true) {
		t.Error("auto-entered contingency returns when the breaker closes")
	}
	if ShouldReturn(c, closed, false) {
		t.Error("a manual contingency is never undone")
	}
	if ShouldReturn(c, open, true) {
		t.Error("open breaker keeps contingency")
	}
}
