package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	mathrand "math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"

	"github.com/artur-oliveira/ctech-dfe/worker/internal/config"
)

const (
	cStatNoDocs     = "137"
	cStatDocFound   = "138"
	cStatMaxNSU     = "238"
	tpEventoCiencia = "210210"
	cOrgaoAN        = "91"

	envProd      = "prod"
	envHom       = "hom"
	sefazEnvProd = "producao"
	sefazEnvHom  = "homologacao"

	rateLimitHours = 1
	triggerUser    = "user"

	// organization_persons table — counterparties (suppliers/customers) persisted
	// from received documents to speed up future issuances.
	personsTableSuffix = "organization_persons"
	personSKCNPJ       = "CNPJ_"
	personSKCPF        = "CPF_"
	cnpjDigits         = 14
	cpfDigits          = 11
)

// docTypeConfig mirrors _DOC_TYPE_CONFIG from distribution.py.
type docTypeConfig struct {
	sefazService      string
	uf                string // empty string means derive from org
	xmlns             string
	version           string
	configTableSuffix string
	distTable         string
	docTable          string
	eventsTable       string
}

var docTypeConfigs = map[string]docTypeConfig{
	"nfe": {
		sefazService:      "NFeDistribuicaoDFe",
		uf:                "AN",
		xmlns:             nsNFe,
		version:           "1.01",
		configTableSuffix: "nfe_configs",
		distTable:         "nfe_distributions",
		docTable:          "nfes",
		eventsTable:       "nfe_events",
	},
	"cte": {
		sefazService:      "CTeDistribuicaoDFe",
		uf:                "AN",
		xmlns:             "http://www.portalfiscal.inf.br/cte",
		version:           "1.00",
		configTableSuffix: "cte_configs",
		distTable:         "cte_distributions",
		docTable:          "ctes",
		eventsTable:       "cte_events",
	},
	"mdfe": {
		sefazService:      "MDFeDistribuicaoDFe",
		uf:                "",
		xmlns:             "http://www.portalfiscal.inf.br/mdfe",
		version:           "1.00",
		configTableSuffix: "mdfe_configs",
		distTable:         "mdfe_distributions",
		docTable:          "mdfes",
		eventsTable:       "mdfe_events",
	},
}

// DistributionDynamoClient is the DynamoDB subset used by DistributionService.
type DistributionDynamoClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

// DistributionClients bundles all external AWS dependencies for DistributionService.
type DistributionClients struct {
	S3     S3Client
	Lambda LambdaClient
	Dynamo DistributionDynamoClient
	SNS    SNSClient
}

// DistributionService processes distribution jobs received from the distribution SQS queue.
type DistributionService struct {
	s3     S3Client
	lam    LambdaClient
	dynamo DistributionDynamoClient
	sns    SNSClient
	cfg    *config.Config
}

// NewDistribution creates a new DistributionService.
func NewDistribution(clients DistributionClients, cfg *config.Config) *DistributionService {
	return &DistributionService{
		s3:     clients.S3,
		lam:    clients.Lambda,
		dynamo: clients.Dynamo,
		sns:    clients.SNS,
		cfg:    cfg,
	}
}

