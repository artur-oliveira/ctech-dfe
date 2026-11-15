package repositories

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/fieldcrypt"
	"gopkg.aoctech.app/api-commons/webhooks"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Outbound webhooks (docs/specs/2026-11-15-webhooks.md). One table, by prefix:
//
//	pk = ORG_{organization_id}   (the ctech-account organization, not the company)
//	sk = ENDPOINT#{id}                                  one per destination
//	     META#counts                                    endpoints count, the limit's guard
//	     DELIVERY#{endpoint_id}#{event_id}              one event on its way to one endpoint
//	     DEDUPE#{endpoint_id}#{hash}                    marker: this document and status was already sent here
//	     ATTEMPT#{endpoint_id}#{event_id}#{NN}          one try, kept as it happened
//
// The event body travels on the delivery row, so there is no event row: an event
// nobody subscribed to writes nothing, and a delivery needs no second read.
// Deliveries and attempts expire in WebhookRetention; the sparse `due-index`
// holds only pending deliveries.
const (
	TableWebhooks   = "webhooks"
	IndexWebhookDue = "due-index"

	// MaxWebhookEndpoints is the most endpoints one organization may register.
	MaxWebhookEndpoints = 5
	// WebhookRetention is how long deliveries and attempts are kept.
	WebhookRetention = 90 * 24 * time.Hour

	webhookDueValue = "1"
	// WebhookPing is the console's test event: sent to one endpoint, whatever its filter.
	WebhookPing = "ping"
)

// Endpoint and delivery states.
const (
	WebhookActive   = "active"
	WebhookDisabled = "disabled"

	DeliveryPending   = "pending"
	DeliveryDelivered = "delivered"
	DeliveryFailed    = "failed"
)

var (
	// ErrWebhookNotFound reports an endpoint that does not exist.
	ErrWebhookNotFound = errors.New("webhook endpoint not found")
	// ErrWebhookLimit reports an organization at MaxWebhookEndpoints.
	ErrWebhookLimit = errors.New("webhook endpoint limit reached")
	// ErrWebhookNoKey reports a repository built without a usable secret key.
	ErrWebhookNoKey = errors.New("webhook secret key is missing or invalid")
)

// WebhookEndpoint is one destination.
type WebhookEndpoint struct {
	ID             string `dynamodbav:"id"              json:"id"`
	OrganizationID string `dynamodbav:"organization_id" json:"organization_id"`
	URL            string `dynamodbav:"url"             json:"url"`
	// Secret signs the deliveries. Sealed at rest, never returned by a read a
	// browser can reach.
	Secret string `dynamodbav:"secret" json:"-"`
	// Events filters by type; empty is every type.
	Events []string `dynamodbav:"events,omitempty" json:"events"`
	// CompanyPKs filters by company (the DF-e organization row); empty is every
	// company of the organization.
	CompanyPKs []string `dynamodbav:"company_pks,omitempty" json:"company_pks"`
	Status     string   `dynamodbav:"status" json:"status"`
	// FailureRun counts consecutive failed deliveries and resets on a success.
	FailureRun         int    `dynamodbav:"failure_run"                     json:"failure_run"`
	DisabledAt         string `dynamodbav:"disabled_at,omitempty"           json:"disabled_at,omitempty"`
	LastDeliveryOK     bool   `dynamodbav:"last_delivery_ok,omitempty"      json:"last_delivery_ok"`
	LastDeliveryAt     string `dynamodbav:"last_delivery_at,omitempty"      json:"last_delivery_at,omitempty"`
	LastDeliveryDetail string `dynamodbav:"last_delivery_detail,omitempty"  json:"last_delivery_detail,omitempty"`
	CreatedBy          string `dynamodbav:"created_by,omitempty"            json:"created_by,omitempty"`
	CreatedAt          string `dynamodbav:"created_at,omitempty"            json:"created_at,omitempty"`
}

// Wants reports whether this endpoint takes an event: it is active, and the
// type and the company pass its filters. A ping names its endpoint and ignores
// the filters (a test must reach the endpoint it was pressed on).
func (e *WebhookEndpoint) Wants(ev WebhookEvent) bool {
	if e.Status != WebhookActive {
		return false
	}
	if ev.Type == WebhookPing {
		return ev.EndpointID == e.ID
	}
	if len(e.Events) > 0 && !slices.Contains(e.Events, ev.Type) {
		return false
	}
	if len(e.CompanyPKs) > 0 && !slices.Contains(e.CompanyPKs, ev.CompanyPK) {
		return false
	}
	return true
}

