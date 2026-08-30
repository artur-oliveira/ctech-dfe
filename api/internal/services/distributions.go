package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

var docTypeSefazService = map[string]string{
	DocTypeNFe:  "NFeDistribuicaoDFe",
	DocTypeCTe:  "CTeDistribuicaoDFe",
	DocTypeMDFe: "MDFeDistribuicaoDFe",
}

var supportedDistributionDocTypes = map[string]struct{}{
	DocTypeNFe: {}, DocTypeCTe: {}, DocTypeMDFe: {}, DocTypeNfse: {},
}

var docTypeXMLNS = map[string]string{
	"nfe":  "http://www.portalfiscal.inf.br/nfe",
	"cte":  "http://www.portalfiscal.inf.br/cte",
	"mdfe": "http://www.portalfiscal.inf.br/mdfe",
}

const distConsQuotaMax = 20

// DistributionService manages DFe distribution received from SEFAZ or the NFS-e ADN.
type DistributionService struct {
	orgRepo           *repositories.OrganizationRepository
	certRepo          *repositories.CertificateRepository
	NfeConfig         *repositories.NfeConfigRepository
	NfceConfig        *repositories.NfceConfigRepository
	CteConfig         *repositories.CteConfigRepository
	MdfeConfig        *repositories.MdfeConfigRepository
	NfseConfig        *repositories.NfseConfigRepository
	nfeDist           *repositories.NFeDistributionRepository
	cteDist           *repositories.CTeDistributionRepository
	mdfeDist          *repositories.MDFeDistributionRepository
	nfseDist          *repositories.NfseDistributionRepository
	clients           *awsclient.Clients
	documentSvc       *documents.Service
	queueURL          string
	bucketDocs        string
	bucketCerts       string
	sefazFunctionName string
}

func NewDistributionService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	NfeConfig *repositories.NfeConfigRepository,
	NfceConfig *repositories.NfceConfigRepository,
	CteConfig *repositories.CteConfigRepository,
	MdfeConfig *repositories.MdfeConfigRepository,
	NfseConfig *repositories.NfseConfigRepository,
	nfeDist *repositories.NFeDistributionRepository,
	cteDist *repositories.CTeDistributionRepository,
	mdfeDist *repositories.MDFeDistributionRepository,
	nfseDist *repositories.NfseDistributionRepository,
	clients *awsclient.Clients,
	documentSvc *documents.Service,
	queueURL, bucketDocs, bucketCerts, sefazFunctionName string,
) *DistributionService {
	return &DistributionService{
		orgRepo: orgRepo, certRepo: certRepo,
		NfeConfig: NfeConfig, NfceConfig: NfceConfig, CteConfig: CteConfig, MdfeConfig: MdfeConfig, NfseConfig: NfseConfig,
		nfeDist: nfeDist, cteDist: cteDist, mdfeDist: mdfeDist, nfseDist: nfseDist,
		clients:           clients,
		documentSvc:       documentSvc,
		queueURL:          queueURL,
		bucketDocs:        bucketDocs,
		bucketCerts:       bucketCerts,
		sefazFunctionName: sefazFunctionName,
	}
}

func (s *DistributionService) fiscalCfg(docType string) *repositories.FiscalConfigRepository {
	switch docType {
	case DocTypeNFe:
		return &s.NfeConfig.FiscalConfigRepository
	case DocTypeNFCe:
		return &s.NfceConfig.FiscalConfigRepository
	case DocTypeCTe:
		return &s.CteConfig.FiscalConfigRepository
	case DocTypeMDFe:
		return &s.MdfeConfig.FiscalConfigRepository
	case DocTypeNfse:
		return &s.NfseConfig.FiscalConfigRepository
	}
	return nil
}

