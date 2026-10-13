package v1

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// dfe has no blocker of its own (plan ruling R2): every sub is eligible, and
// the body must be the exact erasure.Eligibility shape ctech-account decodes.
func TestErasureEligibility_AlwaysEligible(t *testing.T) {
	app := fiber.New()
	app.Get("/e/:sub", eligibility)
	resp, err := app.Test(httptest.NewRequest("GET", "/e/user-1", nil))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	blockers, ok := body["blockers"].([]any)
	if resp.StatusCode != 200 || body["eligible"] != true || !ok || len(blockers) != 0 {
		t.Fatalf("status %d body %s, want 200 {eligible:true, blockers:[]}", resp.StatusCode, raw)
	}
}
