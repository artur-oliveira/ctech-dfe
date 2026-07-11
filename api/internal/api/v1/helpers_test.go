package v1

import (
	"testing"

	"github.com/artur-oliveira/ctech-dfe/api/internal/services"
)

func TestExtractCrtAndRegs_ReadsNestedPersonFields(t *testing.T) {
	item := map[string]any{
		"person": map[string]any{
			"crt": float64(3),
			"state_registrations": []any{
				map[string]any{"uf": "SP", "state_registration": "123456"},
			},
		},
	}
	crt, regs := extractCrtAndRegs(item)
	if crt == nil || *crt != 3 {
		t.Fatalf("crt = %v, want 3", crt)
	}
	if len(regs) != 1 || regs[0] != (services.StateRegistrationEntry{UF: "SP", StateRegistration: "123456"}) {
		t.Fatalf("regs = %+v", regs)
	}
}

func TestExtractCrtAndRegs_MissingPersonReturnsZeroValues(t *testing.T) {
	crt, regs := extractCrtAndRegs(map[string]any{})
	if crt != nil {
		t.Errorf("crt = %v, want nil", crt)
	}
	if len(regs) != 0 {
		t.Errorf("regs = %+v, want empty", regs)
	}
}

func TestToStateRegEntries_ConvertsBodySlice(t *testing.T) {
	out := toStateRegEntries([]StateRegistrationBody{{UF: "RJ", StateRegistration: "999"}})
	want := []services.StateRegistrationEntry{{UF: "RJ", StateRegistration: "999"}}
	if len(out) != 1 || out[0] != want[0] {
		t.Fatalf("got %+v, want %+v", out, want)
	}
}
