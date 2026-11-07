package validate

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Regras do MOC do MDF-e (modelo 58), página "Regras de Validação", provadas só com o documento: nada que precise de
// banco, tabela externa (IBGE, RNTRC, IE por UF) ou relógio da SEFAZ. O ID é o da página. O modo é Warn, exceto nas regras
// em que a rejeição é certa e determinística (DV da chave, DV de CPF/CNPJ, identidades de contagem e de total), que são Block.
// Mensagens não citam chave, CPF nem CNPJ (o chamador as registra em log): só índices e valores monetários.
//
// Check recebe o conteúdo de <MDFe> (doc["infMDFe"], doc["infMDFeSupl"]).

const (
	mdfeMinYear       = 2012
	mdfeCTeMinYear    = 2008
	mdfeNFeMinYear    = 2006
	mdfeTolMoney      = 0.01 // F58 e F62: tolera R$ 0,01 a mais ou a menos
	mdfeMaxSkewFuture = 5 * time.Minute
	mdfeMaxLate       = 24 * time.Hour
	mdfeStaleMonths   = 6 // chave de documento referenciado mais antiga que isto (F30a, F36b, F37a, F45a)
	mdfeSerieMinCPF   = 920
	mdfeSerieMaxCPF   = 969
	mdfeVersaoModal   = "3.00"
	mdfeQRHost        = "dfe-portal.svrs.rs.gov.br"
	mdfeTpEmisCTe     = "14578"    // tipos de emissão válidos do CT-e
	mdfeTpEmisNFe     = "12345679" // e da NF-e
	mdfeTpEmisMDFe    = "123"      // e do MDF-e
	mdfeIndPrazo      = "1"        // indPag: a prazo
	mdfeIndVista      = "0"        // indPag: à vista
	mdfeTpRodCavalo   = "03"       // veicTracao/tpRod: cavalo mecânico
	mdfeRespSegContr  = "2"        // infResp/respSeg: contratante
)

var mdfePlacaRe = regexp.MustCompile(`^[A-Z]{3}[0-9][0-9A-Z][0-9]{2}$`)

func mdfeUFInfo(inf map[string]any) (ini, fim string) {
	ide := sub(inf, "ide")
	return mdfeStr(ide, "UFIni"), mdfeStr(ide, "UFFim")
}

// mdfeRefKeys avalia chaves de um grupo (F30, F37, F45, F36d) e devolve um achado por chave inválida.
func mdfeRefKeys(keys []string, label, mod string, minYear int, tpEmis string) []string {
	var out []string
	for i, k := range keys {
		if r := mdfeKeyReason(k, mod, minYear, tpEmis); r != "" {
			out = append(out, fmt.Sprintf("%s %d: chave de acesso invalida (%s)", label, i+1, r))
		}
	}
	return out
}

// mdfeStaleKeys acha chaves com AAMM mais de 6 meses antes da emissão. O MOC usa a data de autorização;
// dhEmi é o substituto offline (a diferença é de segundos a horas).
func mdfeStaleKeys(inf map[string]any, keys []string, label string) []string {
	emi, ok := mdfeEmi(sub(inf, "ide"))
	if !ok {
		return nil
	}
	ref := emi.Year()*12 + int(emi.Month())
	var out []string
	for i, k := range keys {
		if m := mdfeKeyMonths(k); m > 0 && ref-m > mdfeStaleMonths {
			out = append(out, fmt.Sprintf("%s %d: chave de acesso muito antiga", label, i+1))
		}
	}
	return out
}

func mdfeSegCods(inf map[string]any, group, ch string) (fsda, withSeg []int) {
	n := 0
	for _, mun := range mdfeMuns(inf) {
		for _, d := range list(mun, group) {
			n++
			k := mdfeStr(d, ch)
			isFSDA := len(k) == 44 && k[34:35] == mdfeTpEmisFSDA
			has := mdfeStr(d, "SegCodBarra") != ""
			if isFSDA && !has {
				fsda = append(fsda, n)
			}
			if !isFSDA && has {
				withSeg = append(withSeg, n)
			}
		}
	}
	return
}

func mdfeSum(nodes []map[string]any, key string) float64 {
	var s float64
	for _, n := range nodes {
		v, _ := num(n, key)
		s += v
	}
	return s
}

func mdfeQR(doc map[string]any) (url.Values, bool, bool) {
	raw := mdfeStr(sub(doc, "infMDFeSupl"), "qrCodMDFe")
	u, err := url.Parse(raw)
	if raw == "" || err != nil {
		return nil, false, false
	}
	return u.Query(), strings.EqualFold(u.Host, mdfeQRHost), true
}

func mdfeRuleDoc(id string, cstat int, mode Mode, check func(doc, inf map[string]any) []string) Rule {
	return Rule{ID: id, CStat: cstat, Mode: mode, Doc: "mdfe", Check: func(doc map[string]any) []string {
		inf := sub(doc, "infMDFe")
		if inf == nil {
			return nil
		}
		return check(doc, inf)
	}}
}

// mdfeEveryPag roda check em cada infPag quando vale a condição comum das regras F52 a F63.
func mdfeEveryPag(check func(i int, p, inf map[string]any) []string) func(inf map[string]any) []string {
	return func(inf map[string]any) []string {
		if !mdfePagGate(inf) {
			return nil
		}
		var out []string
		for i, p := range mdfeInfPag(inf) {
			out = append(out, check(i+1, p, inf)...)
		}
		return out
	}
}

