package nfes

// epec_emit.go: NF-e EPEC emission (tpEmis 4; docs/specs/2026-11-08-contingencia.md section 9).
//
// Flow: the NF-e is built with tpEmis 4 (dhCont, xJust), signed by the egress without transmitting
// (SignOnly), and the EPEC event (110140, Ambiente Nacional) is queued through the outbox in the same
// transaction as the document. The document is stored with its signed XML but NOT armed: only when the
// worker sees the event registered (success, or duplicate 573) it sets contingency_sweep, and then the
// retransmission job sends the signed NF-e through the normal authorization.

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	// cUF of the UFs that refuse the EPEC with rejection 121 (MOC EPEC rules as cited in the F9 report;
	// the rejection text itself was not re-read from the MOC page: unverified).
	cUFParana  = "41"
	cUFParaiba = "25"

	// epecMaxIssueAge: rejection 228, dhEmi more than one day older than the EPEC registration.
	epecMaxIssueAge = 24 * time.Hour

	epecEventSeq = 1
	// epecEventUF routes the event to the Ambiente Nacional (cOrgao 91), as the manifestation does.
	epecEventUF = "AN"

	// CodeEPECUFRefused / CodeEPECStale are the problem codes of the local EPEC pre-validation.
	CodeEPECUFRefused = "epec_uf_refused"
	CodeEPECStale     = "epec_dhemi_stale"
)

var epecRefusedCUF = map[string]bool{cUFParana: true, cUFParaiba: true}

// ValidateAt adds the deterministic MOC rules to Validate: PR and PB are refused (121) and dhEmi cannot be
// more than one day old (228). Rule 142 (EPEC environment blocked for the issuer) depends on the SEFAZ
// state and is only known from the event rejection.
func (e EPECSummary) ValidateAt(now time.Time) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if epecRefusedCUF[e.AccessKey[:2]] {
		return problem.UnprocessableEntity("a UF do emitente não aceita EPEC (rejeição 121)").WithCode(CodeEPECUFRefused)
	}
	if now.Sub(e.IssuedAt) > epecMaxIssueAge {
		return problem.UnprocessableEntity("EPEC exige dhEmi com no máximo 1 dia (rejeição 228)").WithCode(CodeEPECStale)
	}
	return nil
}

// epecSummaryOf reads the EPEC summary off the built enviNFe (the same map that gets signed), so the event
// can never disagree with the note.
func epecSummaryOf(enviNFe map[string]any, accessKey, verAplic string) (EPECSummary, error) {
	child := func(m map[string]any, k string) map[string]any { c, _ := m[k].(map[string]any); return c }
	inf := child(child(child(enviNFe, "enviNFe"), "NFe"), "infNFe")
	ide, emit, dest := child(inf, "ide"), child(inf, "emit"), child(inf, "dest")
	tot := child(child(inf, "total"), "ICMSTot")
	issued, err := time.Parse(time.RFC3339, anyStr(ide, "dhEmi", ""))
	if err != nil {
		return EPECSummary{}, problem.InternalServer("dhEmi inválido no EPEC")
	}
	s := EPECSummary{
		AccessKey: accessKey, IssuedAt: issued, TpNF: anyStr(ide, "tpNF", ""), IE: anyStr(emit, "IE", ""),
		DestUF: anyStr(child(dest, "enderDest"), "UF", ""), DestIE: anyStr(dest, "IE", ""),
		VNF: anyStr(tot, "vNF", ""), VICMS: anyStr(tot, "vICMS", "0.00"), VST: anyStr(tot, "vST", "0.00"),
		VerAplic: verAplic,
	}
	for _, tag := range []string{"CNPJ", "CPF", "idEstrangeiro"} {
		if v := anyStr(dest, tag, ""); v != "" {
			s.DestTag, s.DestDoc = tag, v
		}
	}
	return s, nil
}

// epecStage is what the emission commits together with the document.
type epecStage struct {
	Tx          []types.TransactWriteItem // outbox (EPEC event message) and the event row
	OperationID string
}

// stageEPEC runs the EPEC steps that precede the document write: pre-validate, sign only, and stage the
// event. It marks item (the NF-e record) awaiting the event, with the signed XML and the resend message
// (authMsg, the normal authorization message). Nothing reaches SEFAZ here.
func (s *NfeService) stageEPEC(ctx context.Context, item map[string]types.AttributeValue, enviNFe map[string]any,
	authMsg services.WorkerMessage, orgPK string, environment int, userID, userName string, now time.Time) (*epecStage, error) {
	sum, err := epecSummaryOf(enviNFe, authMsg.AccessKey, s.tech.Version)
	if err != nil {
		return nil, err
	}
	if err := sum.ValidateAt(now); err != nil {
		return nil, err
	}
	signed, err := services.SignOnlyViaEgress(ctx, s.egressCaller(), orgPK, authMsg.CNPJ, authMsg.UF, authMsg.SefazEnvironment, "nfe", enviNFe)
	if err != nil {
		return nil, err
	}
	if err := services.MarkContingencyAwaitingEvent(item, authMsg, signed); err != nil {
		return nil, err
	}
	docTag := services.TagCNPJ
	if len(authMsg.CNPJ) == cpfDigits {
		docTag = services.TagCPF
	}
	eventPut, eventSK := s.eventRepo.EventPutTx(authMsg.AccessKey, TpEventoEPEC, epecEventSeq, StatusPending, userID, userName)
	ev := authMsg // billing, certificate and document coordinates are the document's own
	ev.UF, ev.SefazService, ev.Contingency = epecEventUF, "RecepcaoEvento", ""
	ev.ExpectedFileName = authMsg.AccessKey + "_" + TpEventoEPEC + "_001"
	ev.Body = shadowNFeEvent(authMsg.AccessKey, buildEPECBody(docTag, authMsg.CNPJ, environment, sum, epecEventSeq))
	ev.EventsTableName, ev.EventType, ev.EventSK = aws.String("nfe_events"), aws.String(TpEventoEPEC), aws.String(eventSK)
	seq := epecEventSeq
	ev.SequenceNumber = &seq
	outboxTx, opID, err := s.workerSvc.BuildOutboxTx(ev)
	if err != nil {
		return nil, err
	}
	return &epecStage{Tx: []types.TransactWriteItem{outboxTx, eventPut}, OperationID: opID}, nil
}

// egressCaller is the egress the NF-e uses for local operations: the test fake, or the external service.
func (s *NfeService) egressCaller() services.EgressCaller {
	if s.egress != nil {
		return s.egress
	}
	if s.extSvc != nil {
		return s.extSvc
	}
	return nil
}

// WithEgress sets the egress used by the EPEC flow (sign-only); tests inject a fake.
func (s *NfeService) WithEgress(e services.EgressCaller) *NfeService {
	s.egress = e
	return s
}
