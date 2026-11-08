package service

// ciot.go: the worker side of the ANTT CIOT (docs/specs/2026-11-04-ciot-antt-pef-integracao.md sections 4, 8, 10).
// Messages with doc_type "ciot" are claimed on the ciots row (pending_operation + lease), the generator and ANTT calls
// go through the go-dfe-egress, and the outcome is written to the row, the append-only audit table and the results
// topic. Secrets (generator key, fallback URL) come from SSM per invocation and are never logged; neither are bodies.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/dfe/go-dfe/ciot"
)

const (
	docTypeCIOT = "ciot"

	// Row statuses (API: services/ciots).
	ciotDeclared  = "declared"
	ciotRectified = "rectified"
	ciotCancelled = "cancelled"
	ciotClosed    = "closed"
	ciotRejected  = "rejected"
	ciotFailed    = "failed"

	opDeclare = "declare"

	attrPendingOp = "pending_operation"
	tableAudit    = "ciot_audit"

	egressStatusBadConfig = 400
	egressStatusRejection = 422

	// Codes whose implied terminal state confirms the requested one (spec section 8; the B37/B38 mapping is inferred).
	anttAlreadyCancelled = 222
	anttAlreadyClosed    = 251
)

// SSMClient is the SSM subset used to read the generator parameters.
type SSMClient interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// WithCiot supplies what the CIOT path needs beyond the shared clients: SSM for the generator parameters and the
// operator alert publisher. Without SSM the declaration fails as a configuration error.
func (s *DfeService) WithCiot(client SSMClient, alerter alerts.Publisher) *DfeService {
	s.ssm = client
	if alerter != nil {
		s.alerter = alerter
	}
	return s
}

// ssmSecrets is a ciot.SecretProvider over SSM, cached for one invocation.
type ssmSecrets struct {
	client SSMClient
	env    string
	cache  map[string]string
}

func (p *ssmSecrets) Get(ctx context.Context, name string) (string, error) {
	if v, ok := p.cache[name]; ok {
		return v, nil
	}
	if p.client == nil {
		return "", errors.New("ssm client not configured")
	}
	out, err := p.client.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(ciot.ParamName(p.env, name)), WithDecryption: aws.Bool(true)})
	if err != nil {
		if _, missing := errors.AsType[*ssmtypes.ParameterNotFound](err); missing && name == ciot.ParamFallbackURL {
			p.cache[name] = "" // the fallback is optional: no parameter means no fallback
			return "", nil
		}
		return "", fmt.Errorf("read ssm parameter %s: %w", name, err)
	}
	p.cache[name] = aws.ToString(out.Parameter.Value)
	return p.cache[name], nil
}

// ciotOutcome is the decided effect of one egress answer; decideCiot is pure so every mapping is unit tested.
type ciotOutcome struct {
	Status     string            // new row status, "" keeps it
	Attrs      map[string]string // code, ciot, carrier_warning, protocol
	AnttCode   int
	Internal   string
	Message    string
	Retry      bool // transport/5xx: release the lease and let the platform retry
	Alert      bool // credential/configuration problem: operator alert (also retried, then the DLQ)
	Reconcile  bool // 219: adopt the existing CIOT with ConsultarCIOTGerado
	LastError  bool // business rejection to keep on the row
	TerminalOK bool
}

// successStatus is the row status after a successful operation.
var successStatus = map[string]string{opDeclare: ciotDeclared, ciot.OpRectify: ciotRectified, ciot.OpCancel: ciotCancelled, ciot.OpClose: ciotClosed}

type egressRejection struct {
	InternalCode string `json:"internal_code"`
	ANTTCode     int    `json:"antt_code"`
	ANTTMessage  string `json:"antt_message"`
	Detail       string `json:"detail"`
	Group        string `json:"group"`
}

