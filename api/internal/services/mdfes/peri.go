package mdfes

// peri.go deriva o grupo infDoc/.../peri (produto perigoso) a partir dos itens
// da NF-e referenciada, cruzando o código do produto com o cadastro. O operador
// classifica o produto uma vez, na ONU; nunca redigita nada por viagem.

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/shopspring/decimal"
)

// Campos da classificação de produto perigoso no cadastro de produtos.
const (
	periFieldNOnu      = "peri_n_onu"
	periFieldXNomeAE   = "peri_x_nome_ae"
	periFieldXClaRisco = "peri_x_cla_risco"
	periFieldGrEmb     = "peri_gr_emb"
	periFieldQVolTipo  = "peri_q_vol_tipo"
)

// resolvePeri agrupa por número ONU e soma as quantidades. Ordem XSD:
// nONU, xNomeAE, xClaRisco, grEmb, qTotProd, qVolTipo.
func resolvePeri(items []parsedItem, byCode map[string]map[string]any) []map[string]any {
	type acc struct {
		node  map[string]any
		total decimal.Decimal
	}
	order := make([]string, 0, len(items))
	groups := make(map[string]*acc, len(items))

	for _, it := range items {
		prod := byCode[it.CProd]
		onu := periStr(prod, periFieldNOnu)
		if onu == "" {
			continue
		}
		g, ok := groups[onu]
		if !ok {
			g = &acc{total: decimal.Zero, node: map[string]any{
				"nONU":      onu,
				"xNomeAE":   periStr(prod, periFieldXNomeAE),
				"xClaRisco": periStr(prod, periFieldXClaRisco),
				"qVolTipo":  periStr(prod, periFieldQVolTipo),
			}}
			if gr := periStr(prod, periFieldGrEmb); gr != "" {
				g.node["grEmb"] = gr
			}
			groups[onu] = g
			order = append(order, onu)
		}
		if q, err := decimal.NewFromString(it.QCom); err == nil {
			g.total = g.total.Add(q)
		}
	}
	if len(order) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(order))
	for _, onu := range order {
		g := groups[onu]
		g.node["qTotProd"] = g.total.StringFixed(4)
		out = append(out, g.node)
	}
	return out
}

func periStr(prod map[string]any, key string) string {
	v, _ := prod[key].(string)
	return v
}

// resolveDocPeri monta o grupo peri de cada documento da carga, indexado pela
// chave de acesso. Os produtos são buscados uma vez por código, mesmo quando o
// mesmo item aparece em várias notas do manifesto.
func (s *MdfeService) resolveDocPeri(ctx context.Context, orgPK string, docs []*docCargo) (map[string][]map[string]any, error) {
	if s.productRepo == nil {
		return nil, nil
	}
	byCode := map[string]map[string]any{}
	for _, doc := range docs {
		for _, it := range doc.items {
			if it.CProd == "" {
				continue
			}
			if _, seen := byCode[it.CProd]; seen {
				continue
			}
			item, err := s.productRepo.GetByCode(ctx, orgPK, it.CProd)
			if err != nil {
				return nil, err
			}
			// Produto não cadastrado não é erro: a NF-e já foi autorizada com
			// ele, e nada obriga o item a existir no cadastro da organização.
			var attrs map[string]any
			if item != nil {
				if err := attributevalue.UnmarshalMap(item, &attrs); err != nil {
					return nil, err
				}
			}
			byCode[it.CProd] = attrs
		}
	}
	out := map[string][]map[string]any{}
	for _, doc := range docs {
		if peri := resolvePeri(doc.items, byCode); peri != nil {
			out[doc.accessKey] = peri
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
