package service

import (
	"context"
	"testing"
	"time"
)

func TestLeaseForFollowsLambdaDeadline(t *testing.T) {
	now := time.Now()
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(150*time.Second))
	defer cancel()
	if got, want := leaseFor(ctx, now), 150*time.Second+leaseMargin; got != want {
		t.Fatalf("lease = %v, want %v", got, want)
	}
	if got := leaseFor(context.Background(), now); got != processingLeaseDuration {
		t.Fatalf("no deadline: lease = %v, want fallback %v", got, processingLeaseDuration)
	}
	expired, cancel2 := context.WithDeadline(context.Background(), now.Add(-time.Second))
	defer cancel2()
	if got := leaseFor(expired, now); got != processingLeaseDuration {
		t.Fatalf("expired deadline: lease = %v, want fallback", got)
	}
}