func decideCiot(op string, status int, body string) ciotOutcome {
	switch {
	case status == 200:
		var r ciot.Result
		if err := json.Unmarshal([]byte(body), &r); err != nil || !r.Success() {
			return ciotOutcome{Retry: true, Message: "unreadable ANTT response"}
		}
		o := ciotOutcome{Status: successStatus[op], AnttCode: r.Codigo, Message: r.Mensagem, TerminalOK: true, Attrs: map[string]string{}}
		if r.IdOperacao != "" && len(r.IdOperacao) == ciot.IDLen {
			o.Attrs["code"] = r.IdOperacao
		}
		if r.CIOT != "" {
			o.Attrs["ciot"] = r.CIOT
		}
		if r.AvisoTransportador != "" {
			o.Attrs["carrier_warning"] = r.AvisoTransportador
		}
		if r.Protocolo != "" {
			o.Attrs["protocol"] = r.Protocolo
		}
		return o
	case status == egressStatusRejection:
		var rj egressRejection
		_ = json.Unmarshal([]byte(body), &rj)
		o := ciotOutcome{AnttCode: rj.ANTTCode, Internal: rj.InternalCode, Message: rj.ANTTMessage}
		info := ciot.LookupRejection(rj.ANTTCode)
		switch {
		case rj.ANTTCode == ciot.CodeDuplicateID && op == opDeclare:
			o.Reconcile = true
		case (op == ciot.OpCancel && rj.ANTTCode == anttAlreadyCancelled) || (op == ciot.OpClose && rj.ANTTCode == anttAlreadyClosed):
			o.Status, o.TerminalOK = successStatus[op], true
		case info.IsCredentialProblem():
			o.Alert, o.Retry = true, true
		default:
			o.LastError = true
			if op == opDeclare {
				o.Status = ciotRejected
			}
		}
		return o
	case status == egressStatusBadConfig:
		return ciotOutcome{Alert: true, Retry: true, Message: "egress rejected the request (certificate or configuration)"}
	}
	return ciotOutcome{Retry: true, Message: fmt.Sprintf("egress status %d", status)}
}

// --- row access ---

func (s *DfeService) ciotKey(msg WorkerMessage, sk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: msg.DocPK},
		"sk": &types.AttributeValueMemberS{Value: sk},
	}
}

// claimCiot takes the lease on the row for this operation. done=true means the operation already finished (the
// message is a redelivery): the caller republishes the stored outcome. A held lease is a retryable error.
func (s *DfeService) claimCiot(ctx context.Context, msg WorkerMessage, sk, op, owner string) (row map[string]types.AttributeValue, done bool, err error) {
	now := time.Now().UTC()
	table := aws.String(s.cfg.TablePrefix + "_ciots")
	out, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           table,
		Key:                 s.ciotKey(msg, sk),
		UpdateExpression:    aws.String("SET processing_owner = :owner, processing_lease_until = :lease, updated_at = :ts"),
		ConditionExpression: aws.String("attribute_exists(pk) AND pending_operation = :op AND (attribute_not_exists(processing_lease_until) OR processing_lease_until < :now)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":owner": &types.AttributeValueMemberS{Value: owner},
			":lease": &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(leaseFor(ctx, now)).Unix(), 10)},
			":ts":    &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
			":op":    &types.AttributeValueMemberS{Value: op},
			":now":   &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Unix(), 10)},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err == nil {
		return out.Attributes, false, nil
	}
	if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); !ok {
		return nil, false, fmt.Errorf("claim ciot: %w", err)
	}
	got, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{TableName: table, Key: s.ciotKey(msg, sk), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return nil, false, fmt.Errorf("read ciot claim: %w", err)
	}
	if got.Item == nil {
		return nil, false, fmt.Errorf("ciot row %s does not exist", sk)
	}
	if p, _ := got.Item[attrPendingOp].(*types.AttributeValueMemberS); p == nil || p.Value != op {
		return got.Item, true, nil
	}
	return nil, false, errProcessingLeaseHeld
}

func rowStr(row map[string]types.AttributeValue, k string) string {
	if v, ok := row[k].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

// releaseCiot gives the lease back (retryable failure), keeping the pending operation.
func (s *DfeService) releaseCiot(ctx context.Context, msg WorkerMessage, sk, owner, motive string) error {
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(s.cfg.TablePrefix + "_ciots"),
		Key:                 s.ciotKey(msg, sk),
		UpdateExpression:    aws.String("SET last_motive = :m, updated_at = :ts REMOVE processing_owner, processing_lease_until"),
		ConditionExpression: aws.String("processing_owner = :owner"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":m":     &types.AttributeValueMemberS{Value: motive},
			":ts":    &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
			":owner": &types.AttributeValueMemberS{Value: owner},
		},
	})
	return err
}

