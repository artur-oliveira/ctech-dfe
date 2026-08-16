// Package billingclient is the DF-e's client for ctech-billing's M2M surface.
//
// It is the only place in this service that speaks to billing, and it is a
// package rather than inline HTTP calls for three reasons that are properties
// rather than tidiness:
//
//   - **Billing's error bodies never reach a DF-e client.** They are RFC 7807
//     and they are readable, but they are written about billing's entities —
//     "assinatura sub_01J… não encontrada" tells a DF-e user about an id they
//     have never seen and cannot act on. Every response is mapped to a DF-e
//     problem here, and billing's own detail is logged instead.
//   - **A missing configuration is a supported deployment.** `Enabled()` false
//     is no-charge mode: every account unlimited, which is what dev needs. The
//     alternative — a client that exists and fails on first use — turns a dev
//     environment into a broken one.
//   - **One token manager.** api-commons' oauth2client already caches
//     client-credentials tokens across replicas; ctech-wallet's two hand-rolled
//     copies were merged into it, and this is not a third.
package billingclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/oauth2client"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Scopes is what the `dfe-billing` client asks for, space-separated as the token
// endpoint expects.
//
// It is the full set this service needs and no more. `billing:invoices:write`
// is deliberately absent: the DF-e never marks an invoice paid — money is
// wallet's business and billing's, and a credential that could settle an invoice
// from here would be one compromise away from free service.
const Scopes = "billing:customers:read billing:customers:write " +
	"billing:subscriptions:read billing:subscriptions:write " +
	"billing:invoices:read billing:usage:write " +
	"billing:entitlements:read billing:products:read"

// tokenPath is ctech-account's client-credentials endpoint, on the same internal
// base URL this service already uses for userinfo.
const tokenPath = "/v1.0/token"

// requestTimeout bounds one call to billing.
//
// Ten seconds, and it matters where it is spent: this sits inside a DF-e request
// that a person is waiting on, so a billing outage must fail fast enough to
// return an error rather than hold the connection until something upstream gives
// up first.
const requestTimeout = 10 * time.Second

// ErrNotConfigured reports no-charge mode. Callers branch on it to grant
// unlimited service rather than to fail — see services.BillingService.
var ErrNotConfigured = errors.New("billing is not configured")

// ErrCustomerNotFound reports an external_ref billing does not know.
//
// Separated from every other 404 because it is the normal state of a brand-new
// account, not an error: the first thing a user does is not have a customer yet.
var ErrCustomerNotFound = errors.New("billing does not know this customer")

// Config is what the client needs to reach billing.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Cache        cache.Backend
}

// Client calls ctech-billing.
type Client struct {
	http    *http.Client
	tokens  *oauth2client.TokenManager
	baseURL string
}

// New builds a client, or nil when billing is not configured.
//
// Nil is a usable value here on purpose: every method answers ErrNotConfigured
// on a nil receiver, so the no-charge path needs no separate implementation and
// no interface with a stub behind it. What it does need is for callers to treat
// ErrNotConfigured as "unlimited", which is a decision in one place —
// BillingService — rather than at each call.
func New(cfg Config) *Client {
	if cfg.BaseURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	httpClient := &http.Client{Timeout: requestTimeout}
	return &Client{
		http:    httpClient,
		tokens:  oauth2client.New(httpClient, cfg.Cache, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, Scopes),
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
	}
}

// Enabled reports a client that can actually reach billing.
func (c *Client) Enabled() bool { return c != nil }

// TokenURLFor builds the client-credentials endpoint from the ctech-account base
// URL this service already holds, so billing needs no env var of its own for it.
func TokenURLFor(ctechURL string) string {
	if ctechURL == "" {
		return ""
	}
	return strings.TrimSuffix(ctechURL, "/") + tokenPath
}

// ---------------------------------------------------------------------------
// Wire types. These mirror ctech-billing's M2M responses, and only the fields
// this service uses — a struct that mirrored everything would be a second copy
// of billing's DTOs to keep in step.
// ---------------------------------------------------------------------------

// Customer is billing's customer, keyed to a DF-e account by ExternalRef.
type Customer struct {
	ID          string `json:"id"`
	ExternalRef string `json:"external_ref"`
	Name        string `json:"name"`
	Email       string `json:"email"`
}

// Price is one thing a plan bills for. Metadata carries the quotas, which
// billing does not read and this service does (ADR 0008 — metadata is opaque to
// billing).
type Price struct {
	ID         string            `json:"id"`
	ProductID  string            `json:"product_id"`
	Type       string            `json:"type"`
	UnitAmount int64             `json:"unit_amount"`
	Recurrence Recurrence        `json:"recurrence"`
	Timing     string            `json:"billing_timing"`
	Archived   bool              `json:"archived"`
	Metadata   map[string]string `json:"metadata"`
}

