package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/worker/internal/config"
)

// S3Client is the S3 operations subset used by the worker.
type S3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// LambdaClient is the Lambda operations subset used by the worker.
type LambdaClient interface {
	Invoke(ctx context.Context, params *lambdaSDK.InvokeInput, optFns ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error)
}

// DynamoClient is the DynamoDB operations subset used by the worker.
type DynamoClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// SNSClient is the SNS operations subset used by the worker.
type SNSClient interface {
	Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

// Clients bundles all external AWS dependencies for DfeService.
type Clients struct {
	S3     S3Client
	Lambda LambdaClient
	Dynamo DynamoClient
	SNS    SNSClient // optional; set to nil to disable result notifications
}

// DfeService processes DFe SEFAZ operations received from SQS.
type DfeService struct {
	s3     S3Client
	lam    LambdaClient
	dynamo DynamoClient
	sns    SNSClient
	cfg    *config.Config
}

// New creates a new DfeService.
func New(clients Clients, cfg *config.Config) *DfeService {
	return &DfeService{
		s3:     clients.S3,
		lam:    clients.Lambda,
		dynamo: clients.Dynamo,
		sns:    clients.SNS,
		cfg:    cfg,
	}
}

// WorkerMessage is the SQS message body for a DFe SEFAZ operation.
type WorkerMessage struct {
	DocPK     string `json:"doc_pk"`
	AccessKey string `json:"access_key"`
	TableName string `json:"table_name"`
	S3Prefix  string `json:"s3_prefix"`
	// S3Tenant is the folder a document's XML goes under, sent by the API so a
	// company's history stays in one prefix across the company re-key. Empty on
	// a message queued before this field existed; documentS3Key falls back.
	S3Tenant              string         `json:"s3_tenant,omitempty"`
	ExpectedFileName      string         `json:"expected_file_name"`
	CNPJ                  string         `json:"cnpj"`
	UF                    string         `json:"uf"`
	SefazEnvironment      string         `json:"sefaz_environment"`
	CertS3Key             string         `json:"cert_s3_key"`
	CertPassword          string         `json:"cert_password"`
	DocType               string         `json:"doc_type"`
	SefazService          string         `json:"sefaz_service"`
	Body                  map[string]any `json:"body"`
	BillingUserID         string         `json:"billing_user_id,omitempty"`
	BillingPeriod         string         `json:"billing_period,omitempty"`
	BillingSubscriptionID string         `json:"billing_subscription_id,omitempty"`
	BillingPriceID        string         `json:"billing_price_id,omitempty"`
	BillingMeter          string         `json:"billing_meter,omitempty"`
	BillingExempt         bool           `json:"billing_exempt,omitempty"`
	// Event fields — present only for SEFAZ event operations (cancellation, CC-e, …)
	EventsTableName *string `json:"events_table_name,omitempty"`
	EventType       *string `json:"event_type,omitempty"`
	SequenceNumber  *int    `json:"sequence_number,omitempty"`
	EventSK         *string `json:"event_sk,omitempty"`

	processingOwner string
}

type lambdaPayload struct {
	CNPJ                string         `json:"cnpj"`
	CertificateB64      string         `json:"certificate_b64"`
	CertificatePassword string         `json:"certificate_password"`
	UF                  string         `json:"uf"`
	Environment         string         `json:"environment"`
	DocType             string         `json:"doc_type"`
	Service             string         `json:"service"`
	Body                map[string]any `json:"body"`
}

type lambdaResponse struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
}

// docTerminalStatuses / eventTerminalStatuses are the statuses that mean "this
// message was already fully processed — do not invoke SEFAZ again."
var docTerminalStatuses = map[string]bool{
	StatusAuthorized: true, StatusRejected: true, StatusFailed: true,
	StatusCancelled: true, StatusClosed: true,
}

// StatusRejected belongs here too: a SEFAZ business rejection of an event is
// final. Without it, claimProcessing could not claim the row (rejected is not a
// claimable status) and did not recognise it as already-done either, so every
// SQS redelivery errored until the message reached the DLQ.
var eventTerminalStatuses = map[string]bool{
	EventStatusSuccess: true, EventStatusError: true, StatusRejected: true,
}