// finishCiot writes the outcome: status and attributes, the history entry, and clears the in-flight marker.
func (s *DfeService) finishCiot(ctx context.Context, msg WorkerMessage, sk, owner, op string, o ciotOutcome, declaration map[string]any) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	set := []string{"updated_at = :ts", "history = list_append(if_not_exists(history, :empty), :entry)"}
	names := map[string]string{}
	vals := map[string]types.AttributeValue{
		":ts":    &types.AttributeValueMemberS{Value: now},
		":owner": &types.AttributeValueMemberS{Value: owner},
		":empty": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
	}
	histStatus := o.Status
	if histStatus == "" {
		histStatus = "rejected"
	}
	entry := map[string]types.AttributeValue{
		"operation": &types.AttributeValueMemberS{Value: op}, "status": &types.AttributeValueMemberS{Value: histStatus},
		"at": &types.AttributeValueMemberS{Value: now}, "antt_code": &types.AttributeValueMemberN{Value: strconv.Itoa(o.AnttCode)},
		"internal_code": &types.AttributeValueMemberS{Value: o.Internal},
	}
	vals[":entry"] = &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberM{Value: entry}}}
	if o.Status != "" {
		set, names["#st"], vals[":st"] = append(set, "#st = :st"), "status", &types.AttributeValueMemberS{Value: o.Status}
	}
	i := 0
	for k, v := range o.Attrs {
		ph := fmt.Sprintf(":a%d", i)
		set, vals[ph] = append(set, fmt.Sprintf("%s = %s", k, ph)), &types.AttributeValueMemberS{Value: v}
		i++
	}
	remove := "pending_operation, processing_owner, processing_lease_until"
	if o.LastError {
		set = append(set, "last_error = :le")
		vals[":le"] = &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"internal_code": &types.AttributeValueMemberS{Value: o.Internal},
			"antt_code":     &types.AttributeValueMemberN{Value: strconv.Itoa(o.AnttCode)},
			"antt_message":  &types.AttributeValueMemberS{Value: o.Message},
		}}
	} else {
		remove += ", last_error, last_motive"
	}
	if declaration != nil && o.TerminalOK && (op == opDeclare || op == ciot.OpRectify) {
		raw, _ := json.Marshal(declaration)
		set, vals[":dj"] = append(set, "declaration_json = :dj"), &types.AttributeValueMemberS{Value: string(raw)}
	}
	in := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(s.cfg.TablePrefix + "_ciots"),
		Key:                       s.ciotKey(msg, sk),
		UpdateExpression:          aws.String("SET " + strings.Join(set, ", ") + " REMOVE " + remove),
		ConditionExpression:       aws.String("processing_owner = :owner"),
		ExpressionAttributeValues: vals,
	}
	if len(names) > 0 {
		in.ExpressionAttributeNames = names
	}
	_, err := s.dynamo.UpdateItem(ctx, in)
	return err
}

// auditCiot appends the evidence: the body sent (hash + body) and the response. Insert-only; never logged.
func (s *DfeService) auditCiot(ctx context.Context, msg WorkerMessage, sk, op string, sent map[string]any, status int, response string) {
	raw, _ := json.Marshal(sent)
	h := sha256.Sum256(raw)
	now := time.Now().UTC()
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.cfg.TablePrefix + "_" + tableAudit),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: sk},
			"sk": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano) + "#response:" + op},
		},
		UpdateExpression:         aws.String("SET #op = :op, body_hash = :h, body = :b, response = :r, egress_status = :s, created_at = :ts"),
		ConditionExpression:      aws.String("attribute_not_exists(pk)"),
		ExpressionAttributeNames: map[string]string{"#op": "operation"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":op": &types.AttributeValueMemberS{Value: op}, ":h": &types.AttributeValueMemberS{Value: hex.EncodeToString(h[:])},
			":b": &types.AttributeValueMemberS{Value: string(raw)}, ":r": &types.AttributeValueMemberS{Value: response},
			":s": &types.AttributeValueMemberN{Value: strconv.Itoa(status)}, ":ts": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
		},
	})
	if err != nil { // the evidence is best effort here: the API wrote the request side in the same transaction as the command
		slog.Error("ciot audit write failed", "sk", sk, "operation", op, "err", err)
	}
}

// --- processing ---

func (s *DfeService) publishCiotResult(ctx context.Context, msg WorkerMessage, sk, op, status string, o ciotOutcome, row map[string]types.AttributeValue) error {
	payload := map[string]any{
		notifyKeyResultKind: resultKindDocument, notifyKeyAccessKey: sk, notifyKeyDocPK: msg.DocPK,
		notifyKeyTableName: msg.TableName, notifyKeyStatus: status, "operation": op,
		"antt_code": o.AnttCode, "internal_code": o.Internal, "antt_message": o.Message,
	}
	for _, k := range []string{"ciot", "code", "carrier_warning"} {
		if v := rowStr(row, k); v != "" {
			payload[k] = v
		}
	}
	for k, v := range o.Attrs {
		payload[k] = v
	}
	return s.publishResult(ctx, payload)
}

