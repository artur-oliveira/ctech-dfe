package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/api-commons/webpush"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

const (
	// pushTestWindow is the rate limit of the test notification: one per person per minute.
	pushTestWindow = time.Minute
	pushTestType   = "test"
	pushVapidPoint = 65 // an uncompressed P-256 point
)

var errPushOff = problem.ServiceUnavailable("notificações desativadas nesta instalação")

// PushService is the DF-e side of Web Push: it keeps a person's devices and choices and, when a document result or a
// received document happens, decides who is told and hands the rendered notification to the push-sender Lambda.
// Sending is the Lambda's (go-common/webpush). Without a VAPID public key or a sender function the service is off:
// every read answers `enabled: false`, every write 503, and Emit does nothing.
type PushService struct {
	repo         *repositories.PushRepository
	members      *MembershipService
	orgs         *OrganizationService
	invoker      workerInvoker
	functionName string
	publicKey    string
	kid          string
	now          func() time.Time
}

// NewPushService builds the service. publicKey is the VAPID public key (base64url, 65 bytes) the browsers subscribe
// with; functionName is the push-sender Lambda. A malformed key turns the service off rather than serving a key the
// browsers would subscribe to and the sender could never use.
func NewPushService(repo *repositories.PushRepository, members *MembershipService, orgs *OrganizationService, clients *awsclient.Clients, functionName, publicKey string) *PushService {
	s := &PushService{repo: repo, members: members, orgs: orgs, functionName: functionName, now: time.Now}
	if clients != nil {
		s.invoker = clients.WorkerLambda
	}
	if kid, ok := vapidKID(publicKey); ok && functionName != "" {
		s.publicKey, s.kid = publicKey, kid
	}
	return s
}

// PushInvoker is the slice of the Lambda client the service uses to start the sender.
type PushInvoker = workerInvoker

// WithInvoker replaces the Lambda client (tests inject one that records the jobs).
func (s *PushService) WithInvoker(i PushInvoker) *PushService {
	s.invoker = i
	return s
}

// vapidKID is webpush.Key.KID computed from the public key alone (the API never holds the private one).
func vapidKID(publicKey string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(publicKey)
	if err != nil || len(raw) != pushVapidPoint || raw[0] != 4 {
		return "", false
	}
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(sum[:6]), true
}

// Enabled reports whether push can be used at all.
func (s *PushService) Enabled() bool { return s != nil && s.publicKey != "" }

// PushConfig is what the browser needs to subscribe and what the screen lists.
type PushConfig struct {
	Enabled          bool     `json:"enabled"`
	PublicKey        string   `json:"public_key,omitempty"`
	VapidKID         string   `json:"vapid_kid,omitempty"`
	EventTypes       []string `json:"event_types"`
	Defaults         []string `json:"defaults"`
	MaxSubscriptions int      `json:"max_subscriptions"`
}

// Config answers GET /push/config. Disabled is a normal answer, not an error: the screen hides itself.
func (s *PushService) Config() PushConfig {
	c := PushConfig{
		Enabled: s.Enabled(), EventTypes: PushEventTypes, Defaults: PushDefaultEvents,
		MaxSubscriptions: repositories.MaxPushSubscriptionsPerUser,
	}
	if c.Enabled {
		c.PublicKey, c.VapidKID = s.publicKey, s.kid
	}
	return c
}

// PushSubscriptionInput is a browser's PushSubscription.toJSON().
type PushSubscriptionInput struct {
	Endpoint, P256dh, Auth, UserAgent string
}