const processingLeaseDuration = 6 * time.Minute

var errProcessingLeaseHeld = errors.New("dfe processing lease is held by another worker")
var errResultPublish = errors.New("publish terminal result")

type processingTarget struct {
	table    string
	pk       string
	sk       string
	terminal map[string]bool
}

func (s *DfeService) processingTarget(msg WorkerMessage) processingTarget {
	var table, pk, sk string
	var terminal map[string]bool
	if msg.EventsTableName != nil && msg.EventSK != nil {
		table = s.cfg.TablePrefix + "_" + *msg.EventsTableName
		pk, sk = msg.AccessKey, *msg.EventSK
		terminal = eventTerminalStatuses
	} else {
		table = s.cfg.TablePrefix + "_" + msg.TableName
		pk, sk = msg.DocPK, msg.AccessKey
		terminal = docTerminalStatuses
	}
	return processingTarget{table: table, pk: pk, sk: sk, terminal: terminal}
}

func newProcessingOwner() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate processing owner: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// claimProcessing is the idempotency boundary for every external SEFAZ call.
// It atomically claims pending/retryable work or steals an expired lease. A
// DynamoDB failure fails closed; no certificate or SEFAZ operation is touched.
func (s *DfeService) claimProcessing(ctx context.Context, msg WorkerMessage, owner string) (bool, string, updateAttrs, error) {
	target := s.processingTarget(msg)
	now := time.Now().UTC()
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(target.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: target.pk},
			"sk": &types.AttributeValueMemberS{Value: target.sk},
		},
		UpdateExpression:         aws.String("SET #status = :processing, processing_owner = :owner, processing_lease_until = :lease, processing_attempt = if_not_exists(processing_attempt, :zero) + :one, updated_at = :updated"),
		ConditionExpression:      aws.String("attribute_exists(pk) AND (#status = :pending OR #status = :retryable OR #status = :cancel_pending OR #status = :close_pending OR (#status = :processing AND processing_lease_until < :now))"),
		ExpressionAttributeNames: map[string]string{"#status": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pending":        &types.AttributeValueMemberS{Value: StatusPending},
			":retryable":      &types.AttributeValueMemberS{Value: StatusRetryableFailed},
			":cancel_pending": &types.AttributeValueMemberS{Value: StatusCancelPending},
			":close_pending":  &types.AttributeValueMemberS{Value: StatusClosePending},
			":processing":     &types.AttributeValueMemberS{Value: StatusProcessing},
			":owner":          &types.AttributeValueMemberS{Value: owner},
			":now":            &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", now.Unix())},
			":lease":          &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", now.Add(processingLeaseDuration).Unix())},
			":zero":           &types.AttributeValueMemberN{Value: "0"},
			":one":            &types.AttributeValueMemberN{Value: "1"},
			":updated":        &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
		},
	})
	if err == nil {
		return true, "", updateAttrs{}, nil
	}
	if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); !ok {
		return false, "", updateAttrs{}, fmt.Errorf("claim processing %s/%s: %w", target.table, target.sk, err)
	}

	out, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(target.table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: target.pk},
			"sk": &types.AttributeValueMemberS{Value: target.sk},
		},
		ProjectionExpression:     aws.String("#status, sefaz_status, sefaz_motive, sefaz_protocol, xml_s3_key"),
		ExpressionAttributeNames: map[string]string{"#status": "status"},
		ConsistentRead:           aws.Bool(true),
	})
	if err != nil {
		return false, "", updateAttrs{}, fmt.Errorf("read processing claim %s/%s: %w", target.table, target.sk, err)
	}
	if out.Item == nil {
		return false, "", updateAttrs{}, fmt.Errorf("processing target %s/%s does not exist", target.table, target.sk)
	}
	statusAttr, ok := out.Item["status"].(*types.AttributeValueMemberS)
	if !ok {
		return false, "", updateAttrs{}, fmt.Errorf("processing target %s/%s has no string status", target.table, target.sk)
	}
	if target.terminal[statusAttr.Value] {
		return false, statusAttr.Value, updateAttrs{
			SefazStatus:   attributeString(out.Item, "sefaz_status"),
			SefazMotive:   attributeString(out.Item, "sefaz_motive"),
			SefazProtocol: attributeString(out.Item, "sefaz_protocol"),
			XMLS3Key:      attributeString(out.Item, "xml_s3_key"),
		}, nil
	}
	return false, "", updateAttrs{}, errProcessingLeaseHeld
}

