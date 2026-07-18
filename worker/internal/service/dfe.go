package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

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
	DocPK            string         `json:"doc_pk"`
	AccessKey        string         `json:"access_key"`
	TableName        string         `json:"table_name"`
	S3Prefix         string         `json:"s3_prefix"`
	ExpectedFileName string         `json:"expected_file_name"`
	CNPJ             string         `json:"cnpj"`
	UF               string         `json:"uf"`
	SefazEnvironment string         `json:"sefaz_environment"`
	CertS3Key        string         `json:"cert_s3_key"`
	CertPassword     string         `json:"cert_password"`
	DocType          string         `json:"doc_type"`
	SefazService     string         `json:"sefaz_service"`
	Body             map[string]any `json:"body"`
	// Event fields — present only for SEFAZ event operations (cancellation, CC-e, …)
	EventsTableName *string `json:"events_table_name,omitempty"`
	EventType       *string `json:"event_type,omitempty"`
	SequenceNumber  *int    `json:"sequence_number,omitempty"`
	EventSK         *string `json:"event_sk,omitempty"`
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

var eventTerminalStatuses = map[string]bool{
	EventStatusSuccess: true, EventStatusError: true,
}

// alreadyTerminal reports whether msg's target item is already in a terminal
// status, i.e. this is a redelivered message (standard SQS is at-least-once,
// see docs/specs/2026-07-18-audit-findings-remediation-design.md §2). On a
// DynamoDB read error it fails OPEN (returns false, logs a warning) — SEFAZ's
// own duplicate-rejection (DuplicatedEventError) remains the backstop, so a
// transient read error must not block real processing.
func (s *DfeService) alreadyTerminal(ctx context.Context, msg WorkerMessage) bool {
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

	out, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
		ProjectionExpression:     aws.String("#status"),
		ExpressionAttributeNames: map[string]string{"#status": "status"},
	})
	if err != nil {
		slog.Warn("idempotency check failed, proceeding with processing", "access_key", msg.AccessKey, "err", err)
		return false
	}
	if out.Item == nil {
		return false
	}
	statusAttr, ok := out.Item["status"].(*types.AttributeValueMemberS)
	if !ok {
		return false
	}
	return terminal[statusAttr.Value]
}

// Process handles a single DFe SEFAZ operation from an SQS message.
func (s *DfeService) Process(ctx context.Context, msg WorkerMessage) error {
	slog.Info("processing dfe",
		"doc_type", msg.DocType,
		"sefaz_service", msg.SefazService,
		"doc_pk", msg.DocPK,
		"access_key", msg.AccessKey,
	)

	if s.alreadyTerminal(ctx, msg) {
		slog.Info("skipping already-terminal message (redelivery)", "access_key", msg.AccessKey, "doc_pk", msg.DocPK)
		return nil
	}

	certB64, err := s.getCertB64(ctx, msg.CertS3Key)
	if err != nil {
		s.failDoc(ctx, msg, "failed to retrieve certificate: "+err.Error())
		return fmt.Errorf("getCertB64: %w", err)
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
		s.failDoc(ctx, msg, "sefaz invocation error: "+err.Error())
		return fmt.Errorf("sefaz call: %w", err)
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
		slog.Warn("py-dfe returned error", "access_key", msg.AccessKey, "detail", detail)
		s.failDoc(ctx, msg, detail)
		return nil
	}

	var respBody map[string]any
	if err := json.Unmarshal([]byte(lambdaResp.Body), &respBody); err != nil {
		motive := "failed to parse lambda response body"
		s.failDoc(ctx, msg, motive)
		return fmt.Errorf("parse lambda body: %w", err)
	}

	return s.handleSefazResponse(ctx, msg, respBody)
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
		key, err := s.saveResponse(ctx, msg.DocPK, msg.S3Prefix, msg.ExpectedFileName, respBody)
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
			if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusAuthorized, updateAttrs{
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
			if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusRejected, updateAttrs{
				SefazStatus: cStat,
				SefazMotive: xMotivo,
			}, true); err != nil {
				return err
			}
		}

	default:
		eventStatus = EventStatusError
		if isClose || isCancellation {
			if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusAuthorized, updateAttrs{}, false); err != nil {
				return err
			}
		} else if !isSefazEvent {
			motive := strVal(xMotivo)
			if motive == "" {
				motive = "cStat ausente na resposta SEFAZ"
			}
			if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusFailed, updateAttrs{
				SefazMotive: &motive,
			}, true); err != nil {
				return err
			}
		}
	}

	if isSefazEvent {
		eventAttrs := updateAttrs{
			SefazStatus:   cStat,
			SefazMotive:   xMotivo,
			SefazProtocol: nProt,
			XMLS3Key:      xmlS3Key,
		}
		if err := s.updateEvent(ctx, *msg.EventsTableName, msg.AccessKey, *msg.EventSK, eventStatus, eventAttrs); err != nil {
			return err
		}
		// Notify on the event outcome — not the document status, which may have
		// been reverted to "authorized" above after a rejected event.
		s.publishEventResult(ctx, msg, eventStatus, eventAttrs)
	}

	return nil
}

