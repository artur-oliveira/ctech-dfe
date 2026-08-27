package nfes

// builders_extra.go — helpers de saneamento de campos livres da NF-e.
// Extraído de builders_doc.go.

// natOpMaxLen is the SEFAZ ide.natOp limit (xNatOp: 1-60 chars).
const natOpMaxLen = 60

// Campos do CSRT na configuração fiscal (organization_*_configs).
const (
	csrtIDField = "csrt_id"
	csrtField   = "csrt"
)

// truncateNatOp enforces the SEFAZ ide.natOp 60-char limit. The frontend sends a
// summarized CFOP description; this is a rune-safe safety net that truncates with
// an ellipsis suffix when the value exceeds natOpMaxLen.
func truncateNatOp(s string) string {
	r := []rune(s)
	if len(r) <= natOpMaxLen {
		return s
	}
	return string(r[:natOpMaxLen-3]) + "..."
}

// docExtras carrega os grupos opcionais da NF-e que não cabem nos parâmetros
// posicionais de BuildEnviNFe. Cada tarefa de cobertura de tags acrescenta um
// campo aqui em vez de mais um parâmetro na assinatura.
type docExtras struct {
	// NFref são os documentos referenciados já resolvidos (ide/NFref).
	NFRefs []map[string]any
	// Vols e Reboques são os volumes e reboques de transp, já resolvidos.
	Vols     []map[string]any
	Reboques []map[string]any
	// InfAdFisco, ObsCont, ObsFisco e ProcRef completam infAdic; infCpl continua
	// vindo do parâmetro additionalInfo de BuildEnviNFe.
	InfAdFisco string
	ObsCont    []map[string]any
	ObsFisco   []map[string]any
	ProcRef    []map[string]any
	// PaymentTerminals são os terminais de captura citados pelos pagamentos,
	// indexados pelo SK do cadastro.
	PaymentTerminals map[string]map[string]any
	// RetTrib é o perfil de retenções federais da operação (total/retTrib).
	RetTrib map[string]any
	// FinNFe4 diz se a nota é de devolução — o único caso em que impostoDevol
	// pode existir.
	FinNFe4 bool
	// CsrtID/Csrt são o Código de Segurança do Responsável Técnico. O segredo
	// não é gravado no documento: só o hash entra no XML.
	CsrtID string
	Csrt   string
}

// buildInfAdic monta infAdic. Ordem XSD: infAdFisco, infCpl, obsCont, obsFisco,
// procRef. Devolve nil quando não há nada — nó vazio é rejeição.
func buildInfAdic(infAdFisco, infCpl string, obsCont, obsFisco, procRef []map[string]any) map[string]any {
	node := map[string]any{}
	if infAdFisco != "" {
		node["infAdFisco"] = infAdFisco
	}
	if infCpl != "" {
		node["infCpl"] = infCpl
	}
	if len(obsCont) > 0 {
		node["obsCont"] = obsCont
	}
	if len(obsFisco) > 0 {
		node["obsFisco"] = obsFisco
	}
	if len(procRef) > 0 {
		node["procRef"] = procRef
	}
	if len(node) == 0 {
		return nil
	}
	return node
}

// buildProcRef traduz os processos referenciados do request (infAdic/procRef).
func buildProcRef(refs []NfeProcRefBody) []map[string]any {
	out := make([]map[string]any, 0, len(refs))
	for _, r := range refs {
		node := map[string]any{"nProc": r.NProc, "indProc": r.IndProc}
		if r.TpAto != nil && *r.TpAto != "" {
			node["tpAto"] = *r.TpAto
		}
		out = append(out, node)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
