// Package ciots implements the ANTT CIOT module of the API: the row, the pre-flight (certificate, local rules,
// lifecycle deadlines), the outbox that feeds the ciot-operation worker, and the synchronous carrier queries
// through the go-dfe-egress. Contract: docs/specs/2026-11-04-ciot-antt-pef-integracao.md.
package ciots

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/api/internal/dfeproblem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	godfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/ciot"
)

// Row statuses (the worker moves them; PendingOperation says what is in flight).
const (
	StatusPending   = "pending" // declaration queued, nothing confirmed yet
	StatusDeclared  = "declared"
	StatusRectified = "rectified"
	StatusCancelled = "cancelled"
	StatusClosed    = "closed"
	StatusRejected  = "rejected"

	DocType      = "ciot"
	opDeclare    = "declare"
	attrPending  = "pending_operation"
	attrDeclJSON = "declaration_json"

	// CarrierStatusTTL is how long ConsultarSituacaoTransportador answers are cached (spec section 9).
	CarrierStatusTTL = time.Hour
	maxFleetPlates   = 7 // per call [pratica]
	queryRetries     = ciot.QueryRetries

	CodeValidation  = "ciot_validation"
	CodeCertificate = "ciot_certificate"
	CodeNotFound    = "ciot_not_found"
	CodeState       = "ciot_invalid_state"
	CodeFloor       = "ciot_floor_unverified"
)

var (
	// open states accept cancel, rectify and close.
	openStates = []string{StatusDeclared, StatusRectified}
	// dateRulesSkippedOnRectify are the declaration-date windows: a rectification happens during the trip, so
	// they would reject a valid correction; the ANTT remains the judge.
	dateRulesSkippedOnRectify = map[string]bool{"B6": true, "B11": true, "B96": true, "B19": true}
)

// operationService maps the lifecycle operations to the ANTT service names.
var operationService = map[string]string{
	opDeclare:      ciot.ServiceDeclaracao,
	ciot.OpCancel:  ciot.ServiceCancelamento,
	ciot.OpRectify: ciot.ServiceRetificacao,
	ciot.OpClose:   ciot.ServiceEncerramento,
}

// dfeCaller is what the service uses of ExternalService; tests replace it with a fake (never the real ANTT).
type dfeCaller interface {
	CertificateB64(ctx context.Context, orgPK string) (string, string, error)
	CallDfe(ctx context.Context, req godfe.Request) (godfe.Response, error)
}

type outbox interface {
	BuildOutboxTx(msg services.WorkerMessage) (types.TransactWriteItem, string, error)
}

type Service struct {
	orgRepo  *repositories.OrganizationRepository
	certRepo *repositories.CertificateRepository
	repo     *repositories.CiotRepository
	audit    *repositories.CiotAuditRepository
	worker   outbox
	egress   dfeCaller
	now      func() time.Time
}

func NewService(orgRepo *repositories.OrganizationRepository, certRepo *repositories.CertificateRepository,
	repo *repositories.CiotRepository, audit *repositories.CiotAuditRepository,
	worker *services.WorkerService, ext *services.ExternalService) *Service {
	return &Service{orgRepo: orgRepo, certRepo: certRepo, repo: repo, audit: audit, worker: worker, egress: ext, now: time.Now}
}

// --- pre-flight ---

func violationsProblem(detail string, vs []ciot.Violation) error {
	errs := make([]problem.FieldError, 0, len(vs))
	for _, v := range vs {
		errs = append(errs, problem.Field(v.Field, v.Internal, v.Message, "rule", v.Rule, "antt_code", v.ANTTCode))
	}
	return problem.UnprocessableEntityFields(detail, errs...).WithCode(CodeValidation)
}

