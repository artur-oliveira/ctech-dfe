package nfes

// inutilization.go implements NF-e / NFC-e number inutilization (SEFAZ service
// NfeInutilizacao).
//
// Fiscal numbering must have no holes: every number consumed without producing
// an authorized document (definitive rejection, crash between reserving the
// number and transmitting, a series change) leaves a gap the fisco charges for.
// Inutilization is the only way to close one.
//
// Storage reuses the existing per-doc-type events tables (nfe_events /
// nfce_events) — the same tables the nfe-inutilization worker already has IAM
// access to (cdk/lib/worker-definitions.ts). Because an inutilization has no
// chave de acesso, the partition key is synthesized as
// "INUT#{env}#{org_pk}" and the GSI sort key as
// "INUT#{ano}#{serie}#{nNFIni}#{nNFFin}", which makes the whole set of an
// organization's inutilizations a single query.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	// EventTypeInutilizacao is the event_type stored for inutilization rows.
	// Inutilização is not a SEFAZ *event* (it has no tpEvento code), so it gets
	// its own marker rather than borrowing an event code.
	EventTypeInutilizacao = "INUT"

	// SefazServiceInutilizacao is the SEFAZ web service name, matching
	// go-dfe's endpoints/constants tables and the worker's SNS filter
	// (cdk/lib/worker-definitions.ts, sefazServices).
	SefazServiceInutilizacao = "NfeInutilizacao"

	inutRootTag      = "inutNFe"
	inutVersao       = "4.00"
	inutXServ        = "INUTILIZAR"
	inutJustificaMin = 15

	// StatusInutilized is the status the worker writes once SEFAZ homologates
	// the range (cStat 102). It is the worker's generic event-success status
	// (worker/internal/service/helpers.go, EventStatusSuccess) — the rows live
	// in the events tables and go through the same update path.
	StatusInutilized = "success"
)

// InutilizationBody is the payload for POST /nfes/inutilizations and
// POST /nfces/inutilizations.
//
// Year is optional: SEFAZ scopes an inutilization to a two-digit year and it is
// virtually always the current one, so it is derived rather than asked for.
type InutilizationBody struct {
	Serie         int    `json:"serie" validate:"gte=0,lte=999"`
	NumberStart   int    `json:"number_start" validate:"required,gte=1,lte=999999999"`
	NumberEnd     int    `json:"number_end" validate:"required,gte=1,lte=999999999"`
	Justification string `json:"justification" validate:"required,min=15,max=255"`
	Year          *int   `json:"year" validate:"omitempty,gte=2006,lte=2100"`
}

// inutDeps bundles what an inutilization needs from either document service, so
// NF-e and NFC-e share one implementation instead of two copies.
type inutDeps struct {
	orgRepo   *repositories.OrganizationRepository
	certRepo  *repositories.CertificateRepository
	docRepo   *repositories.DocumentRepository
	eventRepo *repositories.DocumentEventRepository
	workerSvc *services.WorkerService

	docType     string // "nfe" | "nfce"
	model       string // nfModel55 | nfModel65
	table       string // "nfes" | "nfces"
	s3Prefix    string // "nfe" | "nfce"
	eventsTable string // "nfe_events" | "nfce_events"
	environment int    // 1 = prod, 2 = hom
}

func (d inutDeps) envPrefix() string { return envToPrefix(d.environment) }

func (d inutDeps) partitionKey(orgPK string) string {
	return fmt.Sprintf("%s#%s", d.envPrefix(), orgPK)
}

// inutEventPK is the synthetic events-table partition key for an organization's
// inutilizations in one environment.
func inutEventPK(envPrefix, orgPK string) string {
	return fmt.Sprintf("%s#%s#%s", EventTypeInutilizacao, envPrefix, orgPK)
}

// inutEventKey is the GSI sort key — one per (year, series, range), which also
// makes a repeated request for the same range detectable.
func inutEventKey(year, serie, start, end int) string {
	return fmt.Sprintf("%s#%04d#%03d#%09d#%09d", EventTypeInutilizacao, year, serie, start, end)
}

