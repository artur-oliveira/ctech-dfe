// Command push-sender sends a Web Push notification the API rendered. The API hands it a PushSendJob (the
// notification text, the link, the people who get it) as an async invoke; this function reads each person's devices
// from the `push` table and sends through go-common/webpush. It decides nothing about content and holds no state of
// its own: a device the push service says is gone (404, 410, or a key mismatch) is deleted from the table.
// docs/specs/2026-11-17-web-push.md.
//
// It lives in the api module because the repository it drives is api/internal. Without the VAPID parameters it does
// nothing and does not fail (Web Push is off until an operator writes them; the API never invokes it before then).
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/api-commons/awsconfig"
	"gopkg.aoctech.app/api-commons/webpush"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// sendWorkers is how many devices are sent to at once.
const sendWorkers = 8

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	region := os.Getenv("AWS_REGION")
	ac, err := awsconfig.Load(rootCtx, region)
	if err != nil {
		panic("aws config: " + err.Error())
	}
	pub, err := alerts.New(rootCtx, region, os.Getenv("ALERTS_TOPIC_ARN"), "dfe-push", os.Getenv("APP_ENVIRONMENT"))
	if err != nil {
		slog.WarnContext(rootCtx, "alerts: publisher unavailable, operator alerts are log-only", "err", err)
		pub = alerts.Nop{}
	}
	repo := repositories.NewPushRepository(dynamodb.NewFromConfig(ac), &config.Config{TablePrefix: os.Getenv("TABLE_PREFIX")})
	h := &handler{store: repo, alerts: pub, now: time.Now}

	// The keys are read on the first invocation, not at start: a function deployed before the operator wrote the
	// parameters stays quiet instead of failing its init.
	var once sync.Once
	var initErr error
	lambda.StartWithOptions(func(ctx context.Context, job services.PushSendJob) error {
		once.Do(func() {
			initErr = h.load(ctx, ssm.NewFromConfig(ac), paramPaths{
				private: os.Getenv("PUSH_VAPID_PRIVATE_KEY_SSM_PATH"), previous: os.Getenv("PUSH_VAPID_PRIVATE_KEY_PREVIOUS_SSM_PATH"),
				public: os.Getenv("PUSH_VAPID_PUBLIC_KEY_SSM_PATH"), subject: os.Getenv("PUSH_SUBJECT_SSM_PATH"),
			})
		})
		if errors.Is(initErr, errNotConfigured) {
			slog.WarnContext(ctx, "push: VAPID parameters are not set; nothing sent")
			return nil
		}
		if initErr != nil {
			once = sync.Once{} // a transient SSM failure must not poison the container: try again next time
			return initErr
		}
		return h.handle(ctx, job)
	}, lambda.WithContext(rootCtx))
}

type parameterReader interface {
	GetParameter(ctx context.Context, in *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

type paramPaths struct{ private, previous, public, subject string }

// errNotConfigured means the operator has not written the VAPID parameters (yet).
var errNotConfigured = errors.New("push: VAPID parameters are not set")

type senderStore interface {
	ListSubscriptions(ctx context.Context, sub string) ([]repositories.PushSubscription, error)
	DeleteSubscription(ctx context.Context, sub, id string) error
}

type pushSender interface {
	Send(ctx context.Context, sub webpush.Subscription, kid string, m webpush.Message) (webpush.Result, error)
}

type handler struct {
	store  senderStore
	sender pushSender
	alerts alerts.Publisher
	now    func() time.Time
}

// readParam reads one parameter. missing is true when it does not exist (not an error: it is how "off" looks).
func readParam(ctx context.Context, c parameterReader, path string) (value string, missing bool, err error) {
	if path == "" {
		return "", true, nil
	}
	out, err := c.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(path), WithDecryption: aws.Bool(true)})
	var notFound *ssmtypes.ParameterNotFound
	if errors.As(err, &notFound) {
		return "", true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading %s: %w", path, err)
	}
	return aws.ToString(out.Parameter.Value), false, nil
}

// load reads the VAPID keys and builds the sender. The public parameter must equal the key derived from the private
// one: a mismatch would make every subscription (made with the public key) fail with 401 at the push service.
func (h *handler) load(ctx context.Context, c parameterReader, p paramPaths) error {
	privatePEM, missing, err := readParam(ctx, c, p.private)
	if err != nil {
		return err
	}
	if missing {
		return errNotConfigured
	}
	key, err := webpush.ParseKey(privatePEM)
	if err != nil {
		return fmt.Errorf("push: the VAPID private key does not parse: %w", err)
	}
	publicKey, missing, err := readParam(ctx, c, p.public)
	if err != nil {
		return err
	}
	if missing || publicKey != key.PublicBase64URL() {
		return errors.New("push: the VAPID public parameter is missing or does not match the private key")
	}
	keys := []webpush.Key{key}
	if previousPEM, missing, err := readParam(ctx, c, p.previous); err != nil {
		return err
	} else if !missing {
		previous, err := webpush.ParseKey(previousPEM)
		if err != nil {
			return fmt.Errorf("push: the previous VAPID private key does not parse: %w", err)
		}
		keys = append(keys, previous)
	}
	subject, missing, err := readParam(ctx, c, p.subject)
	if err != nil {
		return err
	}
	if missing {
		return errors.New("push: the VAPID subject (app url) parameter is missing")
	}
	h.sender = webpush.NewSender(webpush.NewClient(), subject, keys...)
	return nil
}