func attributeString(item map[string]types.AttributeValue, name string) *string {
	if value, ok := item[name].(*types.AttributeValueMemberS); ok {
		return aws.String(value.Value)
	}
	return nil
}

// Process handles a single DFe SEFAZ operation from an SQS message.
func (s *DfeService) Process(ctx context.Context, msg WorkerMessage) error {
	slog.Info("processing dfe",
		"doc_type", msg.DocType,
		"sefaz_service", msg.SefazService,
		"doc_pk", msg.DocPK,
		"access_key", msg.AccessKey,
	)

	owner, err := newProcessingOwner()
	if err != nil {
		return err
	}
	claimed, terminalStatus, terminalAttrs, err := s.claimProcessing(ctx, msg, owner)
	if err != nil {
		return err
	}
	if !claimed {
		slog.Info("republishing already-terminal message", "access_key", msg.AccessKey, "doc_pk", msg.DocPK)
		if msg.EventsTableName != nil {
			return s.publishEventResult(ctx, msg, terminalStatus, terminalAttrs)
		}
		return s.publishDocumentResult(ctx, msg, terminalStatus, terminalAttrs)
	}
	msg.processingOwner = owner

	certB64, err := s.getCertB64(ctx, msg.CertS3Key)
	if err != nil {
		cause := fmt.Errorf("getCertB64: %w", err)
		return s.markRetryable(ctx, msg, "failed to retrieve certificate: "+err.Error(), cause)
	}

	pyDfePayload := lambdaPayload{
		CNPJ:                msg.CNPJ,
		CertificateB64:      certB64,
		CertificatePassword: msg.CertPassword,
		UF:                  msg.UF,
		Environment:         msg.SefazEnvironment,
		DocType:             msg.DocType,
		Service:             msg.SefazService,
		Body:                msg.Body,
	}

	// 2026-07-18: worker cut over to go-dfe in-process for every
	// (docType, service) it implements (see go-dfe/dfe.go's `implemented`
	// map) — done at explicit operator direction during a controlled
	// zero-traffic window, ahead of the plan's normal shadow-mode/
	// byte-identical gates for the newly-promoted signed operations.
	// Revert to py-dfe-only (undo this cutover): comment the if/else block
	// below, uncomment the line under it.
	var lambdaResp lambdaResponse
	if godfeImplements(msg.DocType, msg.SefazService) {
		resp, callErr := godfeCall(ctx, godfe.Request{
			CNPJ: msg.CNPJ, CertificateB64: certB64, CertificatePassword: msg.CertPassword,
			UF: msg.UF, Environment: normalizeSefazEnvironment(msg.SefazEnvironment),
			DocType: msg.DocType, Service: msg.SefazService, Body: msg.Body,
		})
		lambdaResp, err = lambdaResponse{StatusCode: resp.StatusCode, Body: resp.Body}, callErr
	} else {
		lambdaResp, err = s.invokePyDfe(ctx, pyDfePayload)
	}
	// lambdaResp, err = s.invokePyDfe(ctx, pyDfePayload)
	if err != nil {
		cause := fmt.Errorf("sefaz call: %w", err)
		return s.markRetryable(ctx, msg, "sefaz invocation error: "+err.Error(), cause)
	}

	slog.Info("sefaz response", "status_code", lambdaResp.StatusCode, "access_key", msg.AccessKey)

	if lambdaResp.StatusCode != 200 {
		detail := "py-dfe error"
		var bodyMap map[string]any
		if json.Unmarshal([]byte(lambdaResp.Body), &bodyMap) == nil {
			if d, ok := bodyMap["detail"].(string); ok && d != "" {
				detail = d
			}
		}
		slog.Warn("py-dfe returned error", "access_key", msg.AccessKey, "detail", detail, "response_body", lambdaResp.Body)
		if isRetryableEngineStatus(lambdaResp.StatusCode) {
			cause := fmt.Errorf("SEFAZ engine returned retryable status %d: %s", lambdaResp.StatusCode, detail)
			return s.markRetryable(ctx, msg, detail, cause)
		}
		return s.failTerminal(ctx, msg, detail)
	}

	var respBody map[string]any
	if err := json.Unmarshal([]byte(lambdaResp.Body), &respBody); err != nil {
		motive := "failed to parse lambda response body"
		cause := fmt.Errorf("parse lambda body: %w", err)
		return s.markRetryable(ctx, msg, motive, cause)
	}

	if isNfse(msg.DocType) {
		if err := s.handleNfseResponse(ctx, msg, respBody); err != nil {
			if errors.Is(err, errResultPublish) {
				return err
			}
			return s.markRetryable(ctx, msg, err.Error(), err)
		}
		return nil
	}

	if err := s.handleSefazResponse(ctx, msg, respBody); err != nil {
		if errors.Is(err, errResultPublish) {
			return err
		}
		return s.markRetryable(ctx, msg, err.Error(), err)
	}
	return nil
}