func (s *DfeService) alertCiot(ctx context.Context, msg WorkerMessage, sk, op string, o ciotOutcome) {
	s.alerter.Alert(ctx, alerts.Alert{
		Job:     "ciot-credential",
		Summary: "A CIOT operation hit a certificate or credential problem; fix the configuration, it is not a business error.",
		Detail: fmt.Sprintf("ciot: %s\noperation: %s\nenvironment: %s\nantt_code: %d\ninternal_code: %s\nmessage: %s",
			sk, op, msg.SefazEnvironment, o.AnttCode, o.Internal, o.Message),
	})
}

func (s *DfeService) processCiot(ctx context.Context, msg WorkerMessage) error {
	op, sk := strVal(msg.EventType), strVal(msg.EventSK)
	if op == "" || sk == "" || msg.DocPK == "" {
		return errors.New("ciot message without operation, row key or partition")
	}
	owner, err := newProcessingOwner()
	if err != nil {
		return err
	}
	row, done, err := s.claimCiot(ctx, msg, sk, op, owner)
	if err != nil {
		return err
	}
	if done {
		slog.Info("republishing finished ciot operation", "sk", sk, "operation", op)
		return s.publishCiotResult(ctx, msg, sk, op, rowStr(row, "status"), ciotOutcome{}, row)
	}
	retry := func(motive string, cause error) error {
		if rerr := s.releaseCiot(ctx, msg, sk, owner, motive); rerr != nil {
			return errors.Join(cause, fmt.Errorf("release lease: %w", rerr))
		}
		return cause
	}
	certB64, err := s.getCertB64(ctx, msg.CertS3Key)
	if err != nil {
		return retry("failed to retrieve certificate", fmt.Errorf("getCertB64: %w", err))
	}

	// Declaration: identifier first (never fabricated; reused on a retry), then the declaration itself.
	body := msg.Body
	carrier, _ := declarationCarrier(op, body)
	instit, _ := body["payment_institution"].(bool)
	if op == opDeclare {
		code := rowStr(row, "code")
		if code == "" {
			var o ciotOutcome
			code, o, err = s.generateCiotID(ctx, msg, certB64, carrier, instit)
			if err != nil {
				return retry("identifier generator unavailable", err)
			}
			if code == "" {
				return s.settleCiot(ctx, msg, sk, op, owner, o, nil, 0, "", row, retry)
			}
			if err := s.saveCiotCode(ctx, msg, sk, owner, code); err != nil {
				return retry("failed to store identifier", err)
			}
		}
		decl, err := prepareDeclaration(body, code, time.Now())
		if err != nil {
			return retry("declaration unreadable", err)
		}
		return s.callAndSettle(ctx, msg, sk, op, owner, certB64, ciot.ServiceDeclaracao, decl, carrier, instit, decl, row, retry)
	}
	req, _ := body["request"].(map[string]any)
	if req == nil {
		return retry("operation request unreadable", errors.New("ciot message without request"))
	}
	decl, _ := body["declaration"].(map[string]any)
	return s.callAndSettle(ctx, msg, sk, op, owner, certB64, msg.SefazService, req, carrier, instit, decl, row, retry)
}

// declarationCarrier finds the carrier document the certificate must belong to.
func declarationCarrier(op string, body map[string]any) (string, bool) {
	if d, ok := body["declaration"].(map[string]any); ok {
		c, _ := d["CpfCnpjContratado"].(string)
		return c, c != ""
	}
	if r, ok := body["request"].(map[string]any); ok {
		c, _ := r["CpfCnpjContratado"].(string)
		return c, c != ""
	}
	return "", false
}

// prepareDeclaration sets the generated identifier and, outside contingency, a fresh declaration time (the ANTT
// accepts -15 min to +5 min around Brasilia time, B11, and a retried message may be old).
func prepareDeclaration(body map[string]any, code string, now time.Time) (map[string]any, error) {
	d, ok := body["declaration"].(map[string]any)
	if !ok {
		return nil, errors.New("declaration missing")
	}
	d["IdOperacaoTransporte"] = code
	if cont, _ := d["IndContingencia"].(bool); !cont {
		d["DataDeclaracao"] = ciot.FormatDateTime(now)
	}
	return d, nil
}