// handle sends the job to every device of every recipient. A device that refuses is deleted or logged; only a failure of
// this function's own work (the table unreadable or unwritable) returns an error, so Lambda retries the whole job.
func (h *handler) handle(ctx context.Context, job services.PushSendJob) error {
	payload, err := services.BuildPushPayload(job)
	if err != nil {
		slog.ErrorContext(ctx, "push: job refused", "event_id", job.EventID, "error", err)
		return nil // a retry would be refused the same way
	}
	msg := webpush.Message{
		Payload: payload, TTL: time.Duration(job.TTLSeconds) * time.Second, Urgency: webpush.Urgency(job.Urgency),
		Topic: job.Topic, DedupeKey: job.EventID,
	}
	var sent, gone, failed, workErrors atomic.Int64
	var vapidRejected, tooLarge atomic.Bool
	var wg sync.WaitGroup
	sem := make(chan struct{}, sendWorkers)
	for _, recipient := range job.Recipients {
		subs, err := h.store.ListSubscriptions(ctx, recipient)
		if err != nil {
			slog.ErrorContext(ctx, "push: devices not read", "event_id", job.EventID, "sub_hash", subHash(recipient), "error", err)
			workErrors.Add(1)
			continue
		}
		for _, d := range subs {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				switch h.sendOne(ctx, job, recipient, d, msg) {
				case webpush.Delivered:
					sent.Add(1)
				case webpush.Gone:
					gone.Add(1)
				case webpush.Unauthorized:
					gone.Add(1)
					vapidRejected.Store(true)
				case webpush.PayloadTooLarge:
					failed.Add(1)
					tooLarge.Store(true)
				case 0: // the device was not reachable, or its row could not be deleted
					workErrors.Add(1)
				default:
					failed.Add(1)
				}
			}()
		}
	}
	wg.Wait()
	slog.InfoContext(ctx, "push send finished", "event_id", job.EventID, "type", job.Type, "recipients", len(job.Recipients),
		"delivered", sent.Load(), "gone", gone.Load(), "failed", failed.Load(), "work_errors", workErrors.Load())
	if vapidRejected.Load() {
		h.alerts.Alert(ctx, alerts.Alert{Job: "push-vapid-rejected", Summary: "a push service rejected the VAPID key of a subscription", Detail: "event " + job.EventID})
	}
	if tooLarge.Load() {
		h.alerts.Alert(ctx, alerts.Alert{Job: "push-payload-too-large", Summary: "a push service refused a payload as too large", Detail: "event " + job.EventID})
	}
	if n := workErrors.Load(); n > 0 {
		h.alerts.Alert(ctx, alerts.Alert{Job: "push-sender-failed", Summary: "the push sender could not complete its work", Detail: fmt.Sprintf("event %s, %d failures", job.EventID, n)})
		return fmt.Errorf("push: %d job-level failures", n)
	}
	return nil
}

// sendOne sends to one device and acts on the answer: Gone and Unauthorized delete the row. It returns the outcome,
// or 0 when the job's own work failed (a device that could not be sent to at all, or deleted).
func (h *handler) sendOne(ctx context.Context, job services.PushSendJob, recipient string, d repositories.PushSubscription, msg webpush.Message) webpush.Outcome {
	started := h.now()
	sub, err := webpush.ParseSubscription(d.Endpoint, d.P256dh, d.Auth)
	if err != nil {
		// A stored row that no longer parses can never be sent to: drop it.
		slog.WarnContext(ctx, "push: stored subscription is invalid; deleting", "event_id", job.EventID, "sub_hash", subHash(recipient), "device", d.ID)
		return h.drop(ctx, recipient, d.ID, webpush.Gone)
	}
	res, err := h.sender.Send(ctx, sub, d.VapidKID, msg)
	if errors.Is(err, webpush.ErrUnknownKey) {
		return h.drop(ctx, recipient, d.ID, webpush.Unauthorized) // its key was retired: the client resubscribes on its next load
	}
	attrs := []any{
		"event_id", job.EventID, "sub_hash", subHash(recipient), "push_host", hostOf(d.Endpoint),
		"status", res.Status, "outcome", res.Outcome.String(), "duration_ms", h.now().Sub(started).Milliseconds(),
	}
	if err != nil {
		slog.WarnContext(ctx, "push send", append(attrs, "error", err.Error())...) // err carries the host and cause only
		if res.Outcome == 0 {
			return webpush.Transient
		}
	} else {
		slog.InfoContext(ctx, "push send", attrs...)
	}
	switch res.Outcome {
	case webpush.Gone, webpush.Unauthorized:
		return h.drop(ctx, recipient, d.ID, res.Outcome)
	default:
		return res.Outcome
	}
}

func (h *handler) drop(ctx context.Context, recipient, id string, outcome webpush.Outcome) webpush.Outcome {
	if err := h.store.DeleteSubscription(ctx, recipient, id); err != nil && !errors.Is(err, repositories.ErrPushSubscriptionNotFound) {
		slog.ErrorContext(ctx, "push: subscription not deleted", "sub_hash", subHash(recipient), "device", id, "error", err)
		return 0
	}
	return outcome
}

// subHash is a short, one-way tag of a person for the logs: enough to follow one person's events, not who it is.
func subHash(sub string) string {
	sum := sha256.Sum256([]byte(sub))
	return hex.EncodeToString(sum[:4])
}

// hostOf is the push service's host, the only part of an endpoint a log may carry.
func hostOf(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
