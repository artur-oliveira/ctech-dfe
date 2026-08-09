package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func testApp() *fiber.App {
	app := fiber.New()
	RegisterTaxTables(app, func(c fiber.Ctx) error { return c.Next() })
	return app
}

func TestTaxTablesRoute_IcmsAliq_ReturnsResolvedValue(t *testing.T) {
	app := testApp()
	req := httptest.NewRequest(http.MethodGet, "/tax-tables/icms-aliq?emit_uf=SP&dest_uf=RJ&ncm=00000000", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		IcmsAliq string `json:"icms_aliq"`
		FcpAliq  string `json:"fcp_aliq"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.IcmsAliq == "" || body.FcpAliq == "" {
		t.Errorf("expected non-empty rates, got %+v", body)
	}
}

func TestTaxTablesRoute_IcmsAliq_MissingParamsIsBadRequest(t *testing.T) {
	app := testApp()
	req := httptest.NewRequest(http.MethodGet, "/tax-tables/icms-aliq?emit_uf=SP", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