// WebhookEvent is something that happened, ready to fan out. Body is the exact
// payload sent.
type WebhookEvent struct {
	ID        string
	Type      string
	CompanyPK string
	// EndpointID is set on a ping only.
	EndpointID string
	Body       []byte
	// DedupeKey names the thing that happened (document and status), independent of the queue
	// message that reported it. Two messages for one result carry two event ids; a marker per
	// endpoint written with the delivery keeps the receiver from getting both. Empty: no marker.
	DedupeKey string
}

// WebhookDelivery is one event's journey to one endpoint.
type WebhookDelivery struct {
	OrganizationID string `dynamodbav:"organization_id" json:"-"`
	EndpointID     string `dynamodbav:"endpoint_id"     json:"endpoint_id"`
	EventID        string `dynamodbav:"event_id"        json:"event_id"`
	Type           string `dynamodbav:"type"            json:"type"`
	Body           string `dynamodbav:"body"            json:"-"`
	Status         string `dynamodbav:"status"          json:"status"`
	Attempts       int    `dynamodbav:"attempts"        json:"attempts"`
	NextAttemptAt  string `dynamodbav:"next_attempt_at,omitempty" json:"next_attempt_at,omitempty"`
	LastStatusCode int    `dynamodbav:"last_status_code,omitempty" json:"last_status_code,omitempty"`
	LastError      string `dynamodbav:"last_error,omitempty"      json:"last_error,omitempty"`
	DeliveredAt    string `dynamodbav:"delivered_at,omitempty"    json:"delivered_at,omitempty"`
	CreatedAt      string `dynamodbav:"created_at,omitempty"      json:"created_at,omitempty"`
}

// WebhookAttempt is one try at a delivery.
type WebhookAttempt struct {
	Number     int    `dynamodbav:"number"                json:"number"`
	At         string `dynamodbav:"at"                    json:"at"`
	StatusCode int    `dynamodbav:"status_code,omitempty" json:"status_code,omitempty"`
	DurationMs int64  `dynamodbav:"duration_ms"           json:"duration_ms"`
	Error      string `dynamodbav:"error,omitempty"       json:"error,omitempty"`
	Response   string `dynamodbav:"response,omitempty"    json:"response,omitempty"`
}

// Key builders, exported for the tests that read a raw row.
func WebhookPK(organizationID string) string { return "ORG_" + organizationID }
func WebhookEndpointSK(id string) string     { return "ENDPOINT#" + id }

const webhookCountsSK = "META#counts"

func webhookDeliverySK(endpointID, eventID string) string {
	return "DELIVERY#" + endpointID + "#" + eventID
}
func webhookDedupeSK(endpointID, key string) string { return "DEDUPE#" + endpointID + "#" + key }
func webhookAttemptSK(endpointID, eventID string, n int) string {
	return fmt.Sprintf("ATTEMPT#%s#%s#%03d", endpointID, eventID, n)
}

// secretAAD binds a sealed secret to its row: a ciphertext copied to another
// endpoint or organization does not open.
func secretAAD(pk, sk string) []byte { return []byte("webhooks/secret/" + pk + "/" + sk) }

// WebhookRepository stores endpoints, deliveries and attempts.
type WebhookRepository struct {
	base Base
	seal *fieldcrypt.Sealer
}

func NewWebhookRepository(db *dynamodb.Client, cfg *config.Config) *WebhookRepository {
	return &WebhookRepository{
		base: NewBase(db, cfg, TableWebhooks),
		seal: fieldcrypt.NewSealer(cfg.WebhookSecretKey),
	}
}

// Enabled reports whether secrets can be sealed, i.e. whether endpoints can be
// registered at all.
func (r *WebhookRepository) Enabled() bool { return r.seal.Enabled() }

func ttlAt(now time.Time) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(WebhookRetention).Unix(), 10)}
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// ── Endpoints ────────────────────────────────────────────────────────────────

// CreateEndpoint registers a destination. The count row and the endpoint are one
// transaction, so concurrent registrations never pass MaxWebhookEndpoints.
func (r *WebhookRepository) CreateEndpoint(ctx context.Context, e *WebhookEndpoint, now time.Time) error {
	if !r.Enabled() {
		return ErrWebhookNoKey
	}
	pk, sk := WebhookPK(e.OrganizationID), WebhookEndpointSK(e.ID)
	sealed, err := r.seal.SealWith(e.Secret, secretAAD(pk, sk))
	if err != nil {
		return fmt.Errorf("sealing webhook secret: %w", err)
	}
	row := *e
	row.Secret = sealed
	row.CreatedAt = stamp(now)
	item, err := MarshalMapOmitNull(row)
	if err != nil {
		return err
	}
	item["pk"] = &types.AttributeValueMemberS{Value: pk}
	item["sk"] = &types.AttributeValueMemberS{Value: sk}

	count := r.base.BuildRawUpdateTxItem(pk, new(webhookCountsSK),
		"SET endpoints = if_not_exists(endpoints, :zero) + :one",
		"attribute_not_exists(endpoints) OR endpoints < :max",
		nil,
		map[string]types.AttributeValue{
			":zero": numAV(0), ":one": numAV(1), ":max": numAV(MaxWebhookEndpoints),
		})
	err = r.base.TransactWrite(ctx, []types.TransactWriteItem{count, r.base.BuildPutTxItemIfAbsent(item)})
	if IsConditionFailed(err) {
		return ErrWebhookLimit
	}
	return err
}

