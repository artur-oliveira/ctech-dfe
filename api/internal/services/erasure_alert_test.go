package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/api-commons/erasure"
)

type captureAlerts struct{ got []alerts.Alert }

func (c *captureAlerts) Alert(_ context.Context, a alerts.Alert) { c.got = append(c.got, a) }

func TestAlertingPurge(t *testing.T) {
	boom := errors.New("boom")
	for _, tc := range []struct {
		name      string
		err       error
		wantAlert bool
	}{
		{"success", nil, false},
		{"settling is a wait, not a failure", fmt.Errorf("wrapped: %w", ErrErasureSettling), false},
		{"real failure", boom, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &captureAlerts{}
			purge := AlertingPurge(func(context.Context, erasure.Message) (erasure.Ack, error) {
				return erasure.Ack{}, tc.err
			}, c)
			_, err := purge(context.Background(), erasure.Message{RequestID: "req-1"})
			if !errors.Is(err, tc.err) && tc.err != nil {
				t.Fatalf("error must pass through unchanged, got %v", err)
			}
			if (len(c.got) == 1) != tc.wantAlert || len(c.got) > 1 {
				t.Fatalf("alerts = %+v, want alert=%v", c.got, tc.wantAlert)
			}
			if tc.wantAlert && (c.got[0].Err != boom || c.got[0].Detail != "request_id: req-1") {
				t.Fatalf("alert must carry the error and request id: %+v", c.got[0])
			}
		})
	}
}
