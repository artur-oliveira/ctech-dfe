package v1

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Este arquivo é o antídoto contra documentação que envelhece: a spec é escrita
// à mão, então nada além de um teste impede que uma rota nova nasça sem
// documentação (ou que uma rota removida continue documentada).
//
// Os serviços são nil de propósito — registrar rota não invoca handler, e o que
// se verifica aqui é a tabela de rotas, não o comportamento.

// buildFullApp monta o mesmo roteador da produção.
func buildFullApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	Register(app, cache.NewMemoryBackend(1), &config.Config{}, nil, nil, Services{})
	return app
}

// routeKeys devolve as rotas vivas no formato da spec ("GET /v1.0/nfes/{id}").
// Rotas de documentação ficam de fora: elas descrevem a spec, não fazem parte
// dela.
func routeKeys(app *fiber.App) map[string]bool {
	docRoutes := map[string]bool{
		OpenAPIJSONPath: true,
		OpenAPIYAMLPath: true,
		DocsPath:        true,
	}
	keys := map[string]bool{}
	for _, r := range app.GetRoutes(true) {
		if r.Method == fiber.MethodHead || r.Method == fiber.MethodOptions {
			continue
		}
		if docRoutes[r.Path] {
			continue
		}
		keys[r.Method+" "+specPath(r.Path)] = true
	}
	return keys
}

// specPath converte os parâmetros do Fiber (":id") para os da OpenAPI ("{id}").
func specPath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, ":") {
			parts[i] = "{" + strings.TrimSuffix(strings.TrimPrefix(seg, ":"), "?") + "}"
		}
	}
	return strings.Join(parts, "/")
}

// specKeys devolve as operações declaradas na spec unida.
func specKeys(t *testing.T) map[string]bool {
	t.Helper()
	doc, err := mergeSpec()
	if err != nil {
		t.Fatalf("spec inválida: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("spec sem seção paths")
	}
	keys := map[string]bool{}
	for path, raw := range paths {
		ops, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("path %s malformado", path)
		}
		for method := range ops {
			// "parameters" é comum ao path, não é operação.
			if method == "parameters" {
				continue
			}
			keys[strings.ToUpper(method)+" "+path] = true
		}
	}
	return keys
}

func TestOpenAPI_DocumentsEveryRegisteredRoute(t *testing.T) {
	live := routeKeys(buildFullApp(t))
	documented := specKeys(t)

	for route := range live {
		if !documented[route] {
			t.Errorf("rota sem documentação em openapi.yaml: %s", route)
		}
	}
}

func TestOpenAPI_HasNoStaleOperations(t *testing.T) {
	live := routeKeys(buildFullApp(t))
	documented := specKeys(t)

	for route := range documented {
		if !live[route] {
			t.Errorf("operação documentada não existe no roteador: %s", route)
		}
	}
}

// A união dos fragmentos precisa produzir um documento servível — é dela que
// saem /openapi.json e /openapi.yaml.
func TestOpenAPI_SpecLoads(t *testing.T) {
	if err := loadSpec(); err != nil {
		t.Fatalf("carga da spec falhou: %v", err)
	}
	if len(specJSON) == 0 || len(specYAML) == 0 {
		t.Fatal("spec serializada vazia")
	}
	for _, k := range []string{"openapi", "info", "paths", "components"} {
		if _, ok := specDoc[k]; !ok {
			t.Errorf("spec unida sem a seção %q", k)
		}
	}
}

func TestOpenAPI_AuxiliaryDocumentsReturnCachedDownloadDescriptor(t *testing.T) {
	doc, err := mergeSpec()
	if err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	for _, path := range []string{
		"/v1.0/nfes/{access_key}/danfe",
		"/v1.0/nfces/{access_key}/danfce",
		"/v1.0/mdfes/{access_key}/damdfe",
	} {
		operation := paths[path].(map[string]any)["get"].(map[string]any)
		responses := operation["responses"].(map[string]any)
		response := responses["200"].(map[string]any)
		if got := response["$ref"]; got != "#/components/responses/CachedPdfDownload" {
			t.Errorf("%s 200 response = %v", path, got)
		}
	}
}
