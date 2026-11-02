package main

import (
	"context"
	"errors"
	"testing"

	"gopkg.aoctech.app/dfe/worker/internal/service"
)

type fakeProc struct {
	got []service.WorkerMessage
	err error
}

func (f *fakeProc) Process(_ context.Context, m service.WorkerMessage) error {
	f.got = append(f.got, m)
	return f.err
}

const snsEvt = `{"Records":[{"Sns":{"MessageId":"m1","Message":"{\"doc_pk\":\"p1\",\"access_key\":\"K1\"}"}}]}`

func TestHandleParsesSNSMessage(t *testing.T) {
	p := &fakeProc{}
	if err := handle(context.Background(), []byte(snsEvt), p); err != nil {
		t.Fatal(err)
	}
	if len(p.got) != 1 || p.got[0].DocPK != "p1" || p.got[0].AccessKey != "K1" {
		t.Fatalf("got %+v", p.got)
	}
}

func TestHandlePingShortCircuits(t *testing.T) {
	p := &fakeProc{}
	if err := handle(context.Background(), []byte(`{"ping":true}`), p); err != nil || len(p.got) != 0 {
		t.Fatalf("ping must be a no-op: err=%v got=%v", err, p.got)
	}
}

// A failure must reach Lambda (non-nil) or async retry and the DLQ destination never run.
func TestHandleReturnsProcessAndParseErrors(t *testing.T) {
	boom := errors.New("boom")
	if err := handle(context.Background(), []byte(snsEvt), &fakeProc{err: boom}); !errors.Is(err, boom) {
		t.Fatalf("process error not returned: %v", err)
	}
	bad := `{"Records":[{"Sns":{"MessageId":"m2","Message":"not json"}}]}`
	p := &fakeProc{}
	if err := handle(context.Background(), []byte(bad), p); err == nil || len(p.got) != 0 {
		t.Fatalf("unparseable body must fail without processing: err=%v got=%v", err, p.got)
	}
	if err := handle(context.Background(), []byte(`not json`), p); err == nil {
		t.Fatal("non-JSON event must fail")
	}
}
