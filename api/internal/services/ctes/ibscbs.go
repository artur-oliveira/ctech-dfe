package ctes

// ibscbs.go: grupo IBSCBS (TTribCTe) do imp, vTotDFe e autXML do CT-e. Nada é presumido: CST, classificação,
// base de cálculo e alíquotas vêm do usuário; o servidor só calcula os valores (vBC x alíquota) e os totais.
// Escopo: grupo regular gIBSCBS (IBS UF, IBS município, CBS). Ficam de fora, por exigirem decisão fiscal que o
// MOC não fecha para o CT-e: diferimento, devolução, redução, ALC/ZFM, tributação regular, compra governamental
// (CGOV), estorno de crédito e o Simples Nacional (gTribSN, SN-1101/1103).

import (
	"fmt"
	"slices"
	"strings"

	"gopkg.aoctech.app/dfe/api/internal/validation"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"
)

const (
	// Códigos (problem.code) de 422.
	CodeIBSCBSRegime   = "ibscbs_regime_unsupported"
	CodeAuthXMLInvalid = "autxml_invalid"

	// MaxAuthorizedXML é o limite de autXML do leiaute (0-10).
	MaxAuthorizedXML = 10

	// crtRegimeNormal é o único CRT em que o MOC admite gIBSCBS sem gTribSN (SN-1100, SN-1107): CRT 1 e 2 sem
	// tpRegimeIBSCBS exigem gTribSN, e CRT 4 (MEI) não admite o grupo.
	crtRegimeNormal = 3

	// anoTotalComTributos é o ano a partir do qual vTotDFe soma IBS e CBS (MOC, regra IBSCBS-365).
	anoTotalComTributos = 2027
)

// CteIBSCBS é o IBS/CBS informado pelo usuário. Taxas em percentual (ex.: 0.1000).
type CteIBSCBS struct {
	CST        string `json:"cst" validate:"required,len=3,numeric"`
	ClassTrib  string `json:"class_trib" validate:"required,len=6,numeric"`
	Donation   bool   `json:"donation"`
	BaseValue  string `json:"base_value" validate:"required,money2"`
	IBSUFRate  string `json:"ibs_uf_rate" validate:"required,percent"`
	IBSMunRate string `json:"ibs_mun_rate" validate:"required,percent"`
	CBSRate    string `json:"cbs_rate" validate:"required,percent"`
}

// CteAuthorizedXML é um autorizado a baixar o XML (autXML): CPF ou CNPJ.
type CteAuthorizedXML struct {
	Doc string `json:"doc" validate:"required,cpfcnpj"`
}

// ibscbsResult é o grupo calculado e os totais que o vTotDFe usa.
type ibscbsResult struct {
	node     map[string]any
	ibs, cbs decimal.Decimal
}

// resolveIBSCBS valida o regime do emitente e calcula os valores do grupo.
func resolveIBSCBS(in *CteIBSCBS, crt int) (*ibscbsResult, error) {
	if in == nil {
		return nil, nil
	}
	if crt != crtRegimeNormal {
		return nil, unprocessable(CodeIBSCBSRegime, "o IBS/CBS no CT-e só é emitido por regime normal (CRT 3); Simples Nacional exige gTribSN, que ainda não é suportado")
	}
	vBC := nfexml.Dec(in.BaseValue)
	hundred := decimal.NewFromInt(100)
	val := func(rate string) decimal.Decimal { return vBC.Mul(nfexml.Dec(rate)).Div(hundred).RoundBank(2) }
	vUF, vMun, vCBS := val(in.IBSUFRate), val(in.IBSMunRate), val(in.CBSRate)
	vIBS := vUF.Add(vMun)
	node := map[string]any{
		"CST": in.CST, "cClassTrib": in.ClassTrib,
		"gIBSCBS": map[string]any{
			"vBC":     q2(vBC),
			"gIBSUF":  map[string]any{"pIBSUF": rate(in.IBSUFRate), "vIBSUF": q2(vUF)},
			"gIBSMun": map[string]any{"pIBSMun": rate(in.IBSMunRate), "vIBSMun": q2(vMun)},
			"vIBS":    q2(vIBS),
			"gCBS":    map[string]any{"pCBS": rate(in.CBSRate), "vCBS": q2(vCBS)},
		},
	}
	if in.Donation {
		node["indDoacao"] = "1"
	}
	return &ibscbsResult{node: node, ibs: vIBS, cbs: vCBS}, nil
}

// rate formata a alíquota no leiaute TDec_0302_04: 2 ou 4 casas decimais.
func rate(s string) string {
	d := nfexml.Dec(s)
	if d.Equal(d.Round(2)) {
		return d.StringFixed(2)
	}
	return d.RoundBank(4).StringFixed(4)
}

// totalDFe é o vTotDFe: vTPrest, mais IBS e CBS a partir de 2027 (regra IBSCBS-365 do MOC).
func (r *ibscbsResult) totalDFe(vTPrest decimal.Decimal, emissionYear int) string {
	if emissionYear >= anoTotalComTributos {
		vTPrest = vTPrest.Add(r.ibs).Add(r.cbs)
	}
	return q2(vTPrest)
}

// normalizeDoc tira a pontuação do CPF/CNPJ e mantém as letras do CNPJ alfanumérico.
func normalizeDoc(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// resolveAuthorizedXML valida o autXML: até 10, CPF/CNPJ com DV, sem repetição e sem repetir ator do CT-e
// (regras G193 a G196 do MOC). Devolve os nós CNPJ ou CPF.
func resolveAuthorizedXML(in []CteAuthorizedXML, actors []string) ([]map[string]any, error) {
	if len(in) > MaxAuthorizedXML {
		return nil, unprocessable(CodeAuthXMLInvalid, fmt.Sprintf("no máximo %d autorizados a baixar o XML", MaxAuthorizedXML))
	}
	var out []map[string]any
	var seen []string
	for _, a := range in {
		doc := normalizeDoc(a.Doc)
		switch {
		case !validation.ValidCNPJ(doc) && !validation.ValidCPF(doc):
			return nil, unprocessable(CodeAuthXMLInvalid, "CPF ou CNPJ inválido em autorizado a baixar o XML: "+a.Doc)
		case slices.Contains(seen, doc):
			return nil, unprocessable(CodeAuthXMLInvalid, "autorizado repetido: "+doc)
		case slices.Contains(actors, doc):
			return nil, unprocessable(CodeAuthXMLInvalid, "o autorizado já é parte do CT-e: "+doc)
		}
		seen = append(seen, doc)
		if len(doc) == 14 {
			out = append(out, map[string]any{"CNPJ": doc})
		} else {
			out = append(out, map[string]any{"CPF": doc})
		}
	}
	return out, nil
}