func (s *DistributionService) distRepo(docType string) *repositories.DistributionRepository {
	switch docType {
	case DocTypeNFe:
		return &s.nfeDist.DistributionRepository
	case DocTypeCTe:
		return &s.cteDist.DistributionRepository
	case DocTypeMDFe:
		return &s.mdfeDist.DistributionRepository
	case DocTypeNfse:
		return &s.nfseDist.DistributionRepository
	}
	return nil
}

func validateDistDocType(docType string) error {
	if _, ok := supportedDistributionDocTypes[docType]; !ok {
		return problem.BadRequest("doc_type inválido: " + docType)
	}
	return nil
}

func validateSefazDistDocType(docType string) error {
	if _, ok := docTypeSefazService[docType]; !ok {
		return problem.BadRequest("consulta síncrona indisponível para doc_type: " + docType)
	}
	return nil
}

// EnqueueSync validates rate limit then enqueues a background distNSU call for the org.
func (s *DistributionService) EnqueueSync(ctx context.Context, orgPK, docType string) (map[string]any, error) {
	if err := validateDistDocType(docType); err != nil {
		return nil, err
	}
	if s.queueURL == "" {
		return nil, problem.BadRequest("fila de distribuição não configurada")
	}

	cfgRepo := s.fiscalCfg(docType)
	config, err := cfgRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}

	envPrefix := EnvHom
	if config != nil && distIntAttr(config, "environment", 2) == 1 {
		envPrefix = EnvProd
	}

	if config != nil {
		if until := distStrAttr(config, envPrefix+"_improper_usage_until"); until != "" {
			if untilDt, err := time.Parse(time.RFC3339, until); err == nil && time.Now().UTC().Before(untilDt) {
				secs := int(time.Until(untilDt).Seconds())
				return nil, problem.TooManyRequests(fmt.Sprintf("Consumo indevido ativo. Aguarde %ds.", secs))
			}
		}
	}

	// Atomic rate-limit: one distNSU per hour
	ok, err := cfgRepo.ClaimDistNSUSlot(ctx, orgPK, envPrefix)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, problem.TooManyRequests("Limite de consultas ativo, aguarde 1 hora!")
	}

	msg := map[string]any{
		"job_type":     "dist_nsu",
		"org_pk":       orgPK,
		"doc_type":     docType,
		"trigger":      "user",
		"triggered_at": time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, problem.InternalServer("failed to encode sync message").WithCause(err)
	}
	if _, err := s.clients.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		return nil, problem.InternalServer("failed to enqueue sync: " + err.Error())
	}

	nsu := 0
	if config != nil {
		nsu = distIntAttr(config, envPrefix+"_nsu", 0)
	}
	return map[string]any{"status": "enqueued", "nsu": nsu}, nil
}

// ListDistributions returns paginated distribution records (newest-first).
func (s *DistributionService) ListDistributions(ctx context.Context, orgPK, docType string, opts repositories.DistributionListOpts) (*repositories.QueryResult, error) {
	if err := validateDistDocType(docType); err != nil {
		return nil, err
	}
	config, err := s.fiscalCfg(docType).Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	envPrefix := EnvHom
	if config != nil && distIntAttr(config, "environment", 2) == 1 {
		envPrefix = EnvProd
	}
	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	return s.distRepo(docType).ListDistributions(ctx, pk, opts)
}

// GetDistributionXML returns a direct URL for the stored XML of a specific NSU.
func (s *DistributionService) GetDistributionXML(ctx context.Context, orgPK, docType string, nsu int) (*documents.SignedFileDownload, error) {
	if err := validateDistDocType(docType); err != nil {
		return nil, err
	}
	config, err := s.fiscalCfg(docType).Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	envPrefix := EnvHom
	if config != nil && distIntAttr(config, "environment", 2) == 1 {
		envPrefix = EnvProd
	}
	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	record, err := s.distRepo(docType).GetByNSU(ctx, pk, nsu)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, problem.NotFound("NSU não encontrado")
	}
	s3Key, _ := record["xml_s3_key"].(string)
	if s3Key == "" {
		return nil, problem.NotFound("XML do NSU ainda não disponível")
	}
	filename := fmt.Sprintf("NSU_%015d", nsu)
	return s.documentSvc.SignFile(ctx, s3Key, documents.XMLFilename(filename), documents.ContentTypeXML)
}

