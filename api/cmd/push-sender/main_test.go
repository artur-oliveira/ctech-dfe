package main

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/api-commons/webpush"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const testHost = "fcm.googleapis.com"

type fakeStore struct {
	mu        sync.Mutex
	devices   map[string][]repositories.PushSubscription
	deleted   []string
	listErr   error
	deleteErr error
}

func (s *fakeStore) ListSubscriptions(_ context.Context, sub string) ([]repositories.PushSubscription, error) {
	return s.devices[sub], s.listErr
}

func (s *fakeStore) DeleteSubscription(_ context.Context, sub, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, sub+"/"+id)
	return nil
}

type fakeSender struct {
	mu      sync.Mutex
	byToken map[string]webpush.Result
	err     error
	sent    []webpush.Message
}

func (f *fakeSender) Send(_ context.Context, sub webpush.Subscription, _ string, m webpush.Message) (webpush.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	for token, res := range f.byToken {
		if strings.HasSuffix(sub.Endpoint, token) {
			return res, f.err
		}
	}
	return webpush.Result{Status: 201, Outcome: webpush.Delivered}, f.err
}

type recordingAlerts struct {
	mu   sync.Mutex
	jobs []string
}

func (r *recordingAlerts) Alert(_ context.Context, a alerts.Alert) {
	r.mu.Lock()
	r.jobs = append(r.jobs, a.Job)
	r.mu.Unlock()
}

func device(t *testing.T, token string) repositories.PushSubscription {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	endpoint := "https://" + testHost + "/fcm/send/" + token
	return repositories.PushSubscription{
		ID: repositories.PushSubscriptionID(endpoint), Endpoint: endpoint, VapidKID: "kid",
		P256dh: base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(auth),
	}
}

func job(recipients ...string) services.PushSendJob {
	return services.PushSendJob{
		EventID: "01EVENT", Type: services.PushEventRejected, Title: "NF-e rejeitada · Acme", Body: "Toque para ver o motivo.",
		Navigate: "/nfe/detail?key=1&org=CNPJ_1", Tag: "document.rejected:ab12", Topic: "topic", TTLSeconds: 3600, Urgency: "high",
		Recipients: recipients,
	}
}

func newHandler(store *fakeStore, sender *fakeSender) (*handler, *recordingAlerts) {
	a := &recordingAlerts{}
	return &handler{store: store, sender: sender, alerts: a, now: time.Now}, a
}

func TestHandleDeliversToEveryDeviceOfEveryRecipient(t *testing.T) {
	store := &fakeStore{devices: map[string][]repositories.PushSubscription{
		"a": {device(t, "a1"), device(t, "a2")}, "b": {device(t, "b1")},
	}}
	sender := &fakeSender{}
	h, al := newHandler(store, sender)
	if err := h.handle(context.Background(), job("a", "b")); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 3 || len(store.deleted) != 0 || len(al.jobs) != 0 {
		t.Errorf("sent=%d deleted=%v alerts=%v", len(sender.sent), store.deleted, al.jobs)
	}
	m := sender.sent[0]
	if m.TTL != time.Hour || m.Urgency != "high" || m.Topic != "topic" || m.DedupeKey != "01EVENT" || len(m.Payload) == 0 {
		t.Errorf("message = %v", m)
	}
}

func TestHandleDeletesAGoneDeviceAndAlertsOnAKeyMismatch(t *testing.T) {
	gone, bad, ok := device(t, "gone"), device(t, "bad"), device(t, "ok")
	store := &fakeStore{devices: map[string][]repositories.PushSubscription{"a": {gone, bad, ok}}}
	sender := &fakeSender{byToken: map[string]webpush.Result{
		"gone": {Status: 410, Outcome: webpush.Gone}, "bad": {Status: 403, Outcome: webpush.Unauthorized},
	}}
	h, al := newHandler(store, sender)
	if err := h.handle(context.Background(), job("a")); err != nil {
		t.Fatalf("a refusing device is not a job failure: %v", err)
	}
	if len(store.deleted) != 2 {
		t.Errorf("deleted = %v, want the gone and the unauthorized device", store.deleted)
	}
	if len(al.jobs) != 1 || al.jobs[0] != "push-vapid-rejected" {
		t.Errorf("alerts = %v", al.jobs)
	}
}

func TestHandleAlertsOnATooLargePayloadWithoutFailing(t *testing.T) {
	store := &fakeStore{devices: map[string][]repositories.PushSubscription{"a": {device(t, "big")}}}
	sender := &fakeSender{byToken: map[string]webpush.Result{"big": {Status: 413, Outcome: webpush.PayloadTooLarge}}}
	h, al := newHandler(store, sender)
	if err := h.handle(context.Background(), job("a")); err != nil {
		t.Fatal(err)
	}
	if len(al.jobs) != 1 || al.jobs[0] != "push-payload-too-large" || len(store.deleted) != 0 {
		t.Errorf("alerts=%v deleted=%v", al.jobs, store.deleted)
	}
}