func (s *DfeService) generateCiotID(ctx context.Context, msg WorkerMessage, certB64, carrier string, instit bool) (string, ciotOutcome, error) {
	secrets := &ssmSecrets{client: s.ssm, env: msg.SefazEnvironment, cache: map[string]string{}}
	key, err := secrets.Get(ctx, ciot.ParamAPIKey)
	if err != nil {
		s.alerter.Alert(ctx, alerts.Alert{Job: "ciot-generator-config", Summary: "The CIOT generator API key could not be read from SSM.",
			Detail: "environment: " + msg.SefazEnvironment, Err: err})
		return "", ciotOutcome{}, err
	}
	fallback, err := secrets.Get(ctx, ciot.ParamFallbackURL)
	if err != nil {
		return "", ciotOutcome{}, err
	}
	resp, err := s.invokeEgress(ctx, lambdaPayload{
		CNPJ: msg.CNPJ, CertificateB64: certB64, CertificatePassword: msg.CertPassword, Environment: msg.SefazEnvironment,
		DocType: docTypeCIOT, Service: ciot.ServiceGerarIdentificador,
		Body: map[string]any{"cpf_cnpj": carrier, "generator_api_key": key, "generator_fallback_url": fallback,
			"carrier_cpf_cnpj": carrier, "payment_institution": instit},
	})
	if err != nil {
		return "", ciotOutcome{}, fmt.Errorf("generator call: %w", err)
	}
	if resp.StatusCode == 200 {
		var out struct {
			ID string `json:"id"`
		}
		if json.Unmarshal([]byte(resp.Body), &out) != nil || len(out.ID) != ciot.IDLen {
			return "", ciotOutcome{}, errors.New("generator answered without an identifier")
		}
		return out.ID, ciotOutcome{}, nil
	}
	o := decideCiot(opDeclare, resp.StatusCode, resp.Body)
	if !o.Retry && !o.Alert {
		return "", o, nil // a business rejection of the generator step ends the declaration
	}
	if o.Alert {
		s.alertCiot(ctx, msg, strVal(msg.EventSK), opDeclare, o)
	}
	return "", ciotOutcome{}, fmt.Errorf("generator failed: %s", o.Message)
}

func (s *DfeService) saveCiotCode(ctx context.Context, msg WorkerMessage, sk, owner, code string) error {
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.cfg.TablePrefix + "_ciots"), Key: s.ciotKey(msg, sk),
		UpdateExpression:    aws.String("SET code = :c"),
		ConditionExpression: aws.String("processing_owner = :owner"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c": &types.AttributeValueMemberS{Value: code}, ":owner": &types.AttributeValueMemberS{Value: owner},
		},
	})
	return err
}

func (s *DfeService) callAndSettle(ctx context.Context, msg WorkerMessage, sk, op, owner, certB64, service string, payload map[string]any,
	carrier string, instit bool, declaration map[string]any, row map[string]types.AttributeValue, retry func(string, error) error) error {
	sent := make(map[string]any, len(payload)+2)
	for k, v := range payload {
		sent[k] = v
	}
	sent["carrier_cpf_cnpj"], sent["payment_institution"] = carrier, instit
	resp, err := s.invokeEgress(ctx, lambdaPayload{CNPJ: msg.CNPJ, CertificateB64: certB64, CertificatePassword: msg.CertPassword,
		Environment: msg.SefazEnvironment, DocType: docTypeCIOT, Service: service, Body: sent})
	if err != nil {
		return retry("egress invocation error", fmt.Errorf("ciot call: %w", err))
	}
	o := decideCiot(op, resp.StatusCode, resp.Body)
	if o.Reconcile {
		if rec, ok := s.reconcileCiot(ctx, msg, certB64, carrier, instit, rowOrPayloadCode(row, payload)); ok {
			o, resp = rec, lambdaResponse{StatusCode: 200, Body: resp.Body}
		} else {
			return retry("reconciliation of the duplicate declaration failed", errors.New("ciot 219 could not be reconciled"))
		}
	}
	return s.settleCiot(ctx, msg, sk, op, owner, o, declaration, resp.StatusCode, resp.Body, row, retry, sent)
}