// LookupByNSU performs a synchronous consNSU against SEFAZ via the py-dfe Lambda.
func (s *DistributionService) LookupByNSU(ctx context.Context, orgPK, docType string, nsu int) (map[string]any, error) {
	if err := validateSefazDistDocType(docType); err != nil {
		return nil, err
	}
	if err := s.checkConsQuota(ctx, orgPK, docType); err != nil {
		return nil, err
	}
	oc, err := s.orgContext(ctx, orgPK, docType)
	if err != nil {
		return nil, err
	}
	certB64, err := s.certToBase64(ctx, oc.certS3Key)
	if err != nil {
		return nil, err
	}
	return s.invokeAndParse(ctx, s.distPayload(oc, certB64, docType, map[string]any{
		"consNSU": map[string]any{"NSU": fmt.Sprintf("%015d", nsu)},
	}))
}

// EnqueueLookupByKey validates the rate limit then enqueues a background
// consChNFe call for the given access key — the async counterpart of the
// deleted synchronous LookupByKey. Mirrors EnqueueSync's shape but publishes
// job_type "cons_ch_nfe" (worker/internal/service/distribution.go
// runConsAccessKey) instead of "dist_nsu".
func (s *DistributionService) EnqueueLookupByKey(ctx context.Context, orgPK, accessKey string) (map[string]any, error) {
	if s.queueURL == "" {
		return nil, problem.BadRequest("fila de distribuição não configurada")
	}
	if err := s.checkConsQuota(ctx, orgPK, DocTypeNFe); err != nil {
		return nil, err
	}

	msg := map[string]any{
		"job_type":     "cons_ch_nfe",
		"org_pk":       orgPK,
		"doc_type":     DocTypeNFe,
		"access_key":   accessKey,
		"trigger":      "user",
		"triggered_at": time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, problem.InternalServer("failed to encode lookup message").WithCause(err)
	}
	if _, err := s.clients.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		return nil, problem.InternalServer("failed to enqueue lookup: " + err.Error())
	}
	return map[string]any{"status": "enqueued"}, nil
}

// --- internal helpers ---

type distOrgCtx struct {
	cnpj, uf, cUF, sefazEnv string
	environment             int
	certS3Key, certPassword string
}

func (s *DistributionService) orgContext(ctx context.Context, orgPK, docType string) (*distOrgCtx, error) {
	org, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, problem.NotFound("organização não encontrada")
	}
	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, problem.NoCertificate("certificado digital não encontrado")
	}
	config, err := s.fiscalCfg(docType).Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, problem.BadRequest(fmt.Sprintf("configure o %s em Configuração Fiscal antes de usar distribuição", strings.ToUpper(docType)))
	}
	environment := distIntAttr(config, "environment", 2)
	sefazEnv := SefazEnvHom
	if environment == 1 {
		sefazEnv = SefazEnvProd
	}
	// Off the record: the key is a company id since ADR 0022.
	cnpj, _ := IssuerDocAV(org, orgPK)
	uf := distExtractUF(org)
	cUF := UFCode[uf]
	if cUF == "" {
		cUF = "35"
	}
	cert := certs[0]
	return &distOrgCtx{
		cnpj: cnpj, uf: uf, cUF: cUF,
		environment: environment, sefazEnv: sefazEnv,
		certS3Key:    distStrAttr(cert, "s3_key"),
		certPassword: distStrAttr(cert, "password"),
	}, nil
}

