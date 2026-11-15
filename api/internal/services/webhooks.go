package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/api-commons/webhooks"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

const (
	webhookManageDenied = "apenas proprietários e administradores da organização podem gerenciar webhooks"
	// webhookMaxURL bounds a registered URL.
	webhookMaxURL = 2048
)

const (
	// WebhookMaxPerEndpointPerPass bounds the attempts one endpoint gets in a delivery pass; the rest wait a
	// minute so other organizations' deliveries come first. A receiver that answers 200 but slowly would
	// otherwise fill the whole batch, run after run.
	WebhookMaxPerEndpointPerPass = 20
	// WebhookAPIAttemptTimeout is the attempt timeout of the engine the API uses for ping and replay. It runs
	// inside the request, whose deadline (REQUEST_TIMEOUT, 8 s by default) must outlast the attempt and the
	// writes that record it, so it is shorter than the Lambda's webhooks.DeliveryTimeout.
	WebhookAPIAttemptTimeout = 5 * time.Second
)

// NewWebhookEngine builds the shared delivery engine over the DF-e table. The API
// uses it for ping and replay (WebhookAPIAttemptTimeout) and the scheduled Lambda for
// everything due (webhooks.DeliveryTimeout), so the two sign, retry and disable endpoints
// exactly alike. onDisabled is told when an endpoint is disabled by its failure run (the
// operator alert); nil is allowed. opts adjust the engine's config (tests inject a client).
func NewWebhookEngine(repo *repositories.WebhookRepository, onDisabled func(repositories.WebhookEndpoint), workers int, attemptTimeout time.Duration, opts ...func(*webhooks.Config)) *webhooks.Deliverer {
	cfg := webhooks.Config{
		HeaderPrefix: WebhookHeaderPrefix, UserAgent: WebhookUserAgent, Workers: workers,
		MaxPerGroup: WebhookMaxPerEndpointPerPass, Client: webhooks.NewGuardedClient(attemptTimeout),
	}
	for _, o := range opts {
		o(&cfg)
	}
	return webhooks.NewDeliverer(repositories.NewWebhookStore(repo, onDisabled), cfg)
}

// NewWebhookVerifier builds the registration challenge check (guarded client).
func NewWebhookVerifier() *webhooks.Verifier { return webhooks.NewVerifier(WebhookUserAgent) }

// WebhookURLVerifier proves a URL answers the registration challenge.
// webhooks.Verifier satisfies it.
type WebhookURLVerifier interface {
	Verify(ctx context.Context, url string) error
}

// WebhookRegistration is what a person submits to register an endpoint.
type WebhookRegistration struct {
	URL string
	// Events and CompanyPKs filter; empty is everything.
	Events     []string
	CompanyPKs []string
}

// WebhookService is the console side of outbound webhooks: a person who owns or
// administers the organization registers, tests, rotates and removes endpoints,
// and reads and replays deliveries. Delivering is the engine's (go-common/webhooks).
type WebhookService struct {
	repo      *repositories.WebhookRepository
	billing   *BillingService // organization of a company, and the person's role in it
	verifier  WebhookURLVerifier
	deliverer *webhooks.Deliverer // ping and replay attempt at once; nil refuses them
	now       func() time.Time
}

func NewWebhookService(repo *repositories.WebhookRepository, billing *BillingService, verifier WebhookURLVerifier, deliverer *webhooks.Deliverer) *WebhookService {
	return &WebhookService{repo: repo, billing: billing, verifier: verifier, deliverer: deliverer, now: time.Now}
}

// Enabled reports whether webhooks can be used at all (a secret key is configured).
func (s *WebhookService) Enabled() bool { return s.repo.Enabled() }

// scope resolves the company to its organization and requires owner or admin.
// Like reach, every refusal is one answer and the cause goes to the log.
func (s *WebhookService) scope(ctx context.Context, companyPK, userID string) (*BillingScope, error) {
	scope, err := s.billing.ScopeFor(ctx, companyPK, userID)
	if err != nil {
		return nil, err
	}
	ok, err := s.billing.CanManage(ctx, scope)
	if err != nil {
		slog.WarnContext(ctx, "webhooks: could not read the role; refusing", "organization_id", scope.OrganizationID, "error", err)
		return nil, problem.Forbidden(webhookManageDenied)
	}
	if !ok {
		return nil, problem.Forbidden(webhookManageDenied)
	}
	return scope, nil
}

