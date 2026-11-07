package v1

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
)

// contingency_msg carrega a senha do certificado e o XML assinado: nunca pode sair numa resposta da API.
func TestUnmarshalDropsInternalAttrs(t *testing.T) {
	item, err := attributevalue.MarshalMap(map[string]any{
		"sk": "chave", "status": "pending", "contingency_msg": `{"cert_password":"segredo"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := unmarshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := got["contingency_msg"]; leaked {
		t.Fatal("contingency_msg vazou na resposta")
	}
	if got["sk"] != "chave" || got["status"] != "pending" {
		t.Fatalf("campos normais perdidos: %v", got)
	}
}
