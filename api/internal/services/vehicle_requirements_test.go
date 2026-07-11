package services

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func vehicleItem(t *testing.T, fields map[string]any) map[string]types.AttributeValue {
	t.Helper()
	av, err := attributevalue.MarshalMap(fields)
	if err != nil {
		t.Fatalf("MarshalMap: %v", err)
	}
	return av
}

func TestMissing_MdfeTractor_AllPresent(t *testing.T) {
	item := vehicleItem(t, map[string]any{"weight": 8000, "wheelset": "01", "bodywork": "00"})
	if got := Missing(item, DocTypeMdfe, VehicleRoleTractor); len(got) != 0 {
		t.Errorf("Missing() = %v, want empty", got)
	}
}

func TestMissing_MdfeTractor_AllAbsent(t *testing.T) {
	item := vehicleItem(t, map[string]any{})
	got := Missing(item, DocTypeMdfe, VehicleRoleTractor)
	want := []string{"weight", "wheelset", "bodywork"}
	if len(got) != len(want) {
		t.Fatalf("Missing() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Missing()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMissing_MdfeTrailer_RequiresCapKGNotWheelset(t *testing.T) {
	item := vehicleItem(t, map[string]any{"weight": 8000, "bodywork": "00"})
	got := Missing(item, DocTypeMdfe, VehicleRoleTrailer)
	if len(got) != 1 || got[0] != "cap_kg" {
		t.Errorf("Missing() = %v, want [cap_kg]", got)
	}
}

func TestMissing_Nfe_NeverRequiresAnything(t *testing.T) {
	item := vehicleItem(t, map[string]any{})
	if got := Missing(item, DocTypeNfe, VehicleRoleTractor); len(got) != 0 {
		t.Errorf("Missing() = %v, want empty for nfe", got)
	}
}

func TestMissing_CteOS_NeverRequiresAnything(t *testing.T) {
	item := vehicleItem(t, map[string]any{})
	if got := Missing(item, DocTypeCteOS, VehicleRoleTractor); len(got) != 0 {
		t.Errorf("Missing() = %v, want empty for cte_os", got)
	}
}