func numAV(n int) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.Itoa(n)}
}

func (r *WebhookRepository) openEndpoint(item map[string]types.AttributeValue) (*WebhookEndpoint, error) {
	var e WebhookEndpoint
	if err := attributevalue.UnmarshalMap(item, &e); err != nil {
		return nil, err
	}
	secret, err := r.seal.OpenWith(e.Secret, secretAAD(WebhookPK(e.OrganizationID), WebhookEndpointSK(e.ID)))
	if err != nil {
		return nil, fmt.Errorf("opening secret of webhook endpoint %s: %w", e.ID, err)
	}
	e.Secret = secret
	return &e, nil
}

// GetEndpoint reads one endpoint, secret included (it signs a delivery).
func (r *WebhookRepository) GetEndpoint(ctx context.Context, organizationID, id string) (*WebhookEndpoint, error) {
	item, err := r.base.GetItem(ctx, WebhookPK(organizationID), WebhookEndpointSK(id))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("%w: %s", ErrWebhookNotFound, id)
	}
	return r.openEndpoint(item)
}

// ListEndpoints returns every endpoint of the organization (at most
// MaxWebhookEndpoints), disabled ones included, **secrets not opened**: nothing
// that lists (the console, the fan-out) uses them, and opening them would let one
// unreadable value (a rotated key, a corrupt row) break the screen and every
// result's emission. The delivery path reads one endpoint with GetEndpoint.
func (r *WebhookRepository) ListEndpoints(ctx context.Context, organizationID string) ([]WebhookEndpoint, error) {
	res, err := r.base.Query(ctx, QueryOpts{
		PK: WebhookPK(organizationID), SKPrefix: "ENDPOINT#", ScanIndexForward: true, Limit: MaxWebhookEndpoints * 2,
	})
	if err != nil {
		return nil, err
	}
	out := make([]WebhookEndpoint, 0, len(res.Items))
	for _, item := range res.Items {
		var e WebhookEndpoint
		if err := attributevalue.UnmarshalMap(item, &e); err != nil {
			return nil, err
		}
		e.Secret = "" // still sealed in the row; never handed out from a list
		out = append(out, e)
	}
	return out, nil
}

// DeleteEndpoint removes an endpoint and frees its slot. Its pending deliveries
// stay and are closed by the Deliverer ("endpoint no longer exists").
func (r *WebhookRepository) DeleteEndpoint(ctx context.Context, organizationID, id string) error {
	pk := WebhookPK(organizationID)
	del := types.TransactWriteItem{Delete: &types.Delete{
		TableName: aws.String(r.base.TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: WebhookEndpointSK(id)},
		},
		ConditionExpression: aws.String("attribute_exists(pk)"),
	}}
	count := r.base.BuildRawUpdateTxItem(pk, new(webhookCountsSK),
		"SET endpoints = endpoints - :one", "endpoints > :zero", nil,
		map[string]types.AttributeValue{":zero": numAV(0), ":one": numAV(1)})
	err := r.base.TransactWrite(ctx, []types.TransactWriteItem{del, count})
	if IsConditionFailed(err) {
		return ErrWebhookNotFound
	}
	return err
}

// EnableEndpoint turns a disabled endpoint back on and clears its failure run, keeping its id,
// its secret and its history. The caller proves the endpoint answers first.
func (r *WebhookRepository) EnableEndpoint(ctx context.Context, organizationID, id string) error {
	sk := WebhookEndpointSK(id)
	ok, err := r.base.UpdateItem(ctx, WebhookPK(organizationID), &sk, map[string]any{
		"status": WebhookActive, "failure_run": 0, "disabled_at": nil,
	})
	if err != nil {
		return err
	}
	if !ok {
		return ErrWebhookNotFound
	}
	return nil
}