// DistributionMessage is the SQS message body for distribution jobs.
type DistributionMessage struct {
	JobType   string `json:"job_type"`
	OrgPK     string `json:"org_pk"`
	DocType   string `json:"doc_type"`
	Trigger   string `json:"trigger"`
	NSU       *int   `json:"nsu,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
}

// Process routes a distribution job to the correct handler.
func (s *DistributionService) Process(ctx context.Context, msg DistributionMessage) error {
	dtcfg, ok := docTypeConfigs[msg.DocType]
	if !ok {
		slog.Error("unknown doc_type", "doc_type", msg.DocType, "org_pk", msg.OrgPK)
		return nil
	}

	slog.Info("distribution job",
		"job_type", msg.JobType,
		"org_pk", msg.OrgPK,
		"doc_type", msg.DocType,
		"trigger", msg.Trigger,
	)

	switch msg.JobType {
	case "dist_nsu", "":
		return s.runDistNSU(ctx, msg.OrgPK, msg.DocType, msg.Trigger, dtcfg)
	case "cons_nsu":
		if msg.NSU == nil {
			return fmt.Errorf("cons_nsu requires nsu field")
		}
		return s.runConsNSU(ctx, msg.OrgPK, msg.DocType, *msg.NSU, dtcfg)
	case "cons_ch_nfe":
		return s.runConsAccessKey(ctx, msg.OrgPK, msg.DocType, msg.AccessKey, dtcfg)
	default:
		slog.Warn("unknown distribution job_type", "job_type", msg.JobType)
		return nil
	}
}

// ------------------------------------------------------------------
// Main pagination loop — distNSU
// ------------------------------------------------------------------

func (s *DistributionService) runDistNSU(ctx context.Context, orgPK, docType, trigger string, dtcfg docTypeConfig) error {
	configTable := fmt.Sprintf("%s_organization_%s", s.cfg.TablePrefix, dtcfg.configTableSuffix)

	cfg, err := s.loadConfig(ctx, orgPK, configTable)
	if err != nil || cfg == nil {
		slog.Warn("no cfg found", "org_pk", orgPK, "doc_type", docType)
		return nil
	}

	cert, err := s.loadCert(ctx, orgPK, dtcfg.configTableSuffix)
	if err != nil || cert == nil {
		slog.Warn("no certificate found", "org_pk", orgPK)
		return nil
	}

	org, _ := s.loadOrg(ctx, orgPK)

	cnpj := extractCNPJ(orgPK)
	environment := attrN(cfg, "environment", 2)
	uf := extractUF(org)
	sefazEnv := sefazEnvHom
	envPrefix := envHom
	if environment == 1 {
		sefazEnv = sefazEnvProd
		envPrefix = envProd
	}

	certB64, err := s.getCertB64(ctx, attrS(cert, "s3_key"))
	if err != nil {
		return fmt.Errorf("getCertB64: %w", err)
	}
	certPassword := attrS(cert, "password")

	currentNSU := attrN(cfg, envPrefix+"_nsu", 0)

	for firstIter := true; ; firstIter = false {
		now := time.Now().UTC()

		if firstIter && trigger == triggerUser {
			// API already atomically claimed the slot — only check the penalty field,
			// which could have been set between the API request and this invocation.
			penaltyField := envPrefix + "_improper_usage_until"
			if penaltyStr := dynAttrS(cfg, penaltyField); penaltyStr != "" {
				if t, err := time.Parse(time.RFC3339Nano, penaltyStr); err == nil && now.Before(t) {
					slog.Warn("consumo indevido penalty active", "org_pk", orgPK, "until", penaltyStr)
					return nil
				}
			}
		} else {
			claimed, err := s.claimDistNSUSlot(ctx, orgPK, configTable, cfg, envPrefix, now)
			if err != nil || !claimed {
				slog.Info("rate limit active — stopping", "org_pk", orgPK, "doc_type", docType)
				return nil
			}
		}

		body := s.buildPayload(cnpj, certB64, certPassword, uf, sefazEnv, docType, dtcfg, "distNSU",
			map[string]any{"ultNSU": fmt.Sprintf("%015d", max(currentNSU, 1))})

		resp, err := s.invokePyDfe(ctx, body)
		if err != nil {
			return fmt.Errorf("invokePyDfe: %w", err)
		}

		statusCode := int(getFloat(resp, "statusCode"))
		var respBody map[string]any
		if b, ok := resp["body"].(string); ok {
			_ = json.Unmarshal([]byte(b), &respBody)
		}

		if statusCode != 200 {
			detail := mapStr(respBody, "detail", mapStr(respBody, "title", "Erro SEFAZ"))
			slog.Error("py-dfe error", "org_pk", orgPK, "detail", detail)
			if strings.Contains(strings.ToLower(detail), "consumo indevido") {
				_ = s.setImproperUsage(ctx, orgPK, configTable, envPrefix, now)
			}
			return nil
		}

		ret := asMap(respBody, "retDistDFeInt")
		cStat := mapStr(ret, "cStat", "")
		xMotivo := mapStr(ret, "xMotivo", "")

		if strings.Contains(strings.ToLower(xMotivo), "consumo indevido") {
			slog.Warn("consumo indevido detected", "org_pk", orgPK)
			_ = s.setImproperUsage(ctx, orgPK, configTable, envPrefix, now)
			return nil
		}

		if cStat == cStatNoDocs || cStat == cStatMaxNSU {
			slog.Info("no new docs", "org_pk", orgPK, "doc_type", docType, "cStat", cStat)
			ult := intVal(ret, "ultNSU", currentNSU)
			if ult > currentNSU {
				currentNSU = ult
			}
			_ = s.updateNSU(ctx, orgPK, configTable, envPrefix, currentNSU)
			return nil
		}

		if cStat != cStatDocFound {
			slog.Warn("unexpected cStat", "cStat", cStat, "org_pk", orgPK)
			return nil
		}

		lote := asMap(ret, "loteDistDFeInt")
		docZips := asSlice(lote, "docZip")

		ultNSU := intVal(ret, "ultNSU", currentNSU)
		maxNSU := intVal(ret, "maxNSU", ultNSU)

		orgName := ""
		if org != nil {
			orgName = attrS(org, "name")
		}
		for _, doc := range docZips {
			docMap, _ := doc.(map[string]any)
			if docMap == nil {
				continue
			}
			s.processDocZip(ctx, docMap, orgPK, docType, dtcfg, cnpj, orgName, cert, environment, sefazEnv, envPrefix)
		}

		_ = s.updateNSU(ctx, orgPK, configTable, envPrefix, ultNSU)
		currentNSU = ultNSU

		if ultNSU >= maxNSU {
			slog.Info("caught up", "org_pk", orgPK, "doc_type", docType, "ult_nsu", ultNSU)
			return nil
		}
		slog.Info("more docs available — will resume next cycle", "org_pk", orgPK, "ult_nsu", ultNSU, "max_nsu", maxNSU)
	}
}

// ------------------------------------------------------------------
// On-demand: consNSU
// ------------------------------------------------------------------

func (s *DistributionService) runConsNSU(ctx context.Context, orgPK, docType string, nsu int, dtcfg docTypeConfig) error {
	cfg, err := s.loadConfig(ctx, orgPK, fmt.Sprintf("%s_organization_%s", s.cfg.TablePrefix, dtcfg.configTableSuffix))
	if err != nil || cfg == nil {
		return nil
	}
	cert, err := s.loadCert(ctx, orgPK, dtcfg.configTableSuffix)
	if err != nil || cert == nil {
		return nil
	}
	org, _ := s.loadOrg(ctx, orgPK)
	cnpj := extractCNPJ(orgPK)
	environment := attrN(cfg, "environment", 2)
	uf := extractUF(org)
	sefazEnv := sefazEnvHom
	envPrefix := envHom
	if environment == 1 {
		sefazEnv = sefazEnvProd
		envPrefix = envProd
	}
	certB64, err := s.getCertB64(ctx, attrS(cert, "s3_key"))
	if err != nil {
		return err
	}
	certPassword := attrS(cert, "password")
	orgName := ""
	if org != nil {
		orgName = attrS(org, "name")
	}

	body := s.buildPayload(cnpj, certB64, certPassword, uf, sefazEnv, docType, dtcfg, "consNSU",
		map[string]any{"NSU": fmt.Sprintf("%015d", nsu)})
	resp, err := s.invokePyDfe(ctx, body)
	if err != nil || int(getFloat(resp, "statusCode")) != 200 {
		slog.Error("consNSU failed", "org_pk", orgPK, "nsu", nsu)
		return nil
	}

	var respBody map[string]any
	if b, ok := resp["body"].(string); ok {
		_ = json.Unmarshal([]byte(b), &respBody)
	}
	lote := asMap(asMap(respBody, "retDistDFeInt"), "loteDistDFeInt")
	for _, doc := range asSlice(lote, "docZip") {
		docMap, _ := doc.(map[string]any)
		if docMap != nil {
			s.processDocZip(ctx, docMap, orgPK, docType, dtcfg, cnpj, orgName, cert, environment, sefazEnv, envPrefix)
		}
	}
	return nil
}

// ------------------------------------------------------------------
// On-demand: consChNFe / consChCTe / consChMDFe
// ------------------------------------------------------------------

func (s *DistributionService) runConsAccessKey(ctx context.Context, orgPK, docType, accessKey string, dtcfg docTypeConfig) error {
	configTable := fmt.Sprintf("%s_organization_%s", s.cfg.TablePrefix, dtcfg.configTableSuffix)
	cfg, err := s.loadConfig(ctx, orgPK, configTable)
	if err != nil || cfg == nil {
		return nil
	}
	cert, err := s.loadCert(ctx, orgPK, dtcfg.configTableSuffix)
	if err != nil || cert == nil {
		return nil
	}
	org, _ := s.loadOrg(ctx, orgPK)
	cnpj := extractCNPJ(orgPK)
	environment := attrN(cfg, "environment", 2)
	uf := extractUF(org)
	sefazEnv := sefazEnvHom
	envPrefix := envHom
	if environment == 1 {
		sefazEnv = sefazEnvProd
		envPrefix = envProd
	}
	certB64, err := s.getCertB64(ctx, attrS(cert, "s3_key"))
	if err != nil {
		return err
	}
	certPassword := attrS(cert, "password")
	orgName := ""
	if org != nil {
		orgName = attrS(org, "name")
	}

	chKey := map[string]string{"nfe": "consChNFe", "cte": "consChCTe", "mdfe": "consChMDFe"}[docType]
	chTag := map[string]string{"nfe": "chNFe", "cte": "chCTe", "mdfe": "chMDFe"}[docType]
	if chKey == "" {
		chKey = "consChNFe"
		chTag = "chNFe"
	}

	body := s.buildPayload(cnpj, certB64, certPassword, uf, sefazEnv, docType, dtcfg, chKey,
		map[string]any{chTag: accessKey})
	resp, err := s.invokePyDfe(ctx, body)
	if err != nil || int(getFloat(resp, "statusCode")) != 200 {
		slog.Error("consAccessKey failed", "org_pk", orgPK, "access_key", accessKey)
		return nil
	}

	var respBody map[string]any
	if b, ok := resp["body"].(string); ok {
		_ = json.Unmarshal([]byte(b), &respBody)
	}
	lote := asMap(asMap(respBody, "retDistDFeInt"), "loteDistDFeInt")
	for _, doc := range asSlice(lote, "docZip") {
		docMap, _ := doc.(map[string]any)
		if docMap != nil {
			s.processDocZip(ctx, docMap, orgPK, docType, dtcfg, cnpj, orgName, cert, environment, sefazEnv, envPrefix)
		}
	}
	return nil
}

// ------------------------------------------------------------------
// docZip processing
// ------------------------------------------------------------------

func (s *DistributionService) processDocZip(
	ctx context.Context,
	doc map[string]any,
	orgPK, docType string,
	dtcfg docTypeConfig,
	cnpj, orgName string,
	cert map[string]types.AttributeValue,
	environment int,
	sefazEnv, envPrefix string,
) {
	nsuStr, _ := doc["@NSU"].(string)
	nsu, _ := strconv.Atoi(nsuStr)
	schema, _ := doc["@schema"].(string)
	text, _ := doc["#text"].(string)
	schemaType := determineSchemaType(schema)

	now := time.Now().UTC()
	distRecord := map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: envPrefix + "#" + orgPK},
		"nsu":         &types.AttributeValueMemberN{Value: strconv.Itoa(nsu)},
		"schema":      &types.AttributeValueMemberS{Value: schema},
		"schema_type": &types.AttributeValueMemberS{Value: schemaType},
		"created_at":  &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
	}

	defer func() {
		distTable := s.cfg.TablePrefix + "_" + dtcfg.distTable
		_, err := s.dynamo.PutItem(ctx, &dynamodb.PutItemInput{
			TableName:           aws.String(distTable),
			Item:                distRecord,
			ConditionExpression: aws.String("attribute_not_exists(pk) AND attribute_not_exists(nsu)"),
		})
		if err != nil {
			slog.Warn("NSU already exists or PutItem failed", "nsu", nsu, "table", distTable)
		}
	}()

	xmlStr, err := decompressDocZip(text)
	if err != nil {
		slog.Error("failed to decompress docZip", "nsu", nsu, "err", err)
		distRecord["parse_error"] = &types.AttributeValueMemberBOOL{Value: true}
		return
	}

	root, err := parseXMLBytes([]byte(xmlStr))
	if err != nil {
		slog.Error("failed to parse XML", "nsu", nsu, "err", err)
		distRecord["parse_error"] = &types.AttributeValueMemberBOOL{Value: true}
		return
	}

	fields := extractDoc(schemaType, root, docType, cnpj)
	slog.Info("processing NSU", "nsu", nsu, "org_pk", envPrefix+"#"+orgPK, "schema_type", schemaType)

	if fields.AccessKey != "" {
		distRecord["access_key"] = &types.AttributeValueMemberS{Value: fields.AccessKey}
	}
	if schemaType == SchemaProcEventoNFe || schemaType == SchemaProcEventoCTe ||
		schemaType == SchemaProcEventoMDFe || schemaType == SchemaResEvento {
		if fields.EventType != "" {
			distRecord["event_type"] = &types.AttributeValueMemberS{Value: fields.EventType}
		}
		if fields.SequenceNumber != "" {
			distRecord["sequence_number"] = &types.AttributeValueMemberS{Value: fields.SequenceNumber}
		}
	}

	// Upload 1: NSU key — always.
	nsuS3Key := fmt.Sprintf("%s-distribution/%s/%s/NSU_%015d.xml", docType, envPrefix, orgPK, nsu)
	if _, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.DocumentsBucket),
		Key:         aws.String(nsuS3Key),
		Body:        bytes.NewReader([]byte(xmlStr)),
		ContentType: aws.String("application/xml"),
	}); err != nil {
		slog.Error("failed to upload NSU XML", "nsu", nsu, "key", nsuS3Key, "err", err)
	} else {
		distRecord["xml_s3_key"] = &types.AttributeValueMemberS{Value: nsuS3Key}
	}

	// Upload 2: access_key-based key for proc/res documents.
	var docS3Key string
	seqNum := 1
	if fields.SequenceNumber != "" {
		if n, err := strconv.Atoi(fields.SequenceNumber); err == nil {
			seqNum = n
		}
	}
	if isEventSchema(schemaType) && fields.AccessKey != "" && fields.EventType != "" {
		docS3Key = fmt.Sprintf("%s/%s/%s/%s_%s_%03d", docType, envPrefix, orgPK, fields.AccessKey, fields.EventType, seqNum)
	} else if isProcSchema(schemaType) && fields.AccessKey != "" {
		docS3Key = fmt.Sprintf("%s/%s/%s/%s.xml", docType, envPrefix, orgPK, fields.AccessKey)
	}

	if docS3Key != "" {
		fields.XMLS3Key = docS3Key
		if _, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(s.cfg.DocumentsBucket),
			Key:         aws.String(docS3Key),
			Body:        bytes.NewReader([]byte(xmlStr)),
			ContentType: aws.String("application/xml"),
		}); err != nil {
			slog.Error("failed to upload doc XML", "nsu", nsu, "key", docS3Key, "err", err)
		}
	}

	docPK := envPrefix + "#" + orgPK

	switch schemaType {
	case SchemaResNFe:
		fields.DestCPFCNPJ = cnpj
		fields.DestName = orgName
		s.persistIncoming(ctx, docPK, fields, dtcfg)
		s.autoScience(ctx, docPK, cnpj, environment, sefazEnv, cert, fields)

	case SchemaProcNFe, SchemaProcCTe, SchemaProcCTeOS, SchemaProcGTVe, SchemaProcCTeSimp, SchemaProcMDFe:
		s.persistIncoming(ctx, docPK, fields, dtcfg)
		// Persist counterparties for NF-e/CT-e processed docs only — MDF-e is a
		// transport manifest without a fiscal supplier/customer relationship.
		if schemaType != SchemaProcMDFe {
			s.persistCounterparties(ctx, orgPK, cnpj, fields)
		}
		s.notifyResult(ctx, orgPK, fields, nsu, dtcfg)

	case SchemaResCTe, SchemaResMDFe:
		// Summary only — no auto-Ciência for CT-e/MDF-e.

	case SchemaProcEventoNFe, SchemaProcEventoCTe, SchemaProcEventoMDFe, SchemaResEvento:
		s.persistEvent(ctx, fields, dtcfg)
	}
}

func isEventSchema(s string) bool {
	return s == SchemaProcEventoNFe || s == SchemaProcEventoCTe || s == SchemaProcEventoMDFe || s == SchemaResEvento
}

func isProcSchema(s string) bool {
	return s == SchemaProcNFe || s == SchemaProcCTe || s == SchemaProcCTeOS ||
		s == SchemaProcGTVe || s == SchemaProcCTeSimp || s == SchemaProcMDFe
}

// ------------------------------------------------------------------
// Auto-Ciência (NF-e resNFe only)
// ------------------------------------------------------------------

func (s *DistributionService) autoScience(
	ctx context.Context,
	docPK, cnpj string,
	environment int,
	sefazEnv string,
	cert map[string]types.AttributeValue,
	fields DocFields,
) {
	accessKey := fields.AccessKey
	if accessKey == "" {
		return
	}

	seqNum := 1
	eventSK := newUUIDv7()
	eventKey := fmt.Sprintf("%s#%s#%03d", accessKey, tpEventoCiencia, seqNum)
	now := time.Now().UTC()
	dh := fmtDhManifest(now)

	eventsTable := s.cfg.TablePrefix + "_nfe_events"
	_, err := s.dynamo.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(eventsTable),
		Item: map[string]types.AttributeValue{
			"pk":              &types.AttributeValueMemberS{Value: accessKey},
			"sk":              &types.AttributeValueMemberS{Value: eventSK},
			"event_key":       &types.AttributeValueMemberS{Value: eventKey},
			"access_key":      &types.AttributeValueMemberS{Value: accessKey},
			"event_type":      &types.AttributeValueMemberS{Value: tpEventoCiencia},
			"sequence_number": &types.AttributeValueMemberN{Value: strconv.Itoa(seqNum)},
			"status":          &types.AttributeValueMemberS{Value: "pending"},
			"created_at":      &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
		},
	})
	if err != nil {
		slog.Error("failed to create pending Ciência event", "access_key", accessKey, "err", err)
		return
	}

	batchID := strconv.FormatInt(mathrand.Int63n(999_999_999_999_999)+1, 10)
	tpAmb := strconv.Itoa(environment)
	body := map[string]any{
		"envEvento": map[string]any{
			"@versao": "1.00",
			"@xmlns":  nsNFe,
			"idLote":  batchID,
			"evento": map[string]any{
				"@versao": "1.00",
				"infEvento": map[string]any{
					"@Id":        fmt.Sprintf("ID%s%s%02d", tpEventoCiencia, accessKey, seqNum),
					"cOrgao":     cOrgaoAN,
					"tpAmb":      tpAmb,
					"CNPJ":       cnpj,
					"chNFe":      accessKey,
					"dhEvento":   dh,
					"tpEvento":   tpEventoCiencia,
					"nSeqEvento": strconv.Itoa(seqNum),
					"verEvento":  "1.00",
					"detEvento": map[string]any{
						"@versao":    "1.00",
						"@xmlns":     nsNFe,
						"descEvento": "Ciencia da Operacao",
					},
				},
			},
		},
	}

	msgPayload := map[string]any{
		"doc_pk":             docPK,
		"access_key":         accessKey,
		"table_name":         "nfes",
		"s3_prefix":          "nfe",
		"expected_file_name": fmt.Sprintf("%s_%s_%03d", accessKey, tpEventoCiencia, seqNum),
		"cnpj":               cnpj,
		"uf":                 "AN",
		"sefaz_environment":  sefazEnv,
		"cert_s3_key":        certAttrS(cert, "s3_key"),
		"cert_password":      certAttrS(cert, "password"),
		"doc_type":           "nfe",
		"sefaz_service":      "RecepcaoEvento",
		"body":               body,
		"events_table_name":  "nfe_events",
		"event_type":         tpEventoCiencia,
		"sequence_number":    seqNum,
		"event_sk":           eventSK,
	}
	msgJSON, _ := json.Marshal(msgPayload)

	topicARN := s.cfg.EventBusTopicARN
	if topicARN == "" {
		slog.Warn("EVENT_BUS_TOPIC_ARN not set — Ciência skipped", "access_key", accessKey)
		return
	}

	if _, err := s.sns.Publish(ctx, snsInput(topicARN, string(msgJSON))); err != nil {
		slog.Error("failed to publish auto-Ciência", "access_key", accessKey, "err", err)
	}
}

// ------------------------------------------------------------------
// Persist helpers
// ------------------------------------------------------------------

func (s *DistributionService) persistIncoming(ctx context.Context, docPK string, fields DocFields, dtcfg docTypeConfig) {
	if fields.AccessKey == "" {
		return
	}
	table := s.cfg.TablePrefix + "_" + dtcfg.docTable
	now := time.Now().UTC()

	incoming := fields.Incoming
	if incoming == 0 {
		incoming = 1
	}
	year, month, day := fields.Year, fields.Month, fields.Day
	if year == 0 {
		year, month, day = now.Year(), int(now.Month()), now.Day()
	}
	createdAt := fields.CreatedAt
	if createdAt == "" {
		createdAt = now.Format(time.RFC3339Nano)
	}

	item := map[string]types.AttributeValue{
		"pk":             &types.AttributeValueMemberS{Value: docPK},
		"sk":             &types.AttributeValueMemberS{Value: fields.AccessKey},
		"status":         &types.AttributeValueMemberS{Value: "authorized"},
		"incoming":       &types.AttributeValueMemberN{Value: strconv.Itoa(incoming)},
		"year":           &types.AttributeValueMemberN{Value: strconv.Itoa(year)},
		"month":          &types.AttributeValueMemberN{Value: strconv.Itoa(month)},
		"day":            &types.AttributeValueMemberN{Value: strconv.Itoa(day)},
		"xml_s3_key":     &types.AttributeValueMemberS{Value: fields.XMLS3Key},
		"created_at":     &types.AttributeValueMemberS{Value: createdAt},
		"emit_cpf_cnpj":  &types.AttributeValueMemberS{Value: fields.EmitCPFCNPJ},
		"emit_name":      &types.AttributeValueMemberS{Value: fields.EmitName},
		"dest_cpf_cnpj":  &types.AttributeValueMemberS{Value: fields.DestCPFCNPJ},
		"dest_name":      &types.AttributeValueMemberS{Value: fields.DestName},
		"total":          &types.AttributeValueMemberS{Value: fields.Total},
		"sefaz_status":   &types.AttributeValueMemberS{Value: fields.SefazStatus},
		"sefaz_motive":   &types.AttributeValueMemberS{Value: fields.SefazMotive},
		"sefaz_protocol": &types.AttributeValueMemberS{Value: fields.SefazProtocol},
		"dh_emi":         &types.AttributeValueMemberS{Value: fields.DHEmi},
		"serie":          &types.AttributeValueMemberN{Value: strconv.Itoa(fields.Serie)},
		"number":         &types.AttributeValueMemberN{Value: strconv.Itoa(fields.Number)},
	}

	if len(fields.Products) > 0 {
		item["products"] = attributeList(fields.Products)
	}
	if len(fields.Payments) > 0 {
		item["payments"] = attributeList(fields.Payments)
	}

	if _, err := s.dynamo.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      item,
	}); err != nil {
		slog.Warn("could not persist incoming doc", "access_key", fields.AccessKey, "table", table, "err", err)
	}
}

// persistCounterparties upserts a received document's emitter and recipient as
// organization_persons records (suppliers/customers) so future issuances can
// reuse the data. Parties whose CPF/CNPJ equals the org's own are skipped.
func (s *DistributionService) persistCounterparties(ctx context.Context, orgPK, orgCNPJ string, fields DocFields) {
	s.persistPerson(ctx, orgPK, orgCNPJ, fields.EmitCPFCNPJ, fields.EmitName, fields.EmitDetails)
	s.persistPerson(ctx, orgPK, orgCNPJ, fields.DestCPFCNPJ, fields.DestName, fields.DestDetails)
}

// persistPerson creates an organization_persons record for a counterparty when
// its CPF/CNPJ is non-blank and differs from the org's. The nested person object
// (addresses, contacts, state_registrations, ...) is stored under `person` when
// the document carries it. Create-if-absent: a manually curated person is never
// overwritten.
func (s *DistributionService) persistPerson(ctx context.Context, orgPK, orgCNPJ, cpfCNPJ, name string, details map[string]any) {
	digits := onlyDigits(cpfCNPJ)
	if digits == "" || digits == onlyDigits(orgCNPJ) {
		return
	}
	sk, ok := buildPersonSK(digits)
	if !ok {
		slog.Warn("skipping person with invalid CPF/CNPJ", "cpf_cnpj", digits, "org_pk", orgPK)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	table := s.cfg.TablePrefix + "_" + personsTableSuffix
	item := map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: orgPK},
		"sk":          &types.AttributeValueMemberS{Value: sk},
		"org_pk":      &types.AttributeValueMemberS{Value: orgPK},
		"cpf_or_cnpj": &types.AttributeValueMemberS{Value: digits},
		"name":        &types.AttributeValueMemberS{Value: name},
		"created_at":  &types.AttributeValueMemberS{Value: now},
		"updated_at":  &types.AttributeValueMemberS{Value: now},
	}
	if len(details) > 0 {
		item["person"] = mapToAttr(details)
	}

	personTx := types.TransactWriteItem{
		Put: &types.Put{
			TableName:           aws.String(table),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(pk)"),
		},
	}
	auditTx := buildAuditLogTxItem(s.cfg.TablePrefix, orgPK, auditResourcePerson, sk, auditActionCreate, personToModifications(item))

	_, err := s.dynamo.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{personTx, auditTx},
	})
	if err != nil {
		// Most commonly the conditional check failed because the person already
		// exists — expected and harmless.
		slog.Debug("person not persisted (already exists or put failed)", "sk", sk, "org_pk", orgPK, "err", err)
	}
}

// personToModifications turns a freshly-built person item into a CREATE
// modification list (every field, before=nil), for the audit row.
func personToModifications(item map[string]types.AttributeValue) []auditModification {
	mods := make([]auditModification, 0, len(item))
	for k, v := range item {
		if k == "pk" || k == "sk" || k == "created_at" || k == "updated_at" {
			continue
		}
		var plain any
		_ = attributevalue.Unmarshal(v, &plain)
		mods = append(mods, auditModification{Name: k, Before: nil, After: plain})
	}
	return mods
}

func (s *DistributionService) persistEvent(ctx context.Context, fields DocFields, dtcfg docTypeConfig) {
	if fields.AccessKey == "" || fields.EventType == "" {
		return
	}
	seqNum := 1
	if n, err := strconv.Atoi(fields.SequenceNumber); err == nil && n > 0 {
		seqNum = n
	}
	eventSK := newUUIDv7()
	eventKey := fmt.Sprintf("%s#%s#%03d", fields.AccessKey, fields.EventType, seqNum)
	now := time.Now().UTC()
	table := s.cfg.TablePrefix + "_" + dtcfg.eventsTable

	_, err := s.dynamo.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item: map[string]types.AttributeValue{
			"pk":              &types.AttributeValueMemberS{Value: fields.AccessKey},
			"sk":              &types.AttributeValueMemberS{Value: eventSK},
			"event_key":       &types.AttributeValueMemberS{Value: eventKey},
			"access_key":      &types.AttributeValueMemberS{Value: fields.AccessKey},
			"event_type":      &types.AttributeValueMemberS{Value: fields.EventType},
			"sequence_number": &types.AttributeValueMemberN{Value: strconv.Itoa(seqNum)},
			"status":          &types.AttributeValueMemberS{Value: "success"},
			"sefaz_status":    &types.AttributeValueMemberS{Value: fields.SefazStatus},
			"sefaz_motive":    &types.AttributeValueMemberS{Value: fields.SefazMotive},
			"sefaz_protocol":  &types.AttributeValueMemberS{Value: fields.SefazProtocol},
			"dh_evento":       &types.AttributeValueMemberS{Value: fields.DHEvento},
			"created_at":      &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
		},
	})
	if err != nil {
		slog.Warn("could not persist event", "event_type", fields.EventType, "access_key", fields.AccessKey, "err", err)
	}
}

func (s *DistributionService) notifyResult(ctx context.Context, orgPK string, fields DocFields, nsu int, dtcfg docTypeConfig) {
	if s.cfg.ResultsTopicARN == "" || s.sns == nil {
		return
	}
	docTypePrefix := strings.Split(dtcfg.distTable, "_")[0]
	msg, _ := json.Marshal(map[string]any{
		"type":       "new_distribution_nfe",
		"org_pk":     orgPK,
		"access_key": fields.AccessKey,
		"emit_name":  fields.EmitName,
		"total":      fields.Total,
		"nsu":        nsu,
		"doc_type":   docTypePrefix,
	})
	if _, err := s.sns.Publish(ctx, snsInput(s.cfg.ResultsTopicARN, string(msg))); err != nil {
		slog.Warn("failed to notify result", "nsu", nsu, "err", err)
	}
}

// ------------------------------------------------------------------
// Rate limit helpers
// ------------------------------------------------------------------

func (s *DistributionService) claimDistNSUSlot(
	ctx context.Context,
	orgPK, configTable string,
	cfg map[string]types.AttributeValue,
	envPrefix string,
	now time.Time,
) (bool, error) {
	penaltyField := envPrefix + "_improper_usage_until"
	lastField := envPrefix + "_last_dist_nsu_at"

	// Check improper_usage_until penalty.
	if penaltyStr := dynAttrS(cfg, penaltyField); penaltyStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, penaltyStr); err == nil {
			if now.Before(t) {
				slog.Warn("consumo indevido penalty active", "org_pk", orgPK, "until", penaltyStr)
				return false, nil
			}
		}
	}

	// Check 1-hour minimum between distNSU calls.
	thresholdTime := now.Add(-rateLimitHours * time.Hour)
	if lastStr := dynAttrS(cfg, lastField); lastStr != "" {
		if lastTime, err := time.Parse(time.RFC3339Nano, lastStr); err == nil && lastTime.After(thresholdTime) {
			slog.Info("too soon for distNSU", "org_pk", orgPK, "last", lastStr)
			return false, nil
		}
	}

	// Use RFC3339 (second precision, fixed-width UTC) so DynamoDB string < comparison is lexicographically correct.
	nowStr := now.Truncate(time.Second).Format(time.RFC3339)
	thresholdStr := thresholdTime.Truncate(time.Second).Format(time.RFC3339)

	// Atomically claim the slot.
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(configTable),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
		},
		UpdateExpression: aws.String("SET " + lastField + " = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":       &types.AttributeValueMemberS{Value: nowStr},
			":threshold": &types.AttributeValueMemberS{Value: thresholdStr},
		},
		ConditionExpression: aws.String("attribute_not_exists(" + lastField + ") OR " + lastField + " < :threshold"),
	})
	if err != nil {
		slog.Info("slot claim failed (race condition)", "org_pk", orgPK)
		return false, nil
	}
	return true, nil
}

func (s *DistributionService) setImproperUsage(ctx context.Context, orgPK, configTable, envPrefix string, now time.Time) error {
	until := now.Add(rateLimitHours * time.Hour).Format(time.RFC3339Nano)
	field := envPrefix + "_improper_usage_until"
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(configTable),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
		},
		UpdateExpression: aws.String("SET " + field + " = :until"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":until": &types.AttributeValueMemberS{Value: until},
		},
	})
	if err != nil {
		slog.Error("failed to set improper usage", "field", field, "org_pk", orgPK, "err", err)
	} else {
		slog.Warn("set improper usage penalty", "field", field, "until", until, "org_pk", orgPK)
	}
	return err
}

func (s *DistributionService) updateNSU(ctx context.Context, orgPK, configTable, envPrefix string, nsu int) error {
	field := envPrefix + "_nsu"
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(configTable),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
		},
		UpdateExpression: aws.String("SET " + field + " = :nsu"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":nsu": &types.AttributeValueMemberN{Value: strconv.Itoa(nsu)},
		},
	})
	if err != nil {
		slog.Error("failed to update NSU", "field", field, "nsu", nsu, "org_pk", orgPK, "err", err)
	}
	return err
}

// ------------------------------------------------------------------
// Data loading helpers
// ------------------------------------------------------------------

func (s *DistributionService) loadConfig(ctx context.Context, orgPK, configTable string) (map[string]types.AttributeValue, error) {
	out, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(configTable),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
		},
	})
	if err != nil {
		return nil, err
	}
	return out.Item, nil
}

func (s *DistributionService) loadCert(ctx context.Context, orgPK, _ string) (map[string]types.AttributeValue, error) {
	table := s.cfg.TablePrefix + "_organization_certificates"
	out, err := s.dynamo.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(table),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: orgPK},
		},
		Limit:            aws.Int32(1),
		ScanIndexForward: aws.Bool(false),
	})
	if err != nil || len(out.Items) == 0 {
		return nil, err
	}
	return out.Items[0], nil
}

func (s *DistributionService) loadOrg(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	out, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.cfg.TablePrefix + "_organizations"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
		},
	})
	if err != nil {
		return nil, err
	}
	return out.Item, nil
}

func (s *DistributionService) getCertB64(ctx context.Context, s3Key string) (string, error) {
	out, err := s.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.CertsBucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Body.Close() }()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// ------------------------------------------------------------------
// py-dfe Lambda invocation
// ------------------------------------------------------------------

func (s *DistributionService) buildPayload(
	cnpj, certB64, certPassword, uf, sefazEnv, docType string,
	dtcfg docTypeConfig,
	bodyKey string,
	bodyValue map[string]any,
) map[string]any {
	environmentStr := "2"
	if sefazEnv == sefazEnvProd {
		environmentStr = "1"
	}
	effectiveUF := dtcfg.uf
	if effectiveUF == "" {
		effectiveUF = uf
	}

	distVal := map[string]any{
		"@versao":  dtcfg.version,
		"@xmlns":   dtcfg.xmlns,
		"tpAmb":    environmentStr,
		"cUFAutor": ufCode(effectiveUF),
		"CNPJ":     cnpj,
		bodyKey:    bodyValue,
	}
	if docType == "mdfe" {
		delete(distVal, "cUFAutor")
	}

	return map[string]any{
		"cnpj":                 cnpj,
		"certificate_b64":      certB64,
		"certificate_password": certPassword,
		"uf":                   effectiveUF,
		"environment":          sefazEnv,
		"doc_type":             docType,
		"service":              dtcfg.sefazService,
		"validate_schema":      false,
		"max_retries":          2,
		"body":                 map[string]any{"distDFeInt": distVal},
	}
}

func (s *DistributionService) invokePyDfe(ctx context.Context, payload map[string]any) (map[string]any, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	out, err := s.lam.Invoke(ctx, lambdaInput(s.cfg.DfeLambdaName, payloadBytes))
	if err != nil {
		return nil, err
	}
	if out.FunctionError != nil {
		return nil, fmt.Errorf("lambda function error: %s", *out.FunctionError)
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Payload, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ------------------------------------------------------------------
// Small helpers
// ------------------------------------------------------------------

// onlyDigits strips every non-digit rune from s.
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteByte(byte(r))
		}
	}
	return b.String()
}

// buildPersonSK mirrors api BuildPersonSK: "CNPJ_<14 digits>" or "CPF_<11 digits>".
// Returns false when the digit count matches neither a CNPJ nor a CPF.
func buildPersonSK(digits string) (string, bool) {
	switch len(digits) {
	case cnpjDigits:
		return personSKCNPJ + digits, true
	case cpfDigits:
		return personSKCPF + digits, true
	}
	return "", false
}

func extractCNPJ(orgPK string) string {
	if idx := strings.Index(orgPK, "_"); idx >= 0 {
		return orgPK[idx+1:]
	}
	return orgPK
}

func extractUF(org map[string]types.AttributeValue) string {
	if org == nil {
		return "SP"
	}
	// org.person.addresses[0].state_federation
	personAttr, ok := org["person"]
	if !ok {
		return "SP"
	}
	personMap, ok := personAttr.(*types.AttributeValueMemberM)
	if !ok {
		return "SP"
	}
	addrAttr, ok := personMap.Value["addresses"]
	if !ok {
		return "SP"
	}
	addrList, ok := addrAttr.(*types.AttributeValueMemberL)
	if !ok || len(addrList.Value) == 0 {
		return "SP"
	}
	firstAddr, ok := addrList.Value[0].(*types.AttributeValueMemberM)
	if !ok {
		return "SP"
	}
	sfAttr, ok := firstAddr.Value["state_federation"]
	if !ok {
		return "SP"
	}
	sfStr, ok := sfAttr.(*types.AttributeValueMemberS)
	if !ok || sfStr.Value == "" {
		return "SP"
	}
	return sfStr.Value
}

func dynAttrS(item map[string]types.AttributeValue, key string) string {
	if item == nil {
		return ""
	}
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func attrS(item map[string]types.AttributeValue, key string) string {
	return dynAttrS(item, key)
}

func attrN(item map[string]types.AttributeValue, key string, def int) int {
	if item == nil {
		return def
	}
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		if n, err := strconv.Atoi(v.Value); err == nil {
			return n
		}
	}
	return def
}

func certAttrS(cert map[string]types.AttributeValue, key string) string {
	return dynAttrS(cert, key)
}

func asMap(v any, key string) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return nil
	}
	inner, _ := m[key].(map[string]any)
	return inner
}

func asSlice(v map[string]any, key string) []any {
	if v == nil {
		return nil
	}
	switch val := v[key].(type) {
	case []any:
		return val
	case map[string]any:
		return []any{val}
	}
	return nil
}

func mapStr(m map[string]any, key, def string) string {
	if m == nil {
		return def
	}
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func getFloat(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	v, _ := m[key].(float64)
	return v
}

func intVal(m map[string]any, key string, def int) int {
	if m == nil {
		return def
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// mapToAttr converts a map[string]any into a DynamoDB map AttributeValue.
// Supports the subset emitted by buildPersonDetails: string, int, nested
// map[string]any, and []any.
func mapToAttr(m map[string]any) *types.AttributeValueMemberM {
	out := make(map[string]types.AttributeValue, len(m))
	for k, v := range m {
		if v == nil {
			continue // omit null attributes (reduce stored item size)
		}
		out[k] = toAttr(v)
	}
	return &types.AttributeValueMemberM{Value: out}
}

func toAttr(v any) types.AttributeValue {
	switch t := v.(type) {
	case string:
		return &types.AttributeValueMemberS{Value: t}
	case int:
		return &types.AttributeValueMemberN{Value: strconv.Itoa(t)}
	case map[string]any:
		return mapToAttr(t)
	case []any:
		list := make([]types.AttributeValue, 0, len(t))
		for _, e := range t {
			list = append(list, toAttr(e))
		}
		return &types.AttributeValueMemberL{Value: list}
	default:
		return &types.AttributeValueMemberNULL{Value: true}
	}
}

func attributeList(rows []map[string]string) *types.AttributeValueMemberL {
	list := make([]types.AttributeValue, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]types.AttributeValue, len(row))
		for k, v := range row {
			m[k] = &types.AttributeValueMemberS{Value: v}
		}
		list = append(list, &types.AttributeValueMemberM{Value: m})
	}
	return &types.AttributeValueMemberL{Value: list}
}

// newUUIDv7 generates a UUID v7
func newUUIDv7() string {
	id, _ := uuid.NewV7()
	return id.String()
}

func snsInput(topicARN, message string) *sns.PublishInput {
	return &sns.PublishInput{TopicArn: aws.String(topicARN), Message: aws.String(message)}
}

func lambdaInput(functionName string, payload []byte) *lambdaSDK.InvokeInput {
	return &lambdaSDK.InvokeInput{FunctionName: aws.String(functionName), Payload: payload}
}
