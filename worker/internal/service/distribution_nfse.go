package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

const (
	// serviceNFSeDistribuicao espelha constants.ServiceNFSeDistribuicao do go-dfe.
	serviceNFSeDistribuicao = "NFSeDistribuicao"

	// providerNacional é o único provedor com ADN; ABRASF 2.04 não distribui.
	providerNacional = "nacional"

	// maxNfseDistBatches limita a paginação por invocação. O ADN não devolve
	// maxNSU, então a única condição natural de parada é o lote vazio; sem teto
	// uma organização com histórico grande estouraria o timeout do Lambda. O que
	// sobrar é buscado no próximo ciclo do scheduler.
	maxNfseDistBatches = 20

	// Chaves do nfse.Result serializado (go-dfe/nfse/result.go).
	fieldDistribuicao       = "distribuicao"
	fieldStatusDistribuicao = "status_distribuicao"
	fieldNSU                = "nsu"
	fieldTipoDocumento      = "tipo_documento"
	fieldTipoEvento         = "tipo_evento"
	fieldXML                = "xml"
)

type nfseDistItem struct {
	NSU           int64
	ChaveAcesso   string
	TipoDocumento string
	TipoEvento    string
	XML           string
}

type nfseDistBatch struct {
	Status string
	Items  []nfseDistItem
}

// buildNfseDistPayload monta o payload de dfe.Request para o ADN. Não há
// envelope distDFeInt: a distribuição de NFS-e é REST, e o corpo é lido por
// nfse.Dispatch a partir das chaves BodyKey* do go-dfe.
func buildNfseDistPayload(cnpj, certB64, certPassword, sefazEnv, provider string, nsu int64) map[string]any {
	return map[string]any{
		"cnpj":                 cnpj,
		"certificate_b64":      certB64,
		"certificate_password": certPassword,
		"uf":                   "", // NFS-e é competência municipal
		"environment":          sefazEnv,
		"doc_type":             docTypeNfse,
		"service":              serviceNFSeDistribuicao,
		"validate_schema":      false,
		"max_retries":          2,
		"body": map[string]any{
			nfse.BodyKeyProvider:     provider,
			nfse.BodyKeyNSU:          nsu,
			nfse.BodyKeyCNPJConsulta: cnpj,
		},
	}
}

// parseNfseDistResponse traduz o nfse.Result do go-dfe. Item malformado é
// ignorado: perder um NSU do lote é melhor que abortar o ciclo inteiro.
func parseNfseDistResponse(respBody map[string]any) nfseDistBatch {
	batch := nfseDistBatch{Status: strFromAny(respBody[fieldStatusDistribuicao])}

	raw, _ := respBody[fieldDistribuicao].([]any)
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			slog.Warn("item de distribuição não é objeto", "type", fmt.Sprintf("%T", r))
			continue
		}
		batch.Items = append(batch.Items, nfseDistItem{
			NSU:           int64(getFloat(m, fieldNSU)),
			ChaveAcesso:   strFromAny(m[fieldChaveAcesso]),
			TipoDocumento: strFromAny(m[fieldTipoDocumento]),
			TipoEvento:    strFromAny(m[fieldTipoEvento]),
			XML:           strFromAny(m[fieldXML]),
		})
	}
	return batch
}

func maxNSUOf(items []nfseDistItem) int64 {
	var m int64
	for _, it := range items {
		if it.NSU > m {
			m = it.NSU
		}
	}
	return m
}