// buildInutBody produces the inutNFe payload sent to SEFAZ.
//
// The Id is "ID" + cUF(2) + ano(2) + CNPJ(14) + mod(2) + serie(3) + nNFIni(9) +
// nNFFin(9). infInut carries CNPJ only — the layout has no CPF choice, so a
// natural-person issuer cannot inutilize (a SEFAZ layout limit, not ours).
func buildInutBody(cUF, cnpj, model string, environment, year, serie, start, end int, justification string) map[string]any {
	id := fmt.Sprintf("ID%s%02d%s%s%03d%09d%09d",
		cUF, year%100, cnpj, model, serie, start, end)
	return map[string]any{
		inutRootTag: map[string]any{
			"@versao": inutVersao,
			"@xmlns":  nfeXMLNS,
			"infInut": map[string]any{
				"@Id":    id,
				"tpAmb":  strconv.Itoa(environment),
				"xServ":  inutXServ,
				"cUF":    cUF,
				"ano":    fmt.Sprintf("%02d", year%100),
				"CNPJ":   cnpj,
				"mod":    model,
				"serie":  strconv.Itoa(serie),
				"nNFIni": strconv.Itoa(start),
				"nNFFin": strconv.Itoa(end),
				"xJust":  justification,
			},
		},
	}
}

// inutilize validates the range, records the request and dispatches it to the
// SEFAZ worker.
func inutilize(ctx context.Context, d inutDeps, orgPK string, body InutilizationBody, userID, userName string) (map[string]types.AttributeValue, error) {
	if body.NumberEnd < body.NumberStart {
		return nil, problem.BadRequest("o número final deve ser maior ou igual ao número inicial")
	}
	if len([]rune(strings.TrimSpace(body.Justification))) < inutJustificaMin {
		return nil, problem.BadRequest(fmt.Sprintf("a justificativa deve ter ao menos %d caracteres", inutJustificaMin))
	}

	year := time.Now().UTC().Year()
	if body.Year != nil {
		year = *body.Year
	}

	org, err := d.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, problem.NotFound("organização não encontrada")
	}
	certs, err := d.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, problem.NoCertificate("certificado digital não encontrado")
	}

	emitUF := extractEmitUF(org)
	cUF, ok := services.UFCode[emitUF]
	if !ok {
		return nil, problem.BadRequest("UF do emitente inválida ou não configurada")
	}
	cnpj := services.StripPKPrefix(orgPK)
	if strings.HasPrefix(orgPK, services.TagCPF+"_") {
		return nil, problem.BadRequest("o layout de inutilização da SEFAZ aceita apenas emitente pessoa jurídica (CNPJ)")
	}

	// A number that produced an authorized document can never be inutilized.
	used, err := usedNumbersInRange(ctx, d.docRepo, d.partitionKey(orgPK), body.Serie, body.NumberStart, body.NumberEnd)
	if err != nil {
		return nil, err
	}
	if len(used) > 0 {
		return nil, problem.BadRequest(fmt.Sprintf(
			"a faixa contém documentos já emitidos (números %s) — inutilize apenas números não utilizados",
			joinInts(used)))
	}

	pk := inutEventPK(d.envPrefix(), orgPK)
	item, err := d.eventRepo.CreateInutilization(ctx, repositories.InutilizationRecord{
		PK:            pk,
		EventKey:      inutEventKey(year, body.Serie, body.NumberStart, body.NumberEnd),
		Year:          year,
		Serie:         body.Serie,
		NumberStart:   body.NumberStart,
		NumberEnd:     body.NumberEnd,
		Justification: body.Justification,
		Status:        StatusPending,
		UserID:        userID,
		UserName:      userName,
	})
	if err != nil {
		return nil, err
	}

	sefazEnv := SefazEnvHom
	if d.environment == 1 {
		sefazEnv = SefazEnvProd
	}
	seq := 1
	eventSK := strAttr(item, "sk")
	cert := certs[0]
	if err := d.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: d.partitionKey(orgPK), AccessKey: pk,
		TableName: d.table, S3Prefix: d.s3Prefix,
		ExpectedFileName: fmt.Sprintf("%s_%04d_%03d_%09d_%09d",
			strings.ToLower(EventTypeInutilizacao), year, body.Serie, body.NumberStart, body.NumberEnd),
		CNPJ: cnpj, UF: emitUF,
		SefazEnvironment: sefazEnv,
		CertS3Key:        strAttr(cert, "s3_key"), CertPassword: strAttr(cert, "password"),
		DocType: d.docType, SefazService: SefazServiceInutilizacao,
		Body: buildInutBody(cUF, cnpj, d.model, d.environment, year,
			body.Serie, body.NumberStart, body.NumberEnd, body.Justification),
		EventsTableName: aws.String(d.eventsTable),
		EventType:       aws.String(EventTypeInutilizacao),
		SequenceNumber:  &seq,
		EventSK:         aws.String(eventSK),
	}); err != nil {
		return nil, err
	}

	return item, nil
}

