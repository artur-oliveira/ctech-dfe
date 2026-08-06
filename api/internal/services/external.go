package services

// ExternalService mirrors api/app/services/external.py.
// It performs NfeConsultaCadastro queries against SEFAZ using the org's certificate.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	cpfCNPJRe = regexp.MustCompile(`^\d{11}$|^\d{14}$`)
	ufRe      = regexp.MustCompile(`^(AC|AL|AM|AP|BA|CE|DF|ES|GO|MA|MG|MS|MT|PA|PB|PE|PI|PR|RJ|RN|RO|RR|RS|SC|SE|SP|TO)$`)

	// cStat codes from NfeConsultaCadastro indicating the document was found.
	foundStats    = map[string]bool{"111": true, "112": true}
	notFoundStats = map[string]bool{"259": true, "410": true}

	sitLabel = map[string]string{
		"1": "habilitado",
		"2": "credenciado",
		"3": "suspenso",
		"4": "inapto",
		"5": "bloqueado",
		"8": "baixado",
	}
)

// LookupAddress is a single address entry from NfeConsultaCadastro.
type LookupAddress struct {
	Street          *string `json:"street"`
	Number          *string `json:"number"`
	Complement      *string `json:"complement"`
	Neighborhood    *string `json:"neighborhood"`
	City            *string `json:"city"`
	PostalCode      *string `json:"postal_code"`
	StateFederation *string `json:"state_federation"`
	CityIBGECode    *string `json:"city_ibge_code"`
}

// LookupStateRegistration is a single state registration entry.
type LookupStateRegistration struct {
	UF                string `json:"uf"`
	StateRegistration string `json:"state_registration"`
}

// LookupOrganizationResult is the response shape for the external lookup endpoint.
type LookupOrganizationResult struct {
	CPFCNPJ            string                    `json:"cpf_cnpj"`
	Name               string                    `json:"name"`
	CRT                *int                      `json:"crt"`
	UF                 string                    `json:"uf"`
	Status             string                    `json:"status"`
	Addresses          []LookupAddress           `json:"addresses"`
	StateRegistrations []LookupStateRegistration `json:"state_registrations"`
}

// ExternalService wraps SEFAZ external lookup operations (NfeConsultaCadastro).
type ExternalService struct {
	certRepo          *repositories.CertificateRepository
	clients           *awsclient.Clients
	sefazFunctionName string
	s3BucketCerts     string
}

// NewExternalService constructs an ExternalService.
func NewExternalService(
	certRepo *repositories.CertificateRepository,
	clients *awsclient.Clients,
	sefazFunctionName string,
	s3BucketCerts string,
) *ExternalService {
	return &ExternalService{
		certRepo:          certRepo,
		clients:           clients,
		sefazFunctionName: sefazFunctionName,
		s3BucketCerts:     s3BucketCerts,
	}
}

