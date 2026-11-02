package service

import "testing"

func TestSNSMessages(t *testing.T) {
	raw := `{"Records":[{"EventSource":"aws:sns","Sns":{"MessageId":"id-1","Message":"{\"doc_pk\":\"p\"}"}}]}`
	got, err := SNSMessages([]byte(raw))
	if err != nil || len(got) != 1 || got[0].ID != "id-1" || got[0].Body != `{"doc_pk":"p"}` {
		t.Fatalf("got %+v err=%v", got, err)
	}
	if got, err := SNSMessages([]byte(`{}`)); err != nil || len(got) != 0 {
		t.Fatalf("empty event: got %+v err=%v", got, err)
	}
	if _, err := SNSMessages([]byte(`not json`)); err == nil {
		t.Fatal("want error for non-JSON payload")
	}
}