func (s *DfeService) handleSefazResponse(ctx context.Context, msg WorkerMessage, respBody map[string]any) error {
	cStat := findValue(respBody, "cStat")
	xMotivo := findValue(respBody, "xMotivo")
	nProt := findValue(respBody, "nProt")

	// Unwrap envelope-level batch stats (cStat 104/128): use infProt/infEvento.
	if cStat != nil && batchStats[*cStat] {
		if infProt := findDict(respBody, "infProt"); infProt != nil {
			if inner := findValue(infProt, "cStat"); inner != nil {
				cStat = inner
			}
			if inner := findValue(infProt, "xMotivo"); inner != nil {
				xMotivo = inner
			}
			nProt = findValue(infProt, "nProt")
		} else if infEvento := findDict(respBody, "infEvento"); infEvento != nil {
			if inner := findValue(infEvento, "cStat"); inner != nil {
				cStat = inner
			}
			if inner := findValue(infEvento, "xMotivo"); inner != nil {
				xMotivo = inner
			}
			nProt = findValue(infEvento, "nProt")
		}
	}

	slog.Info("sefaz response", "cStat", strVal(cStat), "xMotivo", strVal(xMotivo), "access_key", msg.AccessKey)

	isSefazEvent := msg.EventsTableName != nil && msg.EventType != nil && msg.EventSK != nil
	isCancellation := isCancellationEvent(msg.DocType, msg.EventType)
	isClose := isCloseEvent(msg.DocType, msg.EventType)

	var xmlS3Key *string
	var eventStatus string

	switch {
	case cStat != nil && authorizedStats[*cStat]:
		key, err := s.saveResponse(ctx, msg.DocPK, msg.S3Prefix, msg.CNPJ, msg.ExpectedFileName, respBody)
		if err != nil {
			return fmt.Errorf("saveResponse: %w", err)
		}
		xmlS3Key = &key
		eventStatus = EventStatusSuccess

		if isClose {
			if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusClosed, updateAttrs{}, false); err != nil {
				return err
			}
		} else if isCancellation {
			if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusCancelled, updateAttrs{}, false); err != nil {
				return err
			}
		} else if !isSefazEvent {
			if err := s.updateClaimedDocument(ctx, msg, StatusAuthorized, updateAttrs{
				SefazStatus:   cStat,
				SefazMotive:   xMotivo,
				SefazProtocol: nProt,
				XMLS3Key:      xmlS3Key,
			}, true); err != nil {
				return err
			}
		}

	case cStat != nil:
		eventStatus = StatusRejected
		if isClose {
			// Encerramento rejected → revert document to authorized, unless the
			// manifest was already closed (duplicated event).
			status := StatusAuthorized
			if *cStat == DuplicatedEventError {
				status = StatusClosed
			}
			if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, status, updateAttrs{}, false); err != nil {
				return err
			}
		} else if isCancellation {
			// Cancellation rejected → revert document to authorized.

			var status string
			if *cStat == DuplicatedEventError {
				status = StatusCancelled
			} else {
				status = StatusAuthorized
			}

			if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, status, updateAttrs{}, false); err != nil {
				return err
			}
		} else if !isSefazEvent {
			if err := s.updateClaimedDocument(ctx, msg, StatusRejected, updateAttrs{
				SefazStatus: cStat,
				SefazMotive: xMotivo,
			}, true); err != nil {
				return err
			}
		}

	default:
		motive := strVal(xMotivo)
		if motive == "" {
			motive = "cStat ausente na resposta SEFAZ"
		}
		return errors.New(motive)
	}

	if isSefazEvent {
		eventAttrs := updateAttrs{
			SefazStatus:   cStat,
			SefazMotive:   xMotivo,
			SefazProtocol: nProt,
			XMLS3Key:      xmlS3Key,
		}
		if err := s.updateClaimedEvent(ctx, msg, eventStatus, eventAttrs); err != nil {
			return err
		}
		// Notify on the event outcome — not the document status, which may have
		// been reverted to "authorized" above after a rejected event.
		return s.publishEventResult(ctx, msg, eventStatus, eventAttrs)
	}

	return nil
}

