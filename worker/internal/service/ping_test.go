package service

import "testing"

func TestIsPingEvent(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"ping true", `{"ping":true}`, true},
		{"ping false", `{"ping":false}`, false},
		{"sns event", `{"Records":[{"Sns":{"Message":"{}"}}]}`, false},
		{"distribution job", `{"job_type":"dist_nsu","org_pk":"x"}`, false},
		{"empty object", `{}`, false},
		{"invalid json", `not json`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPingEvent([]byte(tt.raw)); got != tt.want {
				t.Errorf("IsPingEvent(%s) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
