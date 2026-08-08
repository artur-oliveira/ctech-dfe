package v1

// Serve a especificação OpenAPI e a página de documentação.
//
// A spec é escrita à mão em openapi/*.yaml e embarcada no binário — não há
// geração de código nem passo de build extra. Os arquivos são fragmentos do
// mesmo documento (`paths`, `components`), unidos em memória no primeiro
// acesso; as referências continuam sendo internas (`#/components/schemas/X`),
// porque depois da união existe um documento só.
//
// O teste de cobertura em openapi_test.go garante que a spec não fique para
// trás das rotas.
//
// As rotas ficam fora do grupo /v1.0 e são públicas: documentação atrás de
// autenticação não é documentação.

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sync"

	"github.com/gofiber/fiber/v3"
	"gopkg.in/yaml.v3"
)

//go:embed openapi/*.yaml
var openapiFS embed.FS

// openapiDir é a pasta dos fragmentos dentro do FS embarcado.
const openapiDir = "openapi"

// Caminhos da documentação. Ficam na raiz porque descrevem a API inteira, não
// uma versão dela.
const (
	OpenAPIJSONPath = "/openapi.json"
	OpenAPIYAMLPath = "/openapi.yaml"
	DocsPath        = "/docs"
)

const (
	mimeOpenAPIYAML = "application/yaml; charset=utf-8"
	mimeHTML        = "text/html; charset=utf-8"
	mimeJSON        = "application/json; charset=utf-8"
)

// Stoplight Elements vem do CDN com versão fixa e Subresource Integrity: sem o
// pin, um release quebrado derruba a documentação sem nenhuma mudança nossa;
// sem o SRI, um CDN comprometido executa script arbitrário na nossa origem.
// Ao subir a versão, recalcule os hashes:
//
//	curl -sfL https://unpkg.com/@stoplight/elements@<v>/<arquivo> |
//	  openssl dgst -sha384 -binary | openssl base64 -A
const (
	elementsVersion = "8.4.6"
	elementsCSSSRI  = "sha384-oYu9Au1JU1Sd5Za5LYSepn+Sofm8uvVdUCxLWbJYesNAS72Y7G/gQ0pjiB6wyf1Z"
	elementsJSSRI   = "sha384-aVLrUQSddwM9PSF3tnJ7D2Ob6HUFEXaukrJXb5XJWX2b+gQPMNzj479qnLT85/9T"
)

// docsHTML é a página do Stoplight Elements. router="hash" mantém a navegação
// dentro do fragmento da URL, então o servidor não precisa de rota por
// operação.
const docsHTML = `<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>CTech DFe API</title>
  <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements@` + elementsVersion + `/styles.min.css"
        integrity="` + elementsCSSSRI + `" crossorigin="anonymous">
  <script src="https://unpkg.com/@stoplight/elements@` + elementsVersion + `/web-components.min.js"
          integrity="` + elementsJSSRI + `" crossorigin="anonymous"></script>
  <style>html,body{margin:0;height:100%}</style>
</head>
<body>
  <elements-api apiDescriptionUrl="` + OpenAPIJSONPath + `" router="hash" layout="sidebar"></elements-api>
</body>
</html>`

var (
	specOnce sync.Once
	specDoc  map[string]any
	specJSON []byte
	specYAML []byte
	specErr  error
)

// loadSpec une os fragmentos uma única vez e guarda as duas serializações.
// YAML é a fonte da verdade (comentários, ordem, legibilidade); JSON existe
// porque é o que o Elements e os geradores de cliente consomem sem atrito.
func loadSpec() error {
	specOnce.Do(func() {
		specDoc, specErr = mergeSpec()
		if specErr != nil {
			return
		}
		if specJSON, specErr = json.Marshal(specDoc); specErr != nil {
			return
		}
		specYAML, specErr = yaml.Marshal(specDoc)
	})
	return specErr
}

// mergeSpec lê todos os fragmentos e devolve o documento único. Chaves
// repetidas entre arquivos são erro, não sobrescrita silenciosa: duas
// definições do mesmo schema significam que uma delas está sendo ignorada.
func mergeSpec() (map[string]any, error) {
	entries, err := openapiFS.ReadDir(openapiDir)
	if err != nil {
		return nil, err
	}
	merged := map[string]any{}
	for _, e := range entries { // ReadDir devolve ordenado por nome
		name := path.Join(openapiDir, e.Name())
		raw, err := openapiFS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var frag map[string]any
		if err := yaml.Unmarshal(raw, &frag); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if err := mergeInto(merged, frag, name, ""); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

// mergeInto funde src em dst recursivamente. Só mapas são fundidos; qualquer
// outra colisão é reportada com o caminho da chave.
func mergeInto(dst, src map[string]any, file, prefix string) error {
	for k, v := range src {
		cur, exists := dst[k]
		if !exists {
			dst[k] = v
			continue
		}
		curMap, curOK := cur.(map[string]any)
		newMap, newOK := v.(map[string]any)
		if !curOK || !newOK {
			return fmt.Errorf("%s: chave duplicada na spec: %s%s", file, prefix, k)
		}
		if err := mergeInto(curMap, newMap, file, prefix+k+"."); err != nil {
			return err
		}
	}
	return nil
}

// SpecJSON devolve a spec unida em JSON. Exportada para o `make openapi-lint`,
// que valida o documento com uma ferramenta externa.
func SpecJSON() ([]byte, error) {
	if err := loadSpec(); err != nil {
		return nil, err
	}
	return specJSON, nil
}

// RegisterDocs monta /openapi.json, /openapi.yaml e /docs na raiz da aplicação.
func RegisterDocs(app *fiber.App) {
	app.Get(OpenAPIJSONPath, func(c fiber.Ctx) error {
		if err := loadSpec(); err != nil {
			return sendProblem(c, err)
		}
		c.Set(fiber.HeaderContentType, mimeJSON)
		return c.Send(specJSON)
	})

	app.Get(OpenAPIYAMLPath, func(c fiber.Ctx) error {
		if err := loadSpec(); err != nil {
			return sendProblem(c, err)
		}
		c.Set(fiber.HeaderContentType, mimeOpenAPIYAML)
		return c.Send(specYAML)
	})

	app.Get(DocsPath, func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, mimeHTML)
		return c.SendString(docsHTML)
	})
}
