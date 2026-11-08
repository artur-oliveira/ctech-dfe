package ciot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const testKey = "test-key-not-a-secret"

type fakeGen struct {
	tokens, gerars atomic.Int32
	genStatus      int
	id             string
}

func (f *fakeGen) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/token":
		f.tokens.Add(1)
		if r.Header.Get("Chave") != testKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	case "/gerar":
		f.gerars.Add(1)
		if r.Header.Get("Authorization") != "Bearer jwt" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if f.genStatus != 0 {
			w.WriteHeader(f.genStatus)
			return
		}
		_, _ = w.Write([]byte(`{"sucesso":true,"dados":{"ciot":"` + f.id + `"}}`))
	}
}

func genFor(t *testing.T, env string, primary, fallback *fakeGen) *Generator {
	t.Helper()
	ResetTokenCache()
	ps := httptest.NewServer(primary)
	t.Cleanup(ps.Close)
	pc, _ := NewClient(env, ps.Client(), ps.URL)
	secrets := StaticSecrets{ParamAPIKey: testKey}
	if fallback != nil {
		fs := httptest.NewServer(fallback)
		t.Cleanup(fs.Close)
		secrets[ParamFallbackURL] = fs.URL
	}
	g, err := NewGenerator(context.Background(), env, pc, http.DefaultClient, secrets)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestGeneratorRefusesPlaceholderAndEmptyKeys(t *testing.T) {
	pc, _ := NewClient(EnvHom, http.DefaultClient, "http://x")
	for _, k := range []string{"", "  ", PlaceholderAPIKey} {
		if _, err := NewGenerator(context.Background(), EnvHom, pc, nil, StaticSecrets{ParamAPIKey: k}); err == nil {
			t.Fatalf("key %q accepted", k)
		}
	}
}

func TestGeneratorCachesTokenAndRenews(t *testing.T) {
	p := &fakeGen{id: "ABCDEF123456"}
	g := genFor(t, EnvHom, p, nil)
	clock := time.Now()
	g.now = func() time.Time { return clock }
	for i := 0; i < 3; i++ {
		if id, err := g.Generate(context.Background(), "123.456.789-09"); err != nil || id != "ABCDEF123456" {
			t.Fatal(id, err)
		}
	}
	if p.tokens.Load() != 1 {
		t.Fatalf("token requested %d times", p.tokens.Load())
	}
	clock = clock.Add(TokenLifetime - TokenRenewMargin + time.Second) // inside the renew margin
	if _, err := g.Generate(context.Background(), "12345678909"); err != nil || p.tokens.Load() != 2 {
		t.Fatalf("not renewed: %v %d", err, p.tokens.Load())
	}
}

func TestGeneratorFallbackOnlyOn500InProd(t *testing.T) {
	p := &fakeGen{genStatus: 500}
	f := &fakeGen{id: "FALLBACK0001"}
	g := genFor(t, EnvProd, p, f)
	if id, err := g.Generate(context.Background(), "12345678909"); err != nil || id != "FALLBACK0001" {
		t.Fatal(id, err)
	}
	// 400 from the primary does not fall back.
	p2 := &fakeGen{genStatus: 400}
	g = genFor(t, EnvProd, p2, &fakeGen{id: "FALLBACK0001"})
	if _, err := g.Generate(context.Background(), "12345678909"); err == nil || errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("err %v", err)
	}
}

func TestGeneratorNoFallbackInHomOrWhenUnset(t *testing.T) {
	g := genFor(t, EnvHom, &fakeGen{genStatus: 500}, &fakeGen{id: "FALLBACK0001"})
	if g.fallback != nil {
		t.Fatal("hom must not have a fallback")
	}
	if _, err := g.Generate(context.Background(), "12345678909"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("err %v", err)
	}
	g = genFor(t, EnvProd, &fakeGen{genStatus: 500}, nil)
	if _, err := g.Generate(context.Background(), "12345678909"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("err %v", err)
	}
}
