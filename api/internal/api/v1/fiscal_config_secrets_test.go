package v1

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// O CSRT e o CSC identificam o emitente perante a SEFAZ: quem os tem assina no
// lugar dele. A API nunca pode devolvê-los, nem no PUT que acabou de gravá-los.
func TestRedactFiscalSecretsRemoveCSRTeCSC(t *testing.T) {
	item := map[string]types.AttributeValue{
		"environment": &types.AttributeValueMemberN{Value: "2"},
		"csrt_id":     &types.AttributeValueMemberS{Value: "01"},
		"csrt":        &types.AttributeValueMemberS{Value: "G8063NG5H4YQ01M4L3AKG25OZ4A2GL123456"},
		"prod_csc":    &types.AttributeValueMemberS{Value: "segredo-prod"},
		"hom_csc":     &types.AttributeValueMemberS{Value: "segredo-hom"},
	}
	got := redactFiscalSecrets(item)
	for _, k := range []string{"csrt", "prod_csc", "hom_csc"} {
		if _, ok := got[k]; ok {
			t.Errorf("%q vazou na resposta", k)
		}
	}
	// O identificador do CSRT não é segredo: o XML o carrega em claro.
	if _, ok := got["csrt_id"]; !ok {
		t.Error("csrt_id não é segredo e deveria permanecer")
	}
	if _, ok := got["environment"]; !ok {
		t.Error("campos não-secretos não podem ser removidos")
	}
	// Cópia, não deleção no lugar: o mesmo item pode estar no cache.
	if _, ok := item["csrt"]; !ok {
		t.Error("redactFiscalSecrets não pode mutar o item de origem")
	}
}

func TestRedactFiscalSecretsNilPassaLimpo(t *testing.T) {
	if got := redactFiscalSecrets(nil); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}
