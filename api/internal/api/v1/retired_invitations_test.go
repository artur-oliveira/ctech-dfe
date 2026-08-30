package v1

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Creating an invitation here is retired: invitations are ctech-account's, and
// only the platform's can name WHICH COMPANIES the invitation grants — the case
// this product is for, an accountant inviting a junior to five of forty CNPJs.
//
// It must ANSWER rather than disappear. Somebody with this call in a script
// deserves to be told where it moved, and a 404 would send them looking for a
// typo in their own integration.
func TestCreatingAnInvitationAnswersRetired(t *testing.T) {
	app := fiber.New()
	app.Post("/invitations", func(c fiber.Ctx) error {
		return sendProblem(c, problem.Retired(
			"os convites agora são criados na sua conta CTech, em Organizações — lá é possível escolher as empresas que a pessoa poderá usar"))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/invitations", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("status = %d, want 410 — a 404 reads as \"you got the path wrong\"", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	body := string(raw)
	if !strings.Contains(body, "conta CTech") {
		t.Errorf("the answer does not say where invitations moved: %s", body)
	}
	// A machine-readable type, so a client branches on "retired" rather than on
	// prose somebody will rewrite.
	if !strings.Contains(body, "retired") {
		t.Errorf("the answer carries no type a client can branch on: %s", body)
	}
	// And it names no organization or company: a retired route is not a place
	// to leak what the caller could not otherwise see.
	for _, leak := range []string{"CNPJ_", "cmp_", "org_"} {
		if strings.Contains(body, leak) {
			t.Errorf("the retirement answer leaks %q: %s", leak, body)
		}
	}
}