// Register validates, proves the URL answers the challenge, and only then stores
// the endpoint. The returned endpoint carries the secret: this is the one time
// it is shown.
func (s *WebhookService) Register(ctx context.Context, companyPK, userID string, in WebhookRegistration) (*repositories.WebhookEndpoint, error) {
	if !s.Enabled() {
		return nil, problem.NotImplemented("webhooks desativados nesta instalação")
	}
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return nil, err
	}
	if len(in.URL) > webhookMaxURL {
		return nil, problem.BadRequest("a URL é longa demais")
	}
	if err := webhooks.CheckURL(in.URL); err != nil {
		if errors.Is(err, webhooks.ErrHostNotAllowed) {
			return nil, problem.BadRequest("o endereço precisa ser um site público na internet")
		}
		return nil, problem.BadRequest("informe uma URL https válida, sem usuário e senha")
	}
	events := dedupe(in.Events)
	for _, t := range events {
		if !slices.Contains(WebhookEventTypes, t) {
			return nil, problem.BadRequest("tipo de evento inválido: " + t)
		}
	}
	companies := dedupe(in.CompanyPKs)
	for _, pk := range companies {
		orgID, err := s.billing.OrganizationOf(ctx, pk)
		if err != nil || orgID != scope.OrganizationID {
			return nil, problem.BadRequest("empresa não pertence à organização")
		}
	}
	if err := s.verifier.Verify(ctx, in.URL); err != nil {
		return nil, problem.UnprocessableEntity(err.Error())
	}
	e := &repositories.WebhookEndpoint{
		ID: repositories.GenerateID(), OrganizationID: scope.OrganizationID, URL: in.URL,
		Secret: webhooks.NewSecret(), Events: events, CompanyPKs: companies,
		Status: repositories.WebhookActive, CreatedBy: scope.UserID,
	}
	if err := s.repo.CreateEndpoint(ctx, e, s.now()); err != nil {
		if errors.Is(err, repositories.ErrWebhookLimit) {
			return nil, problem.Conflict("limite de endpoints de webhook da organização atingido")
		}
		return nil, err
	}
	return e, nil
}

func dedupe(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// WebhookList is the console's whole picture: the endpoints and the companies a
// filter may pick from (the keys only; the screen already holds their names).
type WebhookList struct {
	Endpoints  []repositories.WebhookEndpoint
	CompanyPKs []string
}

// List returns the organization's endpoints, secrets never included, and the
// keys of its companies for the endpoint filter.
func (s *WebhookService) List(ctx context.Context, companyPK, userID string) (*WebhookList, error) {
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return nil, err
	}
	list, err := s.repo.ListEndpoints(ctx, scope.OrganizationID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Secret = ""
	}
	companies, err := s.billing.orgs.CompaniesOf(ctx, scope.OrganizationID)
	if err != nil {
		return nil, err
	}
	pks := make([]string, 0, len(companies))
	for _, c := range companies {
		pks = append(pks, c.PK)
	}
	return &WebhookList{Endpoints: list, CompanyPKs: pks}, nil
}

func notFound(err error) error {
	if errors.Is(err, repositories.ErrWebhookNotFound) {
		return problem.NotFound("webhook não encontrado")
	}
	return err
}

// Delete removes an endpoint and frees its slot.
func (s *WebhookService) Delete(ctx context.Context, companyPK, userID, endpointID string) error {
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return err
	}
	return notFound(s.repo.DeleteEndpoint(ctx, scope.OrganizationID, endpointID))
}