var mdfeRules = []Rule{
	// --- 2.1 Gerais ---
	mdfeRule("F02", 247, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "tpEmis") == mdfeTpEmisNFF {
			return nil
		}
		k := mdfeKeyOf(inf)
		uf := mdfeStr(sub(inf, "emit", "enderEmit"), "UF")
		if len(k) != 44 || uf == "" {
			return nil
		}
		return mdfeOne(mdfeUFCode[uf] != k[:2], "UF do emitente diverge da UF da chave de acesso")
	}),
	mdfeRule("F03", 227, ModeWarn, func(inf map[string]any) []string {
		id, ide := mdfeStr(inf, "@Id"), sub(inf, "ide")
		if !strings.HasPrefix(id, "MDFe") {
			return []string{"Id sem o literal MDFe"}
		}
		emi, ok := mdfeEmi(ide)
		doc := mdfeEmitDoc(inf)
		serie, e1 := strconv.Atoi(mdfeStr(ide, "serie"))
		nmdf, e2 := strconv.Atoi(mdfeStr(ide, "nMDF"))
		if !ok || doc == "" || e1 != nil || e2 != nil {
			return nil
		}
		if len(doc) == 11 {
			doc = "000" + doc
		}
		want := mdfeStr(ide, "cUF") + emi.Format("0601") + doc + mdfeStr(ide, "mod") + fmt.Sprintf("%03d%09d", serie, nmdf) +
			mdfeStr(ide, "tpEmis") + mdfeStr(ide, "cMDF") + mdfeStr(ide, "cDV")
		return mdfeOne(id[4:] != want, "chave do Id difere da concatenacao dos campos do documento")
	}),
	mdfeRule("F04", 666, ModeWarn, func(inf map[string]any) []string {
		k := mdfeKeyOf(inf)
		if !mdfeKeyOK(k) {
			return nil
		}
		return mdfeOne(2000+int(k[2]-'0')*10+int(k[3]-'0') < mdfeMinYear, "ano da chave de acesso inferior a 2012")
	}),
	mdfeRule("F05", 253, ModeBlock, func(inf map[string]any) []string {
		k := mdfeKeyOf(inf)
		if !mdfeKeyOK(k) {
			return nil
		}
		return mdfeOne(!mdfeKeyDVOK(k), "digito verificador da chave de acesso invalido")
	}),
	mdfeRule("F06", 579, ModeWarn, func(inf map[string]any) []string {
		v := mdfeStr(sub(inf, "infModal"), "@versaoModal")
		return mdfeOne(v != "" && v != mdfeVersaoModal, "versao do modal nao suportada")
	}),
	mdfeRule("F08", 456, ModeWarn, func(inf map[string]any) []string {
		ini, _ := mdfeUFInfo(inf)
		cod, ok := mdfeUFCode[ini]
		if !ok {
			return nil
		}
		var out []string
		for i, m := range list(sub(inf, "ide"), "infMunCarrega") {
			if c := mdfeStr(m, "cMunCarrega"); len(c) >= 2 && c[:2] != cod {
				out = append(out, fmt.Sprintf("municipio de carregamento %d diverge da UF de carregamento", i+1))
			}
		}
		return out
	}),
	mdfeRule("F10", 685, ModeWarn, func(inf map[string]any) []string {
		var codes []string
		for _, m := range list(sub(inf, "ide"), "infMunCarrega") {
			codes = append(codes, mdfeStr(m, "cMunCarrega"))
		}
		var out []string
		for _, i := range mdfeDup(codes) {
			out = append(out, fmt.Sprintf("municipio de carregamento %d duplicado", i))
		}
		return out
	}),
	// F11: o leiaute não traz a UF de cada município de descarga. O MOC compara com a UF de descarregamento; offline
	// só dá para exigir que a UF do código seja uma das UFs da viagem (UFIni, UFPer, UFFim).
	mdfeRule("F11", 612, ModeWarn, func(inf map[string]any) []string {
		ini, fim := mdfeUFInfo(inf)
		allowed := map[string]bool{}
		for _, uf := range append([]string{ini, fim}, mdfeUFPer(inf)...) {
			allowed[mdfeUFCode[uf]] = true
		}
		if ini == mdfeUFExterior || fim == mdfeUFExterior {
			return nil
		}
		var out []string
		for i, m := range mdfeMuns(inf) {
			if c := mdfeStr(m, "cMunDescarga"); len(c) >= 2 && !allowed[c[:2]] {
				out = append(out, fmt.Sprintf("municipio de descarregamento %d diverge das UFs da viagem", i+1))
			}
		}
		return out
	}),
	mdfeRule("F13", 680, ModeWarn, func(inf map[string]any) []string {
		var codes []string
		for _, m := range mdfeMuns(inf) {
			codes = append(codes, mdfeStr(m, "cMunDescarga"))
		}
		var out []string
		for _, i := range mdfeDup(codes) {
			out = append(out, fmt.Sprintf("municipio de descarregamento %d duplicado", i))
		}
		return out
	}),

	// --- 2.2 Tipo de emitente ---
	mdfeRule("F14", 638, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfeStr(sub(inf, "ide"), "tpEmit") == mdfeTpEmitPrestador && len(mdfeKeys(inf, "infNFe", "chNFe")) > 0,
			"NF-e nao pode ser informada para emitente prestador de servico de transporte")
	}),
	mdfeRule("F15", 639, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfeStr(sub(inf, "ide"), "tpEmit") == mdfeTpEmitPropria && len(mdfeKeys(inf, "infCTe", "chCTe")) > 0,
			"CT-e nao pode ser informado para emitente de carga propria")
	}),
	mdfeRule("F16", 540, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfeStr(sub(inf, "ide"), "tpEmit") == mdfeTpEmitGlobal && len(mdfeKeys(inf, "infCTe", "chCTe")) > 0,
			"CT-e nao pode ser informado para emitente que emitira CT-e globalizado")
	}),
	mdfeRule("F17", 541, ModeWarn, func(inf map[string]any) []string {
		ini, fim := mdfeUFInfo(inf)
		inter := ini != fim || ini == mdfeUFExterior
		return mdfeOne(mdfeStr(sub(inf, "ide"), "tpEmit") == mdfeTpEmitGlobal && inter,
			"tipo de emitente CT-e globalizado invalido para operacao interestadual ou com exterior")
	}),

	// --- 2.3 Tipo de transportador ---
	mdfeRule("F18", 743, ModeWarn, func(inf map[string]any) []string {
		prop := sub(mdfeRodo(inf), "veicTracao", "prop")
		return mdfeOne(mdfeIsRodo(inf) && mdfeStr(prop, "CPF") != "" && mdfeStr(sub(inf, "ide"), "tpTransp") != mdfeTpTranspTAC,
			"tipo de transportador deve ser TAC quando o proprietario da tracao e CPF")
	}),
	mdfeRule("F19", 744, ModeWarn, func(inf map[string]any) []string {
		prop := sub(mdfeRodo(inf), "veicTracao", "prop")
		tp := mdfeStr(sub(inf, "ide"), "tpTransp")
		return mdfeOne(mdfeIsRodo(inf) && mdfeStr(prop, "CNPJ") != "" && tp != mdfeTpTranspETC && tp != mdfeTpTranspCTC,
			"tipo de transportador deve ser ETC ou CTC quando o proprietario da tracao e CNPJ")
	}),
	mdfeRule("F20", 745, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfeIsRodo(inf) && sub(mdfeRodo(inf), "veicTracao", "prop") == nil && mdfeStr(sub(inf, "ide"), "tpTransp") != "",
			"tipo de transportador nao deve ser informado sem proprietario da tracao")
	}),

	// --- 2.4 Carregamento posterior ---
	mdfeRule("F21", 703, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "indCarregaPosterior") != "1" {
			return nil
		}
		car, des := list(sub(inf, "ide"), "infMunCarrega"), mdfeMuns(inf)
		ok := len(car) == 1 && len(des) == 1 && mdfeStr(car[0], "cMunCarrega") == mdfeStr(des[0], "cMunDescarga")
		return mdfeOne(!ok, "carregamento posterior exige um unico municipio de carregamento igual ao de descarregamento")
	}),
	mdfeRule("F22", 704, ModeWarn, func(inf map[string]any) []string {
		ini, fim := mdfeUFInfo(inf)
		return mdfeOne(mdfeStr(sub(inf, "ide"), "indCarregaPosterior") == "1" && (ini != fim || ini == mdfeUFExterior),
			"carregamento posterior nao permitido para operacao interestadual ou com exterior")
	}),
	mdfeRule("F23", 705, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfeStr(sub(inf, "ide"), "indCarregaPosterior") == "1" && !mdfeIsRodo(inf),
			"carregamento posterior so vale para o modal rodoviario")
	}),
	mdfeRule("F24", 707, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		return mdfeOne(mdfeStr(ide, "indCarregaPosterior") == "1" && mdfeStr(ide, "tpEmit") != mdfeTpEmitPropria,
			"carregamento posterior exige tipo de emitente carga propria")
	}),

	// --- 2.5 Documentos originários ---
	mdfeRule("F26", 616, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "indCarregaPosterior") == "1" {
			return nil
		}
		muns := mdfeMuns(inf)
		if len(muns) == 0 {
			return []string{"nenhum grupo de documentos informado"}
		}
		var out []string
		for i, m := range muns {
			if len(list(m, "infCTe"))+len(list(m, "infNFe"))+len(list(m, "infMDFeTransp")) == 0 {
				out = append(out, fmt.Sprintf("municipio de descarregamento %d sem DF-e vinculado", i+1))
			}
		}
		return out
	}),
	mdfeRule("F27", 706, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfeStr(sub(inf, "ide"), "indCarregaPosterior") == "1" && mdfeDocCount(inf) > 0,
			"carregamento posterior nao admite documentos transportados (usar o evento de inclusao de DF-e)")
	}),
	mdfeRule("F28", 668, ModeWarn, func(inf map[string]any) []string { return mdfeDupDocs(inf, "infCTe", "chCTe", "CT-e") }),
	mdfeRule("F29", 669, ModeWarn, func(inf map[string]any) []string { return mdfeDupDocs(inf, "infNFe", "chNFe", "NF-e") }),
	mdfeRule("F30", 601, ModeWarn, func(inf map[string]any) []string {
		return mdfeRefKeys(mdfeKeys(inf, "infCTe", "chCTe"), "CT-e", "57", mdfeCTeMinYear, mdfeTpEmisCTe)
	}),
	mdfeRule("F30a", 518, ModeWarn, func(inf map[string]any) []string {
		return mdfeStaleKeys(inf, mdfeKeys(inf, "infCTe", "chCTe"), "CT-e")
	}),
	mdfeRule("F34", 702, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "modal") == mdfeModalAereo {
			return nil
		}
		var out []string
		for i, m := range mdfeMuns(inf) {
			for _, c := range list(m, "infCTe") {
				if sub(c, "infEntregaParcial") != nil {
					out = append(out, fmt.Sprintf("municipio %d: entrega parcial so vale para o modal aereo", i+1))
				}
			}
		}
		return out
	}),
	mdfeRule("F35", 602, ModeWarn, func(inf map[string]any) []string {
		miss, _ := mdfeSegCods(inf, "infCTe", "chCTe")
		return mdfeOne(len(miss) > 0, "CT-e em FS-DA sem segundo codigo de barras")
	}),
	mdfeRule("F36", 603, ModeWarn, func(inf map[string]any) []string {
		_, extra := mdfeSegCods(inf, "infCTe", "chCTe")
		return mdfeOne(len(extra) > 0, "segundo codigo de barras informado para CT-e que nao e FS-DA")
	}),
	mdfeRule("F36b", 525, ModeWarn, func(inf map[string]any) []string {
		return mdfeStaleKeys(inf, mdfePrestParcial(inf), "NF-e de prestacao parcial")
	}),
	mdfeRule("F36c", 526, ModeWarn, func(inf map[string]any) []string {
		var out []string
		for _, i := range mdfeDup(mdfePrestParcial(inf)) {
			out = append(out, fmt.Sprintf("NF-e de prestacao parcial %d duplicada", i))
		}
		return out
	}),
	mdfeRule("F36d", 527, ModeWarn, func(inf map[string]any) []string {
		return mdfeRefKeys(mdfePrestParcial(inf), "NF-e de prestacao parcial", "55", mdfeNFeMinYear, mdfeTpEmisNFe)
	}),
	mdfeRule("F37", 604, ModeWarn, func(inf map[string]any) []string {
		return mdfeRefKeys(mdfeKeys(inf, "infNFe", "chNFe"), "NF-e", "55", mdfeNFeMinYear, mdfeTpEmisNFe)
	}),
	mdfeRule("F37a", 519, ModeWarn, func(inf map[string]any) []string {
		return mdfeStaleKeys(inf, mdfeKeys(inf, "infNFe", "chNFe"), "NF-e")
	}),
	// F41: o texto da regra cita só FS-DA; a mensagem fala em FS-DA e FS-IA. Segue-se o texto da regra.
	mdfeRule("F41", 606, ModeWarn, func(inf map[string]any) []string {
		miss, _ := mdfeSegCods(inf, "infNFe", "chNFe")
		return mdfeOne(len(miss) > 0, "NF-e em FS-DA sem segundo codigo de barras")
	}),
	mdfeRule("F42", 607, ModeWarn, func(inf map[string]any) []string {
		_, extra := mdfeSegCods(inf, "infNFe", "chNFe")
		return mdfeOne(len(extra) > 0, "segundo codigo de barras informado para NF-e que nao e FS-DA")
	}),
	mdfeRule("F43", 647, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(len(mdfeKeys(inf, "infMDFeTransp", "chMDFe")) > 0 && mdfeStr(sub(inf, "ide"), "modal") != mdfeModalAquav,
			"MDF-e so pode ser referenciado por manifesto do modal aquaviario")
	}),
	mdfeRule("F45", 649, ModeWarn, func(inf map[string]any) []string {
		return mdfeRefKeys(mdfeKeys(inf, "infMDFeTransp", "chMDFe"), "MDF-e", "58", mdfeMinYear, mdfeTpEmisMDFe)
	}),
	mdfeRule("F45a", 520, ModeWarn, func(inf map[string]any) []string {
		return mdfeStaleKeys(inf, mdfeKeys(inf, "infMDFeTransp", "chMDFe"), "MDF-e")
	}),
	mdfeRule("F51", 667, ModeWarn, func(inf map[string]any) []string {
		tot := sub(inf, "tot")
		var out []string
		for _, c := range []struct {
			tag   string
			count int
		}{
			{"qCTe", len(mdfeKeys(inf, "infCTe", "chCTe"))},
			{"qNFe", len(mdfeKeys(inf, "infNFe", "chNFe"))},
			{"qMDFe", len(mdfeKeys(inf, "infMDFeTransp", "chMDFe"))},
		} {
			if v, ok := num(tot, c.tag); ok && int(v) != c.count {
				out = append(out, fmt.Sprintf("%s informado %d difere dos %d documentos relacionados", c.tag, int(v), c.count))
			}
		}
		return out
	}),

	// --- 2.5 (cont.) pagamento e carga lotação ---
	mdfeRule("F52", 724, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		return mdfeOne(mdfeStr(p, "indPag") == mdfeIndPrazo && len(list(p, "infPrazo")) == 0, fmt.Sprintf("pagamento %d a prazo sem infPrazo", i))
	})),
	mdfeRule("F53", 729, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		return mdfeOne(mdfeStr(p, "indPag") == mdfeIndVista && len(list(p, "infPrazo")) > 0, fmt.Sprintf("pagamento %d a vista com infPrazo", i))
	})),
	mdfeRule("F54", 725, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfePagGate(inf) && sub(inf, "prodPred") == nil, "produto predominante nao informado para modal rodoviario")
	}),
	mdfeRule("F55", 726, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfePagGate(inf) && mdfeDocCount(inf) == 1 && sub(inf, "prodPred", "infLotacao") == nil,
			"carga lotacao (infLotacao) nao informada para um unico DF-e")
	}),
	mdfeRule("F55a", 301, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfePagGate(inf) && mdfeDocCount(inf) == 1 && mdfeStr(sub(inf, "prodPred"), "NCM") == "",
			"NCM do produto predominante nao informado para carga lotacao")
	}),
	mdfeRule("F55b", 302, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfePagGate(inf) && mdfeDocCount(inf) == 1 && len(mdfeInfPag(inf)) == 0,
			"informacoes de pagamento nao informadas para carga lotacao")
	}),
	mdfeRule("F56", 727, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		return mdfeBadDocs([]map[string]any{p}, fmt.Sprintf("pagamento %d: responsavel", i))
	})),
	mdfeRule("F57", 728, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		c := mdfeStr(sub(p, "infBanc"), "CNPJIPEF")
		return mdfeOne(c != "" && !mdfeValidCNPJ(c), fmt.Sprintf("pagamento %d: CNPJ da IPEF invalido", i))
	})),
	mdfeRule("F58", 746, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		vc, ok := num(p, "vContrato")
		comps := list(p, "Comp")
		if !ok || len(comps) == 0 {
			return nil
		}
		s := mdfeSum(comps, "vComp")
		return mdfeOne(math.Abs(s-vc) > mdfeTolMoney+1e-9, fmt.Sprintf("pagamento %d: componentes somam %.2f e o contrato vale %.2f", i, s, vc))
	})),
	mdfeRule("F59", 735, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		if mdfeStr(p, "indPag") != mdfeIndPrazo {
			return nil
		}
		var out []string
		for j, par := range list(p, "infPrazo") {
			if mdfeStr(par, "nParcela") != fmt.Sprintf("%03d", j+1) {
				out = append(out, fmt.Sprintf("pagamento %d: numero da parcela %d invalido", i, j+1))
			}
		}
		return out
	})),
	mdfeRule("F60", 736, ModeWarn, mdfeEveryPag(func(i int, p, inf map[string]any) []string {
		emi, ok := mdfeEmi(sub(inf, "ide"))
		if !ok || mdfeStr(p, "indPag") != mdfeIndPrazo {
			return nil
		}
		day := emi.Format("2006-01-02")
		var out []string
		for j, par := range list(p, "infPrazo") {
			if d := mdfeStr(par, "dVenc"); d != "" && d < day {
				out = append(out, fmt.Sprintf("pagamento %d: parcela %d vence antes da emissao", i, j+1))
			}
		}
		return out
	})),
	mdfeRule("F61", 737, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		if mdfeStr(p, "indPag") != mdfeIndPrazo {
			return nil
		}
		var out []string
		prev := ""
		for j, par := range list(p, "infPrazo") {
			d := mdfeStr(par, "dVenc")
			if prev != "" && d != "" && d < prev {
				out = append(out, fmt.Sprintf("pagamento %d: parcela %d vence antes da anterior", i, j+1))
			}
			if d != "" {
				prev = d
			}
		}
		return out
	})),
	mdfeRule("F62", 738, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		vc, ok := num(p, "vContrato")
		parcelas := list(p, "infPrazo")
		if !ok || mdfeStr(p, "indPag") != mdfeIndPrazo || len(parcelas) == 0 {
			return nil
		}
		adiant, _ := num(p, "vAdiant")
		s := mdfeSum(parcelas, "vParcela") + adiant
		return mdfeOne(math.Abs(s-vc) > mdfeTolMoney+1e-9, fmt.Sprintf("pagamento %d: parcelas e adiantamento somam %.2f e o contrato vale %.2f", i, s, vc))
	})),
	mdfeRule("F63", 739, ModeWarn, mdfeEveryPag(func(i int, p, _ map[string]any) []string {
		return mdfeOne(mdfeStr(p, "indPag") == mdfeIndVista && mdfeStr(p, "vAdiant") != "", fmt.Sprintf("pagamento %d a vista com adiantamento", i))
	})),
	mdfeRule("F64", 740, ModeWarn, func(inf map[string]any) []string {
		d, ok := mdfePartyDoc(sub(mdfeRodo(inf), "veicTracao", "prop"))
		return mdfeOne(mdfeIsRodo(inf) && ok && d == mdfeEmitDoc(inf), "proprietario da tracao igual ao emitente")
	}),
	mdfeRule("F65", 741, ModeWarn, func(inf map[string]any) []string {
		if !mdfeIsRodo(inf) || sub(mdfeRodo(inf), "veicTracao", "prop") == nil {
			return nil
		}
		ct := list(sub(mdfeRodo(inf), "infANTT"), "infContratante")
		ok := false
		if len(ct) == 1 {
			d, has := mdfePartyDoc(ct[0])
			ok = has && d == mdfeEmitDoc(inf)
		}
		return mdfeOne(!ok, "com proprietario da tracao, o contratante deve ser unico e igual ao emitente")
	}),
	mdfeRule("F66", 742, ModeWarn, func(inf map[string]any) []string {
		var ids []string
		for _, c := range list(sub(mdfeRodo(inf), "infANTT"), "infContratante") {
			d, _ := mdfePartyDoc(c)
			if d == "" {
				d = mdfeStr(c, "idEstrangeiro")
			}
			ids = append(ids, d)
		}
		var out []string
		for _, i := range mdfeDup(ids) {
			out = append(out, fmt.Sprintf("contratante %d duplicado", i))
		}
		return out
	}),

	// --- 2.6 Emitente ---
	mdfeRule("F67", 207, ModeBlock, func(inf map[string]any) []string {
		c := mdfeStr(sub(inf, "emit"), "CNPJ")
		return mdfeOne(c != "" && !mdfeValidCNPJ(c), "CNPJ do emitente invalido")
	}),
	mdfeRule("F68", 210, ModeBlock, func(inf map[string]any) []string {
		c := mdfeStr(sub(inf, "emit"), "CPF")
		return mdfeOne(c != "" && !mdfeValidCPF(c), "CPF do emitente invalido")
	}),
	mdfeRule("F69", 232, ModeWarn, func(inf map[string]any) []string {
		s, err := strconv.Atoi(mdfeStr(sub(inf, "ide"), "serie"))
		return mdfeOne(mdfeStr(sub(inf, "emit"), "CNPJ") != "" && err == nil && s >= mdfeSerieMinCPF && s <= mdfeSerieMaxCPF,
			"CNPJ do emitente com serie na faixa 920-969")
	}),
	mdfeRule("F70", 233, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		s, err := strconv.Atoi(mdfeStr(ide, "serie"))
		return mdfeOne(mdfeStr(sub(inf, "emit"), "CPF") != "" && mdfeStr(ide, "tpEmis") != mdfeTpEmisNFF && err == nil && (s < mdfeSerieMinCPF || s > mdfeSerieMaxCPF),
			"CPF do emitente exige serie na faixa 920-969")
	}),
	mdfeRule("F71", 234, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		tp := mdfeStr(ide, "tpEmit")
		bad := tp != mdfeTpEmitPropria && !(mdfeStr(ide, "tpEmis") == mdfeTpEmisNFF && tp == mdfeTpEmitPrestador)
		return mdfeOne(mdfeStr(sub(inf, "emit"), "CPF") != "" && bad, "emitente pessoa fisica exige tipo de emitente carga propria")
	}),
	mdfeRule("F72", 229, ModeWarn, func(inf map[string]any) []string {
		ie := mdfeStr(sub(inf, "emit"), "IE")
		if mdfeStr(sub(inf, "ide"), "tpEmis") == mdfeTpEmisNFF || sub(inf, "infPAA") != nil {
			return nil
		}
		return mdfeOne(ie == "" || strings.Trim(ie, "0") == "", "IE do emitente nao informada")
	}),
	mdfeRule("F77", 407, ModeWarn, func(inf map[string]any) []string {
		if mdfeStr(sub(inf, "ide"), "tpEmis") == mdfeTpEmisNFF {
			return nil
		}
		e := sub(inf, "emit", "enderEmit")
		c, uf := mdfeStr(e, "cMun"), mdfeStr(e, "UF")
		cod, ok := mdfeUFCode[uf]
		return mdfeOne(ok && len(c) >= 2 && c[:2] != cod, "municipio do emitente diverge da UF do emitente")
	}),

	// --- 2.7 Data e hora ---
	mdfeRule("F79", 212, ModeWarn, func(inf map[string]any) []string {
		emi, ok := mdfeEmi(sub(inf, "ide"))
		return mdfeOne(ok && emi.Sub(mdfeNow()) > mdfeMaxSkewFuture, "data/hora de emissao posterior a data/hora atual")
	}),
	mdfeRule("F80", 228, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		emi, ok := mdfeEmi(ide)
		return mdfeOne(ok && mdfeStr(ide, "tpEmis") == mdfeTpEmisNormal && mdfeNow().Sub(emi) > mdfeMaxLate, "data de emissao com atraso superior a 24 horas")
	}),

	// --- 2.9 Modal rodoviário ---
	mdfeRule("F89", 646, ModeWarn, func(inf map[string]any) []string {
		ini, fim := mdfeUFInfo(inf)
		if !mdfeIsRodo(inf) || ini == mdfeUFExterior || fim == mdfeUFExterior {
			return nil
		}
		var out []string
		if p := mdfeStr(sub(mdfeRodo(inf), "veicTracao"), "placa"); p != "" && !mdfePlacaRe.MatchString(p) {
			out = append(out, "placa do veiculo de tracao em formato invalido")
		}
		for i, r := range list(mdfeRodo(inf), "veicReboque") {
			if p := mdfeStr(r, "placa"); p != "" && !mdfePlacaRe.MatchString(p) {
				out = append(out, fmt.Sprintf("placa do reboque %d em formato invalido", i+1))
			}
		}
		return out
	}),
	mdfeRule("F89c", 523, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfeIsRodo(inf) && mdfeStr(sub(mdfeRodo(inf), "veicTracao"), "tpRod") == mdfeTpRodCavalo && len(list(mdfeRodo(inf), "veicReboque")) == 0,
			"cavalo mecanico exige ao menos um reboque")
	}),
	mdfeRule("F90", 663, ModeWarn, func(inf map[string]any) []string {
		ini, fim := mdfeUFInfo(inf)
		per := mdfeUFPer(inf)
		if !mdfeIsRodo(inf) || len(per) == 0 || ini == "" || fim == "" {
			return nil
		}
		path := append(append([]string{ini}, per...), fim)
		var out []string
		for i := 0; i+1 < len(path); i++ {
			a, b := path[i], path[i+1]
			if a == mdfeUFExterior || b == mdfeUFExterior || a == b {
				continue
			}
			if !mdfeAdjacent[a][b] {
				out = append(out, fmt.Sprintf("percurso invalido: trecho %d nao e divisa entre UFs", i+1))
			}
		}
		return out
	}),
	mdfeRule("F91", 698, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		tp := mdfeStr(ide, "tpEmit")
		return mdfeOne(mdfeIsRodo(inf) && (tp == mdfeTpEmitPrestador || tp == mdfeTpEmitGlobal) && mdfeStr(ide, "tpEmis") != mdfeTpEmisNFF && len(list(inf, "seg")) == 0,
			"seguro da carga obrigatorio para prestador de servico de transporte")
	}),
	mdfeRule("F92", 699, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		tp := mdfeStr(ide, "tpEmit")
		if !mdfeIsRodo(inf) || (tp != mdfeTpEmitPrestador && tp != mdfeTpEmitGlobal) {
			return nil
		}
		nff := mdfeStr(ide, "tpEmis") == mdfeTpEmisNFF
		var out []string
		for i, s := range list(inf, "seg") {
			if mdfeStr(sub(s, "infSeg"), "CNPJ") == "" || mdfeStr(sub(s, "infSeg"), "xSeg") == "" || mdfeStr(s, "nApol") == "" || (!nff && mdfeStr(s, "nAver") == "") {
				out = append(out, fmt.Sprintf("seguro %d incompleto", i+1))
			}
		}
		return out
	}),
	mdfeRule("F93", 542, ModeWarn, func(inf map[string]any) []string {
		ide := sub(inf, "ide")
		tp := mdfeStr(ide, "tpEmit")
		if !mdfeIsRodo(inf) || (tp != mdfeTpEmitPrestador && tp != mdfeTpEmitGlobal) || mdfeStr(ide, "tpEmis") == mdfeTpEmisNFF {
			return nil
		}
		var out []string
		for i, s := range list(inf, "seg") {
			resp := sub(s, "infResp")
			if _, ok := mdfePartyDoc(resp); mdfeStr(resp, "respSeg") == mdfeRespSegContr && !ok {
				out = append(out, fmt.Sprintf("seguro %d: responsavel contratante sem CNPJ/CPF", i+1))
			}
		}
		return out
	}),
	mdfeRule("F94", 578, ModeWarn, func(inf map[string]any) []string {
		if !mdfePagGate(inf) {
			return nil
		}
		antt := sub(mdfeRodo(inf), "infANTT")
		payer := false
		for _, d := range list(sub(antt, "valePed"), "disp") {
			if mdfeStr(d, "CNPJPg") != "" || mdfeStr(d, "CPFPg") != "" {
				payer = true
			}
		}
		if len(list(antt, "infCIOT")) > 0 || payer {
			return nil
		}
		return mdfeOne(len(list(antt, "infContratante")) == 0, "informacoes dos tomadores obrigatorias para esta operacao")
	}),
	mdfeRule("F95", 731, ModeWarn, func(inf map[string]any) []string {
		vp := sub(mdfeRodo(inf), "infANTT", "valePed")
		return mdfeOne(vp != nil && mdfeStr(vp, "categCombVeic") == "", "vale pedagio sem categoria de combinacao veicular")
	}),
	mdfeRule("F96", 732, ModeBlock, func(inf map[string]any) []string {
		var out []string
		for i, d := range list(sub(mdfeRodo(inf), "infANTT", "valePed"), "disp") {
			if c := mdfeStr(d, "CNPJForn"); c != "" && !mdfeValidCNPJ(c) {
				out = append(out, fmt.Sprintf("vale pedagio %d: CNPJ do fornecedor invalido", i+1))
			}
		}
		return out
	}),
	mdfeRule("F98", 734, ModeBlock, func(inf map[string]any) []string {
		var out []string
		for i, d := range list(sub(mdfeRodo(inf), "infANTT", "valePed"), "disp") {
			if c := mdfeStr(d, "CNPJPg"); c != "" && !mdfeValidCNPJ(c) {
				out = append(out, fmt.Sprintf("vale pedagio %d: CNPJ do pagador invalido", i+1))
			}
			if c := mdfeStr(d, "CPFPg"); c != "" && !mdfeValidCPF(c) {
				out = append(out, fmt.Sprintf("vale pedagio %d: CPF do pagador invalido", i+1))
			}
		}
		return out
	}),
	mdfeRule("F99", 577, ModeWarn, func(inf map[string]any) []string {
		var cpfs []string
		for _, c := range list(sub(mdfeRodo(inf), "veicTracao"), "condutor") {
			cpfs = append(cpfs, mdfeStr(c, "CPF"))
		}
		var out []string
		for _, i := range mdfeDup(cpfs) {
			out = append(out, fmt.Sprintf("condutor %d duplicado", i))
		}
		return out
	}),
	mdfeRule("F100", 645, ModeBlock, func(inf map[string]any) []string {
		var out []string
		for i, c := range list(sub(mdfeRodo(inf), "veicTracao"), "condutor") {
			if d := mdfeStr(c, "CPF"); d != "" && !mdfeValidCPF(d) {
				out = append(out, fmt.Sprintf("CPF do condutor %d invalido", i+1))
			}
		}
		return out
	}),
	mdfeRule("F101", 716, ModeBlock, func(inf map[string]any) []string {
		return mdfeBadDocs(list(sub(mdfeRodo(inf), "infANTT"), "infCIOT"), "CIOT")
	}),
	mdfeRule("F102", 717, ModeBlock, func(inf map[string]any) []string {
		return mdfeBadDocs(list(sub(mdfeRodo(inf), "infANTT"), "infContratante"), "contratante")
	}),
	mdfeRule("F103", 718, ModeBlock, func(inf map[string]any) []string {
		p := sub(mdfeRodo(inf), "veicTracao", "prop")
		if p == nil {
			return nil
		}
		return mdfeBadDocs([]map[string]any{p}, "proprietario da tracao")
	}),
	mdfeRule("F104", 719, ModeBlock, func(inf map[string]any) []string {
		var props []map[string]any
		for _, r := range list(mdfeRodo(inf), "veicReboque") {
			if p := sub(r, "prop"); p != nil {
				props = append(props, p)
			}
		}
		return mdfeBadDocs(props, "proprietario do reboque")
	}),

	// --- 2.10 Autorizados ao XML ---
	mdfeRule("F105", 660, ModeBlock, func(inf map[string]any) []string {
		var out []string
		for i, a := range list(inf, "autXML") {
			if c := mdfeStr(a, "CNPJ"); c != "" && !mdfeValidCNPJ(c) {
				out = append(out, fmt.Sprintf("autorizado ao XML %d: CNPJ invalido", i+1))
			}
		}
		return out
	}),
	mdfeRule("F106", 661, ModeBlock, func(inf map[string]any) []string {
		var out []string
		for i, a := range list(inf, "autXML") {
			if c := mdfeStr(a, "CPF"); c != "" && !mdfeValidCPF(c) {
				out = append(out, fmt.Sprintf("autorizado ao XML %d: CPF invalido", i+1))
			}
		}
		return out
	}),
	mdfeRule("F107", 459, ModeWarn, func(inf map[string]any) []string {
		var ids []string
		for _, a := range list(inf, "autXML") {
			d, _ := mdfePartyDoc(a)
			ids = append(ids, d)
		}
		var out []string
		for _, i := range mdfeDup(ids) {
			out = append(out, fmt.Sprintf("autorizado ao XML %d duplicado", i))
		}
		return out
	}),

	// --- 2.11 ANTT (só o que se prova pela presença dos grupos) ---
	mdfeRule("F108", 688, ModeWarn, func(inf map[string]any) []string {
		ini, fim := mdfeUFInfo(inf)
		tp := mdfeStr(sub(inf, "ide"), "tpEmit")
		return mdfeOne(mdfeIsRodo(inf) && ini != fim && (tp == mdfeTpEmitPrestador || tp == mdfeTpEmitGlobal) &&
			mdfeStr(sub(mdfeRodo(inf), "infANTT"), "RNTRC") == "", "RNTRC deve ser informado para prestador de servico de transporte")
	}),
	// F113: o MOC traz dois textos. O que se prova offline é o segundo (emitente na condição comum, RNTRC informado,
	// fora do exterior: infCIOT deve existir). Se o CIOT é obrigatório para aquele RNTRC depende do cadastro da ANTT.
	mdfeRule("F113", 684, ModeWarn, func(inf map[string]any) []string {
		ini, fim := mdfeUFInfo(inf)
		antt := sub(mdfeRodo(inf), "infANTT")
		return mdfeOne(mdfePagGate(inf) && ini != mdfeUFExterior && fim != mdfeUFExterior && mdfeStr(antt, "RNTRC") != "" && len(list(antt, "infCIOT")) == 0,
			"CIOT deve ser informado para o RNTRC informado")
	}),

	// --- 2.12 QR Code ---
	mdfeRuleDoc("F114", 480, ModeWarn, func(doc, _ map[string]any) []string {
		return mdfeOne(sub(doc, "infMDFeSupl") == nil, "grupo do QR Code (infMDFeSupl) nao informado")
	}),
	mdfeRuleDoc("F115", 479, ModeWarn, func(doc, _ map[string]any) []string {
		_, hostOK, ok := mdfeQR(doc)
		return mdfeOne(ok && !hostOK, "endereco do QR Code difere do previsto")
	}),
	mdfeRuleDoc("F116", 481, ModeWarn, func(doc, inf map[string]any) []string {
		q, _, ok := mdfeQR(doc)
		return mdfeOne(ok && q.Get("chMDFe") != mdfeKeyOf(inf), "chave de acesso do QR Code diverge da do MDF-e")
	}),
	mdfeRuleDoc("F117", 482, ModeWarn, func(doc, inf map[string]any) []string {
		q, _, ok := mdfeQR(doc)
		return mdfeOne(ok && mdfeStr(sub(inf, "ide"), "tpEmis") == "2" && q.Get("sign") == "", "QR Code de contingencia sem o parametro sign")
	}),
	mdfeRuleDoc("F118", 488, ModeWarn, func(doc, inf map[string]any) []string {
		q, _, ok := mdfeQR(doc)
		return mdfeOne(ok && mdfeStr(sub(inf, "ide"), "tpEmis") == mdfeTpEmisNormal && q.Get("sign") != "", "QR Code de emissao normal com o parametro sign")
	}),

	// --- 2.13 a 2.14 e PAA ---
	mdfeRule("F121", 713, ModeWarn, func(inf map[string]any) []string {
		c := mdfeStr(sub(inf, "infRespTec"), "CNPJ")
		return mdfeOne(c != "" && !mdfeValidCNPJ(c), "CNPJ do responsavel tecnico invalido")
	}),
	mdfeRule("F123", 902, ModeWarn, func(inf map[string]any) []string {
		return mdfeOne(mdfeStr(sub(inf, "ide"), "tpEmis") != mdfeTpEmisNFF && sub(inf, "infSolicNFF") != nil,
			"grupo do pedido da NFF informado fora do Regime Especial NFF")
	}),
	mdfeRule("PAA01", 909, ModeWarn, func(inf map[string]any) []string {
		paa := sub(inf, "infPAA")
		return mdfeOne(paa != nil && !mdfeValidCNPJ(mdfeStr(paa, "CNPJPAA")), "CNPJ do PAA invalido")
	}),
}