// usedNumbersInRange returns the document numbers inside [start,end] of the
// given series that already exist and are not failed/rejected — those are the
// numbers that must never be inutilized.
func usedNumbersInRange(ctx context.Context, repo *repositories.DocumentRepository, pk string, serie, start, end int) ([]int, error) {
	items, err := repo.NumbersInRange(ctx, pk, start, end)
	if err != nil {
		return nil, err
	}
	var used []int
	for _, it := range items {
		if intAttr(it, "serie", -1) != serie {
			continue
		}
		if inutilizableStatuses[strAttr(it, "status")] {
			continue
		}
		used = append(used, intAttr(it, "number", 0))
	}
	return used, nil
}

// inutilizableStatuses are the document statuses that leave the number free:
// the document never reached SEFAZ authorization and never will.
var inutilizableStatuses = map[string]bool{
	"rejected": true,
	"failed":   true,
}

// NumberGap is a contiguous run of unused numbers in one series.
type NumberGap struct {
	Serie       int `json:"serie"`
	NumberStart int `json:"number_start"`
	NumberEnd   int `json:"number_end"`
}

// detectGaps scans the issued numbers of a series up to the current counter and
// returns the runs that produced no usable document. This is what turns the
// inutilization screen into a tool instead of a blank form.
func detectGaps(ctx context.Context, repo *repositories.DocumentRepository, pk string, serie, currentNumber int, covered []NumberGap) ([]NumberGap, error) {
	if currentNumber < 1 {
		return []NumberGap{}, nil
	}
	items, err := repo.NumbersInRange(ctx, pk, 1, currentNumber)
	if err != nil {
		return nil, err
	}
	usable := make(map[int]bool, len(items))
	for _, it := range items {
		if intAttr(it, "serie", -1) != serie {
			continue
		}
		if inutilizableStatuses[strAttr(it, "status")] {
			continue
		}
		usable[intAttr(it, "number", 0)] = true
	}
	// A number already inutilized is closed, not a gap.
	for _, c := range covered {
		if c.Serie != serie {
			continue
		}
		for n := c.NumberStart; n <= c.NumberEnd && n <= currentNumber; n++ {
			usable[n] = true
		}
	}

	return gapRuns(usable, serie, currentNumber), nil
}

// gapRuns turns the set of usable numbers into the contiguous runs that are not
// usable — the pure core of gap detection.
func gapRuns(usable map[int]bool, serie, currentNumber int) []NumberGap {
	// gaps começa vazia, não nil: a resposta JSON precisa ser `[]`, não `null`.
	var gaps []NumberGap
	runStart := 0
	// currentNumber é o PRÓXIMO número a ser emitido, não o último emitido
	// (TransactReserveAndCreate usa currentNumber na nota e grava
	// currentNumber+1). Ele ainda pode virar documento, então nunca é lacuna:
	// a varredura para antes dele.
	for n := 1; n < currentNumber; n++ {
		if usable[n] {
			if runStart != 0 {
				gaps = append(gaps, NumberGap{Serie: serie, NumberStart: runStart, NumberEnd: n - 1})
				runStart = 0
			}
			continue
		}
		if runStart == 0 {
			runStart = n
		}
	}
	if runStart != 0 {
		gaps = append(gaps, NumberGap{Serie: serie, NumberStart: runStart, NumberEnd: currentNumber - 1})
	}
	if gaps == nil {
		return []NumberGap{}
	}
	return gaps
}

func joinInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

// ── Service entry points ─────────────────────────────────────────────────────

func (s *NfeService) inutDeps(environment int) inutDeps {
	return inutDeps{
		orgRepo: s.orgRepo, certRepo: s.certRepo,
		docRepo: &s.nfeRepo.DocumentRepository, eventRepo: s.eventRepo,
		workerSvc:   s.workerSvc,
		docType:     services.DocTypeNFe,
		model:       nfModel55,
		table:       "nfes",
		s3Prefix:    "nfe",
		eventsTable: "nfe_events",
		environment: environment,
	}
}

// Inutilize requests the inutilization of an unused NF-e number range.
func (s *NfeService) Inutilize(ctx context.Context, orgPK string, body InutilizationBody, userID, userName string) (map[string]types.AttributeValue, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return inutilize(ctx, s.inutDeps(env), orgPK, body, userID, userName)
}

// ListInutilizations returns the organization's NF-e inutilizations.
func (s *NfeService) ListInutilizations(ctx context.Context, orgPK string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return s.eventRepo.ListInutilizations(ctx, inutEventPK(envToPrefix(env), orgPK), limit, startKey)
}