// runNfseDistNSU consome a distribuição do ADN por cursor de NSU. Não reusa
// runDistNSU: aquele é SOAP distDFeInt com paginação ultNSU/maxNSU, cStat 137/238
// como fim de lote e punição por consumo indevido — nada disso existe no ADN, que
// é REST, sequencial por NSU e para quando o lote vem vazio. O que é comum
// (loadConfig, loadCert, getCertB64, claimDistNSUSlot, invokePyDfe, updateNSU)
// é reusado sem cópia.
func (s *DistributionService) runNfseDistNSU(ctx context.Context, orgPK, trigger string, dtcfg docTypeConfig) error {
	configTable := fmt.Sprintf("%s_organization_%s", s.cfg.TablePrefix, dtcfg.configTableSuffix)

	cfg, err := s.loadConfig(ctx, orgPK, configTable)
	if err != nil || cfg == nil {
		slog.Warn("no nfse cfg found", "org_pk", orgPK, "err", err)
		return nil
	}

	provider := attrS(cfg, "provider")
	if provider != providerNacional {
		slog.Info("provider sem ADN — distribuição ignorada", "org_pk", orgPK, "provider", provider)
		return nil
	}

	cert, err := s.loadCert(ctx, orgPK, dtcfg.configTableSuffix)
	if err != nil || cert == nil {
		slog.Warn("no certificate found", "org_pk", orgPK, "err", err)
		return nil
	}
	certB64, err := s.getCertB64(ctx, attrS(cert, "s3_key"))
	if err != nil {
		return fmt.Errorf("getCertB64: %w", err)
	}
	certPassword := attrS(cert, "password")

	_, cnpj, err := s.loadOrgIdentity(ctx, orgPK)
	if err != nil {
		return err
	}
	sefazEnv, envPrefix := sefazEnvHom, envHom
	if attrN(cfg, "environment", 2) == 1 {
		sefazEnv, envPrefix = sefazEnvProd, envProd
	}

	// Trigger de usuário já reservou o slot na API (mesmo contrato de runDistNSU).
	if trigger != triggerUser {
		claimed, err := s.claimDistNSUSlot(ctx, orgPK, configTable, cfg, envPrefix, time.Now().UTC())
		if err != nil || !claimed {
			slog.Info("rate limit active — stopping", "org_pk", orgPK, "doc_type", docTypeNfse)
			return nil
		}
	}

	// O cursor mora em organization_nfse_configs ({env}_nsu), igual à família
	// NF-e: homologação e produção têm sequências de NSU independentes.
	currentNSU := int64(attrN(cfg, envPrefix+"_nsu", 0))
	docPK := envPrefix + "#" + orgPK

	for range maxNfseDistBatches {
		// O ADN devolve documentos a partir do NSU informado — pede-se o próximo.
		resp, err := s.invokePyDfe(ctx, buildNfseDistPayload(cnpj, certB64, certPassword, sefazEnv, provider, currentNSU+1))
		if err != nil {
			return fmt.Errorf("invokePyDfe nfse: %w", err)
		}

		var respBody map[string]any
		var rawBody string
		if b, ok := resp["body"].(string); ok {
			rawBody = b
			if err := json.Unmarshal([]byte(b), &respBody); err != nil {
				return fmt.Errorf("unmarshal ADN response: %w", err)
			}
		}
		if statusCode := int(getFloat(resp, "statusCode")); statusCode != 200 {
			// Erro do ADN é terminal: repetir a chamada devolve a mesma recusa.
			slog.Error("ADN error", "org_pk", orgPK, "status", statusCode,
				"detail", mapStr(respBody, "detail", mapStr(respBody, "title", "Erro ADN")), "response_body", rawBody)
			return nil
		}

		batch := parseNfseDistResponse(respBody)
		if len(batch.Items) == 0 {
			slog.Info("nenhum documento novo", "org_pk", orgPK, "nsu", currentNSU, "status", batch.Status)
			return nil
		}

		for _, it := range batch.Items {
			// Erro de upload S3 aborta o ciclo sem avançar o cursor: o item
			// fica "não entregue" (não gravado no DynamoDB), e o próximo
			// ciclo pede o mesmo NSU de novo ao ADN — perder o upload e ainda
			// assim avançar o cursor perderia o XML para sempre.
			if err := s.persistNfseIncoming(ctx, docPK, orgPK, envPrefix, it, dtcfg); err != nil {
				return fmt.Errorf("persistNfseIncoming nsu=%d: %w", it.NSU, err)
			}
		}

		currentNSU = maxNSUOf(batch.Items)
		if err := s.updateNSU(ctx, orgPK, configTable, envPrefix, int(currentNSU)); err != nil {
			return fmt.Errorf("updateNSU nfse: %w", err)
		}
	}

	slog.Info("teto de lotes atingido — retoma no próximo ciclo", "org_pk", orgPK, "nsu", currentNSU)
	return nil
}

// persistNfseIncoming grava o XML recebido e o registro do NSU. Não passa por
// processDocZip: aquele descompacta o gzip+base64 do DistDFe e faz parsing de
// procNFe/resNFe, que não existem em NFS-e — o go-dfe já entrega o XML pronto.
//
// Falha de upload no S3 retorna erro (em vez de só logar) para que o chamador
// pare o ciclo sem avançar o cursor de NSU: sem isso o XML seria perdido para
// sempre, já que o ADN não permite re-consultar um NSU já ultrapassado.
func (s *DistributionService) persistNfseIncoming(
	ctx context.Context,
	docPK, orgPK, envPrefix string,
	it nfseDistItem,
	dtcfg docTypeConfig,
) error {
	item := map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: docPK},
		"nsu":         &types.AttributeValueMemberN{Value: strconv.FormatInt(it.NSU, 10)},
		"doc_type":    &types.AttributeValueMemberS{Value: docTypeNfse},
		"schema_type": &types.AttributeValueMemberS{Value: it.TipoDocumento},
		"created_at":  &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
	}
	if it.ChaveAcesso != "" {
		item["access_key"] = &types.AttributeValueMemberS{Value: it.ChaveAcesso}
	}
	if it.TipoEvento != "" {
		item["event_type"] = &types.AttributeValueMemberS{Value: it.TipoEvento}
	}

	// Mesma convenção dos demais docTypes (processDocZip): NSU_{015d} sob
	// {doc_type}-distribution/{env}/{org_pk}.
	s3Key := fmt.Sprintf("%s-distribution/%s/%s/NSU_%015d.xml", docTypeNfse, envPrefix, orgPK, it.NSU)
	if _, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.DocumentsBucket),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader([]byte(it.XML)),
		ContentType: aws.String(contentTypeXML),
	}); err != nil {
		return fmt.Errorf("upload S3 %s: %w", s3Key, err)
	}
	item["xml_s3_key"] = &types.AttributeValueMemberS{Value: s3Key}

	// Condicional: a re-entrega do SQS não pode duplicar o registro do NSU.
	// Falha aqui não é propagada — inclui o caso esperado de re-entrega
	// (ConditionalCheckFailedException), que não deve interromper o ciclo.
	table := s.cfg.TablePrefix + "_" + dtcfg.distTable
	if _, err := s.dynamo.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(table),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(pk) AND attribute_not_exists(nsu)"),
	}); err != nil {
		slog.Warn("NSU already exists or PutItem failed", "nsu", it.NSU, "table", table, "err", err)
	}
	return nil
}
