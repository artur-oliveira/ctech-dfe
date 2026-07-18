package service

import "encoding/json"

// pingEvent is the synthetic invoke payload EventBridge Scheduler sends on a
// keep-warm tick — never a real SQS batch (SQS event source payloads always
// have a top-level "Records" array, never "ping").
type pingEvent struct {
	Ping bool `json:"ping"`
}

// IsPingEvent reports whether raw is a keep-warm invoke rather than a real
// SQS batch. Keep-warm invokes exist only to hold a Lambda execution
// environment open (avoid cold start) — they must never reach SEFAZ, touch
// DynamoDB/S3, or require any argument.
func IsPingEvent(raw json.RawMessage) bool {
	var probe pingEvent
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.Ping
}
