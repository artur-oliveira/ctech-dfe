package validate

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Regras do MOC da NFCom (modelo 62), Anexo I (v1.00a), seção 2.2, provadas só com o documento: nada que precise de
// banco (duplicidade, substituída, cancelada), CCC (IE vinculada ao CNPJ), tabelas externas (cClass, CFOP, IBGE) ou o
// relógio da SEFAZ (G37, G38). O ID é o do MOC; os nomes de campo seguem o XSD vigente (NT 2026.005), que é mais novo
// que o MOC: gCofat e gFat ficam sob infNFCom. O modo é Warn, exceto onde a rejeição é certa e determinística
// (composição do Id, DV da chave, DV de CNPJ/CPF e campos de contingência), que são Block. Mensagens não citam chave,
// CPF nem CNPJ.
//
// Fora desta lista, de propósito: G01 a G03, G07, G14 a G21, G28 a G34, G36 a G42, G45 a G66, G69 a G77, G81 a G85,
// G87 em diante (dependem de banco, CCC, tabelas IBGE/cClass/CFOP, relógio da SEFAZ ou do leiaute de IBS/CBS que ainda
// precisa ser lido nas NT 2026.002, 2026.004 e 2026.005); o autorizador devolve a rejeição.
//
// Check recebe o conteúdo de <NFCom> (doc["infNFCom"]).

const (
	nfcomModel         = "62"
	nfcomMinYear       = 2022 // G10: ano da chave inferior a 2022
	nfcomTolMoney      = 0.05 // G86: tolerância de R$ 0,05 para mais ou para menos
	nfcomTpEmisNormal  = "1"
	nfcomTpEmisOffline = "2"
	nfcomFinNormal     = "0"
	nfcomFinSubst      = "3"
	nfcomFinAjuste     = "4"
	nfcomFatNormal     = "0"
	nfcomFatCentral    = "1"
	nfcomFatCofat      = "2"
	nfcomIndContrib    = "1"
	nfcomIndIsento     = "2"
	nfcomIndNaoContrib = "9"
	nfcomIEIsento      = "ISENTO"
)

func nfcomRule(id string, cstat int, mode Mode, check func(inf map[string]any) []string) Rule {
	return Rule{ID: id, CStat: cstat, Mode: mode, Doc: "nfcom", Check: func(doc map[string]any) []string {
		inf := sub(doc, "infNFCom")
		if inf == nil {
			return nil
		}
		return check(inf)
	}}
}

func nfcomHas(m map[string]any, key string) bool { return mdfeStr(m, key) != "" }

func nfcomHasGroup(inf map[string]any, key string) bool { return sub(inf, key) != nil }

// nfcomKeyFromFields monta os 43 primeiros caracteres da chave a partir dos campos do documento (G09).
func nfcomKeyFromFields(inf map[string]any) (string, bool) {
	ide := sub(inf, "ide")
	emi, err := time.Parse(time.RFC3339, mdfeStr(ide, "dhEmi"))
	if ide == nil || err != nil {
		return "", false
	}
	serie, e1 := strconv.Atoi(mdfeStr(ide, "serie"))
	nNF, e2 := strconv.Atoi(mdfeStr(ide, "nNF"))
	if e1 != nil || e2 != nil {
		return "", false
	}
	return fmt.Sprintf("%s%02d%02d%s%s%03d%09d%s%s%s", mdfeStr(ide, "cUF"), emi.Year()%100, int(emi.Month()),
		mdfeStr(sub(inf, "emit"), "CNPJ"), nfcomModel, serie, nNF, mdfeStr(ide, "tpEmis"),
		mdfeStr(ide, "nSiteAutoriz"), mdfeStr(ide, "cNF")), true
}

// nfcomIDKey tira o literal "NFCom" do Id e devolve a chave.
func nfcomIDKey(inf map[string]any) (string, bool) {
	id := mdfeStr(inf, "@Id")
	if !strings.HasPrefix(id, "NFCom") {
		return "", false
	}
	return strings.TrimPrefix(id, "NFCom"), true
}

func nfcomMoney(m map[string]any, key string) float64 {
	f, _ := num(m, key)
	return f
}

