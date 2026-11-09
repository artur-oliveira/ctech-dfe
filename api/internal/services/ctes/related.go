package ctes

// related.go: o detalhe do CT-e (GET /v1.0/ctes/{access_key}) com o que o registro sozinho não diz:
//   - a referência reversa: quais complementares e substitutos apontam para este CT-e;
//   - o expedidor e o recebedor, lidos do XML guardado (como o original.go lê os participantes).
//
// Referência reversa sem Scan e sem índice novo: na mesma transação da emissão do complementar ou do substituto
// (persist), uma linha por par (original, derivado) é gravada na própria tabela ctes, na partição do ambiente e da
// organização, com sk "ref#{chave do original}#{chave do derivado}". A leitura é um Query da chave de partição com
// begins_with em "ref#{chave}#". As linhas não têm number, incoming nem year, então não entram em number-index-v2
// nem em dfe-index; a chave de 44 dígitos das notas nunca começa com "ref#".

import (
	"context"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	refSKPrefix = "ref#"
	// maxRelatedLinks limita a leitura: o MOC admite até 10 complementos por original (G167) e 1 substituto.
	maxRelatedLinks = 50
)

// refSK é a sk da linha de referência original -> derivado.
func refSK(originalKey, childKey string) string { return refSKPrefix + originalKey + "#" + childKey }

// refPutItems monta as linhas de referência de um complementar (várias originais) ou de um substituto (uma).
func refPutItems(table, pk, childKey, cteType string, originalKeys []string, createdAt string) ([]types.TransactWriteItem, error) {
	out := make([]types.TransactWriteItem, 0, len(originalKeys))
	for _, o := range originalKeys {
		item, err := attributevalue.MarshalMap(map[string]any{
			"pk": pk, "sk": refSK(o, childKey), "original_key": o, "child_key": childKey,
			"cte_type": cteType, "created_at": createdAt,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(table), Item: item}})
	}
	return out, nil
}

// relatedLink é um derivado (complementar ou substituto) como o detalhe do original o mostra.
func relatedLink(child map[string]types.AttributeValue) types.AttributeValue {
	m := map[string]types.AttributeValue{}
	for _, k := range []string{"sk", "status", "number", "serie", "dh_emi", "cte_type"} {
		if v, ok := child[k]; ok {
			m[k] = v
		}
	}
	m["access_key"] = m["sk"]
	delete(m, "sk")
	return &types.AttributeValueMemberM{Value: m}
}

// partyAV é {doc, name} de um participante do XML, o mesmo formato de sender e recipient do registro.
func partyAV(p *nfexml.CTeActor) types.AttributeValue {
	return &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
		"doc": &types.AttributeValueMemberS{Value: p.Doc()}, "name": &types.AttributeValueMemberS{Value: p.Name},
	}}
}

// GetCTeDetail é o GetCTe com expedidor, recebedor, complemented_by e substituted_by. Falha ao ler o XML ou as
// referências não derruba o detalhe: o campo só não aparece (o registro continua sendo a verdade).
func (s *CteService) GetCTeDetail(ctx context.Context, orgPK, accessKey string) (map[string]types.AttributeValue, error) {
	item, err := s.GetCTe(ctx, orgPK, accessKey)
	if err != nil || item == nil {
		return item, err
	}
	s.addParticipants(ctx, item)
	if err := s.addRelated(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// addParticipants lê expedidor e recebedor do XML autorizado (modelo 57); o CT-e OS não os tem.
func (s *CteService) addParticipants(ctx context.Context, item map[string]types.AttributeValue) {
	s3Key := strAttr(item, "xml_s3_key")
	if s3Key == "" || strAttr(item, "model") == ModelCTeOS {
		return
	}
	data, err := services.DownloadS3(ctx, s.clients, s.bucketDocs, s3Key)
	if err != nil {
		return
	}
	cte, err := nfexml.ParseCTe(data)
	if err != nil {
		return
	}
	if cte.Exped != nil {
		item["expedidor"] = partyAV(cte.Exped)
	}
	if cte.Receb != nil {
		item["recebedor"] = partyAV(cte.Receb)
	}
}

// addRelated acrescenta os derivados que apontam para este CT-e. Só CT-e emitido pode ter derivados.
func (s *CteService) addRelated(ctx context.Context, item map[string]types.AttributeValue) error {
	if intAttr(item, "incoming", 0) == 1 {
		return nil
	}
	key, pk := strAttr(item, "sk"), strAttr(item, "pk")
	res, err := s.cteRepo.Query(ctx, repositories.QueryOpts{
		PK: pk, SKPrefix: refSKPrefix + key + "#", ScanIndexForward: true, Limit: maxRelatedLinks,
	})
	if err != nil {
		return err
	}
	var complements, substitutes []types.AttributeValue
	for _, ref := range res.Items {
		child, err := s.cteRepo.Get(ctx, pk, strAttr(ref, "child_key"))
		if err != nil {
			return err
		}
		if child == nil {
			continue
		}
		if strAttr(child, "cte_type") == cteTypeSubstitute {
			substitutes = append(substitutes, relatedLink(child))
		} else {
			complements = append(complements, relatedLink(child))
		}
	}
	item["complemented_by"] = &types.AttributeValueMemberL{Value: complements}
	item["substituted_by"] = &types.AttributeValueMemberL{Value: substitutes}
	return nil
}
