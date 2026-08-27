package nfes

// references.go resolve o grupo ide/NFref. A regra de produto: o cliente
// escolhe uma nota da própria base e o tipo de referência é derivado do modelo
// do documento; formulário só existe para documento que o sistema nunca viu
// (NF modelo 1/1A em papel, nota de produtor, cupom de ECF).

import (
	"context"
	"fmt"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Tipos de referência (ide/NFref, leiauteNFe_v4.00).
const (
	refKindNFe    = "nfe"    // refNFe    — chave de 44 dígitos de NF-e/NFC-e
	refKindNFeSig = "nfesig" // refNFeSig — chave com sigilo do destinatário
	refKindNF     = "nf"     // refNF     — NF modelo 1/1A
	refKindNFP    = "nfp"    // refNFP    — NF de produtor rural
	refKindCTe    = "cte"    // refCTe    — chave de CT-e
	refKindECF    = "ecf"    // refECF    — cupom fiscal
)

// finNFeExigeRef: complementar, ajuste e devolução só existem contra um
// documento anterior (leiauteNFe_v4.00, regra B25a).
var finNFeExigeRef = map[string]bool{"2": true, "3": true, "4": true}

// buildNFref traduz as referências já resolvidas para os nós do XSD.
// Uma entrada com kind desconhecido é descartada em silêncio de propósito: o
// domínio é fechado pela validação do request (oneof), então chegar aqui com
// outro valor é bug, não input.
func buildNFref(refs []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(refs))
	for _, r := range refs {
		switch anyStr(r, "kind", "") {
		case refKindNFe:
			out = append(out, map[string]any{"refNFe": anyStr(r, "access_key", "")})
		case refKindNFeSig:
			out = append(out, map[string]any{"refNFeSig": anyStr(r, "access_key", "")})
		case refKindCTe:
			out = append(out, map[string]any{"refCTe": anyStr(r, "access_key", "")})
		case refKindNF:
			out = append(out, map[string]any{"refNF": map[string]any{
				"cUF": anyStr(r, "c_uf", ""), "AAMM": anyStr(r, "aamm", ""),
				"CNPJ": anyStr(r, "cnpj", ""), "mod": anyStr(r, "mod", ""),
				"serie": anyStr(r, "serie", ""), "nNF": anyStr(r, "n_nf", ""),
			}})
		case refKindNFP:
			inner := map[string]any{
				"cUF": anyStr(r, "c_uf", ""), "AAMM": anyStr(r, "aamm", ""),
			}
			if v := anyStr(r, "cnpj", ""); v != "" {
				inner["CNPJ"] = v
			} else {
				inner["CPF"] = anyStr(r, "cpf", "")
			}
			inner["IE"] = anyStr(r, "ie", "")
			inner["mod"] = anyStr(r, "mod", "")
			inner["serie"] = anyStr(r, "serie", "")
			inner["nNF"] = anyStr(r, "n_nf", "")
			out = append(out, map[string]any{"refNFP": inner})
		case refKindECF:
			out = append(out, map[string]any{"refECF": map[string]any{
				"mod": anyStr(r, "mod", ""), "nECF": anyStr(r, "n_ecf", ""),
				"nCOO": anyStr(r, "n_coo", ""),
			}})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// resolveNFRefs converte o request em entradas prontas para buildNFref. Uma
// referência com nfe_id vira refNFe lendo a nota da base; as demais passam
// direto.
func (s *NfeService) resolveNFRefs(ctx context.Context, orgPK, envPrefix string, refs []NfeRefBody) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(refs))
	for _, r := range refs {
		if r.NfeID == nil || *r.NfeID == "" {
			if r.Kind == nil {
				return nil, problem.BadRequest("nf_refs: informe nfe_id ou kind")
			}
			out = append(out, map[string]any{
				"kind": *r.Kind, "access_key": ptrStr(r.AccessKey),
				"c_uf": ptrStr(r.CUF), "aamm": ptrStr(r.AAMM), "cnpj": ptrStr(r.CNPJ),
				"cpf": ptrStr(r.CPF), "ie": ptrStr(r.IE), "mod": ptrStr(r.Mod),
				"serie": ptrStr(r.Serie), "n_nf": ptrStr(r.NNF),
				"n_ecf": ptrStr(r.NECF), "n_coo": ptrStr(r.NCOO),
			})
			continue
		}
		item, err := s.nfeRepo.Get(ctx, fmt.Sprintf("%s#%s", envPrefix, orgPK), *r.NfeID)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, problem.NotFound("documento referenciado não encontrado: " + *r.NfeID)
		}
		out = append(out, map[string]any{"kind": refKindNFe, "access_key": strAttr(item, "sk")})
	}
	return out, nil
}