func isRetryableEngineStatus(statusCode int) bool {
	return statusCode == 408 || statusCode == 425 || statusCode == 429 || statusCode >= 500
}

// markRetryable records an infrastructure/transport failure without making
// the target terminal. Releasing the lease lets the next SQS delivery claim it.
func (s *DfeService) markRetryable(ctx context.Context, msg WorkerMessage, motive string, cause error) error {
	attrs := updateAttrs{SefazMotive: &motive}
	var err error
	if msg.EventsTableName != nil && msg.EventSK != nil {
		err = s.updateClaimedEvent(ctx, msg, StatusRetryableFailed, attrs)
	} else {
		err = s.updateClaimedDocument(ctx, msg, StatusRetryableFailed, attrs, false)
	}
	if err != nil {
		return errors.Join(cause, fmt.Errorf("persist retryable failure: %w", err))
	}
	return cause
}

// failTerminal records a non-retryable engine validation failure. SEFAZ
// business rejections are finalized separately from their cStat response.
func (s *DfeService) failTerminal(ctx context.Context, msg WorkerMessage, motive string) error {
	isSefazEvent := msg.EventsTableName != nil && msg.EventType != nil && msg.EventSK != nil
	isCancellation := isCancellationEvent(msg.DocType, msg.EventType)
	isClose := isCloseEvent(msg.DocType, msg.EventType)

	if isCancellation || isClose {
		if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusAuthorized, updateAttrs{}, false); err != nil {
			return err
		}
	} else if !isSefazEvent {
		return s.updateClaimedDocument(ctx, msg, StatusFailed, updateAttrs{
			SefazMotive: &motive,
		}, true)
	}

	if isSefazEvent {
		eventAttrs := updateAttrs{SefazMotive: &motive}
		if err := s.updateClaimedEvent(ctx, msg, EventStatusError, eventAttrs); err != nil {
			return err
		}
		return s.publishEventResult(ctx, msg, EventStatusError, eventAttrs)
	}
	return nil
}

func (s *DfeService) getCertB64(ctx context.Context, certS3Key string) (string, error) {
	if cached, ok := certCache.get(certS3Key); ok {
		return cached, nil
	}
	out, err := s.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.CertsBucket),
		Key:    aws.String(certS3Key),
	})
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(out.Body)
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return "", err
	}
	certB64 := base64.StdEncoding.EncodeToString(data)
	certCache.set(certS3Key, certB64)
	return certB64, nil
}

func (s *DfeService) invokePyDfe(ctx context.Context, payload lambdaPayload) (lambdaResponse, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return lambdaResponse{}, err
	}
	out, err := s.lam.Invoke(ctx, &lambdaSDK.InvokeInput{
		FunctionName: aws.String(s.cfg.DfeLambdaName),
		Payload:      payloadBytes,
	})
	if err != nil {
		return lambdaResponse{}, err
	}
	if out.FunctionError != nil {
		return lambdaResponse{}, fmt.Errorf("lambda function error: %s", *out.FunctionError)
	}
	var resp lambdaResponse
	if err := json.Unmarshal(out.Payload, &resp); err != nil {
		return lambdaResponse{}, err
	}
	return resp, nil
}

const (
	extXML          = "xml"
	extJSON         = "json"
	contentTypeXML  = "application/xml"
	contentTypeJSON = "application/json"
)

