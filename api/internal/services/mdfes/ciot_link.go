package mdfes

// ciot_link.go: CIOT do MDF-e escolhido entre os declarados (ciot_id) ou emitido por fora (código e
// responsável informados). O vínculo (mdfe_access_key) entra na transação da emissão (LinkMdfeTx), então
// o MDF-e e o CIOT nunca divergem; o worker desfaz o vínculo se o MDF-e for rejeitado (spec CIOT, C5).

import (
	"context"
	"regexp"

	"gopkg.aoctech.app/api-commons/problem"
)

// ciotOpenStatuses são os estados do CIOT que aceitam vínculo (mesmos de ciots.openStates).
var ciotOpenStatuses = []string{"declared", "rectified"}

// ciotCodeRe é o padrão do XSD (infCIOT/CIOT): 12 dígitos.
var ciotCodeRe = regexp.MustCompile(`^[0-9]{12}$`)

// resolveCIOTs troca cada ciot_id pelo código e pelo responsável da linha declarada e devolve os ids a vincular.
// O responsável é o transportador (carrier_doc), para quem o identificador foi gerado.
func (s *MdfeService) resolveCIOTs(ctx context.Context, pk string, list []MdfeCIOT) ([]MdfeCIOT, []string, error) {
	out := make([]MdfeCIOT, 0, len(list))
	var links []string
	for _, c := range list {
		if c.CiotID == "" {
			if c.CIOT == "" || c.ResponsibleDoc == "" {
				return nil, nil, problem.BadRequest("informe o CIOT declarado (ciot_id) ou o código e o responsável do CIOT emitido por fora")
			}
			out = append(out, c)
			continue
		}
		if c.CIOT != "" || c.ResponsibleDoc != "" {
			return nil, nil, problem.BadRequest("informe ciot_id ou o código e o responsável do CIOT, não os dois")
		}
		if s.ciotRepo == nil {
			return nil, nil, problem.InternalServer("repositório de CIOT indisponível")
		}
		row, err := s.ciotRepo.Get(ctx, pk, c.CiotID)
		if err != nil {
			return nil, nil, err
		}
		if row == nil {
			return nil, nil, problem.NotFound("CIOT não encontrado: " + c.CiotID)
		}
		if st := strAttr(row, "status"); st != ciotOpenStatuses[0] && st != ciotOpenStatuses[1] {
			return nil, nil, problem.Conflict("o CIOT precisa estar declarado para entrar no MDF-e (situação: " + st + ")")
		}
		if strAttr(row, "pending_operation") != "" {
			return nil, nil, problem.Conflict("o CIOT tem uma operação em andamento")
		}
		if k := strAttr(row, "mdfe_access_key"); k != "" {
			return nil, nil, problem.Conflict("o CIOT já está vinculado ao MDF-e " + k)
		}
		code := strAttr(row, "code")
		if !ciotCodeRe.MatchString(code) {
			return nil, nil, problem.UnprocessableEntity("o identificador do CIOT declarado não tem os 12 dígitos que o leiaute do MDF-e exige")
		}
		out = append(out, MdfeCIOT{CIOT: code, ResponsibleDoc: strAttr(row, "carrier_doc")})
		links = append(links, c.CiotID)
	}
	if err := validateCIOTs(out); err != nil {
		return nil, nil, err
	}
	return out, links, nil
}
