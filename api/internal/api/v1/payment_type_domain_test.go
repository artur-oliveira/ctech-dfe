package v1

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services/nfes"
)

// F-07 (auditoria MOC NF-e): PaymentTermBody e NfePaymentItem usam o mesmo domínio de tPag e ele inclui
// 23, 24 e 91 (tabela de meios de pagamento; o XSD aceita [0-9]{2}).
func TestPaymentTypeDomainIgualENaoRecusaVigentes(t *testing.T) {
	oneof := func(typ reflect.Type) string {
		f, _ := typ.FieldByName("PaymentType")
		for _, p := range strings.Split(f.Tag.Get("validate"), ",") {
			if v, ok := strings.CutPrefix(p, "oneof="); ok {
				return v
			}
		}
		return ""
	}
	a, b := oneof(reflect.TypeOf(PaymentTermBody{})), oneof(reflect.TypeOf(nfes.NfePaymentItem{}))
	if a != b {
		t.Fatalf("domínios diferentes:\n%s\n%s", a, b)
	}
	for _, c := range []string{"23", "24", "91"} {
		if !strings.Contains(" "+a+" ", " "+c+" ") {
			t.Errorf("tPag %s fora do domínio", c)
		}
	}
}
