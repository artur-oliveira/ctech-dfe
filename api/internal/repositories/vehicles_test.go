package repositories

import "testing"

func TestBuildVehicleSK_AddsPrefix(t *testing.T) {
	if got := buildVehicleSK("abc123"); got != "VEHICLE_abc123" {
		t.Errorf("buildVehicleSK(%q) = %q, want VEHICLE_abc123", "abc123", got)
	}
}

func TestBuildVehicleSK_IdempotentWithPrefix(t *testing.T) {
	if got := buildVehicleSK("VEHICLE_abc123"); got != "VEHICLE_abc123" {
		t.Errorf("buildVehicleSK(%q) = %q, want unchanged", "VEHICLE_abc123", got)
	}
}
