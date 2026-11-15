package repositories

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/webhooks"
)

// WebhookStore is the DF-e side of the shared delivery engine
// (go-common/webhooks): the engine decides when and how to attempt, this maps
// its calls onto the `webhooks` table. The stored rows travel in Delivery.Ref
// and Endpoint.Ref, so nothing is parsed back out of an id.
type WebhookStore struct {
	repo *WebhookRepository
	// onDisabled is told when an endpoint is disabled by its failure run (the
	// operator alert); nil is allowed.
	onDisabled func(WebhookEndpoint)
}

var _ webhooks.Store = (*WebhookStore)(nil)

func NewWebhookStore(repo *WebhookRepository, onDisabled func(WebhookEndpoint)) *WebhookStore {
	return &WebhookStore{repo: repo, onDisabled: onDisabled}
}

var errWebhookNoRow = errors.New("webhook store: delivery without its stored row")

func rowOf(d webhooks.Delivery) (WebhookDelivery, error) {
	row, ok := d.Ref.(WebhookDelivery)
	if !ok {
		return WebhookDelivery{}, errWebhookNoRow
	}
	return row, nil
}

func (s *WebhookStore) updateDelivery(ctx context.Context, d webhooks.Delivery, updates map[string]any) error {
	row, err := rowOf(d)
	if err != nil {
		return err
	}
	ok, err := s.repo.base.UpdateItem(ctx, WebhookPK(row.OrganizationID), new(webhookDeliverySK(row.EndpointID, row.EventID)), updates)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: delivery %s", ErrWebhookNotFound, row.EventID)
	}
	return nil
}

// DueDeliveries lists pending deliveries oldest due first, from the sparse index.
func (s *WebhookStore) DueDeliveries(ctx context.Context, limit int) ([]webhooks.Due, error) {
	res, err := s.repo.base.Query(ctx, QueryOpts{
		IndexName: IndexWebhookDue, PKField: "due", SKField: "next_attempt_at", PK: webhookDueValue,
		ScanIndexForward: true, Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]webhooks.Due, 0, len(res.Items))
	for _, item := range res.Items {
		var d WebhookDelivery
		if err := attributevalue.UnmarshalMap(item, &d); err != nil {
			return nil, err
		}
		out = append(out, DueOf(d))
	}
	return out, nil
}

// DueOf maps a stored delivery to what the engine attempts. Group is one
// endpoint of one organization: its deliveries are made in order, one at a time.
func DueOf(d WebhookDelivery) webhooks.Due {
	return webhooks.Due{
		Delivery: webhooks.Delivery{
			EventID: d.EventID, EndpointID: d.EndpointID,
			Group:    d.OrganizationID + "|" + d.EndpointID,
			Attempts: d.Attempts, NextAttemptAt: d.NextAttemptAt, Ref: d,
		},
		Event: webhooks.Event{ID: d.EventID, Type: d.Type, Body: []byte(d.Body)},
	}
}

// GetEndpoint reads the destination of a delivery. Only "not found" is ErrEndpointGone (the
// engine closes the delivery); any other failure (a secret that does not open, DynamoDB down)
// is returned as it is, so the engine leaves the delivery as it is and counts it as the job's
// failure. It is also logged here: an unreadable secret would otherwise be silent.
func (s *WebhookStore) GetEndpoint(ctx context.Context, d webhooks.Delivery) (*webhooks.Endpoint, error) {
	row, err := rowOf(d)
	if err != nil {
		return nil, err
	}
	e, err := s.repo.GetEndpoint(ctx, row.OrganizationID, row.EndpointID)
	if err != nil {
		if errors.Is(err, ErrWebhookNotFound) {
			return nil, fmt.Errorf("%w: %v", webhooks.ErrEndpointGone, err)
		}
		slog.ErrorContext(ctx, "webhook endpoint unreadable, delivery left as it is",
			"organization_id", row.OrganizationID, "endpoint_id", row.EndpointID, "error", err)
		return nil, err
	}
	return &webhooks.Endpoint{
		ID: e.ID, URL: e.URL, Secret: e.Secret, Active: e.Status == WebhookActive, Ref: *e,
	}, nil
}

