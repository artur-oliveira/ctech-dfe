package bpes

// substitution.go: BP-e substituto (tpBPe 3, grupo infBPeSub). As regras F143 a F154 da NT 2025.002 (e as do MOC do comum) que se
// provam com o bilhete original da própria organização viram 422 aqui; o que depende da base da SEFAZ (duplicidade, estado) fica
// com ela. Quando o substituto é autorizado, o worker marca o original como `substituted` (SubstitutesAccessKey na mensagem).

import (
	"context"
	"time"

	"gopkg.aoctech.app/api-commons/problem"
)

const (
	// StatusSubstituted é o status do bilhete que foi substituído por outro autorizado.
	StatusSubstituted = "substituted"
	// eventStatusSuccess é o status de um evento homologado, como o worker o grava.
	eventStatusSuccess = "success"
	// maxEventsScan limita a leitura dos eventos do original na regra do não embarque.
	maxEventsScan = 50
)

// docFacts são os dados do bilhete que o substituto precisa repetir (F152 e F154), guardados na linha do documento.
type docFacts struct{ originUF, originIBGE, destUF, destIBGE, buyerDoc string }

func factsOf(origin, dest BpePlace, buyer *BpeBuyer) docFacts {
	f := docFacts{originUF: origin.UF, originIBGE: origin.IBGECode, destUF: dest.UF, destIBGE: dest.IBGECode}
	if buyer != nil {
		switch {
		case buyer.CNPJ != nil && *buyer.CNPJ != "":
			f.buyerDoc = docAlnum(*buyer.CNPJ)
		case buyer.CPF != nil && *buyer.CPF != "":
			f.buyerDoc = onlyDigits(*buyer.CPF)
		case buyer.ForeignID != nil:
			f.buyerDoc = *buyer.ForeignID
		}
	}
	return f
}

// attrs são os atributos gravados na linha do documento.
func (f docFacts) attrs() map[string]any {
	return map[string]any{
		"origin_uf": f.originUF, "origin_ibge": f.originIBGE, "dest_uf": f.destUF, "dest_ibge": f.destIBGE, "buyer_doc": f.buyerDoc,
	}
}

// checkSubstitution confere o bilhete original: existe na organização (o mesmo CNPJ e IE do emitente, F150 e F151), está
// autorizado (F146 e F147), é da mesma variante (o TA só substitui TA e o comum não substitui TA, F148 e 470; o TM não é
// substituído), repete UF, município e comprador (F152 e F154) e, se o embarque já passou, tem o evento de não embarque (F149).
func (s *BpeService) checkSubstitution(ctx context.Context, orgPK, variant string, sub *BpeSubstitution, f docFacts, now time.Time) error {
	orig, err := s.GetBPe(ctx, orgPK, sub.OriginalKey)
	if err != nil {
		return err
	}
	if orig == nil {
		return problem.UnprocessableEntity("o BP-e a substituir não existe nesta organização")
	}
	if st := strAttr(orig, "status"); st != StatusAuthorized {
		return problem.UnprocessableEntity("o BP-e a substituir deve estar autorizado (situação atual: " + st + ")")
	}
	origVariant := strAttr(orig, "variant")
	if origVariant == "" {
		origVariant = VariantNormal
	}
	if origVariant == VariantTM || origVariant != variant {
		return problem.UnprocessableEntity("o BP-e a substituir deve ser da mesma variante do substituto (comum com comum, TA com TA)")
	}
	// Linhas anteriores a estes atributos não os trazem: a comparação fica com a SEFAZ.
	if strAttr(orig, "origin_ibge") != "" {
		o := docFacts{
			originUF: strAttr(orig, "origin_uf"), originIBGE: strAttr(orig, "origin_ibge"),
			destUF: strAttr(orig, "dest_uf"), destIBGE: strAttr(orig, "dest_ibge"), buyerDoc: strAttr(orig, "buyer_doc"),
		}
		if o.originUF != f.originUF || o.originIBGE != f.originIBGE || o.destUF != f.destUF || o.destIBGE != f.destIBGE {
			return problem.UnprocessableEntity("UF e município de início e fim da viagem devem ser iguais aos do BP-e substituído")
		}
		if o.buyerDoc != f.buyerDoc {
			return problem.UnprocessableEntity("o comprador deve ser o mesmo do BP-e substituído (CNPJ, CPF ou documento estrangeiro)")
		}
	}
	boarding, err := dateTime(strAttr(orig, "boarding"))
	if err != nil || boarding.After(now) {
		return nil
	}
	has, err := s.hasNoShowEvent(ctx, sub.OriginalKey)
	if err != nil {
		return err
	}
	if !has {
		return problem.UnprocessableEntity("o embarque do BP-e substituído já passou: registre antes o evento de não embarque")
	}
	return nil
}

// hasNoShowEvent diz se o BP-e tem o evento de não embarque homologado.
func (s *BpeService) hasNoShowEvent(ctx context.Context, accessKey string) (bool, error) {
	res, err := s.eventRepo.GetDocumentEvents(ctx, accessKey, maxEventsScan, nil)
	if err != nil {
		return false, err
	}
	for _, ev := range res.Items {
		if strAttr(ev, "event_type") == TpEventoNaoEmbarque && strAttr(ev, "status") == eventStatusSuccess {
			return true, nil
		}
	}
	return false, nil
}

// validateCommonSubstitution: no bilhete comum o tpSub é 1, 2 ou 3 (os demais valem só no TA).
func validateCommonSubstitution(s *BpeSubstitution) error {
	if s == nil {
		return nil
	}
	switch s.Type {
	case "1", "2", "3":
		return nil
	}
	return problem.BadRequest("no BP-e comum o tipo de substituição é 1 remarcação, 2 transferência ou 3 transferência e remarcação")
}
