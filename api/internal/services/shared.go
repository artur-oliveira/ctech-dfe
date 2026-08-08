package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Environment and SEFAZ string constants shared across all DFe services.
const (
	EnvProd      = "prod"
	EnvHom       = "hom"
	SefazEnvProd = "producao"
	SefazEnvHom  = "homologacao"
)

// EnvToPrefix converts the environment code (1=produção, 2=homologação) to the
// prefix used by every per-environment config field (`{prefix}_current_number`,
// `{prefix}_nsu`, …) and by the document PK (`{prefix}#{org_pk}`).
func EnvToPrefix(environment int) string {
	if environment == 1 {
		return EnvProd
	}
	return EnvHom
}

// DownloadS3 reads an object from the documents bucket into memory. Shared by
// every DFe service that serves stored XML/PDF (NF-e, MDF-e, NFS-e); a 404 is
// reported as NotFound because the only caller-visible cause is a key written
// by the worker that has not landed yet.
func DownloadS3(ctx context.Context, clients *awsclient.Clients, bucket, s3Key string) ([]byte, error) {
	out, err := clients.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return nil, problem.NotFound("arquivo não encontrado no armazenamento")
	}
	defer func(body io.ReadCloser) { _ = body.Close() }(out.Body)
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(out.Body); err != nil {
		return nil, problem.InternalServer("failed to read S3 object")
	}
	return buf.Bytes(), nil
}

// Fiscal document model codes (campo "mod" da chave de acesso).
const (
	ModelMDFe = "58"
)

// py-dfe doc_type values (LambdaRequest.doc_type).
const (
	DocTypeNFe  = "nfe"
	DocTypeNFCe = "nfce"
	DocTypeCTe  = "cte"
	DocTypeMDFe = "mdfe"
	DocTypeNfse = "nfse"
)

// py-dfe render-only service keys (PDF generation; no certificate required).
const (
	ServiceGerarDanfe  = "GerarDanfe"  // DANFE (NF-e mod 55) / DANFC-e (NFC-e mod 65)
	ServiceGerarDamdfe = "GerarDamdfe" // DAMDFE (MDF-e mod 58)
)

// StatusCancelled is the final document status written by the worker after a
// successful cancellation event, shared across all doc types.
const StatusCancelled = "cancelled"

// UFCode maps UF abbreviation to IBGE cUF code.
var UFCode = map[string]string{
	"AC": "12", "AL": "27", "AM": "13", "AP": "16",
	"BA": "29", "CE": "23", "DF": "53", "ES": "32",
	"GO": "52", "MA": "21", "MG": "31", "MS": "50",
	"MT": "51", "PA": "15", "PB": "25", "PE": "26",
	"PI": "22", "PR": "41", "RJ": "33", "RN": "24",
	"RO": "11", "RR": "14", "RS": "43", "SC": "42",
	"SE": "28", "SP": "35", "TO": "17",
}

// UFFromCode is the reverse of UFCode: IBGE cUF code → UF abbreviation.
var UFFromCode = func() map[string]string {
	m := make(map[string]string, len(UFCode))
	for uf, code := range UFCode {
		m[code] = uf
	}
	return m
}()

// attributeMapToPlain converts a DynamoDB attribute map to a plain Go map, for
// diffing against a JSON-shaped `updates` map from a request body.
func attributeMapToPlain(item map[string]types.AttributeValue) (map[string]any, error) {
	var m map[string]any
	if err := attributevalue.UnmarshalMap(item, &m); err != nil {
		return nil, problem.InternalServer("failed to unmarshal item for audit diff")
	}
	return m, nil
}

// attrStrAV extracts a string attribute from a DynamoDB item, or "" if absent.
// Duplicated from internal/api/v1/helpers.go's attrStr (same one-liner) because
// v1 imports services — importing v1 from here would create a cycle.
func attrStrAV(item map[string]types.AttributeValue, key string) string {
	if av, ok := item[key].(*types.AttributeValueMemberS); ok {
		return av.Value
	}
	return ""
}

// StripPKPrefix removes "CNPJ_" or "CPF_" prefix from a DynamoDB PK.
func StripPKPrefix(pk string) string {
	for _, p := range []string{"CNPJ_", "CPF_"} {
		if strings.HasPrefix(pk, p) {
			return strings.TrimPrefix(pk, p)
		}
	}
	return pk
}

// CalcMod11DV computes the mod-11 check digit for a 43-char DFe access key.
// Shared by all document types (NF-e, NFC-e, CT-e, MDF-e).
func CalcMod11DV(key43 string) string {
	weights := []int{2, 3, 4, 5, 6, 7, 8, 9}
	total := 0
	wi := 0
	for i := len(key43) - 1; i >= 0; i-- {
		total += int(key43[i]-'0') * weights[wi%len(weights)]
		wi++
	}
	rem := total % 11
	if rem < 2 {
		return "0"
	}
	return fmt.Sprintf("%d", 11-rem)
}

// GenerateAccessKey builds a 44-digit DFe access key:
// cUF(2) + AAMM(4) + CNPJ(14) + mod(2) + serie(3) + num(9) + tpEmis(1) + cNF(8) + cDV(1).
// model is one of ModelNFe/ModelNFCe/ModelCTe/ModelMDFe. Returns "" if uf is unknown.
func GenerateAccessKey(uf, cnpj, model string, serie, number int, now time.Time) string {
	cUF, ok := UFCode[uf]
	if !ok {
		return ""
	}
	aamm := now.Format("0601") // Go layout: "06"=2-digit year, "01"=2-digit month
	cnpj = StripPKPrefix(cnpj)
	if len(cnpj) > 14 {
		cnpj = cnpj[:14]
	}
	for len(cnpj) < 14 {
		cnpj += "0"
	}
	cNF := fmt.Sprintf("%08d", 10_000_000+rand.Intn(90_000_000))
	key43 := fmt.Sprintf("%s%s%s%s%03d%09d1%s", cUF, aamm, cnpj, model, serie, number, cNF)
	return key43 + CalcMod11DV(key43)
}
