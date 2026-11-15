package nfses

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/validation"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// Rejeições do Emissor Nacional que dá para prever sem consultar o fisco. Cada
// regra aqui evita queimar um número de DPS (a numeração é reservada na
// emissão). Só entram regras determinísticas, checáveis com os dados que a
// emissão já tem; as que dependem de cadastro externo (Receita, município)
// continuam sendo decididas pelo fisco. Códigos do guia de erros da NFS-e Nacional.
const (
	maxAliqISSQN  = 5.0 // E0429: alíquota do ISSQN não pode passar de 5%
	tpRetNaoRet   = 1   // tpRetISSQN: 1 = não retido; 2 e 3 = retido
	msgTpRetISSQN = "E0204: com retenção de ISSQN é obrigatório informar o CNPJ ou CPF do tomador"
)

const (
	tribISSQNTributavel = 1 // tribISSQN: 1 = tributável; 2 a 4 = imunidade, exportação, não incidência
	regApTribSNSimples  = 1 // regApTribSN: ISSQN apurado pelo Simples Nacional
)

// dropAliqNaoPermitida limpa a alíquota nos casos em que o Sefin rejeita a DPS
// se ela vier preenchida: o Sefin calcula o ISSQN sozinho. Vale com o município
// ativo ou não no Sistema Nacional (E0600, E0602, E0604, E0625, E0631).
func dropAliqNaoPermitida(doc *nfse.Document) {
	reg := doc.Prestador.RegTrib
	trib := &doc.Valores.Trib.TribMun
	semAliq := reg.OpSimpNac == opSimpNacMEI || // E0600
		trib.TribISSQN != tribISSQNTributavel || // E0602
		reg.RegEspTrib != 0 || // E0604
		(reg.OpSimpNac == opSimpNacMEEPP && reg.RegApTribSN == regApTribSNSimples &&
			trib.TpRetISSQN == tpRetNaoRet) // E0625, E0631
	if semAliq {
		trib.PAliq = ""
	}
}

// preValidate devolve a primeira regra violada como 400 com o código do fisco
// na mensagem, para o usuário corrigir antes de gastar a numeração.
func preValidate(doc nfse.Document) error {
	checkDoc := func(who string, p *nfse.Pessoa, codeCNPJ, codeCPF string) error {
		if p == nil {
			return nil
		}
		if p.CNPJ != "" && !validation.ValidCNPJ(p.CNPJ) {
			return problem.BadRequest(fmt.Sprintf("%s: o CNPJ do %s é inválido", codeCNPJ, who))
		}
		if p.CPF != "" && !validation.ValidCPF(p.CPF) {
			return problem.BadRequest(fmt.Sprintf("%s: o CPF do %s é inválido", codeCPF, who))
		}
		return nil
	}
	if err := checkDoc("prestador", &doc.Prestador.Pessoa, "E0080", "E0096"); err != nil {
		return err
	}
	if err := checkDoc("tomador", doc.Tomador, "E0188", "E0206"); err != nil {
		return err
	}
	if err := checkDoc("intermediário", doc.Intermediario, "E0254", "E0254"); err != nil {
		return err
	}

	prest := pessoaDoc(&doc.Prestador.Pessoa)
	if prest != "" && prest == pessoaDoc(doc.Tomador) {
		return problem.BadRequest("E0202: o prestador e o tomador do serviço não podem ser a mesma pessoa ou empresa")
	}
	if prest != "" && prest == pessoaDoc(doc.Intermediario) {
		return problem.BadRequest("E0250: o intermediário não pode ser igual ao prestador")
	}

	// dhEmi e competência são ISO: os 10 primeiros caracteres comparam como data.
	if len(doc.DhEmi) >= len(doc.Competencia) && doc.Competencia > doc.DhEmi[:len(doc.Competencia)] {
		return problem.BadRequest("E0015: a data de competência não pode ser posterior à data de emissão")
	}

	if strings.TrimSpace(doc.Servico.CServ.XDescServ) == "" {
		return problem.BadRequest("E0341: a descrição do serviço é obrigatória")
	}
	if v, err := strconv.ParseFloat(doc.Valores.VServPrest.VServ, 64); err != nil || v <= 0 {
		return problem.BadRequest("E0404: o valor do serviço deve ser maior que zero")
	}

	trib := doc.Valores.Trib.TribMun
	if a, err := strconv.ParseFloat(trib.PAliq, 64); err == nil && a > maxAliqISSQN {
		return problem.BadRequest("E0429: a alíquota do ISSQN não pode ser superior a 5%")
	}
	reg := doc.Prestador.RegTrib
	if reg.OpSimpNac == opSimpNacMEEPP && reg.RegApTribSN == regApTribSNSimples &&
		trib.TpRetISSQN > tpRetNaoRet && trib.TribISSQN == tribISSQNTributavel &&
		reg.RegEspTrib == 0 && strings.TrimSpace(trib.PAliq) == "" {
		return problem.BadRequest("E0621: com retenção de ISSQN, o prestador ME/EPP no Simples Nacional deve informar a alíquota")
	}
	if trib.TpRetISSQN > tpRetNaoRet && pessoaDoc(doc.Tomador) == "" {
		return problem.BadRequest(msgTpRetISSQN)
	}
	return nil
}

// pessoaDoc devolve o CNPJ ou CPF (só dígitos) da pessoa, ou "" sem identificação.
func pessoaDoc(p *nfse.Pessoa) string {
	if p == nil {
		return ""
	}
	if p.CNPJ != "" {
		return p.CNPJ
	}
	return p.CPF
}
