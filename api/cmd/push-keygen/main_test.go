package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPrintsTheCommandsAndWritesNothing(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out, "prod", false); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{
		"aws ssm put-parameter --type SecureString --overwrite --name /ctech-dfe/prod/push/vapid-private-key --value '-----BEGIN PRIVATE KEY-----",
		"aws ssm put-parameter --type String --overwrite --name /ctech-dfe/prod/push/vapid-public-key --value '",
		"Nothing has been written",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "-previous") {
		t.Error("a plain run must not touch -previous")
	}
}

func TestRunRotateKeepsTheCurrentKeyAsPreviousFirst(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out, "dev", true); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	prev := strings.Index(s, "vapid-private-key-previous")
	current := strings.Index(s, "--name /ctech-dfe/dev/push/vapid-private-key --value")
	if prev < 0 || current < 0 || prev > current {
		t.Errorf("-previous must be written before the new key:\n%s", s)
	}
}

func TestRunRefusesABadEnvironment(t *testing.T) {
	for _, env := range []string{"", "  ", "pr od", "prod/../x", "p'rod"} {
		if err := run(&bytes.Buffer{}, env, false); err == nil {
			t.Errorf("env %q must be refused (it is interpolated into a shell command)", env)
		}
	}
}

func TestEachRunMakesAFreshKey(t *testing.T) {
	var a, b bytes.Buffer
	if err := run(&a, "dev", false); err != nil {
		t.Fatal(err)
	}
	if err := run(&b, "dev", false); err != nil {
		t.Fatal(err)
	}
	if a.String() == b.String() {
		t.Error("two runs printed the same key pair")
	}
}