// preflightCertificate runs ciot.CheckCertificate on the organization certificate before anything is queued.
func (s *Service) preflightCertificate(ctx context.Context, orgPK, carrierDoc string, paymentInstitution bool) error {
	b64, pwd, err := s.egress.CertificateB64(ctx, orgPK)
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return problem.InternalServer("certificado em formato inesperado")
	}
	cert, _, _, err := services.ParsePFX(raw, pwd)
	if err != nil {
		return problem.UnprocessableEntity("certificado digital inválido para o CIOT").WithCode(CodeCertificate)
	}
	if err := ciot.CheckCertificate(cert, carrierDoc, paymentInstitution, s.now()); err != nil {
		return problem.UnprocessableEntity(err.Error()).WithCode(CodeCertificate)
	}
	return nil
}

func (s *Service) validateDeclaration(d *ciot.Declaracao, skipDateRules bool) error {
	vs := ciot.Validate(d, ciot.Context{Now: s.now()})
	if skipDateRules {
		kept := vs[:0]
		for _, v := range vs {
			if !dateRulesSkippedOnRectify[v.Rule] {
				kept = append(kept, v)
			}
		}
		vs = kept
	}
	if len(vs) > 0 {
		return violationsProblem("declaração do CIOT inválida", vs)
	}
	return nil
}

// --- context ---

type orgContext struct {
	doc       string
	certS3Key string
	certPass  string
}

func (s *Service) orgContext(ctx context.Context, orgPK string) (*orgContext, error) {
	org, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, problem.NotFound("organização não encontrada")
	}
	doc, _ := services.IssuerDocAV(org, orgPK)
	if doc == "" {
		return nil, problem.BadRequest("documento da organização não encontrado")
	}
	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, dfeproblem.NoCertificate("certificado digital não encontrado")
	}
	return &orgContext{doc: doc, certS3Key: avStr(certs[0], "s3_key"), certPass: avStr(certs[0], "password")}, nil
}