// Enable brings a disabled endpoint back (spec: manual reactivation). It repeats the registration
// challenge first, so an endpoint that is still down stays disabled, and keeps the id, the secret
// and the history. An endpoint that is already active is left alone, without a second challenge.
func (s *WebhookService) Enable(ctx context.Context, companyPK, userID, endpointID string) error {
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return err
	}
	e, err := s.repo.GetEndpoint(ctx, scope.OrganizationID, endpointID)
	if err != nil {
		return notFound(err)
	}
	if e.Status == repositories.WebhookActive {
		return nil
	}
	if err := s.verifier.Verify(ctx, e.URL); err != nil {
		return problem.UnprocessableEntity(err.Error())
	}
	return notFound(s.repo.EnableEndpoint(ctx, scope.OrganizationID, endpointID))
}

// RotateSecret replaces the signing secret and returns the new one, shown once.
func (s *WebhookService) RotateSecret(ctx context.Context, companyPK, userID, endpointID string) (string, error) {
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return "", err
	}
	secret := webhooks.NewSecret()
	if err := s.repo.RotateSecret(ctx, scope.OrganizationID, endpointID, secret); err != nil {
		if errors.Is(err, repositories.ErrWebhookNoKey) {
			return "", problem.NotImplemented("webhooks desativados nesta instalação")
		}
		return "", notFound(err)
	}
	return secret, nil
}

// Ping sends the console's test event to one endpoint, whatever its filters, and
// attempts it at once; how the receiver answered is in its deliveries.
func (s *WebhookService) Ping(ctx context.Context, companyPK, userID, endpointID string) error {
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return err
	}
	e, err := s.repo.GetEndpoint(ctx, scope.OrganizationID, endpointID)
	if err != nil {
		return notFound(err)
	}
	if e.Status != repositories.WebhookActive {
		return problem.Conflict("o endpoint está desativado")
	}
	now := s.now()
	id := repositories.GenerateID()
	body, err := BuildWebhookBody(id, repositories.WebhookPing, now, WebhookDocument{})
	if err != nil {
		return err
	}
	ev := repositories.WebhookEvent{ID: id, Type: repositories.WebhookPing, EndpointID: e.ID, Body: body}
	if _, err := s.repo.WriteDeliveries(ctx, scope.OrganizationID, ev, []repositories.WebhookEndpoint{*e}, now); err != nil {
		return err
	}
	return s.attempt(ctx, scope.OrganizationID, e.ID, id, now)
}

// attempt tries a stored delivery now. The delivery stays pending for the
// scheduled job either way, so a nil deliverer only skips the immediacy.
func (s *WebhookService) attempt(ctx context.Context, organizationID, endpointID, eventID string, now time.Time) error {
	if s.deliverer == nil {
		return nil
	}
	d, err := s.repo.GetDelivery(ctx, organizationID, endpointID, eventID)
	if err != nil {
		return err
	}
	err = s.deliverer.Redeliver(ctx, repositories.DueOf(*d), now)
	switch {
	case errors.Is(err, webhooks.ErrEndpointInactive):
		return problem.Conflict("o endpoint está desativado")
	case errors.Is(err, webhooks.ErrEndpointGone):
		return problem.NotFound("webhook não encontrado")
	}
	return err
}

// Deliveries lists an endpoint's deliveries newest first, one page.
func (s *WebhookService) Deliveries(ctx context.Context, companyPK, userID, endpointID string, start map[string]types.AttributeValue, limit int) ([]repositories.WebhookDelivery, map[string]types.AttributeValue, error) {
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return nil, nil, err
	}
	if _, err := s.repo.GetEndpoint(ctx, scope.OrganizationID, endpointID); err != nil {
		return nil, nil, notFound(err)
	}
	return s.repo.ListDeliveries(ctx, scope.OrganizationID, endpointID, start, limit)
}

// Delivery is one delivery with every try made at it.
func (s *WebhookService) Delivery(ctx context.Context, companyPK, userID, endpointID, eventID string) (*repositories.WebhookDelivery, []repositories.WebhookAttempt, error) {
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return nil, nil, err
	}
	d, err := s.repo.GetDelivery(ctx, scope.OrganizationID, endpointID, eventID)
	if err != nil {
		return nil, nil, notFound(err)
	}
	attempts, err := s.repo.Attempts(ctx, scope.OrganizationID, endpointID, eventID)
	return d, attempts, err
}

