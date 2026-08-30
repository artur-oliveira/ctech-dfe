package app

import (
	"testing"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Off unless ctech-account issued a credential. Reach is an authorization
// check, and an unconfigured deployment must leave the previous behaviour in
// place rather than refuse everybody — so nil here is the safe default and not
// an oversight.
func TestReachIsOffWithoutACredential(t *testing.T) {
	c := cache.NewMemoryBackend(8)
	if got := newReachService(&config.Config{CtechURL: "https://accounts.example"}, c); got != nil {
		t.Fatal("reach was built with no client id or secret")
	}
	if got := newReachService(&config.Config{CtechURL: "https://accounts.example", AccountClientID: "dfe"}, c); got != nil {
		t.Fatal("reach was built with an id and no secret")
	}
	if got := newReachService(&config.Config{AccountClientID: "dfe", AccountClientSecret: "s3cr3t"}, c); got != nil {
		t.Fatal("reach was built with no ctech-account URL to ask")
	}
}

// And on when it is complete. This is the flip: with it wired, a membership row
// whose company edge was revoked grants nothing.
func TestReachIsOnWithAFullCredential(t *testing.T) {
	got := newReachService(&config.Config{
		CtechURL:            "https://accounts.example",
		AccountClientID:     "dfe",
		AccountClientSecret: "s3cr3t",
	}, cache.NewMemoryBackend(8))
	if got == nil {
		t.Fatal("a complete credential did not turn reach on")
	}
}