// RecordAttempt keeps one try. Idempotent on the number: a repeat is ignored.
func (s *WebhookStore) RecordAttempt(ctx context.Context, d webhooks.Delivery, a webhooks.Attempt, now time.Time) error {
	row, err := rowOf(d)
	if err != nil {
		return err
	}
	att := WebhookAttempt{
		Number: a.Number, At: stamp(now), StatusCode: a.StatusCode, DurationMs: a.DurationMs,
		Error: truncate(a.Error, 500), Response: truncate(a.Response, webhooks.ResponseKeep),
	}
	item, err := MarshalMapOmitNull(att)
	if err != nil {
		return err
	}
	pk := WebhookPK(row.OrganizationID)
	item["pk"] = &types.AttributeValueMemberS{Value: pk}
	item["sk"] = &types.AttributeValueMemberS{Value: webhookAttemptSK(row.EndpointID, row.EventID, a.Number)}
	item["ttl"] = ttlAt(now)
	err = s.repo.base.TransactWrite(ctx, []types.TransactWriteItem{s.repo.base.BuildPutTxItemIfAbsent(item)})
	if IsConditionFailed(err) {
		return nil
	}
	return err
}

func (s *WebhookStore) MarkDelivered(ctx context.Context, d webhooks.Delivery, status int, now time.Time) error {
	return s.updateDelivery(ctx, d, map[string]any{
		"status": DeliveryDelivered, "attempts": d.Attempts + 1, "last_status_code": status,
		"delivered_at": stamp(now), "last_error": nil,
		// Leaving the index is what "done" means: a delivered row that stayed would be retried forever.
		"due": nil, "next_attempt_at": nil,
	})
}

// MarkAttemptFailed counts the failure and reschedules with Backoff, or gives up
// after MaxDeliveryAttempts (a terminal status, not a deletion: "we tried and
// gave up" is an answer somebody will ask for).
func (s *WebhookStore) MarkAttemptFailed(ctx context.Context, d webhooks.Delivery, status int, cause string, now time.Time) error {
	attempts := d.Attempts + 1
	updates := map[string]any{"attempts": attempts, "last_error": truncate(cause, 500)}
	if status > 0 {
		updates["last_status_code"] = status
	}
	if attempts >= webhooks.MaxDeliveryAttempts {
		updates["status"] = DeliveryFailed
		updates["due"] = nil
		updates["next_attempt_at"] = nil
	} else {
		// Rewriting the sort key is what moves the row down the queue; without it a
		// failing delivery stays at the head and starves everything behind it.
		updates["next_attempt_at"] = stamp(now.Add(webhooks.Backoff(attempts)))
	}
	return s.updateDelivery(ctx, d, updates)
}

// Defer moves a delivery down the queue without counting an attempt.
func (s *WebhookStore) Defer(ctx context.Context, d webhooks.Delivery, until, _ time.Time) error {
	return s.updateDelivery(ctx, d, map[string]any{"next_attempt_at": stamp(until)})
}

// Close ends a delivery as failed without an attempt: there is nowhere to send it.
func (s *WebhookStore) Close(ctx context.Context, d webhooks.Delivery, cause string, _ time.Time) error {
	return s.updateDelivery(ctx, d, map[string]any{
		"status": DeliveryFailed, "last_error": truncate(cause, 500), "due": nil, "next_attempt_at": nil,
	})
}

// Requeue makes a delivery pending and due now, keeping its attempts and log.
func (s *WebhookStore) Requeue(ctx context.Context, d webhooks.Delivery, now time.Time) error {
	return s.updateDelivery(ctx, d, map[string]any{
		"status": DeliveryPending, "due": webhookDueValue, "next_attempt_at": stamp(now), "delivered_at": nil,
	})
}

// RecordHealth keeps the consecutive-failure run and disables the endpoint at
// MaxFailureRun. The run resets on any success: an endpoint that fails one in a
// hundred is a working endpoint.
func (s *WebhookStore) RecordHealth(ctx context.Context, e webhooks.Endpoint, ok bool, detail string, now time.Time) error {
	row, isRow := e.Ref.(WebhookEndpoint)
	if !isRow {
		return errWebhookNoRow
	}
	updates := map[string]any{}
	disabled := false
	switch {
	case ok:
		if row.FailureRun == 0 && row.LastDeliveryOK {
			return nil // a healthy endpoint costs no extra write per delivery
		}
		updates = map[string]any{"failure_run": 0, "last_delivery_ok": true, "last_delivery_at": stamp(now), "last_delivery_detail": ""}
	default:
		run := row.FailureRun + 1
		updates = map[string]any{
			"failure_run": run, "last_delivery_ok": false, "last_delivery_at": stamp(now),
			"last_delivery_detail": truncate(detail, 200),
		}
		if run >= webhooks.MaxFailureRun && row.Status == WebhookActive {
			updates["status"] = WebhookDisabled
			updates["disabled_at"] = stamp(now)
			disabled = true
		}
	}
	sk := WebhookEndpointSK(row.ID)
	if _, err := s.repo.base.UpdateItem(ctx, WebhookPK(row.OrganizationID), &sk, updates); err != nil {
		return err
	}
	if disabled && s.onDisabled != nil {
		row.Status = WebhookDisabled
		s.onDisabled(row)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