func (s *DistributionService) distPayload(oc *distOrgCtx, certB64, docType string, query map[string]any) map[string]any {
	xmlns := docTypeXMLNS[docType]
	dist := map[string]any{
		"@versao":  "1.01",
		"@xmlns":   xmlns,
		"tpAmb":    fmt.Sprintf("%d", oc.environment),
		"cUFAutor": oc.cUF,
		"CNPJ":     oc.cnpj,
	}
	for k, v := range query {
		dist[k] = v
	}
	return map[string]any{
		"cnpj": oc.cnpj, "uf": oc.uf,
		"environment":          oc.sefazEnv,
		"doc_type":             docType,
		"service":              docTypeSefazService[docType],
		"certificate_b64":      certB64,
		"certificate_password": oc.certPassword,
		"validate_schema":      false,
		"max_retries":          2,
		"body":                 map[string]any{"distDFeInt": dist},
	}
}

func (s *DistributionService) checkConsQuota(ctx context.Context, orgPK, docType string) error {
	cfgRepo := s.fiscalCfg(docType)
	config, err := cfgRepo.Get(ctx, orgPK)
	if err != nil {
		return err
	}
	envPrefix := EnvHom
	if config != nil && distIntAttr(config, "environment", 2) == 1 {
		envPrefix = EnvProd
	}
	if config != nil {
		windowStart := distStrAttr(config, envPrefix+"_cons_quota_window_start")
		if windowStart != "" {
			if ws, err := time.Parse(time.RFC3339, windowStart); err != nil {
				return fmt.Errorf("invalid distribution quota window: %w", err)
			} else if time.Since(ws) >= time.Hour {
				if err := cfgRepo.ResetConsQuota(ctx, orgPK, envPrefix); err != nil {
					return err
				}
			}
		}
	}
	if count := cfgRepo.IncrementConsQuota(ctx, orgPK, envPrefix); count > distConsQuotaMax {
		return problem.TooManyRequests("Limite de 20 consultas/hora (consNSU/consChNFe) atingido.")
	}
	return nil
}

func (s *DistributionService) certToBase64(ctx context.Context, s3Key string) (string, error) {
	out, err := s.clients.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketCerts),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return "", problem.NoCertificate("certificado não encontrado no armazenamento")
	}
	defer closeReadCloser(ctx, out.Body, "distribution certificate S3 download")
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, out.Body); err != nil {
		return "", problem.InternalServer("failed to read certificate")
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (s *DistributionService) downloadDocs(ctx context.Context, s3Key string) ([]byte, error) {
	out, err := s.clients.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketDocs),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return nil, problem.NotFound("arquivo não encontrado no armazenamento")
	}
	defer closeReadCloser(ctx, out.Body, "distribution document S3 download")
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, out.Body); err != nil {
		return nil, problem.InternalServer("failed to read S3 object")
	}
	return buf.Bytes(), nil
}

func (s *DistributionService) invokeAndParse(ctx context.Context, payload map[string]any) (map[string]any, error) {
	respBody, err := invokeSefazLambda(ctx, s.clients.Lambda, s.sefazFunctionName, payload)
	if err != nil {
		return nil, err
	}

	ret := map[string]any{}
	if respBody != nil {
		if r, ok := respBody["retDistDFeInt"].(map[string]any); ok {
			ret = r
		}
	}
	lote, _ := ret["loteDistDFeInt"].(map[string]any)
	var docZips []any
	if lote != nil {
		switch dz := lote["docZip"].(type) {
		case []any:
			docZips = dz
		case map[string]any:
			docZips = []any{dz}
		}
	}
	docs := make([]map[string]any, 0, len(docZips))
	for _, d := range docZips {
		if dm, ok := d.(map[string]any); ok {
			nsu := 0
			if n, ok := dm["@NSU"].(string); ok {
				if _, err := fmt.Sscanf(n, "%d", &nsu); err != nil {
					slog.Warn("distribution NSU parse failed", "err", err)
				}
			}
			docs = append(docs, map[string]any{"nsu": nsu, "schema": dm["@schema"]})
		}
	}
	return map[string]any{
		"c_stat":   fmt.Sprintf("%v", ret["cStat"]),
		"x_motivo": ret["xMotivo"],
		"ult_nsu":  ret["ultNSU"],
		"max_nsu":  ret["maxNSU"],
		"docs":     docs,
	}, nil
}