// Replay sends a delivery again, at once. A disabled or deleted endpoint is
// refused and nothing is sent.
func (s *WebhookService) Replay(ctx context.Context, companyPK, userID, endpointID, eventID string) error {
	scope, err := s.scope(ctx, companyPK, userID)
	if err != nil {
		return err
	}
	if s.deliverer == nil {
		return problem.NotImplemented("reenvio indisponível")
	}
	if _, err := s.repo.GetDelivery(ctx, scope.OrganizationID, endpointID, eventID); err != nil {
		return notFound(err)
	}
	return s.attempt(ctx, scope.OrganizationID, endpointID, eventID, s.now())
}

// WebhookResult is a document's terminal outcome as the results consumer sees it.
type WebhookResult struct {
	CompanyPK string
	// OrganizationID is the ctech-account organization when the message carries
	// it; empty is resolved from the company.
	OrganizationID string
	// DocumentKey is the document's row key: the chave de acesso for NF-e, NFC-e,
	// CT-e and MDF-e, the id_dps for NFS-e.
	DocumentKey string
	Table       string
	Status      string
	// SefazStatus and SefazMotive are the authority's code and reason, used only
	// for a rejection.
	SefazStatus, SefazMotive string
	// SentAt is when the result was published (the queue's SentTimestamp); the
	// event id is derived from it so a redelivery repeats the same id.
	SentAt time.Time
}

var webhookEventTypeOf = map[string]string{
	"authorized": WebhookEventAuthorized,
	"rejected":   WebhookEventRejected,
	"failed":     WebhookEventFailed,
}

// Emit turns a terminal document result into a delivery for each endpoint that
// wants it. Nothing here makes an HTTP call: the scheduled job delivers. An event
// nobody wants writes nothing; webhooks off, an unlinked organization and a
// result that is not a document are skipped (nothing the queue could fix). A
// failed write is returned so the message stays on the queue: the write is
// idempotent (the id is derived from the event), so a retry cannot duplicate it.
func (s *WebhookService) Emit(ctx context.Context, in WebhookResult) error {
	if !s.Enabled() {
		return nil
	}
	eventType, ok := webhookEventTypeOf[in.Status]
	docType, isDoc := MeterForTable[in.Table]
	if !ok || !isDoc || in.DocumentKey == "" {
		return nil
	}
	organizationID := in.OrganizationID
	if organizationID == "" {
		id, err := s.billing.OrganizationOf(ctx, in.CompanyPK)
		var p *problem.Problem
		switch {
		case errors.As(err, &p) && p.Status == http.StatusNotFound, err == nil && id == "":
			// No such company, or one not linked to an organization: nothing to deliver to, and the queue cannot fix it.
			slog.WarnContext(ctx, "webhooks: result has no organization; skipped", "company", in.CompanyPK)
			return nil
		case err != nil:
			return err // a read that failed (throttling): the message stays on the queue and is tried again
		}
		organizationID = id
	}
	endpoints, err := s.repo.ListEndpoints(ctx, organizationID)
	if err != nil {
		return err
	}
	doc := WebhookDocument{ID: in.DocumentKey, Status: in.Status, DocumentType: docType}
	if docType != MeterNFSe {
		doc.AccessKey = in.DocumentKey // the id_dps of an NFS-e is not an access key
	}
	if in.Status == "rejected" {
		doc.RejectionCode, doc.RejectionMessage = in.SefazStatus, in.SefazMotive
	}
	at := in.SentAt
	if at.IsZero() {
		at = s.now()
	}
	id := repositories.WebhookEventID(at, in.CompanyPK+"|"+in.DocumentKey+"|"+in.Status)
	body, err := BuildWebhookBody(id, eventType, at, doc)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(in.CompanyPK + "|" + in.DocumentKey + "|" + in.Status))
	_, err = s.repo.WriteDeliveries(ctx, organizationID, repositories.WebhookEvent{
		ID: id, Type: eventType, CompanyPK: in.CompanyPK, Body: body, DedupeKey: hex.EncodeToString(sum[:16]),
	}, endpoints, s.now())
	return err
}