func mdfeUFPer(inf map[string]any) []string {
	var out []string
	for _, p := range list(sub(inf, "ide"), "infPercurso") {
		out = append(out, mdfeStr(p, "UFPer"))
	}
	return out
}

func mdfePrestParcial(inf map[string]any) []string {
	var out []string
	for _, m := range mdfeMuns(inf) {
		for _, c := range list(m, "infCTe") {
			for _, n := range list(c, "infNFePrestParcial") {
				out = append(out, mdfeStr(n, "chNFe"))
			}
		}
	}
	return out
}

// mdfeDupDocs (F28 e F29): interestadual, a chave não repete no MDF-e; interna, não repete dentro do município.
func mdfeDupDocs(inf map[string]any, group, ch, label string) []string {
	ini, fim := mdfeUFInfo(inf)
	var out []string
	if ini != fim {
		for _, i := range mdfeDup(mdfeKeys(inf, group, ch)) {
			out = append(out, fmt.Sprintf("chave de %s %d duplicada", label, i))
		}
		return out
	}
	for mi, m := range mdfeMuns(inf) {
		var ks []string
		for _, d := range list(m, group) {
			ks = append(ks, mdfeStr(d, ch))
		}
		for _, i := range mdfeDup(ks) {
			out = append(out, fmt.Sprintf("municipio %d: chave de %s %d duplicada", mi+1, label, i))
		}
	}
	return out
}

func init() { catalog = append(catalog, mdfeRules...) }