// Only a failure of this function's own work (the table) fails the run, so Lambda retries; a transient push-service
// failure is dropped, best effort.
func TestHandleFailsOnlyOnItsOwnWork(t *testing.T) {
	failing := &fakeStore{devices: map[string][]repositories.PushSubscription{}, listErr: errors.New("throttled")}
	h, al := newHandler(failing, &fakeSender{})
	if err := h.handle(context.Background(), job("a")); err == nil {
		t.Error("an unreadable table must fail the run")
	}
	if len(al.jobs) != 1 || al.jobs[0] != "push-sender-failed" {
		t.Errorf("alerts = %v", al.jobs)
	}

	store := &fakeStore{devices: map[string][]repositories.PushSubscription{"a": {device(t, "slow")}}}
	sender := &fakeSender{byToken: map[string]webpush.Result{"slow": {Outcome: webpush.Transient}}, err: errors.New("webpush: send to " + testHost + ": timeout")}
	h, al = newHandler(store, sender)
	if err := h.handle(context.Background(), job("a")); err != nil {
		t.Errorf("a transient push-service failure must not fail the run: %v", err)
	}
	if len(al.jobs) != 0 {
		t.Errorf("alerts = %v", al.jobs)
	}

	// A delete that fails is the function's own work failing: retried.
	gone := &fakeStore{devices: map[string][]repositories.PushSubscription{"a": {device(t, "gone")}}, deleteErr: errors.New("throttled")}
	h, _ = newHandler(gone, &fakeSender{byToken: map[string]webpush.Result{"gone": {Status: 410, Outcome: webpush.Gone}}})
	if err := h.handle(context.Background(), job("a")); err == nil {
		t.Error("a gone device that cannot be deleted must fail the run")
	}
}

func TestHandleDropsAStoredRowThatNoLongerParsesAndRefusesABadJob(t *testing.T) {
	broken := repositories.PushSubscription{ID: "deadbeefdeadbeef", Endpoint: "http://insecure.example/x", P256dh: "x", Auth: "y"}
	store := &fakeStore{devices: map[string][]repositories.PushSubscription{"a": {broken}}}
	sender := &fakeSender{}
	h, _ := newHandler(store, sender)
	if err := h.handle(context.Background(), job("a")); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 1 || len(sender.sent) != 0 {
		t.Errorf("deleted=%v sent=%d: a row that cannot parse must be dropped, never sent", store.deleted, len(sender.sent))
	}

	bad := job("a")
	bad.Navigate = "https://evil.example/"
	if err := h.handle(context.Background(), bad); err != nil || len(sender.sent) != 0 {
		t.Errorf("a refused job must not retry or send: %v %d", err, len(sender.sent))
	}
}

type fakeSSM map[string]string

func (f fakeSSM) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	v, ok := f[aws.ToString(in.Name)]
	if !ok {
		return nil, &ssmtypes.ParameterNotFound{}
	}
	return &ssm.GetParameterOutput{Parameter: &ssmtypes.Parameter{Value: aws.String(v)}}, nil
}

var paths = paramPaths{private: "/p/private", previous: "/p/previous", public: "/p/public", subject: "/p/subject"}

func TestLoadIsQuietWithoutParametersAndStrictAboutMismatch(t *testing.T) {
	h := &handler{}
	if err := h.load(context.Background(), fakeSSM{}, paths); !errors.Is(err, errNotConfigured) {
		t.Errorf("no parameters: %v, want errNotConfigured (off, not failing)", err)
	}

	pem, key, err := webpush.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	ok := fakeSSM{"/p/private": pem, "/p/public": key.PublicBase64URL(), "/p/subject": "https://dfe.example"}
	if err := h.load(context.Background(), ok, paths); err != nil || h.sender == nil {
		t.Fatalf("a complete set must load: %v", err)
	}

	_, other, _ := webpush.GenerateKey()
	mismatch := fakeSSM{"/p/private": pem, "/p/public": other.PublicBase64URL(), "/p/subject": "https://dfe.example"}
	if err := (&handler{}).load(context.Background(), mismatch, paths); err == nil || errors.Is(err, errNotConfigured) {
		t.Errorf("a public key that does not match the private one must fail loudly: %v", err)
	}

	noSubject := fakeSSM{"/p/private": pem, "/p/public": key.PublicBase64URL()}
	if err := (&handler{}).load(context.Background(), noSubject, paths); err == nil {
		t.Error("a missing subject must fail")
	}

	prevPEM, _, _ := webpush.GenerateKey()
	withPrev := fakeSSM{"/p/private": pem, "/p/public": key.PublicBase64URL(), "/p/subject": "https://dfe.example", "/p/previous": prevPEM}
	if err := (&handler{}).load(context.Background(), withPrev, paths); err != nil {
		t.Errorf("a previous key must load: %v", err)
	}
}

func TestLogHelpersNeverCarryTheEndpoint(t *testing.T) {
	endpoint := "https://" + testHost + "/fcm/send/SECRET-TOKEN"
	if got := hostOf(endpoint); got != testHost {
		t.Errorf("hostOf = %q", got)
	}
	if strings.Contains(subHash("user-1"), "user") || len(subHash("user-1")) != 8 {
		t.Errorf("subHash = %q", subHash("user-1"))
	}
}