// NumberGaps returns the NF-e numbering holes still open in the current series.
func (s *NfeService) NumberGaps(ctx context.Context, orgPK string) ([]NumberGap, error) {
	cfg, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, problem.BadRequest("configure a NF-e em Configuração Fiscal antes de usar este recurso")
	}
	return numberGaps(ctx, s.inutDeps(intAttr(cfg, "environment", 2)), s.eventRepo, orgPK, cfg)
}

func (s *NfceService) inutDeps(environment int) inutDeps {
	return inutDeps{
		orgRepo: s.orgRepo, certRepo: s.certRepo,
		docRepo: &s.nfceRepo.DocumentRepository, eventRepo: s.eventRepo,
		workerSvc:   s.workerSvc,
		docType:     services.DocTypeNFCe,
		model:       nfModel65,
		table:       "nfces",
		s3Prefix:    "nfce",
		eventsTable: "nfce_events",
		environment: environment,
	}
}

// Inutilize requests the inutilization of an unused NFC-e number range.
func (s *NfceService) Inutilize(ctx context.Context, orgPK string, body InutilizationBody, userID, userName string) (map[string]types.AttributeValue, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return inutilize(ctx, s.inutDeps(env), orgPK, body, userID, userName)
}

// ListInutilizations returns the organization's NFC-e inutilizations.
func (s *NfceService) ListInutilizations(ctx context.Context, orgPK string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return s.eventRepo.ListInutilizations(ctx, inutEventPK(envToPrefix(env), orgPK), limit, startKey)
}

// NumberGaps returns the NFC-e numbering holes still open in the current series.
func (s *NfceService) NumberGaps(ctx context.Context, orgPK string) ([]NumberGap, error) {
	cfg, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, problem.BadRequest("configure a NFC-e em Configuração Fiscal antes de usar este recurso")
	}
	return numberGaps(ctx, s.inutDeps(intAttr(cfg, "environment", 2)), s.eventRepo, orgPK, cfg)
}

// inutilizationXML downloads the stored ProcInutNFe of one inutilization.
func inutilizationXML(ctx context.Context, d inutDeps, clients *awsclient.Clients, bucket, orgPK, sk string) ([]byte, error) {
	item, err := d.eventRepo.GetEvent(ctx, inutEventPK(d.envPrefix(), orgPK), sk)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("inutilização não encontrada")
	}
	s3Key := strAttr(item, "xml_s3_key")
	if s3Key == "" {
		return nil, problem.NotFound("XML da inutilização ainda não disponível")
	}
	return services.DownloadS3(ctx, clients, bucket, s3Key)
}

// GetInutilizationXML returns the ProcInutNFe of an NF-e inutilization.
func (s *NfeService) GetInutilizationXML(ctx context.Context, orgPK, sk string) ([]byte, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return inutilizationXML(ctx, s.inutDeps(env), s.clients, s.bucketDocs, orgPK, sk)
}

// GetInutilizationXML returns the ProcInutNFe of an NFC-e inutilization.
func (s *NfceService) GetInutilizationXML(ctx context.Context, orgPK, sk string) ([]byte, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return inutilizationXML(ctx, s.inutDeps(env), s.clients, s.bucketDocs, orgPK, sk)
}

// numberGaps resolves the active series/counter from the fiscal config and runs
// gap detection, excluding ranges already inutilized.
func numberGaps(ctx context.Context, d inutDeps, eventRepo *repositories.DocumentEventRepository, orgPK string, cfg map[string]types.AttributeValue) ([]NumberGap, error) {
	envPrefix := d.envPrefix()
	serie := intAttr(cfg, envPrefix+"_current_serie", 1)
	currentNumber := intAttr(cfg, envPrefix+"_current_number", 0)

	res, err := eventRepo.ListInutilizations(ctx, inutEventPK(envPrefix, orgPK), 0, nil)
	if err != nil {
		return nil, err
	}
	covered := make([]NumberGap, 0, len(res.Items))
	for _, it := range res.Items {
		if strAttr(it, "status") != StatusInutilized {
			continue
		}
		covered = append(covered, NumberGap{
			Serie:       intAttr(it, "serie", -1),
			NumberStart: intAttr(it, "number_start", 0),
			NumberEnd:   intAttr(it, "number_end", 0),
		})
	}
	return detectGaps(ctx, d.docRepo, d.partitionKey(orgPK), serie, currentNumber, covered)
}
