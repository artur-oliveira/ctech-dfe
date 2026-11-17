package bpes

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"gopkg.aoctech.app/api-commons/problem"
)

var (
	passengersRe = regexp.MustCompile(`^[0-9]{1,6}$`)
	cfopRe       = regexp.MustCompile(`^[123567][0-9]([0-9][1-9]|[1-9][0-9])$`)
)

// validateTMBody confere o que as tags do DTO não alcançam no BP-e TM: números em texto, a competência, os contadores
// da catraca e a alíquota do ICMS. Erros 400 em texto claro; o portão fiscal, depois do builder, cuida do schema.
func validateTMBody(b BpeTMEmitBody) error {
	if !cfopRe.MatchString(b.CFOP) {
		return problem.BadRequest("CFOP inválido para o BP-e (primeiro dígito 1, 2, 3, 5, 6 ou 7; os dois últimos não podem ser 00)")
	}
	if _, err := time.Parse("2006-01-02", b.Competence); err != nil {
		return problem.BadRequest("competência inválida (use AAAA-MM-DD)")
	}
	for gi, g := range b.Groups {
		for ii, it := range g.Items {
			at := fmt.Sprintf("grupo %d, item %d", gi+1, ii+1)
			if !passengersRe.MatchString(it.Passengers) {
				return problem.BadRequest(at + ": passageiros deve ser um inteiro de 1 a 6 dígitos")
			}
			if _, err := nonNegative(it.Value); err != nil {
				return problem.BadRequest(at + ": valor inválido")
			}
			if it.ICMS.CST == "00" || it.ICMS.CST == "90" {
				if it.ICMS.Rate == nil {
					return problem.BadRequest(at + ": informe a alíquota do ICMS")
				}
				if _, err := nonNegative(*it.ICMS.Rate); err != nil {
					return problem.BadRequest(at + ": alíquota do ICMS inválida")
				}
			}
			if it.StartCounter != nil && it.EndCounter != nil {
				s, e1 := strconv.ParseUint(*it.StartCounter, 10, 64)
				f, e2 := strconv.ParseUint(*it.EndCounter, 10, 64)
				if e1 != nil || e2 != nil {
					return problem.BadRequest(at + ": contador fora do intervalo")
				}
				if f < s {
					return problem.BadRequest(at + ": o contador final não pode ser menor que o inicial")
				}
			}
		}
	}
	return nil
}