var nfcomRules = []Rule{
	nfcomRule("G04", 415, ModeBlock, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		if mdfeStr(ide, "tpEmis") == nfcomTpEmisNormal && (nfcomHas(ide, "dhCont") || nfcomHas(ide, "xJust")) {
			return []string{"data e justificativa de contingencia nao devem ser informadas na emissao normal"}
		}
		return nil
	}),
	nfcomRule("G05", 416, ModeBlock, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		if mdfeStr(ide, "tpEmis") == nfcomTpEmisOffline && (!nfcomHas(ide, "dhCont") || !nfcomHas(ide, "xJust")) {
			return []string{"data e justificativa de contingencia devem ser informadas na contingencia offline"}
		}
		return nil
	}),
	nfcomRule("G06", 417, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		cont, e1 := time.Parse(time.RFC3339, mdfeStr(ide, "dhCont"))
		emi, e2 := time.Parse(time.RFC3339, mdfeStr(ide, "dhEmi"))
		if e1 == nil && e2 == nil && cont.After(emi) {
			return []string{"data de entrada em contingencia posterior a data de emissao"}
		}
		return nil
	}),
	nfcomRule("G08", 419, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		if mdfeStr(ide, "tpEmis") == nfcomTpEmisOffline && mdfeStr(ide, "finNFCom") != nfcomFinNormal {
			return []string{"emissao offline so e permitida para NFCom do tipo normal"}
		}
		return nil
	}),
	nfcomRule("G09", 227, ModeBlock, func(inf map[string]any) []string {
		key, ok := nfcomIDKey(inf)
		if !ok {
			return []string{"Id sem o literal NFCom"}
		}
		want, ok := nfcomKeyFromFields(inf)
		if !ok {
			return nil // campo ausente ou malformado: o esquema acusa
		}
		if len(key) != 44 || key[:43] != want || key[43:] != mdfeStr(sub(inf, "ide"), "cDV") {
			return []string{"chave do Id difere da concatenacao dos campos"}
		}
		return nil
	}),
	nfcomRule("G10", 421, ModeWarn, func(inf map[string]any) []string {
		key, ok := nfcomIDKey(inf)
		if !ok || !mdfeKeyOK(key) {
			return nil
		}
		if 2000+int(key[2]-'0')*10+int(key[3]-'0') < nfcomMinYear {
			return []string{"ano da chave de acesso inferior a 2022"}
		}
		return nil
	}),
	nfcomRule("G11", 253, ModeBlock, func(inf map[string]any) []string {
		key, ok := nfcomIDKey(inf)
		if !ok || !mdfeKeyOK(key) {
			return nil
		}
		if !mdfeKeyDVOK(key) {
			return []string{"digito verificador da chave de acesso invalido"}
		}
		return nil
	}),
	nfcomRule("G12", 207, ModeBlock, func(inf map[string]any) []string {
		if c := mdfeStr(sub(inf, "emit"), "CNPJ"); c != "" && !mdfeValidCNPJ(c) {
			return []string{"CNPJ do emitente invalido"}
		}
		return nil
	}),
	nfcomRule("G13", 229, ModeWarn, func(inf map[string]any) []string {
		if ie := mdfeStr(sub(inf, "emit"), "IE"); strings.Trim(ie, "0") == "" {
			return []string{"IE do emitente nao informada"}
		}
		return nil
	}),
	nfcomRule("G22", 422, ModeBlock, func(inf map[string]any) []string {
		if c := mdfeStr(sub(inf, "dest"), "CNPJ"); c != "" && !mdfeValidCNPJ(c) {
			return []string{"CNPJ do destinatario invalido"}
		}
		return nil
	}),
	nfcomRule("G23", 423, ModeBlock, func(inf map[string]any) []string {
		if c := mdfeStr(sub(inf, "dest"), "CPF"); c != "" && !mdfeValidCPF(c) {
			return []string{"CPF do destinatario invalido"}
		}
		return nil
	}),
	nfcomRule("G24", 424, ModeWarn, func(inf map[string]any) []string {
		d := sub(inf, "dest")
		if nfcomHas(d, "idOutros") && nfcomHas(d, "IE") {
			return []string{"IE nao deve ser informada para destinatario identificado por idOutros"}
		}
		return nil
	}),
	nfcomRule("G25", 426, ModeWarn, func(inf map[string]any) []string {
		d := sub(inf, "dest")
		if mdfeStr(d, "indIEDest") == nfcomIndContrib && (!nfcomHas(d, "IE") || strings.EqualFold(mdfeStr(d, "IE"), nfcomIEIsento)) {
			return []string{"IE deve ser informada para destinatario contribuinte"}
		}
		return nil
	}),
	nfcomRule("G26", 420, ModeWarn, func(inf map[string]any) []string {
		d := sub(inf, "dest")
		if mdfeStr(d, "indIEDest") == nfcomIndIsento && mdfeStr(d, "IE") != nfcomIEIsento {
			return []string{"IE do destinatario isento deve ser preenchida com ISENTO"}
		}
		return nil
	}),
	nfcomRule("G27", 431, ModeWarn, func(inf map[string]any) []string {
		d := sub(inf, "dest")
		if mdfeStr(d, "indIEDest") == nfcomIndNaoContrib && nfcomHas(d, "IE") {
			return []string{"IE nao pode ser informada para destinatario nao contribuinte"}
		}
		return nil
	}),
	nfcomRule("G35", 425, ModeWarn, func(inf map[string]any) []string {
		a := sub(inf, "assinante")
		ini, fim := mdfeStr(a, "dContratoIni"), mdfeStr(a, "dContratoFim")
		if ini != "" && fim != "" && fim < ini { // AAAA-MM-DD ordena como texto
			return []string{"data de fim do contrato anterior a data inicial"}
		}
		return nil
	}),
	nfcomRule("G43", 411, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "finNFCom") == nfcomFinSubst && !nfcomHasGroup(inf, "gSub") {
			return []string{"grupo de substituicao deve ser informado na NFCom de substituicao"}
		}
		return nil
	}),
	nfcomRule("G44", 412, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "finNFCom") != nfcomFinSubst && nfcomHasGroup(inf, "gSub") {
			return []string{"grupo de substituicao so vale para NFCom de substituicao"}
		}
		return nil
	}),
	nfcomRule("G67", 515, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "tpFat") == nfcomFatCofat && !nfcomHasGroup(inf, "gCofat") {
			return []string{"chave da NFCom da operadora local deve ser informada no cofaturamento"}
		}
		return nil
	}),
	nfcomRule("G68", 532, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "tpFat") != nfcomFatCofat && nfcomHasGroup(inf, "gCofat") {
			return []string{"grupo de cofaturamento incompativel com o tipo de faturamento"}
		}
		return nil
	}),
	nfcomRule("G78", 270, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		if mdfeStr(ide, "tpFat") == nfcomFatNormal && !nfcomHasGroup(inf, "gFat") &&
			!nfcomHas(ide, "indPrePago") && !nfcomHas(ide, "indCessaoMeiosRede") {
			return []string{"grupo da fatura deve ser informado no faturamento normal"}
		}
		return nil
	}),
	nfcomRule("G79", 272, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "tpFat") == nfcomFatCentral && !nfcomHasGroup(inf, "gFatCentral") {
			return []string{"grupo do faturamento centralizado deve ser informado"}
		}
		return nil
	}),
	nfcomRule("G80", 273, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		if (mdfeStr(ide, "finNFCom") == nfcomFinAjuste || mdfeStr(ide, "tpFat") == nfcomFatCofat) &&
			(nfcomHasGroup(inf, "gFat") || nfcomHasGroup(inf, "gFatCentral")) {
			return []string{"grupos de fatura nao devem ser informados no cofaturamento nem na NFCom de ajuste"}
		}
		return nil
	}),
	nfcomRule("G86", 435, ModeWarn, func(inf map[string]any) []string {
		var out []string
		for i, d := range list(inf, "det") {
			p := sub(d, "prod")
			want := nfcomMoney(p, "vItem")*nfcomMoney(p, "qFaturada") - nfcomMoney(p, "vDesc") + nfcomMoney(p, "vOutro")
			if math.Abs(nfcomMoney(p, "vProd")-want) > nfcomTolMoney {
				out = append(out, fmt.Sprintf("item %d: valor do item difere de valor unitario x quantidade faturada", i+1))
			}
		}
		return out
	}),
}

func init() { catalog = append(catalog, nfcomRules...) }