// RotateSecret replaces an endpoint's signing secret.
func (r *WebhookRepository) RotateSecret(ctx context.Context, organizationID, id, secret string) error {
	if !r.Enabled() {
		return ErrWebhookNoKey
	}
	pk, sk := WebhookPK(organizationID), WebhookEndpointSK(id)
	sealed, err := r.seal.SealWith(secret, secretAAD(pk, sk))
	if err != nil {
		return err
	}
	ok, err := r.base.UpdateItem(ctx, pk, &sk, map[string]any{"secret": sealed})
	if err != nil {
		return err
	}
	if !ok {
		return ErrWebhookNotFound
	}
	return nil
}

// ── Events and deliveries ────────────────────────────────────────────────────

// WriteDeliveries fans an event out to the endpoints that want it, one pending
// delivery each, in one transaction. It returns how many were written; an event
// nobody wants writes nothing.
func (r *WebhookRepository) WriteDeliveries(ctx context.Context, organizationID string, ev WebhookEvent, endpoints []WebhookEndpoint, now time.Time) (int, error) {
	var items []types.TransactWriteItem
	for i := range endpoints {
		if !endpoints[i].Wants(ev) {
			continue
		}
		d := WebhookDelivery{
			OrganizationID: organizationID, EndpointID: endpoints[i].ID, EventID: ev.ID, Type: ev.Type,
			Body: string(ev.Body), Status: DeliveryPending, NextAttemptAt: stamp(now), CreatedAt: stamp(now),
		}
		item, err := MarshalMapOmitNull(d)
		if err != nil {
			return 0, err
		}
		item["pk"] = &types.AttributeValueMemberS{Value: WebhookPK(organizationID)}
		item["sk"] = &types.AttributeValueMemberS{Value: webhookDeliverySK(d.EndpointID, d.EventID)}
		item["due"] = &types.AttributeValueMemberS{Value: webhookDueValue}
		item["ttl"] = ttlAt(now)
		items = append(items, r.base.BuildPutTxItemIfAbsent(item))
		if ev.DedupeKey != "" {
			items = append(items, r.base.BuildPutTxItemIfAbsent(map[string]types.AttributeValue{
				"pk":  &types.AttributeValueMemberS{Value: WebhookPK(organizationID)},
				"sk":  &types.AttributeValueMemberS{Value: webhookDedupeSK(d.EndpointID, ev.DedupeKey)},
				"ttl": ttlAt(now),
			}))
		}
	}
	if len(items) == 0 {
		return 0, nil
	}
	if err := r.base.TransactWrite(ctx, items); err != nil {
		if IsConditionFailed(err) {
			return 0, nil // the same event twice (an SQS redelivery): already written
		}
		return 0, err
	}
	return len(items), nil
}

// ListDeliveries returns an endpoint's deliveries newest first, one page, with
// the cursor for the next (nil at the end).
func (r *WebhookRepository) ListDeliveries(ctx context.Context, organizationID, endpointID string, start map[string]types.AttributeValue, limit int) ([]WebhookDelivery, map[string]types.AttributeValue, error) {
	res, err := r.base.Query(ctx, QueryOpts{
		PK: WebhookPK(organizationID), SKPrefix: "DELIVERY#" + endpointID + "#",
		ScanIndexForward: false, Limit: limit, ExclusiveStartKey: start,
	})
	if err != nil {
		return nil, nil, err
	}
	out := make([]WebhookDelivery, 0, len(res.Items))
	for _, item := range res.Items {
		var d WebhookDelivery
		if err := attributevalue.UnmarshalMap(item, &d); err != nil {
			return nil, nil, err
		}
		out = append(out, d)
	}
	return out, res.LastEvaluatedKey, nil
}

// GetDelivery reads one delivery.
func (r *WebhookRepository) GetDelivery(ctx context.Context, organizationID, endpointID, eventID string) (*WebhookDelivery, error) {
	item, err := r.base.GetItem(ctx, WebhookPK(organizationID), webhookDeliverySK(endpointID, eventID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("%w: delivery %s", ErrWebhookNotFound, eventID)
	}
	var d WebhookDelivery
	if err := attributevalue.UnmarshalMap(item, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Attempts lists the tries made for one delivery, oldest first.
func (r *WebhookRepository) Attempts(ctx context.Context, organizationID, endpointID, eventID string) ([]WebhookAttempt, error) {
	res, err := r.base.Query(ctx, QueryOpts{
		PK: WebhookPK(organizationID), SKPrefix: "ATTEMPT#" + endpointID + "#" + eventID + "#",
		ScanIndexForward: true, Limit: webhooks.MaxDeliveryAttempts + 20,
	})
	if err != nil {
		return nil, err
	}
	out := make([]WebhookAttempt, 0, len(res.Items))
	for _, item := range res.Items {
		var a WebhookAttempt
		if err := attributevalue.UnmarshalMap(item, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