// documentS3Key builds the object key for a document artifact. The docPK's "#"
// separators become path segments so the bucket mirrors the tenant hierarchy.
// documentS3Key builds where a document's XML lives.
//
// The tenant segment is CNPJ_{document} — derived from the issuer document the
// message already carries, NOT from the partition key. Since ctech-billing ADR
// 0022 that key is a company id, and using it would split a company's history
// into an old folder and a new one at the moment of the re-key. Documents group
// by the CNPJ that issued them, so the prefix stays continuous.
//
// It falls back to the DocPK's own tenant half when the message carries no
// document. That is not decoration: messages queued before the issuer document
// was read off the record are still in flight, and a path that changed under
// them would write their XML somewhere their row does not point to.
//
// Two organizations sharing a CNPJ therefore share a prefix. Safe and
// deliberate: every object is addressed by the xml_s3_key stored on its own
// row, the série claim keeps their access keys apart, and nothing lists by
// prefix. Adding prefix listing is what would make this need revisiting.
func (s *DfeService) documentS3Key(docPK, s3Prefix, issuerDoc, fileName, ext string) string {
	path := strings.ReplaceAll(docPK, "#", "/")
	if segment := tenantSegment(issuerDoc); segment != "" {
		if env, _, found := strings.Cut(docPK, "#"); found {
			path = env + "/" + segment
		}
	}
	return fmt.Sprintf("%s/%s/%s.%s", s3Prefix, path, fileName, ext)
}

// tenantSegment names a document's folder the way the retired partition key
// did. Length is what tells the two apart, and it is the same rule the old key
// encoded: eleven is a CPF, fourteen a CNPJ.
//
// Anything else returns "" and the caller falls back rather than inventing a
// folder — a wrong prefix is an XML nobody finds by browsing.
func tenantSegment(issuerDoc string) string {
	switch len(issuerDoc) {
	case 11:
		return "CPF_" + issuerDoc
	case 14:
		return "CNPJ_" + issuerDoc
	default:
		return ""
	}
}

func (s *DfeService) putObject(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	if _, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.DocumentsBucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", err
	}
	return key, nil
}

func (s *DfeService) saveResponse(ctx context.Context, docPK, s3Prefix, issuerDoc, expectedFileName string, respBody map[string]any) (string, error) {
	if xmlRaw, ok := respBody["@xml"].(string); ok && xmlRaw != "" {
		return s.putObject(ctx, s.documentS3Key(docPK, s3Prefix, issuerDoc, expectedFileName, extXML), []byte(xmlRaw), contentTypeXML)
	}

	data, err := json.Marshal(respBody)
	if err != nil {
		return "", err
	}
	return s.putObject(ctx, s.documentS3Key(docPK, s3Prefix, issuerDoc, expectedFileName, extJSON), data, contentTypeJSON)
}

// updateStatus updates a document's status in DynamoDB. When notify is true a
// document-result notification is published to SNS. Internal status reverts
// triggered by an event outcome (e.g. reverting to "authorized" after a failed
// cancellation) must pass notify=false — the user-facing notification for those
// is published by publishEventResult, not by the reverted document status.
func (s *DfeService) updateStatus(ctx context.Context, docPK, accessKey, tableName, status string, attrs updateAttrs, notify bool) error {
	return s.updateStatusOwned(ctx, WorkerMessage{DocPK: docPK, AccessKey: accessKey, TableName: tableName}, status, attrs, notify, "")
}

func (s *DfeService) updateClaimedDocument(ctx context.Context, msg WorkerMessage, status string, attrs updateAttrs, notify bool) error {
	return s.updateStatusOwned(ctx, msg, status, attrs, notify, msg.processingOwner)
}

func (s *DfeService) updateStatusOwned(ctx context.Context, msg WorkerMessage, status string, attrs updateAttrs, notify bool, owner string) error {
	if err := updateDocumentStatus(ctx, s.dynamo, s.cfg.TablePrefix, msg.DocPK, msg.AccessKey, msg.TableName, status, attrs, owner); err != nil {
		return err
	}

	if notify {
		return s.publishDocumentResult(ctx, msg, status, attrs)
	}

	return nil
}