// LookupOrganization queries SEFAZ NfeConsultaCadastro for cpfCNPJ in uf,
// using the org's certificate.
func (s *ExternalService) LookupOrganization(ctx context.Context, orgPK, cpfCNPJ, uf string) (*LookupOrganizationResult, error) {
	if !cpfCNPJRe.MatchString(cpfCNPJ) {
		return nil, problem.BadRequest("cpf_cnpj deve conter exatamente 11 (CPF) ou 14 (CNPJ) dígitos numéricos")
	}
	uf = strings.ToUpper(uf)
	if !ufRe.MatchString(uf) {
		return nil, problem.BadRequest(fmt.Sprintf("UF '%s' inválida", uf))
	}

	if s.sefazFunctionName == "" {
		return nil, problem.BadRequest("Serviço de consulta não configurado (sefaz_function_name)")
	}

	certB64, certPassword, err := s.CertificateB64(ctx, orgPK)
	if err != nil {
		return nil, err
	}

	orgCNPJ := StripPKPrefix(orgPK)

	isCNPJ := len(cpfCNPJ) == 14
	docKey := "CPF"
	if isCNPJ {
		docKey = "CNPJ"
	}

	payload := map[string]any{
		"cnpj":                 orgCNPJ,
		"certificate_b64":      certB64,
		"certificate_password": certPassword,
		"uf":                   uf,
		"environment":          "prod",
		"doc_type":             "nfe",
		"service":              "NfeConsultaCadastro",
		"validate_schema":      false,
		"max_retries":          2,
		"body": map[string]any{
			"ConsCad": map[string]any{
				"@versao": "2.00",
				"@xmlns":  "http://www.portalfiscal.inf.br/nfe",
				"infCons": map[string]any{
					"xServ": "CONS-CAD",
					"UF":    uf,
					docKey:  cpfCNPJ,
				},
			},
		},
	}

	respBody, err := s.invokeAndParse(ctx, payload)
	if err != nil {
		return nil, err
	}

	// Navigate retConsCad → infCons
	retConsCad := getMap(respBody, "retConsCad")
	infCons := getMap(retConsCad, "infCons")
	cStat := getStr(infCons, "cStat")

	if !foundStats[cStat] {
		xMotivo := getStr(infCons, "xMotivo")
		display := cStat + " - " + xMotivo
		if notFoundStats[cStat] {
			return nil, problem.NotFound(fmt.Sprintf("%s %s não localizado na UF %s: %s", docKey, cpfCNPJ, uf, display))
		}
		return nil, problem.SefazRejection(display)
	}

	infCadList := asSliceOfMaps(infCons, "infCad")
	if len(infCadList) == 0 {
		return nil, problem.NotFound(fmt.Sprintf("%s %s não localizado na UF %s", docKey, cpfCNPJ, uf))
	}

	var crt *int
	var addresses []LookupAddress
	var stateRegs []LookupStateRegistration
	seenUFs := map[string]bool{}

	for _, cad := range infCadList {
		cadUF := getStr(cad, "UF")
		if cadUF == "" {
			cadUF = uf
		}

		if xRegApur := getStr(cad, "xRegApur"); xRegApur != "" {
			v := 3
			if strings.Contains(strings.ToLower(xRegApur), "simples nacional") {
				v = 1
			}
			crt = &v
		}

		ender := getMap(cad, "ender")
		addresses = append(addresses, LookupAddress{
			Street:          optStr(getStr(ender, "xLgr")),
			Number:          optStr(getStr(ender, "nro")),
			Complement:      optStr(getStr(ender, "xCpl")),
			Neighborhood:    optStr(getStr(ender, "xBairro")),
			City:            optStr(getStr(ender, "xMun")),
			PostalCode:      optStr(getStr(ender, "CEP")),
			StateFederation: optStr(cadUF),
			CityIBGECode:    optStr(getStr(ender, "cMun")),
		})

		ie := getStr(cad, "IE")
		if ie != "" && !seenUFs[cadUF] {
			seenUFs[cadUF] = true
			stateRegs = append(stateRegs, LookupStateRegistration{UF: cadUF, StateRegistration: ie})
		}
	}

	primary := infCadList[0]
	statusLabel := sitLabel[getStr(primary, "cSit")]
	if statusLabel == "" {
		statusLabel = getStr(primary, "cSit")
	}

	if addresses == nil {
		addresses = []LookupAddress{}
	}
	if stateRegs == nil {
		stateRegs = []LookupStateRegistration{}
	}

	return &LookupOrganizationResult{
		CPFCNPJ:            firstNonEmpty(getStr(primary, docKey), cpfCNPJ),
		Name:               getStr(primary, "xNome"),
		CRT:                crt,
		UF:                 firstNonEmpty(getStr(primary, "UF"), uf),
		Status:             statusLabel,
		Addresses:          addresses,
		StateRegistrations: stateRegs,
	}, nil
}

// CertificateB64 returns the organization's first certificate base64-encoded
// plus its password — the exact pair `dfe.Request` wants. Exported for the
// services that call go-dfe synchronously from a handler (NFS-e municipal
// parameters, DANFSE) so the S3 read and the encoding are not re-implemented
// per doc type.
func (s *ExternalService) CertificateB64(ctx context.Context, orgPK string) (string, string, error) {
	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return "", "", err
	}
	if len(certs) == 0 {
		return "", "", problem.NoCertificate("organização sem certificado digital")
	}
	cert := certs[0]
	pfxBytes, err := s.downloadPFX(ctx, avStr(cert, "s3_key"))
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pfxBytes), avStr(cert, "password"), nil
}

func (s *ExternalService) downloadPFX(ctx context.Context, s3Key string) ([]byte, error) {
	out, err := s.clients.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.s3BucketCerts),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return nil, problem.NoCertificate("falha ao obter certificado digital")
	}
	defer func() { _ = out.Body.Close() }()
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, out.Body); err != nil {
		return nil, problem.InternalServer("failed to read certificate from S3")
	}
	return buf.Bytes(), nil
}

// pdfPlaceholderCNPJ satisfies py-dfe's required 14-digit cnpj field for
// render-only services. The render path ignores it (the PDF is derived entirely
// from the supplied XML), so a placeholder is used when the issuer is not a CNPJ
// (e.g. an MDF-e emitted by an autonomous carrier identified by CPF).
const pdfPlaceholderCNPJ = "00000000000000"

var digits14Re = regexp.MustCompile(`^\d{14}$`)

