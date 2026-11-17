package validate

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Regras do MOC do BP-e (modelo 63; Anexo I v1.00b e NT 2025.002 v2.00 para o TA), provadas só com o documento: nada que
// precise de banco, CCC, tabelas (IBGE, CST, cClassTrib) ou o relógio da SEFAZ. O ID é o do MOC; as mesmas regras valem
// para o BP-e comum, o TM e o TA (famílias "bpe", "bpetm" e "bpeta"). O modo é Warn, exceto onde a rejeição é certa e
// determinística (campos de contingência, composição do Id, DV da chave, DV de CNPJ e CPF), que são Block. Mensagens não
// citam chave, CPF nem CNPJ.
//
// Fora desta lista, de propósito: F01 a F04, F08, F12, F14 a F17, F19, F20, F26, F27, F32, F33, F36 a F46, F47, F48,
// F50 a F63, F71 a F111 e as seguintes (dependem de banco, CCC, tabelas, relógio da SEFAZ, BP-e vinculado ou do leiaute de
// IBS/CBS ainda não lido nas NT 2026.002, 2026.004 e 2026.005); o autorizador devolve a rejeição.
//
// Check recebe o conteúdo de <BPe> (doc["infBPe"]).

const (
	bpeModel         = "63"
	bpeMinYear       = 2025 // F10: ano da chave inferior a 2025
	bpeTolMoney      = 1.00 // F64 e F69: tolerância de R$ 1,00
	bpeTolMoneyTight = 0.01
	bpeTpEmisNormal  = "1"
	bpeTpEmisOffline = "2"
	bpeTrechoConexao = "3"
)

var bpeFamilies = []string{"bpe", "bpetm", "bpeta"}

func bpeRule(fam, id string, cstat int, mode Mode, check func(inf map[string]any) []string) Rule {
	return Rule{ID: id, CStat: cstat, Mode: mode, Doc: fam, Check: func(doc map[string]any) []string {
		inf := sub(doc, "infBPe")
		if inf == nil {
			return nil
		}
		return check(inf)
	}}
}

func bpeHas(m map[string]any, key string) bool { return mdfeStr(m, key) != "" }

func bpeTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

func bpeMoney(m map[string]any, key string) float64 {
	f, _ := num(m, key)
	return f
}

func bpeIDKey(inf map[string]any) (string, bool) {
	id := mdfeStr(inf, "@Id")
	if !strings.HasPrefix(id, "BPe") {
		return "", false
	}
	return strings.TrimPrefix(id, "BPe"), true
}

// bpeKeyFromFields monta os 43 primeiros caracteres da chave a partir dos campos do documento (F09).
func bpeKeyFromFields(inf map[string]any) (string, bool) {
	ide := sub(inf, "ide")
	emi, err := time.Parse(time.RFC3339, mdfeStr(ide, "dhEmi"))
	if ide == nil || err != nil {
		return "", false
	}
	serie, e1 := strconv.Atoi(mdfeStr(ide, "serie"))
	nBP, e2 := strconv.Atoi(mdfeStr(ide, "nBP"))
	if e1 != nil || e2 != nil {
		return "", false
	}
	return fmt.Sprintf("%s%02d%02d%s%s%03d%09d%s%s", mdfeStr(ide, "cUF"), emi.Year()%100, int(emi.Month()),
		mdfeStr(sub(inf, "emit"), "CNPJ"), bpeModel, serie, nBP, mdfeStr(ide, "tpEmis"), mdfeStr(ide, "cBP")), true
}