// PushDevice is one device as the screen shows it (never the endpoint).
type PushDevice struct {
	ID         string `json:"id"`
	Device     string `json:"device"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
}

// Subscribe registers or renews the person's device. The first device of a person who has no choice for the company
// starts with the defaults (only the received documents). The endpoint is validated by the same parser the sender
// uses: https, a known push service host, a valid P-256 key and a 16-byte secret.
func (s *PushService) Subscribe(ctx context.Context, companyPK, sub string, in PushSubscriptionInput) (*PushDevice, error) {
	if !s.Enabled() {
		return nil, errPushOff
	}
	if _, err := webpush.ParseSubscription(in.Endpoint, in.P256dh, in.Auth); err != nil {
		return nil, problem.BadRequest("inscrição de notificação inválida")
	}
	// One browser endpoint belongs to one person: whoever held it before (a shared computer where the previous person never
	// logged out cleanly) loses it now, so only the person signed in on that browser is notified. Failing here aborts the
	// subscription instead of leaving two owners.
	owners, err := s.repo.OtherOwners(ctx, in.Endpoint, sub)
	if err != nil {
		return nil, err
	}
	endpointID := repositories.PushSubscriptionID(in.Endpoint)
	for _, previous := range owners {
		if err := s.repo.DeleteSubscription(ctx, previous, endpointID); err != nil && !errors.Is(err, repositories.ErrPushSubscriptionNotFound) {
			return nil, err
		}
	}
	label := pushDeviceLabel(in.UserAgent)
	id, err := s.repo.UpsertSubscription(ctx, sub, repositories.PushSubscription{
		Endpoint: in.Endpoint, P256dh: in.P256dh, Auth: in.Auth, VapidKID: s.kid, Device: label,
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.SeedPreference(ctx, companyPK, sub, PushDefaultEvents); err != nil {
		return nil, err
	}
	return &PushDevice{ID: id, Device: label}, nil
}

// Devices lists the person's devices.
func (s *PushService) Devices(ctx context.Context, sub string) ([]PushDevice, error) {
	list, err := s.repo.ListSubscriptions(ctx, sub)
	if err != nil {
		return nil, err
	}
	out := make([]PushDevice, 0, len(list))
	for _, d := range list {
		out = append(out, PushDevice{ID: d.ID, Device: d.Device, CreatedAt: d.CreatedAt, LastSeenAt: d.LastSeenAt})
	}
	return out, nil
}

// Unsubscribe removes one of the person's devices.
func (s *PushService) Unsubscribe(ctx context.Context, sub, id string) error {
	err := s.repo.DeleteSubscription(ctx, sub, id)
	if errors.Is(err, repositories.ErrPushSubscriptionNotFound) {
		return problem.NotFound("dispositivo não encontrado")
	}
	return err
}

// Preferences returns the events the person wants for the company ([] when they never chose).
func (s *PushService) Preferences(ctx context.Context, companyPK, sub string) ([]string, error) {
	events, err := s.repo.GetPreference(ctx, companyPK, sub)
	if events == nil {
		events = []string{}
	}
	return events, err
}

// SetPreferences saves the person's events for the company. Only known types are accepted, each once; an empty list
// turns everything off for that company.
func (s *PushService) SetPreferences(ctx context.Context, companyPK, sub string, events []string) ([]string, error) {
	if !s.Enabled() {
		return nil, errPushOff
	}
	clean := make([]string, 0, len(events))
	for _, e := range events {
		if !slices.Contains(PushEventTypes, e) {
			return nil, problem.BadRequest("tipo de notificação desconhecido")
		}
		if !slices.Contains(clean, e) {
			clean = append(clean, e)
		}
	}
	if err := s.repo.PutPreference(ctx, companyPK, sub, clean); err != nil {
		return nil, err
	}
	return clean, nil
}

// ForgetMember drops a removed member's choice for the company. Best effort: the fan-out checks the membership of
// every recipient anyway, so a row that survives never sends.
func (s *PushService) ForgetMember(ctx context.Context, companyPK, sub string) {
	if err := s.repo.DeletePreference(ctx, companyPK, sub); err != nil {
		slog.WarnContext(ctx, "push: could not drop a removed member's preference", "company", companyPK, "error", err)
	}
}

// Test sends the person a notification on every device, at most once a minute.
func (s *PushService) Test(ctx context.Context, companyPK, sub string) error {
	if !s.Enabled() {
		return errPushOff
	}
	open, err := s.repo.OpenUserWindow(ctx, sub, pushTestType, pushTestWindow)
	if err != nil {
		return err
	}
	if !open {
		return problem.TooManyRequestsAfter("aguarde um minuto para enviar outro teste", int(pushTestWindow.Seconds()))
	}
	at := s.now()
	id := repositories.WebhookEventID(at, repositories.RawUserID(sub)+"|test|"+at.UTC().Format(time.RFC3339Nano))
	tag, topic := pushTag(pushTestType, companyPK)
	return s.invoke(ctx, PushSendJob{
		EventID: id, Type: pushTestType, Title: "Teste de notificação", Body: "As notificações estão funcionando neste dispositivo.",
		Navigate: pushNavigate("", "", "", companyPK), Tag: tag, Topic: topic, TTLSeconds: int(time.Hour.Seconds()), Urgency: "normal",
		Recipients: []string{repositories.RawUserID(sub)},
	})
}

// PushResult is a terminal document result as the results consumer knows it.
type PushResult struct {
	CompanyPK   string
	DocumentKey string
	Table       string
	Status      string
	SentAt      time.Time
}

// EmitResult tells the people who asked about a terminal result (authorized, rejected or failed). Anything else, a
// table that is not a document, an unwanted event or a service that is off, writes nothing.
func (s *PushService) EmitResult(ctx context.Context, in PushResult) error {
	if !s.Enabled() {
		return nil
	}
	eventType, ok := pushEventOf(in.Status)
	docType, isDoc := MeterForTable[in.Table]
	if !ok || !isDoc || in.DocumentKey == "" {
		return nil
	}
	return s.emit(ctx, pushEmission{
		companyPK: in.CompanyPK, eventType: eventType, docType: docType, key: in.DocumentKey,
		dedupe: ResultDedupeKey(in.CompanyPK, in.DocumentKey, in.Status), at: in.SentAt,
	})
}

// EmitDistribution tells the people who asked about a document received through distribution.
func (s *PushService) EmitDistribution(ctx context.Context, companyPK, accessKey, docType string, sentAt time.Time) error {
	if !s.Enabled() || accessKey == "" {
		return nil
	}
	return s.emit(ctx, pushEmission{
		companyPK: companyPK, eventType: PushEventDistribution, docType: docType, key: accessKey,
		dedupe: ResultDedupeKey(companyPK, accessKey, "distribution"), at: sentAt,
	})
}

type pushEmission struct {
	companyPK, eventType, docType, key, dedupe string
	at                                         time.Time
}

// emit is the common path: who wants it, is each still a member, is the window open, was the event already handed
// over, then the Lambda. A failed hand-off releases the two markers and returns the error, so the queue redelivers
// and the retry starts clean; the markers keep a redelivery after a success from sending twice.
func (s *PushService) emit(ctx context.Context, e pushEmission) error {
	recipients, err := s.recipients(ctx, e.companyPK, e.eventType)
	if err != nil || len(recipients) == 0 {
		return err
	}
	open, err := s.repo.OpenWindow(ctx, e.companyPK, e.eventType, pushWindow(e.eventType))
	if err != nil || !open {
		return err
	}
	fresh, err := s.repo.MarkEvent(ctx, e.companyPK, e.dedupe)
	if err != nil || !fresh {
		if err != nil {
			s.release(ctx, e, false)
		}
		return err
	}
	at := e.at
	if at.IsZero() {
		at = s.now()
	}
	ttl, urgency := pushDelivery(e.eventType)
	tag, topic := pushTag(e.eventType, e.companyPK)
	job := PushSendJob{
		EventID: repositories.WebhookEventID(at, e.dedupe), Type: e.eventType,
		Title: pushTitle(e.eventType, e.docType, s.companyName(ctx, e.companyPK)), Body: pushBody(e.eventType),
		Navigate: pushNavigate(e.eventType, e.docType, e.key, e.companyPK), Tag: tag, Topic: topic,
		TTLSeconds: int(ttl.Seconds()), Urgency: urgency, Recipients: recipients,
	}
	if err := s.invoke(ctx, job); err != nil {
		s.release(ctx, e, true)
		return err
	}
	return nil
}

func (s *PushService) release(ctx context.Context, e pushEmission, marked bool) {
	if marked {
		if err := s.repo.UnmarkEvent(ctx, e.companyPK, e.dedupe); err != nil {
			slog.WarnContext(ctx, "push: could not release the event marker", "company", e.companyPK, "error", err)
		}
	}
	if err := s.repo.CloseWindow(ctx, e.companyPK, e.eventType); err != nil {
		slog.WarnContext(ctx, "push: could not reopen the window", "company", e.companyPK, "error", err)
	}
}

// recipients pages the company's preference rows and keeps the members who ticked the event.
func (s *PushService) recipients(ctx context.Context, companyPK, eventType string) ([]string, error) {
	var out []string
	var start map[string]types.AttributeValue
	for page := 0; page < MaxPushRecipientPages; page++ {
		res, err := s.repo.ListRecipients(ctx, companyPK, eventType, MaxPushRecipientsPerPage, start)
		if err != nil {
			return nil, err
		}
		for _, sub := range res.Subs {
			m, err := s.members.Get(ctx, companyPK, sub)
			if err != nil {
				return nil, err
			}
			if m != nil {
				out = append(out, sub)
			}
		}
		if res.Next == nil {
			break
		}
		start = res.Next
	}
	return out, nil
}

// companyName reads the company's name for the title; a failed read only costs the suffix.
func (s *PushService) companyName(ctx context.Context, companyPK string) string {
	item, err := s.orgs.Get(ctx, companyPK)
	if err != nil || item == nil {
		return ""
	}
	return avStr(item, "name")
}

// invoke hands the job to the push-sender Lambda asynchronously: Lambda queues it and retries twice.
func (s *PushService) invoke(ctx context.Context, job PushSendJob) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	out, err := s.invoker.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(s.functionName), InvocationType: lambdatypes.InvocationTypeEvent, Payload: body,
	})
	if err != nil {
		return fmt.Errorf("push: invoking the sender: %w", err)
	}
	if out.StatusCode != http.StatusAccepted {
		return fmt.Errorf("push: unexpected async invoke status %d", out.StatusCode)
	}
	return nil
}