func avStr(m map[string]types.AttributeValue, k string) string {
	if v, ok := m[k].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func avInt(m map[string]types.AttributeValue, k string) int {
	var n int
	if v, ok := m[k].(*types.AttributeValueMemberN); ok {
		_, _ = fmt.Sscanf(v.Value, "%d", &n)
	}
	return n
}

func hashHex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// --- declare ---

// Declare validates, then records the row, the audit request and the outbox command in one transaction. The
// 12-character identifier is requested from the generator by the worker, never produced here.
func (s *Service) Declare(ctx context.Context, orgPK string, body DeclareBody, userID, userName string) (map[string]types.AttributeValue, error) {
	now := s.now()
	d := body.toDeclaracao("", now)
	// The generator fills IdOperacaoTransporte; the rule catalog needs a placeholder of the right shape.
	check := d
	check.IdOperacaoTransporte = strings.Repeat("0", ciot.IDLen)
	if err := s.validateDeclaration(&check, false); err != nil {
		return nil, err
	}
	if err := s.preflightCertificate(ctx, orgPK, d.CpfCnpjContratado, body.PaymentInstitution); err != nil {
		return nil, err
	}
	oc, err := s.orgContext(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	declJSON, err := json.Marshal(d)
	if err != nil {
		return nil, problem.InternalServer("falha ao serializar o CIOT")
	}
	id := repositories.GenerateID()
	sk := repositories.CiotSK(id)
	pk := body.Environment + "#" + orgPK
	row := map[string]any{
		"pk": pk, "sk": sk, "id": id, "status": StatusPending, attrPending: opDeclare, "environment": body.Environment,
		"operation_type": d.TipoOperacao, "carrier_doc": d.CpfCnpjContratado, "hirer_doc": d.CpfCnpjContratante,
		"freight_value": d.ValorFrete, "declared_at": d.DataDeclaracao, "trip_start": d.DataInicioViagem,
		"trip_end": d.DataFimViagem, attrDeclJSON: string(declJSON), "payment_institution": body.PaymentInstitution,
		"history": []any{}, "created_by": userID, "created_by_name": userName,
		"created_at": repositories.NowStr(), "updated_at": repositories.NowStr(),
	}
	if body.MdfeAccessKey != nil {
		row[repositories.AttrCiotMdfeKey] = *body.MdfeAccessKey
	}
	item, err := repositories.EncodeItem(row)
	if err != nil {
		return nil, problem.InternalServer("falha ao codificar o CIOT")
	}
	msg := s.message(pk, sk, sk, opDeclare, body.Environment, oc, map[string]any{
		"declaration": d, "payment_institution": body.PaymentInstitution,
	})
	if err := s.commit(ctx, item, nil, sk, opDeclare, declJSON, msg); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) message(pk, sk, opKey, op, env string, oc *orgContext, payload map[string]any) services.WorkerMessage {
	return services.WorkerMessage{
		DocPK: pk, AccessKey: opKey, TableName: repositories.TableCiots, S3Prefix: DocType,
		CNPJ: oc.doc, SefazEnvironment: env, CertS3Key: oc.certS3Key, CertPassword: oc.certPass,
		DocType: DocType, SefazService: operationService[op], Body: payload, EventType: &op, EventSK: &sk,
	}
}

// commit writes (create row | queue the in-flight marker) + audit request + outbox atomically.
func (s *Service) commit(ctx context.Context, newItem map[string]types.AttributeValue, marker *types.TransactWriteItem,
	sk, op string, body []byte, msg services.WorkerMessage) error {
	outboxTx, _, err := s.worker.BuildOutboxTx(msg)
	if err != nil {
		return err
	}
	var items []types.TransactWriteItem
	if newItem != nil {
		items = append(items, s.repo.CreateTx(newItem))
	} else {
		items = append(items, *marker)
	}
	items = append(items, s.audit.AppendTx(sk, "request:"+op, hashHex(body), string(body), s.now()), outboxTx)
	if err := s.repo.TransactWrite(ctx, items); err != nil {
		if repositories.IsConditionFailed(err) {
			return problem.Conflict("o CIOT mudou de estado ou já tem uma operação em andamento").WithCode(CodeState)
		}
		return err
	}
	return nil
}

// --- reads ---

// Get finds a CIOT of the organization in either environment. Another organization's id is simply absent (404).
func (s *Service) Get(ctx context.Context, orgPK, id string) (map[string]types.AttributeValue, error) {
	for _, env := range []string{services.EnvProd, services.EnvHom} {
		item, err := s.repo.Get(ctx, env+"#"+orgPK, id)
		if err != nil {
			return nil, err
		}
		if item != nil {
			return item, nil
		}
	}
	return nil, problem.NotFound("CIOT não encontrado").WithCode(CodeNotFound)
}

// List pages one environment's CIOTs, newest first.
func (s *Service) List(ctx context.Context, orgPK, env string, limit int, start map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	return s.repo.List(ctx, env+"#"+orgPK, limit, start)
}

// --- lifecycle operations ---

func (s *Service) open(ctx context.Context, orgPK, id string) (map[string]types.AttributeValue, error) {
	row, err := s.Get(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	if st := avStr(row, "status"); st != StatusDeclared && st != StatusRectified {
		return nil, problem.Conflict("o CIOT não está em um estado que aceita esta operação: " + st).WithCode(CodeState)
	}
	if avStr(row, attrPending) != "" {
		return nil, problem.Conflict("o CIOT já tem uma operação em andamento").WithCode(CodeState)
	}
	return row, nil
}

func lifecycleContext(row map[string]types.AttributeValue, op string, now time.Time) ciot.LifecycleContext {
	lc := ciot.LifecycleContext{Op: op, Tipo: avInt(row, "operation_type"), Now: now}
	lc.Start = ciot.TripStart(avStr(row, "trip_start"), avStr(row, "declared_at"))
	lc.End, _ = ciot.ParseDate(avStr(row, "trip_end"))
	return lc
}

func (s *Service) queueOp(ctx context.Context, orgPK string, row map[string]types.AttributeValue, op string, payload map[string]any, body []byte) (map[string]types.AttributeValue, error) {
	oc, err := s.orgContext(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	pk, sk := avStr(row, "pk"), avStr(row, "sk")
	id := avStr(row, "id")
	env := avStr(row, "environment")
	marker := s.repo.QueueOperationTx(pk, id, op, openStates)
	opKey := fmt.Sprintf("%s#%s#%d", sk, op, s.now().UnixNano())
	if err := s.commit(ctx, nil, &marker, sk, op, body, s.message(pk, sk, opKey, op, env, oc, payload)); err != nil {
		return nil, err
	}
	row[attrPending] = &types.AttributeValueMemberS{Value: op}
	return row, nil
}

func (s *Service) Cancel(ctx context.Context, orgPK, id string, body CancelBody) (map[string]types.AttributeValue, error) {
	row, err := s.open(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	if vs := ciot.ValidateLifecycle(lifecycleContext(row, ciot.OpCancel, s.now())); len(vs) > 0 {
		return nil, violationsProblem("cancelamento do CIOT fora do prazo ou proibido", vs)
	}
	if err := s.preflightCertificate(ctx, orgPK, avStr(row, "carrier_doc"), boolAttr(row, "payment_institution")); err != nil {
		return nil, err
	}
	req := ciot.Cancelamento{CodigoIdentificacaoOperacao: avStr(row, "ciot"), MotivoCancelamento: strings.TrimSpace(body.Reason)}
	raw, _ := json.Marshal(req)
	return s.queueOp(ctx, orgPK, row, ciot.OpCancel, map[string]any{"request": req}, raw)
}

func (s *Service) Rectify(ctx context.Context, orgPK, id string, body RectifyBody) (map[string]types.AttributeValue, error) {
	row, err := s.open(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	if vs := ciot.ValidateLifecycle(lifecycleContext(row, ciot.OpRectify, s.now())); len(vs) > 0 {
		return nil, violationsProblem("retificação do CIOT fora do prazo ou proibida", vs)
	}
	d := body.toDeclaracao(avStr(row, "code"), s.now())
	if err := s.validateDeclaration(&d, true); err != nil {
		return nil, err
	}
	if err := s.preflightCertificate(ctx, orgPK, d.CpfCnpjContratado, boolAttr(row, "payment_institution")); err != nil {
		return nil, err
	}
	req := ciot.Retificacao{CodigoIdentificacaoOperacao: avStr(row, "ciot"), Declaracao: d}
	raw, _ := json.Marshal(req)
	return s.queueOp(ctx, orgPK, row, ciot.OpRectify, map[string]any{"request": req, "declaration": d}, raw)
}

func (s *Service) Close(ctx context.Context, orgPK, id string, body CloseBody) (map[string]types.AttributeValue, error) {
	row, err := s.open(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	lc := lifecycleContext(row, ciot.OpClose, s.now())
	lc.PesoCarga, lc.Viagens = body.CargoWeight, body.TripCount
	if body.ClosedAt != nil {
		lc.CloseDate, _ = ciot.ParseDate(*body.ClosedAt)
	}
	if vs := ciot.ValidateLifecycle(lc); len(vs) > 0 {
		return nil, violationsProblem("encerramento do CIOT inválido", vs)
	}
	if err := s.preflightCertificate(ctx, orgPK, avStr(row, "carrier_doc"), boolAttr(row, "payment_institution")); err != nil {
		return nil, err
	}
	req := ciot.Encerramento{CodigoIdentificacaoOperacao: avStr(row, "ciot"), PesoCarga: body.CargoWeight,
		OrigemDestino: routesToANTT(body.Routes), QtdViagens: body.TripCount}
	if body.ClosedAt != nil {
		req.DataEncerramento = *body.ClosedAt
	}
	raw, _ := json.Marshal(req)
	return s.queueOp(ctx, orgPK, row, ciot.OpClose, map[string]any{"request": req}, raw)
}

func boolAttr(m map[string]types.AttributeValue, k string) bool {
	v, ok := m[k].(*types.AttributeValueMemberBOOL)
	return ok && v.Value
}