func bpeRules(fam string) []Rule {
	r := func(id string, cstat int, mode Mode, check func(inf map[string]any) []string) Rule {
		return bpeRule(fam, id, cstat, mode, check)
	}
	return []Rule{
		r("F05", 415, ModeBlock, func(inf map[string]any) []string {
			ide := sub(inf, "ide")
			if mdfeStr(ide, "tpEmis") == bpeTpEmisNormal && (bpeHas(ide, "dhCont") || bpeHas(ide, "xJust")) {
				return []string{"data e justificativa de contingencia nao devem ser informadas na emissao normal"}
			}
			return nil
		}),
		r("F06", 416, ModeBlock, func(inf map[string]any) []string {
			ide := sub(inf, "ide")
			if mdfeStr(ide, "tpEmis") == bpeTpEmisOffline && (!bpeHas(ide, "dhCont") || !bpeHas(ide, "xJust")) {
				return []string{"data e justificativa de contingencia devem ser informadas na contingencia offline"}
			}
			return nil
		}),
		r("F07", 417, ModeWarn, func(inf map[string]any) []string {
			ide := sub(inf, "ide")
			cont, ok1 := bpeTime(mdfeStr(ide, "dhCont"))
			emi, ok2 := bpeTime(mdfeStr(ide, "dhEmi"))
			if ok1 && ok2 && cont.After(emi) {
				return []string{"data de entrada em contingencia posterior a data de emissao"}
			}
			return nil
		}),
		r("F09", 227, ModeBlock, func(inf map[string]any) []string {
			key, ok := bpeIDKey(inf)
			if !ok {
				return []string{"Id sem o literal BPe"}
			}
			want, ok := bpeKeyFromFields(inf)
			if !ok {
				return nil // campo ausente ou malformado: o esquema acusa
			}
			if len(key) != 44 || key[:43] != want || key[43:] != mdfeStr(sub(inf, "ide"), "cDV") {
				return []string{"chave do Id difere da concatenacao dos campos"}
			}
			return nil
		}),
		r("F10", 421, ModeWarn, func(inf map[string]any) []string {
			key, ok := bpeIDKey(inf)
			if !ok || !mdfeKeyOK(key) {
				return nil
			}
			if 2000+int(key[2]-'0')*10+int(key[3]-'0') < bpeMinYear {
				return []string{"ano da chave de acesso inferior a 2025"}
			}
			return nil
		}),
		r("F11", 253, ModeBlock, func(inf map[string]any) []string {
			key, ok := bpeIDKey(inf)
			if !ok || !mdfeKeyOK(key) {
				return nil
			}
			if !mdfeKeyDVOK(key) {
				return []string{"digito verificador da chave de acesso invalido"}
			}
			return nil
		}),
		r("F13", 505, ModeWarn, func(inf map[string]any) []string {
			uf := mdfeStr(sub(inf, "emit", "enderEmit"), "UF")
			if ini := mdfeStr(sub(inf, "ide"), "UFIni"); ini != "" && uf != "" && ini != uf {
				return []string{"UF de inicio da viagem deve ser igual a UF do emitente"}
			}
			return nil
		}),
		r("F18", 497, ModeBlock, func(inf map[string]any) []string {
			if c := mdfeStr(sub(inf, "infPassagem", "infPassageiro"), "CPF"); c != "" && !mdfeValidCPF(c) {
				return []string{"CPF do passageiro invalido"}
			}
			return nil
		}),
		r("F21", 484, ModeWarn, func(inf map[string]any) []string {
			var out []string
			for i, v := range list(inf, "infViagem") {
				if mdfeStr(v, "tpTrecho") == bpeTrechoConexao && !bpeHas(v, "dhConexao") {
					out = append(out, fmt.Sprintf("trecho %d: data-hora da conexao deve ser informada", i+1))
				}
			}
			return out
		}),
		r("F22", 485, ModeWarn, func(inf map[string]any) []string {
			var out []string
			for i, v := range list(inf, "infViagem") {
				if t := mdfeStr(v, "tpTrecho"); t != "" && t != bpeTrechoConexao && bpeHas(v, "dhConexao") {
					out = append(out, fmt.Sprintf("trecho %d: data-hora da conexao nao deve ser informada", i+1))
				}
			}
			return out
		}),
		r("F23", 486, ModeWarn, func(inf map[string]any) []string {
			emb, ok := bpeTime(mdfeStr(sub(inf, "infPassagem"), "dhEmb"))
			if !ok {
				return nil
			}
			var out []string
			for i, v := range list(inf, "infViagem") {
				if c, ok := bpeTime(mdfeStr(v, "dhConexao")); ok && !c.After(emb) {
					out = append(out, fmt.Sprintf("trecho %d: data-hora da conexao deve ser maior que a do embarque", i+1))
				}
			}
			return out
		}),
		r("F24", 207, ModeBlock, func(inf map[string]any) []string {
			if c := mdfeStr(sub(inf, "emit"), "CNPJ"); c != "" && !mdfeValidCNPJ(c) {
				return []string{"CNPJ do emitente invalido"}
			}
			return nil
		}),
		r("F34", 422, ModeBlock, func(inf map[string]any) []string {
			if c := mdfeStr(sub(inf, "comp"), "CNPJ"); c != "" && !mdfeValidCNPJ(c) {
				return []string{"CNPJ do comprador invalido"}
			}
			return nil
		}),
		r("F35", 423, ModeBlock, func(inf map[string]any) []string {
			if c := mdfeStr(sub(inf, "comp"), "CPF"); c != "" && !mdfeValidCPF(c) {
				return []string{"CPF do comprador invalido"}
			}
			return nil
		}),
		r("F49", 254, ModeWarn, func(inf map[string]any) []string {
			emb, ok1 := bpeTime(mdfeStr(sub(inf, "infPassagem"), "dhEmb"))
			emi, ok2 := bpeTime(mdfeStr(sub(inf, "ide"), "dhEmi"))
			if ok1 && ok2 && emb.Before(emi) {
				return []string{"data-hora do embarque anterior a data de emissao"}
			}
			return nil
		}),
		r("F64", 436, ModeWarn, func(inf map[string]any) []string {
			v := sub(inf, "infValorBPe")
			comps := list(v, "Comp")
			if v == nil || len(comps) == 0 || !bpeHas(v, "vBP") {
				return nil
			}
			sum := 0.0
			for _, c := range comps {
				sum += bpeMoney(c, "vComp")
			}
			if math.Abs(sum-bpeMoney(v, "vBP")) > bpeTolMoney {
				return []string{"soma dos componentes difere do valor total do bilhete"}
			}
			return nil
		}),
		r("F65", 437, ModeWarn, func(inf map[string]any) []string {
			v := sub(inf, "infValorBPe")
			if bpeHas(v, "tpDesconto") && !bpeHas(v, "xDesconto") {
				return []string{"descricao do tipo de desconto deve ser informada"}
			}
			return nil
		}),
		r("F66", 401, ModeWarn, func(inf map[string]any) []string {
			v := sub(inf, "infValorBPe")
			if bpeHas(v, "tpDesconto") && bpeMoney(v, "vDesconto") <= 0 {
				return []string{"valor do desconto deve ser maior que zero quando ha tipo de desconto"}
			}
			return nil
		}),
		r("F67", 500, ModeWarn, func(inf map[string]any) []string {
			v := sub(inf, "infValorBPe")
			if !bpeHas(v, "tpDesconto") && (bpeMoney(v, "vDesconto") > 0 || bpeHas(v, "xDesconto")) {
				return []string{"tipo do desconto deve ser informado"}
			}
			return nil
		}),
		r("F68", 501, ModeWarn, func(inf map[string]any) []string {
			v := sub(inf, "infValorBPe")
			if v != nil && !bpeHas(v, "tpDesconto") && bpeHas(v, "vBP") && bpeMoney(v, "vBP") == 0 {
				return []string{"valor do bilhete sem desconto deve ser maior que zero"}
			}
			return nil
		}),
		r("F69", 438, ModeWarn, func(inf map[string]any) []string {
			v := sub(inf, "infValorBPe")
			pags := list(inf, "pag")
			if v == nil || len(pags) == 0 {
				return nil
			}
			sum := 0.0
			for _, p := range pags {
				sum += bpeMoney(p, "vPag")
			}
			if math.Abs(sum-(bpeMoney(v, "vPgto")+bpeMoney(v, "vTroco"))) > bpeTolMoney {
				return []string{"soma das formas de pagamento difere do valor pago mais o troco"}
			}
			return nil
		}),
		r("F70", 403, ModeWarn, func(inf map[string]any) []string {
			v := sub(inf, "infValorBPe")
			if v == nil || !bpeHas(v, "vBP") || !bpeHas(v, "vPgto") {
				return nil
			}
			if math.Abs(bpeMoney(v, "vPgto")-(bpeMoney(v, "vBP")-bpeMoney(v, "vDesconto"))) > bpeTolMoneyTight {
				return []string{"valor pago difere do valor do bilhete menos o desconto"}
			}
			return nil
		}),
	}
}

func init() {
	for _, fam := range bpeFamilies {
		catalog = append(catalog, bpeRules(fam)...)
	}
}