// failDoc marks a document or event as failed depending on the message type.
// Errors are logged but not returned — this is a best-effort recovery path.
//
// Routing rules:
//   - Cancellation event failure → revert original document to "authorized" + update event as error.
//   - Other event failure        → update event as error only; original document is NOT touched.
//   - Non-event failure          → mark original document as failed.
func (s *DfeService) failDoc(ctx context.Context, msg WorkerMessage, motive string) {
	isSefazEvent := msg.EventsTableName != nil && msg.EventType != nil && msg.EventSK != nil
	isCancellation := isCancellationEvent(msg.DocType, msg.EventType)
	isClose := isCloseEvent(msg.DocType, msg.EventType)

	if isCancellation || isClose {
		if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusAuthorized, updateAttrs{}, false); err != nil {
			slog.Error("failed to revert document to authorized after event failure", "access_key", msg.AccessKey, "err", err)
		}
	} else if !isSefazEvent {
		if err := s.updateStatus(ctx, msg.DocPK, msg.AccessKey, msg.TableName, StatusFailed, updateAttrs{
			SefazMotive: &motive,
		}, true); err != nil {
			slog.Error("failed to mark document as failed", "access_key", msg.AccessKey, "err", err)
		}
	}

	if isSefazEvent {
		eventAttrs := updateAttrs{SefazMotive: &motive}
		if err := s.updateEvent(ctx, *msg.EventsTableName, msg.AccessKey, *msg.EventSK, EventStatusError, eventAttrs); err != nil {
			slog.Error("failed to mark event as failed", "event_sk", *msg.EventSK, "err", err)
		}
		// Surface the event failure to the client — the reverted document
		// status above is intentionally not notified.
		s.publishEventResult(ctx, msg, EventStatusError, eventAttrs)
	}
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

func (s *DfeService) saveResponse(ctx context.Context, docPK, s3Prefix, expectedFileName string, respBody map[string]any) (string, error) {
	docPath := strings.ReplaceAll(docPK, "#", "/")

	var key string
	var data []byte
	var contentType string

	if xmlRaw, ok := respBody["@xml"].(string); ok && xmlRaw != "" {
		key = fmt.Sprintf("%s/%s/%s.xml", s3Prefix, docPath, expectedFileName)
		data = []byte(xmlRaw)
		contentType = "application/xml"
	} else {
		key = fmt.Sprintf("%s/%s/%s.json", s3Prefix, docPath, expectedFileName)
		var err error
		data, err = json.Marshal(respBody)
		if err != nil {
			return "", err
		}
		contentType = "application/json"
	}

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

// updateStatus updates a document's status in DynamoDB. When notify is true a
// document-result notification is published to SNS. Internal status reverts
// triggered by an event outcome (e.g. reverting to "authorized" after a failed
// cancellation) must pass notify=false — the user-facing notification for those
// is published by publishEventResult, not by the reverted document status.
func (s *DfeService) updateStatus(ctx context.Context, docPK, accessKey, tableName, status string, attrs updateAttrs, notify bool) error {
	table := s.cfg.TablePrefix + "_" + tableName
	parts := buildUpdateExpression(status, attrs)

	if _, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: docPK},
			"sk": &types.AttributeValueMemberS{Value: accessKey},
		},
		UpdateExpression:          aws.String(parts.expression),
		ExpressionAttributeNames:  parts.attrNames,
		ExpressionAttributeValues: parts.attrValues,
	}); err != nil {
		return fmt.Errorf("updateStatus %s %s: %w", table, accessKey, err)
	}
	slog.Info("updated document status", "table", table, "access_key", accessKey, "status", status)

	if notify {
		s.publishResult(ctx, map[string]any{
			notifyKeyResultKind:    resultKindDocument,
			notifyKeyAccessKey:     accessKey,
			notifyKeyDocPK:         docPK,
			notifyKeyTableName:     tableName,
			notifyKeyStatus:        status,
			notifyKeySefazStatus:   strVal(attrs.SefazStatus),
			notifyKeySefazMotive:   strVal(attrs.SefazMotive),
			notifyKeySefazProtocol: strVal(attrs.SefazProtocol),
			notifyKeyXMLS3Key:      strVal(attrs.XMLS3Key),
		})
	}
	return nil
}

// publishEventResult publishes a SEFAZ event outcome (cancellation,
// encerramento, …) to SNS. Unlike a document result, the payload carries
// result_kind=event plus event_type/event_sk and the event's own status, so
// the frontend reports the event outcome instead of the (possibly reverted)
// document status. table_name is the *document* table so the client can map
// the event to its document and invalidate the right queries.
func (s *DfeService) publishEventResult(ctx context.Context, msg WorkerMessage, eventStatus string, attrs updateAttrs) {
	s.publishResult(ctx, map[string]any{
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
func (s *DfeService) publishResult(ctx context.Context, payload map[string]any) {
	if s.sns == nil || s.cfg.ResultsTopicARN == "" {
		slog.Warn("results notification skipped: SNS disabled or RESULTS_TOPIC_ARN unset",
			"access_key", payload[notifyKeyAccessKey])
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("sns marshal failed", "err", err)
		return
	}
	if _, err := s.sns.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(s.cfg.ResultsTopicARN),
		Message:  aws.String(string(body)),
	}); err != nil {
		slog.Error("sns publish failed", "err", err)
	}
}

func (s *DfeService) updateEvent(ctx context.Context, eventsTableName, accessKey, eventSK, status string, attrs updateAttrs) error {
	table := s.cfg.TablePrefix + "_" + eventsTableName
	parts := buildUpdateExpression(status, attrs)

	if _, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: accessKey},
			"sk": &types.AttributeValueMemberS{Value: eventSK},
		},
		UpdateExpression:          aws.String(parts.expression),
		ExpressionAttributeNames:  parts.attrNames,
		ExpressionAttributeValues: parts.attrValues,
		ConditionExpression:       aws.String("attribute_exists(pk)"),
	}); err != nil {
		return fmt.Errorf("updateEvent %s %s: %w", table, eventSK, err)
	}
	slog.Info("updated event status", "table", table, "event_sk", eventSK, "status", status)
	return nil
}
