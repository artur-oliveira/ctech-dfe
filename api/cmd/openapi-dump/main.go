// Command openapi-dump imprime a spec OpenAPI unida em JSON.
//
// Existe para o `make openapi-lint`: a validação da spec é feita pelo Redocly,
// que precisa do documento único, e a união acontece dentro do pacote de rotas.
package main

import (
	"fmt"
	"os"

	v1 "gopkg.aoctech.app/dfe/api/internal/api/v1"
)

func main() {
	spec, err := v1.SpecJSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, "openapi-dump:", err)
		os.Exit(1)
	}
	os.Stdout.Write(spec)
}
