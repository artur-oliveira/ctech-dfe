package v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	ctesvc "gopkg.aoctech.app/dfe/api/internal/services/ctes"

	"github.com/gofiber/fiber/v3"
)

func TestRegisterCTes_MountsAllRoutes(t *testing.T) {
	app := fiber.New()
	RegisterCTes(app, nil, nil, func(c fiber.Ctx) error { return c.Next() }, nil)
	mounted := map[string]bool{}
	for _, r := range app.GetRoutes() {
		mounted[r.Method+" "+r.Path] = true
	}
	for _, w := range []string{"POST /ctes/derive", "POST /ctes", "GET /ctes", "GET /ctes/:access_key"} {
		if !mounted[w] {
			t.Errorf("rota não montada: %s", w)
		}
	}
}

// O corpo da emissão passa pelo mesmo bindJSON dos handlers: campo desconhecido é 400, regra violada é 422.
func TestCteEmitBody_ValidationErrors(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		var body ctesvc.CteEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	key := services.GenerateAccessKey("SP", "11222333000181", services.ModelNFe, 1, 1, time.Now(), services.TpEmisNormal)
	cte := services.GenerateAccessKey("SP", "11222333000181", services.ModelCTe, 1, 1, time.Now(), services.TpEmisNormal)
	ok := `"taker":{"role":"remetente"},"freight":{"total":"1850.00"}`

	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"válido":                 {`{"nfe_keys":["` + key + `"],` + ok + `}`, fiber.StatusNoContent},
		"sem notas":              {`{"nfe_keys":[],` + ok + `}`, fiber.StatusUnprocessableEntity},
		"chave de CT-e":          {`{"nfe_keys":["` + cte + `"],` + ok + `}`, fiber.StatusUnprocessableEntity},
		"chave repetida":         {`{"nfe_keys":["` + key + `","` + key + `"],` + ok + `}`, fiber.StatusUnprocessableEntity},
		"papel do tomador":       {`{"nfe_keys":["` + key + `"],"taker":{"role":"x"},"freight":{"total":"1"}}`, fiber.StatusUnprocessableEntity},
		"frete com 3 casas":      {`{"nfe_keys":["` + key + `"],"taker":{"role":"remetente"},"freight":{"total":"1.123"}}`, fiber.StatusUnprocessableEntity},
		"componente longo":       {`{"nfe_keys":["` + key + `"],"taker":{"role":"remetente"},"freight":{"total":"1","components":[{"name":"NOME COM MAIS DE 15","value":"1"}]}}`, fiber.StatusUnprocessableEntity},
		"CST fora da lista":      {`{"nfe_keys":["` + key + `"],` + ok + `,"icms":{"cst":"10"}}`, fiber.StatusUnprocessableEntity},
		"município de 6 dígitos": {`{"nfe_keys":["` + key + `"],` + ok + `,"overrides":{"origin":{"city_ibge":"355030"}}}`, fiber.StatusUnprocessableEntity},
		"campo desconhecido":     {`{"nfe_keys":["` + key + `"],` + ok + `,"x":1}`, fiber.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.want {
				t.Fatalf("status %d, esperado %d", resp.StatusCode, tc.want)
			}
		})
	}
}