// Recurrence is a price's billing cycle.
type Recurrence struct {
	Interval string `json:"interval"`
	Count    int    `json:"count"`
}

// Product groups prices and is what the customer reads on the invoice line.
type Product struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Active   bool              `json:"active"`
	Metadata map[string]string `json:"metadata"`
	Prices   []Price           `json:"prices"`
}

// Period is a billing window, as civil dates in America/Sao_Paulo.
type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Invoice is what a customer owes for a period.
type Invoice struct {
	ID     string `json:"id"`
	Number int64  `json:"number"`
	Status string `json:"status"`
	// SubscriptionID is what the invoice was generated for. Billing's M2M invoice
	// list is tenant-wide, so this is the only field that says which account an
	// invoice belongs to — publishing the list without filtering on it would show
	// every CTech customer's invoices to whoever asked.
	SubscriptionID string `json:"subscription_id"`
	// Overdue is derived by billing on read, never stored.
	Overdue     bool   `json:"overdue"`
	Period      Period `json:"period"`
	DueDate     string `json:"due_date"`
	Total       int64  `json:"total"`
	AmountPaid  int64  `json:"amount_paid"`
	AmountDue   int64  `json:"amount_due"`
	CheckoutURL string `json:"checkout_url"`
}

// EntitlementItem is one price a subscription bills for.
type EntitlementItem struct {
	PriceID    string            `json:"price_id"`
	ProductID  string            `json:"product_id"`
	Type       string            `json:"type"`
	UnitAmount int64             `json:"unit_amount"`
	Quantity   int64             `json:"quantity"`
	Metadata   map[string]string `json:"metadata"`
}

// EntitlementInvoice is the bill waiting to be paid, when there is one. Its
// presence is what turns "pagamento pendente" into a button.
type EntitlementInvoice struct {
	ID          string `json:"id"`
	TotalCents  int64  `json:"total_cents"`
	DueDate     string `json:"due_date"`
	CheckoutURL string `json:"checkout_url"`
}

// EntitlementSubscription is one subscription's standing.
type EntitlementSubscription struct {
	ID                string              `json:"id"`
	Status            string              `json:"status"`
	Entitled          bool                `json:"entitled"`
	Plan              string              `json:"plan"`
	Items             []EntitlementItem   `json:"items"`
	CancelAtPeriodEnd bool                `json:"cancel_at_period_end"`
	Period            Period              `json:"current_period"`
	OpenInvoice       *EntitlementInvoice `json:"open_invoice"`
}

// Entitlements answers "can this account use the product, and under what plan".
type Entitlements struct {
	CustomerID    string                    `json:"customer_id"`
	Entitled      bool                      `json:"entitled"`
	Subscriptions []EntitlementSubscription `json:"subscriptions"`
}

// Subscription is billing's subscription row.
type Subscription struct {
	ID                string `json:"id"`
	CustomerID        string `json:"customer_id"`
	Status            string `json:"status"`
	Entitled          bool   `json:"entitled"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	Period            Period `json:"current_period"`
}

// SubscriptionResult is what creating or changing a subscription returns: the
// subscription, and the invoice when the operation produced one.
//
// The invoice is absent on a free plan, on the first period of a usage-based
// plan, and on a downgrade — three ordinary cases, not failures, and a caller
// must branch on its presence rather than assume a bill.
type SubscriptionResult struct {
	Subscription Subscription `json:"subscription"`
	Invoice      *Invoice     `json:"invoice"`
}

// ---------------------------------------------------------------------------
// Calls
// ---------------------------------------------------------------------------

// ListProducts reads the catalogue, prices and quota metadata included.
func (c *Client) ListProducts(ctx context.Context) ([]Product, error) {
	var out struct {
		Data []Product `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1.0/products", "", nil, &out); err != nil {
		return nil, err
	}
	// The list response carries no prices — billing publishes them on the detail
	// read, where archived ones are included too. One extra call per product, on
	// a catalogue of five that is cached for five minutes upstream.
	full := make([]Product, 0, len(out.Data))
	for _, p := range out.Data {
		detail, err := c.GetProduct(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		full = append(full, *detail)
	}
	return full, nil
}