func (s *DfeService) publishDocumentResult(ctx context.Context, msg WorkerMessage, status string, attrs updateAttrs) error {
	return s.publishResult(ctx, map[string]any{
		notifyKeyResultKind:            resultKindDocument,
		notifyKeyAccessKey:             msg.AccessKey,
		notifyKeyDocPK:                 msg.DocPK,
		notifyKeyTableName:             msg.TableName,
		notifyKeyStatus:                status,
		notifyKeySefazStatus:           strVal(attrs.SefazStatus),
		notifyKeySefazMotive:           strVal(attrs.SefazMotive),
		notifyKeySefazProtocol:         strVal(attrs.SefazProtocol),
		notifyKeyXMLS3Key:              strVal(attrs.XMLS3Key),
		notifyKeyBillingUserID:         msg.BillingUserID,
		notifyKeyBillingPeriod:         msg.BillingPeriod,
		notifyKeyBillingSubscriptionID: msg.BillingSubscriptionID,
		notifyKeyBillingPriceID:        msg.BillingPriceID,
		notifyKeyBillingMeter:          msg.BillingMeter,
		notifyKeyBillingExempt:         msg.BillingExempt,
	})
}

// publishEventResult publishes a SEFAZ event outcome (cancellation,
// encerramento, …) to SNS. Unlike a document result, the payload carries
// result_kind=event plus event_type/event_sk and the event's own status, so
// the frontend reports the event outcome instead of the (possibly reverted)
// document status. table_name is the *document* table so the client can map
// the event to its document and invalidate the right queries.
func (s *DfeService) publishEventResult(ctx context.Context, msg WorkerMessage, eventStatus string, attrs updateAttrs) error {
	return s.publishResult(ctx, map[string]any{
		notifyKeyResultKind:  resultKindEvent,
		notifyKeyAccessKey:   msg.AccessKey,
		notifyKeyDocPK:       msg.DocPK,
		notifyKeyTableName:   msg.TableName,
		notifyKeyStatus:      eventStatus,
		notifyKeyEventType:   strVal(msg.EventType),
		notifyKeyEventSK:     strVal(msg.EventSK),
		notifyKeySefazStatus: strVal(attrs.SefazStatus),
		notifyKeySefazMotive: strVal(attrs.SefazMotive),
	})
}

// publishResult marshals and publishes a result notification to the results
// SNS topic. No-op when SNS is disabled or no topic is configured.
func (s *DfeService) publishResult(ctx context.Context, payload map[string]any) error {
	if s.sns == nil || s.cfg.ResultsTopicARN == "" {
		slog.Warn("results notification skipped: SNS disabled or RESULTS_TOPIC_ARN unset",
			"access_key", payload[notifyKeyAccessKey])
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("sns marshal failed", "err", err)
		return fmt.Errorf("%w: marshal notification: %v", errResultPublish, err)
	}
	out, err := s.sns.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(s.cfg.ResultsTopicARN),
		Message:  aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("%w: %v", errResultPublish, err)
	} else if out == nil {
		return fmt.Errorf("%w: no response", errResultPublish)
	}
	slog.Info("sns publish result", "result", out)
	return nil
}

func (s *DfeService) updateClaimedEvent(ctx context.Context, msg WorkerMessage, status string, attrs updateAttrs) error {
	if msg.EventsTableName == nil || msg.EventSK == nil {
		return errors.New("event processing target is incomplete")
	}
	table := s.cfg.TablePrefix + "_" + *msg.EventsTableName
	parts := buildUpdateExpression(status, attrs)
	parts.attrValues[":owner"] = &types.AttributeValueMemberS{Value: msg.processingOwner}

	if _, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: msg.AccessKey},
			"sk": &types.AttributeValueMemberS{Value: *msg.EventSK},
		},
		UpdateExpression:          aws.String(parts.expression + " REMOVE processing_owner, processing_lease_until"),
		ExpressionAttributeNames:  parts.attrNames,
		ExpressionAttributeValues: parts.attrValues,
		ConditionExpression:       aws.String("processing_owner = :owner"),
	}); err != nil {
		return fmt.Errorf("updateEvent %s %s: %w", table, *msg.EventSK, err)
	}
	slog.Info("updated event status", "table", table, "event_sk", *msg.EventSK, "status", status)
	return nil
}
