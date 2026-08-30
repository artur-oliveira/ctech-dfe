package problem

import (
	"net/http"
	"strings"
	"testing"
)

// A retired route answers 410 and says where the thing moved.
//
// Not a 404: 404 says "there is nothing here", which sends somebody looking for
// a typo in their own integration. 410 says "this existed and is gone", which is
// the true thing and the one that makes them read the message.
func TestRetiredSaysWhereItWent(t *testing.T) {
	p := Retired("os convites agora são gerenciados na sua conta CTech")
	if p.Status != http.StatusGone {
		t.Fatalf("status = %d, want 410", p.Status)
	}
	if !strings.Contains(p.Detail, "conta CTech") {
		t.Errorf("the detail does not say where it moved: %q", p.Detail)
	}
	// A machine-readable type, so a client can branch on "retired" rather than
	// matching prose that will be rewritten.
	if p.Type == "" || !strings.Contains(p.Type, "retired") {
		t.Errorf("type = %q, want something a client can branch on", p.Type)
	}
}