// --- DynamoDB attribute helpers (scoped to this file to avoid import cycles) ---

func distStrAttr(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

// ImportXML validates an uploaded NF-e/NFC-e XML, stages it in S3, and
// enqueues an "import_xml" distribution job — the worker (runImportXML,
// worker/internal/service/distribution.go) does the actual classification/
// digest-check/persistence. See docs/specs/2026-08-13-importacao-nfe-xml.md.
func (s *DistributionService) ImportXML(ctx context.Context, orgPK, docType string, xmlBytes []byte) (map[string]any, error) {
	if !validImportDocType(docType) {
		return nil, problem.BadRequest("doc_type inválido para importação por XML: " + docType)
	}
	if len(xmlBytes) == 0 {
		return nil, problem.BadRequest("arquivo XML vazio")
	}
	if len(xmlBytes) > maxImportXMLSize {
		return nil, problem.PayloadTooLarge("arquivo XML excede o limite de 1 MiB")
	}
	root, err := peekXMLRoot(xmlBytes)
	if err != nil || (root != "nfeProc" && root != "NFe") {
		return nil, problem.BadRequest("XML inválido: raiz deve ser nfeProc ou NFe")
	}
	if s.queueURL == "" {
		return nil, problem.BadRequest("fila de distribuição não configurada")
	}
	if err := s.checkConsQuota(ctx, orgPK, docType); err != nil {
		return nil, err
	}

	// staging não precisa de env (hom/prod) no path — é uma área de espera
	// efêmera; o worker (runImportXML) já resolve o ambiente de novo a
	// partir do fiscal config ao processar o job.
	// The path segment is the partition key, whatever era it belongs to. Nothing
	// reads this object later — it is staging — so it needs no migration and no
	// stable shape.
	stagingKey := fmt.Sprintf("%s-import-staging/%s/%s.xml", docType, orgPK, repositories.GenerateID())
	if _, err := s.clients.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketDocs),
		Key:         aws.String(stagingKey),
		Body:        bytes.NewReader(xmlBytes),
		ContentType: aws.String("application/xml"),
	}); err != nil {
		return nil, problem.InternalServer("falha ao enviar XML: " + err.Error())
	}

	msg := map[string]any{
		"job_type":     "import_xml",
		"org_pk":       orgPK,
		"doc_type":     docType,
		"staging_key":  stagingKey,
		"trigger":      "user",
		"triggered_at": time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, problem.InternalServer("failed to encode import message").WithCause(err)
	}
	if _, err := s.clients.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		return nil, problem.InternalServer("failed to enqueue import: " + err.Error())
	}
	return map[string]any{"status": "enqueued"}, nil
}

func distIntAttr(item map[string]types.AttributeValue, key string, def int) int {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		var n int
		if _, err := fmt.Sscanf(v.Value, "%d", &n); err != nil {
			slog.Warn("distribution numeric attribute parse failed", "err", err)
		}
		return n
	}
	return def
}

func distExtractUF(org map[string]types.AttributeValue) string {
	person, ok := org["person"].(*types.AttributeValueMemberM)
	if !ok {
		return "SP"
	}
	if addrs, ok := person.Value["addresses"].(*types.AttributeValueMemberL); ok && len(addrs.Value) > 0 {
		if addr, ok := addrs.Value[0].(*types.AttributeValueMemberM); ok {
			if uf, ok := addr.Value["state_federation"].(*types.AttributeValueMemberS); ok && uf.Value != "" {
				return uf.Value
			}
		}
	}
	if addr, ok := person.Value["address"].(*types.AttributeValueMemberM); ok {
		if uf, ok := addr.Value["state_federation"].(*types.AttributeValueMemberS); ok {
			return uf.Value
		}
	}
	return "SP"
}
