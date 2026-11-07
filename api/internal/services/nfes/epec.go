package nfes

import (
	"fmt"
	"time"

	"gopkg.aoctech.app/api-commons/problem"
)

// EPEC da NF-e (evento 110140, tpEmis=4; docs/specs/2026-11-08-contingencia.md).
// O resumo da nota vai ao Ambiente Nacional (cOrgao 91, mesma rota da
// manifestação) antes da nota ser transmitida à SEFAZ de origem.
// ponytail: leiaute do detEvento conforme leiauteEPEC do conhecimento público e
// a ordem do xsdorder; a página do MOC do EPEC não pôde ser relida nesta rodada.
const (
	TpEventoEPEC = "110140"
	descEPEC     = "EPEC"

	cOrgaoAN     = "91" // Ambiente Nacional
	tpAutorEmit  = "1"  // 1 = empresa emitente
	epecDeadline = 168 * time.Hour

	// CodeEPECInvalid: o resumo do EPEC não fecha com a nota.
	CodeEPECInvalid = "epec_invalid"
)

// EPECSummary é o resumo prévio da NF-e que o EPEC registra.
type EPECSummary struct {
	AccessKey string    // chave da nota com tpEmis=4
	IssuedAt  time.Time // dhEmi
	TpNF      string    // 0 entrada, 1 saída
	IE        string    // IE do emitente
	DestUF    string    // UF do destinatário ("EX" no exterior)
	DestTag   string    // "CNPJ", "CPF" ou "idEstrangeiro"
	DestDoc   string
	DestIE    string // opcional
	VNF       string // total da nota, 2 casas
	VICMS     string // total do ICMS, 2 casas
	VST       string // total do ICMS ST, 2 casas
	VerAplic  string
}

// Validate recusa o que a SEFAZ recusaria: a chave tem de ser de uma nota EPEC.
func (e EPECSummary) Validate() error {
	switch {
	case len(e.AccessKey) != accessKeyLen || e.AccessKey[accessKeyTpEmisPos:accessKeyTpEmisPos+1] != TpEmisEPEC:
		return problem.UnprocessableEntity("a chave do EPEC deve ser de uma NF-e com tpEmis 4").WithCode(CodeEPECInvalid)
	case e.IssuedAt.IsZero(), e.TpNF != "0" && e.TpNF != "1", e.IE == "", e.DestUF == "", e.DestDoc == "",
		e.VNF == "", e.VICMS == "", e.VST == "", e.VerAplic == "":
		return problem.UnprocessableEntity("resumo do EPEC incompleto").WithCode(CodeEPECInvalid)
	}
	return nil
}

// EPECDeadline é o prazo para transmitir a NF-e à SEFAZ de origem (168 h).
func EPECDeadline(registeredAt time.Time) time.Time { return registeredAt.Add(epecDeadline) }

// buildEPECBody monta o envEvento do 110140 para a chave com tpEmis=4.
func buildEPECBody(docTag, cnpj string, environment int, e EPECSummary, seq int) map[string]any {
	dest := map[string]any{"UF": e.DestUF, e.DestTag: e.DestDoc}
	if e.DestIE != "" {
		dest["IE"] = e.DestIE
	}
	dest["vNF"], dest["vICMS"], dest["vST"] = e.VNF, e.VICMS, e.VST
	return map[string]any{
		"envEvento": map[string]any{
			"@versao": "1.00",
			"@xmlns":  nfeXMLNS,
			"idLote":  sefazBatchID(),
			"evento": map[string]any{
				"@versao": "1.00",
				"infEvento": map[string]any{
					"@Id":        fmt.Sprintf("ID%s%s%02d", TpEventoEPEC, e.AccessKey, seq),
					"cOrgao":     cOrgaoAN,
					"tpAmb":      fmt.Sprintf("%d", environment),
					docTag:       cnpj,
					"chNFe":      e.AccessKey,
					"dhEvento":   dhEvento(),
					"tpEvento":   TpEventoEPEC,
					"nSeqEvento": fmt.Sprintf("%d", seq),
					"verEvento":  "1.00",
					"detEvento": map[string]any{
						"@versao":     "1.00",
						"@xmlns":      nfeXMLNS,
						"descEvento":  descEPEC,
						"cOrgaoAutor": e.AccessKey[:2],
						"tpAutor":     tpAutorEmit,
						"verAplic":    e.VerAplic,
						"dhEmi":       fmtDhEmi(e.IssuedAt),
						"tpNF":        e.TpNF,
						"IE":          e.IE,
						"dest":        dest,
					},
				},
			},
		},
	}
}