// GeneratePDF renders a fiscal document XML to PDF through py-dfe's render-only
// services (ServiceGerarDanfe for NF-e/NFC-e, ServiceGerarDamdfe for MDF-e). It
// invokes the same py-dfe Lambda used for SEFAZ calls — no certificate is sent —
// and returns the decoded PDF bytes. docType/service select the renderer; uf and
// cnpj only satisfy the request schema and are unused by the render path.
func (s *ExternalService) GeneratePDF(
	ctx context.Context, docType, service, uf, cnpj, xml string, canceled bool,
) ([]byte, error) {
	if s.sefazFunctionName == "" {
		return nil, problem.BadRequest("Serviço de geração de PDF não configurado (sefaz_function_name)")
	}
	respBody, err := s.invokeAndParse(ctx, pdfRequestPayload(docType, service, uf, cnpj, xml, canceled))
	if err != nil {
		return nil, err
	}
	return decodePDFResponse(respBody)
}

// pdfRequestPayload builds the py-dfe LambdaRequest for a render-only PDF service.
// uf and cnpj are normalized to satisfy the request schema (the render path
// ignores them); a non-CNPJ issuer falls back to a placeholder CNPJ.
func pdfRequestPayload(docType, service, uf, cnpj, xml string, canceled bool) map[string]any {
	if !digits14Re.MatchString(cnpj) {
		cnpj = pdfPlaceholderCNPJ
	}
	if len(uf) != 2 {
		uf = "AN"
	}
	return map[string]any{
		"cnpj":            cnpj,
		"uf":              uf,
		"environment":     "prod",
		"doc_type":        docType,
		"service":         service,
		"validate_schema": false,
		"max_retries":     0,
		"body": map[string]any{
			"xml":      xml,
			"canceled": canceled,
		},
	}
}

// decodePDFResponse extracts and base64-decodes the pdf_b64 field returned by
// py-dfe's render-only services.
func decodePDFResponse(respBody map[string]any) ([]byte, error) {
	b64, _ := respBody["pdf_b64"].(string)
	if b64 == "" {
		return nil, problem.InternalServer("resposta de geração de PDF sem conteúdo")
	}
	pdf, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, problem.InternalServer("falha ao decodificar PDF gerado")
	}
	return pdf, nil
}

func (s *ExternalService) invokeAndParse(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return invokeSefazLambda(ctx, s.clients.Lambda, s.sefazFunctionName, payload)
}

// invokeSefazLambda invokes a py-dfe Lambda, parses the response envelope,
// and returns the inner body map. Returns a *problem.Problem on non-200 responses.
func invokeSefazLambda(ctx context.Context, lam *lambda.Client, funcName string, payload map[string]any) (map[string]any, error) {
	body, _ := json.Marshal(payload)
	out, err := lam.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(funcName),
		Payload:      body,
	})
	if err != nil {
		return nil, problem.InternalServer("failed to invoke SEFAZ Lambda: " + err.Error())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Payload, &resp); err != nil {
		return nil, problem.InternalServer("invalid Lambda response")
	}
	statusCode := 500
	if sc, ok := resp["statusCode"].(float64); ok {
		statusCode = int(sc)
	}
	var respBody map[string]any
	bodyStr, _ := resp["body"].(string)
	if bodyStr != "" {
		_ = json.Unmarshal([]byte(bodyStr), &respBody)
	}

	shadowCallGoDfeFromMap(ctx, payload, statusCode, bodyStr)

	if statusCode != 200 {
		detail := "Erro na consulta à SEFAZ"
		if respBody != nil {
			if d, ok := respBody["detail"].(string); ok && d != "" {
				detail = d
			} else if t, ok := respBody["title"].(string); ok && t != "" {
				detail = t
			}
		}
		return nil, problem.BadRequest(detail)
	}
	return respBody, nil
}

// --- helpers ----------------------------------------------------------------

// avStr extracts a string from a DynamoDB attribute map.
func avStr(item map[string]types.AttributeValue, key string) string {
	if av, ok := item[key]; ok {
		if sv, ok := av.(*types.AttributeValueMemberS); ok {
			return sv.Value
		}
	}
	return ""
}

func getStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func getMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]any)
	return v
}

// asSliceOfMaps returns the value at key as []map[string]any.
// Handles both a single map and a JSON array.
func asSliceOfMaps(m map[string]any, key string) []map[string]any {
	if m == nil {
		return nil
	}
	val, ok := m[key]
	if !ok {
		return nil
	}
	if list, ok := val.([]any); ok {
		out := make([]map[string]any, 0, len(list))
		for _, item := range list {
			if mm, ok := item.(map[string]any); ok {
				out = append(out, mm)
			}
		}
		return out
	}
	if mm, ok := val.(map[string]any); ok {
		return []map[string]any{mm}
	}
	return nil
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
