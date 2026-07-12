package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/awsclient"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

var docTypeSefazService = map[string]string{
	"nfe":  "NFeDistribuicaoDFe",
	"cte":  "CTeDistribuicaoDFe",
	"mdfe": "MDFeDistribuicaoDFe",
}

var docTypeXMLNS = map[string]string{
	"nfe":  "http://www.portalfiscal.inf.br/nfe",
	"cte":  "http://www.portalfiscal.inf.br/cte",
	"mdfe": "http://www.portalfiscal.inf.br/mdfe",
}

const distConsQuotaMax = 20

// DistributionService manages DFe distribution (NF-e/CT-e/MDF-e received from SEFAZ).
type DistributionService struct {
	orgRepo           *repositories.OrganizationRepository
	certRepo          *repositories.CertificateRepository
	NfeConfig         *repositories.NfeConfigRepository
	CteConfig         *repositories.CteConfigRepository
	MdfeConfig        *repositories.MdfeConfigRepository
	nfeDist           *repositories.NFeDistributionRepository
	cteDist           *repositories.CTeDistributionRepository
	mdfeDist          *repositories.MDFeDistributionRepository
	clients           *awsclient.Clients
	queueURL          string
	bucketDocs        string
	bucketCerts       string
	sefazFunctionName string
}

func NewDistributionService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	NfeConfig *repositories.NfeConfigRepository,
	CteConfig *repositories.CteConfigRepository,
	MdfeConfig *repositories.MdfeConfigRepository,
	nfeDist *repositories.NFeDistributionRepository,
	cteDist *repositories.CTeDistributionRepository,
	mdfeDist *repositories.MDFeDistributionRepository,
	clients *awsclient.Clients,
	queueURL, bucketDocs, bucketCerts, sefazFunctionName string,
) *DistributionService {
	return &DistributionService{
		orgRepo: orgRepo, certRepo: certRepo,
		NfeConfig: NfeConfig, CteConfig: CteConfig, MdfeConfig: MdfeConfig,
		nfeDist: nfeDist, cteDist: cteDist, mdfeDist: mdfeDist,
		clients:           clients,
		queueURL:          queueURL,
		bucketDocs:        bucketDocs,
		bucketCerts:       bucketCerts,
		sefazFunctionName: sefazFunctionName,
	}
}

func (s *DistributionService) fiscalCfg(docType string) *repositories.FiscalConfigRepository {
	switch docType {
	case "nfe":
		return &s.NfeConfig.FiscalConfigRepository
	case "cte":
		return &s.CteConfig.FiscalConfigRepository
	case "mdfe":
		return &s.MdfeConfig.FiscalConfigRepository
	}
	return nil
}

func (s *DistributionService) distRepo(docType string) *repositories.DistributionRepository {
	switch docType {
	case "nfe":
		return &s.nfeDist.DistributionRepository
	case "cte":
		return &s.cteDist.DistributionRepository
	case "mdfe":
		return &s.mdfeDist.DistributionRepository
	}
	return nil
}

func validateDistDocType(docType string) error {
	if _, ok := docTypeSefazService[docType]; !ok {
		return problem.BadRequest("doc_type inválido: " + docType)
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
	body, _ := json.Marshal(msg)
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

// GetDistributionXML downloads the stored XML for a specific NSU.
func (s *DistributionService) GetDistributionXML(ctx context.Context, orgPK, docType string, nsu int) ([]byte, error) {
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
	return s.downloadDocs(ctx, s3Key)
}

// LookupByNSU performs a synchronous consNSU against SEFAZ via the py-dfe Lambda.
func (s *DistributionService) LookupByNSU(ctx context.Context, orgPK, docType string, nsu int) (map[string]any, error) {
	if err := validateDistDocType(docType); err != nil {
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

// LookupByKey performs a synchronous consChNFe against SEFAZ via the py-dfe Lambda.
func (s *DistributionService) LookupByKey(ctx context.Context, orgPK, docType, accessKey string) (map[string]any, error) {
	if err := validateDistDocType(docType); err != nil {
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
		"consChNFe": map[string]any{"chNFe": accessKey},
	}))
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
	cnpj := StripPKPrefix(orgPK)
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
	config, _ := cfgRepo.Get(ctx, orgPK)
	envPrefix := EnvHom
	if config != nil && distIntAttr(config, "environment", 2) == 1 {
		envPrefix = EnvProd
	}
	if config != nil {
		windowStart := distStrAttr(config, envPrefix+"_cons_quota_window_start")
		if windowStart != "" {
			if ws, err := time.Parse(time.RFC3339, windowStart); err == nil && time.Since(ws) >= time.Hour {
				_ = cfgRepo.ResetConsQuota(ctx, orgPK, envPrefix)
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
	defer func() { _ = out.Body.Close() }()
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
	defer func() { _ = out.Body.Close() }()
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
				_, _ = fmt.Sscanf(n, "%d", &nsu)
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

func distIntAttr(item map[string]types.AttributeValue, key string, def int) int {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		var n int
		_, _ = fmt.Sscanf(v.Value, "%d", &n)
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