// GetProduct reads one product with its prices, active and archived together.
func (c *Client) GetProduct(ctx context.Context, productID string) (*Product, error) {
	var out Product
	if err := c.do(ctx, http.MethodGet, "/v1.0/products/"+url.PathEscape(productID), "", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEntitlements reads an account's standing by the reference this service owns.
//
// A customer billing does not know answers ErrCustomerNotFound, which is the
// normal state of an account that has never chosen a plan.
func (c *Client) GetEntitlements(ctx context.Context, externalRef string) (*Entitlements, error) {
	var out Entitlements
	path := "/v1.0/entitlements?customer_ref=" + url.QueryEscape(externalRef)
	if err := c.do(ctx, http.MethodGet, path, "", nil, &out); err != nil {
		if errors.Is(err, errUpstreamNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrCustomerNotFound, externalRef)
		}
		return nil, err
	}
	return &out, nil
}

// CreateCustomerInput is a new billing customer for a DF-e account.
type CreateCustomerInput struct {
	// ExternalRef is this service's key for the account, `USER_{sub}`. It is what
	// every later read uses, so it must be stable for the life of the account.
	ExternalRef string `json:"external_ref"`
	// UserID is the bare ctech-account subject. Billing needs it to let the
	// person open the payment portal and to charge them through wallet — without
	// it there is nobody to collect from.
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

// CreateCustomer registers the account with billing.
func (c *Client) CreateCustomer(ctx context.Context, in CreateCustomerInput) (*Customer, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out Customer
	// The external ref as the idempotency key: creating the customer for one
	// account is the same operation however many times two tabs ask for it.
	if err := c.do(ctx, http.MethodPost, "/v1.0/customers", "customer:"+in.ExternalRef, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Item is one price a subscription is being put on.
type Item struct {
	PriceID  string `json:"price_id"`
	Quantity int64  `json:"quantity,omitempty"`
}

// CreateSubscription puts the customer on a plan.
//
// idempotencyKey is the caller's, and it must be deterministic for one intent:
// billing replays the first response for a repeated key, which is what keeps a
// double-clicked "assinar" from producing two subscriptions and two invoices.
func (c *Client) CreateSubscription(ctx context.Context, customerID string, items []Item, idempotencyKey string) (*SubscriptionResult, error) {
	body, err := json.Marshal(struct {
		CustomerID string `json:"customer_id"`
		Items      []Item `json:"items"`
	}{CustomerID: customerID, Items: items})
	if err != nil {
		return nil, err
	}
	var out SubscriptionResult
	if err := c.do(ctx, http.MethodPost, "/v1.0/subscriptions", idempotencyKey, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ChangeSubscription moves a live subscription onto a different price set,
// effective now, billing the prorated difference.
func (c *Client) ChangeSubscription(ctx context.Context, subscriptionID string, items []Item, idempotencyKey string) (*SubscriptionResult, error) {
	body, err := json.Marshal(struct {
		Items     []Item `json:"items"`
		Effective string `json:"effective"`
	}{Items: items, Effective: "now"})
	if err != nil {
		return nil, err
	}
	var out SubscriptionResult
	path := "/v1.0/subscriptions/" + url.PathEscape(subscriptionID) + "/change"
	if err := c.do(ctx, http.MethodPost, path, idempotencyKey, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelSubscription ends a subscription, at the period end or immediately.
//
// The two are different operations, not a shade of one: at the period end the
// customer keeps what they paid for until it runs out, and immediately revokes
// service now. The flag is passed through rather than decided here because the
// caller is the one who knows which the person asked for.
func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string, atPeriodEnd bool, idempotencyKey string) (*Subscription, error) {
	body, err := json.Marshal(struct {
		AtPeriodEnd bool `json:"at_period_end"`
	}{AtPeriodEnd: atPeriodEnd})
	if err != nil {
		return nil, err
	}
	var out Subscription
	path := "/v1.0/subscriptions/" + url.PathEscape(subscriptionID) + "/cancel"
	if err := c.do(ctx, http.MethodPost, path, idempotencyKey, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSubscription re-reads a subscription.
//
// This is what a webhook is for: the delivery says something changed and names
// an id, and this says what is actually true. Nothing in a webhook body is
// trusted beyond the id it carries.
func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*Subscription, error) {
	var out Subscription
	if err := c.do(ctx, http.MethodGet, "/v1.0/subscriptions/"+url.PathEscape(subscriptionID), "", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCustomer re-reads a customer, which is how a subscription id is resolved
// back to the DF-e account that owns it.
func (c *Client) GetCustomer(ctx context.Context, customerID string) (*Customer, error) {
	var out Customer
	if err := c.do(ctx, http.MethodGet, "/v1.0/customers/"+url.PathEscape(customerID), "", nil, &out); err != nil {
		if errors.Is(err, errUpstreamNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrCustomerNotFound, customerID)
		}
		return nil, err
	}
	return &out, nil
}

// ListInvoices reads one month of the tenant's invoices.
//
// **Tenant-wide, not per customer.** Billing's M2M invoice list has no customer
// filter, so the caller must not publish this to an end user without narrowing
// it — BillingService does that by subscription id.
func (c *Client) ListInvoices(ctx context.Context, year, month int) ([]Invoice, error) {
	var out struct {
		Data []Invoice `json:"data"`
	}
	path := fmt.Sprintf("/v1.0/invoices?year=%d&month=%d", year, month)
	if err := c.do(ctx, http.MethodGet, path, "", nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// ReportUsage records one metered event.
//
// eventKey identifies the consumption itself, separately from the HTTP
// idempotency key: the worker sends the document's access key, so a redelivered
// SQS message reports the same emission rather than a second one. Billing
// answers `duplicate: true` for a repeat, which is success.
func (c *Client) ReportUsage(ctx context.Context, subscriptionID, priceID string, quantity int64, eventKey string) error {
	body, err := json.Marshal(struct {
		SubscriptionID string `json:"subscription_id"`
		PriceID        string `json:"price_id"`
		Quantity       int64  `json:"quantity"`
		IdempotencyKey string `json:"idempotency_key"`
	}{SubscriptionID: subscriptionID, PriceID: priceID, Quantity: quantity, IdempotencyKey: eventKey})
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/v1.0/usage", eventKey, body, nil)
}

// ---------------------------------------------------------------------------
// Transport
// ---------------------------------------------------------------------------

// errUpstreamNotFound is billing's 404, before it is mapped. It exists so the
// two callers that treat a missing row as an ordinary state — a customer who has
// never subscribed — can tell it apart from every other failure without parsing
// a message.
var errUpstreamNotFound = errors.New("billing: not found")

func (c *Client) do(ctx context.Context, method, path, idempotencyKey string, body []byte, out any) error {
	if c == nil {
		return ErrNotConfigured
	}
	token, err := c.tokens.Get(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "billing: could not mint a token", "error", err)
		return problem.InternalServer("não foi possível falar com o serviço de cobrança")
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return problem.InternalServer("não foi possível falar com o serviço de cobrança")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "billing: request failed", "method", method, "path", path, "error", err)
		return problem.InternalServer("o serviço de cobrança está indisponível")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return mapUpstreamError(ctx, method, path, resp.StatusCode, raw)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		slog.ErrorContext(ctx, "billing: unreadable response", "method", method, "path", path, "error", err)
		return problem.InternalServer("resposta inesperada do serviço de cobrança")
	}
	return nil
}

// mapUpstreamError turns billing's RFC 7807 into a DF-e problem.
//
// Billing's own `detail` is logged and never returned. It is well written, but
// it is written about billing's entities: a DF-e user told "assinatura sub_01J…
// não encontrada" has been given an id from a system they cannot see and an
// instruction they cannot follow. What they get instead is a message about the
// thing they were trying to do.
//
// The status mapping is not a pass-through either. A 403 from billing means
// *this service's* credential lacks a scope — a deployment fault — so it must
// not reach the user as "você não tem permissão", which would send them to their
// account settings to fix something that is not theirs.
func mapUpstreamError(ctx context.Context, method, path string, status int, raw []byte) error {
	var p struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(raw, &p)
	logAt := slog.LevelError
	if status == http.StatusNotFound {
		logAt = slog.LevelInfo
	}
	slog.Log(ctx, logAt, "billing: upstream refused",
		"method", method, "path", path, "status", status, "title", p.Title, "detail", p.Detail)

	switch status {
	case http.StatusNotFound:
		return errUpstreamNotFound
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		// Billing rejected what this service sent. The caller supplied part of it
		// — a price id from a stale catalogue is the realistic case — so it is
		// answered as a bad request rather than an internal error.
		return problem.BadRequest("a operação de cobrança foi recusada; recarregue os planos e tente de novo")
	case http.StatusConflict:
		return problem.Conflict("a assinatura mudou desde a última leitura; recarregue e tente de novo")
	case http.StatusUnauthorized, http.StatusForbidden:
		// This service's own credential, not the user's. Never phrased as the
		// user's permission problem.
		return problem.InternalServer("o serviço de cobrança recusou as credenciais desta aplicação")
	default:
		return problem.InternalServer("o serviço de cobrança está indisponível")
	}
}