func rowOrPayloadCode(row map[string]types.AttributeValue, payload map[string]any) string {
	if c := rowStr(row, "code"); c != "" {
		return c
	}
	c, _ := payload["IdOperacaoTransporte"].(string)
	return c
}

// reconcileCiot asks ConsultarCIOTGerado for the CIOT that a lost declaration response already created (219).
func (s *DfeService) reconcileCiot(ctx context.Context, msg WorkerMessage, certB64, carrier string, instit bool, code string) (ciotOutcome, bool) {
	if len(code) != ciot.IDLen {
		return ciotOutcome{}, false
	}
	resp, err := s.invokeEgress(ctx, lambdaPayload{CNPJ: msg.CNPJ, CertificateB64: certB64, CertificatePassword: msg.CertPassword,
		Environment: msg.SefazEnvironment, DocType: docTypeCIOT, Service: ciot.ServiceCIOTGerado,
		Body: map[string]any{"CodigoIdentificacaoOperacao": code, "AnoDeclaracao": time.Now().In(ciot.Brasilia).Year(),
			"carrier_cpf_cnpj": carrier, "payment_institution": instit}})
	if err != nil || resp.StatusCode != 200 {
		return ciotOutcome{}, false
	}
	var r ciot.Result
	if json.Unmarshal([]byte(resp.Body), &r) != nil || !r.Success() {
		return ciotOutcome{}, false
	}
	ciotFull := r.CIOT
	if ciotFull == "" && len(r.IdOperacao) == ciot.IDLen && r.CodigoVerificador != "" {
		ciotFull = r.IdOperacao + r.CodigoVerificador
	}
	if ciotFull == "" {
		return ciotOutcome{}, false
	}
	o := ciotOutcome{Status: ciotDeclared, AnttCode: ciot.CodeDuplicateID, Message: "adopted existing CIOT (219)", TerminalOK: true,
		Attrs: map[string]string{"code": code, "ciot": ciotFull}}
	if r.AvisoTransportador != "" {
		o.Attrs["carrier_warning"] = r.AvisoTransportador
	}
	return o, true
}

func (s *DfeService) settleCiot(ctx context.Context, msg WorkerMessage, sk, op, owner string, o ciotOutcome, declaration map[string]any,
	egressStatus int, response string, row map[string]types.AttributeValue, retry func(string, error) error, sent ...map[string]any) error {
	if len(sent) > 0 {
		s.auditCiot(ctx, msg, sk, op, sent[0], egressStatus, response)
	}
	if o.Retry {
		if o.Alert {
			s.alertCiot(ctx, msg, sk, op, o)
		}
		return retry(o.Message, fmt.Errorf("ciot %s: %s", op, o.Message))
	}
	if err := s.finishCiot(ctx, msg, sk, owner, op, o, declaration); err != nil {
		return fmt.Errorf("finish ciot: %w", err)
	}
	status := o.Status
	if status == "" { // a rejected lifecycle operation leaves the row as it was
		status = rowStr(row, "status")
	}
	return s.publishCiotResult(ctx, msg, sk, op, status, o, row)
}

// CiotDLQ settles a CIOT message that exhausted its retries: a declaration becomes failed, a lifecycle operation
// leaves the row status and only clears the in-flight marker; both keep the reason on the row.
func CiotDLQ(ctx context.Context, dynamo interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}, tablePrefix string, msg WorkerMessage, motive string) (status string, err error) {
	sk := strVal(msg.EventSK)
	set := "SET last_motive = :m, updated_at = :ts"
	vals := map[string]types.AttributeValue{
		":m": &types.AttributeValueMemberS{Value: motive}, ":ts": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
	}
	in := &dynamodb.UpdateItemInput{
		TableName:           aws.String(tablePrefix + "_ciots"),
		Key:                 map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: msg.DocPK}, "sk": &types.AttributeValueMemberS{Value: sk}},
		ConditionExpression: aws.String("attribute_exists(pk)"),
	}
	status = "operation_failed"
	if strVal(msg.EventType) == opDeclare {
		status = ciotFailed
		set += ", #st = :st"
		vals[":st"] = &types.AttributeValueMemberS{Value: ciotFailed}
		in.ExpressionAttributeNames = map[string]string{"#st": "status"}
	}
	in.UpdateExpression = aws.String(set + " REMOVE pending_operation, processing_owner, processing_lease_until")
	in.ExpressionAttributeValues = vals
	_, err = dynamo.UpdateItem(ctx, in)
	return status, err
}
