package validate

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Regras de negócio do CT-e modelo 57 (MOC "Regras de Validação", item 2.1 e atualizações do item 3) provadas
// offline a partir do dicionário do documento. Check recebe o conteúdo de <infCte>. O id é o da regra no MOC;
// as regras do item 3 sem id próprio (ou com id que repete o do item 2) levam prefixo da seção (PAA-, CGOV-,
// IBSCBS-, SN-, ANT-, NT2026.004-). Warn por padrão; Block só para identidades determinísticas (DV de chave,
// CPF/CNPJ, totais exatos). Regras que dependem de banco de dados, cadastros, relógio ou tabelas externas ficam fora.

func init() { catalog = append(catalog, cteRules...) }

var cteRules = cteBuild()

// Como nas outras famílias, Check recebe o conteúdo de <CTe> (doc["infCte"]); o catálogo e seus testes
// trabalham sobre o infCte, que também é aceito direto.
func cteR(id string, cstat int, mode Mode, f func(inf map[string]any) []string) Rule {
	return Rule{ID: id, CStat: cstat, Mode: mode, Doc: "cte", Check: func(doc map[string]any) []string {
		if inf := sub(doc, "infCte"); inf != nil {
			return f(inf)
		}
		return f(doc)
	}}
}

// cteCond monta uma regra de achado único: cond verdadeiro gera a mensagem.
func cteCond(id string, cstat int, mode Mode, msg string, cond func(inf map[string]any) bool) Rule {
	return cteR(id, cstat, mode, func(inf map[string]any) []string {
		if cond(inf) {
			return []string{msg}
		}
		return nil
	})
}

func cteBuild() []Rule {
	var rs []Rule
	rs = append(rs, cteIdeRules()...)
	rs = append(rs, cteActorRules()...)
	rs = append(rs, cteDocRules()...)
	rs = append(rs, cteValueRules()...)
	rs = append(rs, cteGlobalRules()...)
	rs = append(rs, cteRTCRules()...)
	return rs
}

// --- ide, chave, contingência, tomador ---

func cteIdeRules() []Rule {
	var rs []Rule
	for _, h := range []struct {
		id    string
		cstat int
		tag   string
	}{{"G002", 646, "rem"}, {"G003", 647, "exped"}, {"G004", 648, "receb"}, {"G005", 649, "dest"}} {
		h := h
		rs = append(rs, cteCond(h.id, h.cstat, ModeWarn, "em homologação o xNome de "+h.tag+" deve ser a literal do MOC", func(inf map[string]any) bool {
			a := sub(inf, h.tag)
			return cteS(sub(inf, "ide"), "tpAmb") == "2" && !cteNFF(inf) && a != nil && cteS(a, "xNome") != cteHomolXNome
		}))
	}
	rs = append(rs,
		cteCond("G006", 670, ModeWarn, "série na faixa reservada 890-899", func(inf map[string]any) bool {
			s := cteS(sub(inf, "ide"), "serie")
			return !cteNFF(inf) && len(s) == 3 && s >= "890" && s <= "899"
		}),
		cteCond("G007", 247, ModeWarn, "UF do emitente difere da UF (cUF) da chave de acesso", func(inf map[string]any) bool {
			uf := cteUF[cteS(sub(inf, "ide"), "cUF")]
			return !cteNFF(inf) && uf != "" && cteActorUF(inf, "emit") != "" && uf != cteActorUF(inf, "emit")
		}),
		cteCond("G008", 586, ModeWarn, "dhCont e xJust só podem ser informados em contingência offline", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return cteS(ide, "tpEmis") != "2" && (ide["dhCont"] != nil || ide["xJust"] != nil)
		}),
		cteCond("G008b", 937, ModeWarn, "dhCont e xJust não podem ser informados na emissão normal", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return cteS(ide, "tpEmis") == "1" && (ide["dhCont"] != nil || ide["xJust"] != nil)
		}),
		cteCond("G009", 587, ModeWarn, "dhCont e xJust devem ser informados em contingência offline", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return cteS(ide, "tpEmis") == "2" && (cteS(ide, "dhCont") == "" || cteS(ide, "xJust") == "")
		}),
		cteCond("G010", 588, ModeWarn, "dhCont posterior a dhEmi", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			c, e1 := time.Parse(time.RFC3339, cteS(ide, "dhCont"))
			e, e2 := time.Parse(time.RFC3339, cteS(ide, "dhEmi"))
			return e1 == nil && e2 == nil && c.After(e)
		}),
		cteCond("G012", 517, ModeWarn, "CT-e em SVC (tpEmis 7 ou 8) deve ser do tipo normal", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return cteIn(cteS(ide, "tpEmis"), "7", "8") && cteS(ide, "tpCTe") != "0"
		}),
		cteCond("G013", 732, ModeWarn, "modelo da chave de acesso diferente de 57", func(inf map[string]any) bool {
			id := cteS(inf, "@Id")
			return cteS(sub(inf, "ide"), "mod") != "57" || (len(id) == 47 && id[23:25] != "57")
		}),
		cteCond("G014", 227, ModeWarn, "campo Id difere da concatenação dos campos da chave", func(inf map[string]any) bool {
			body := cteKeyBody(inf)
			id := cteS(inf, "@Id")
			return body != "" && (!strings.HasPrefix(id, "CTe") || len(id) != 47 || id[3:46] != body || id[46:] != cteS(sub(inf, "ide"), "cDV"))
		}),
		cteCond("G015", 253, ModeBlock, "dígito verificador da chave de acesso inválido", func(inf map[string]any) bool {
			body := cteKeyBody(inf)
			return body != "" && cteS(sub(inf, "ide"), "cDV") != fmt.Sprint(cteDV(body))
		}),
		cteCond("G016", 458, ModeWarn, "CT-e normal ou substituto exige o grupo infCTeNorm", func(inf map[string]any) bool {
			return cteNormal(inf) && sub(inf, "infCTeNorm") == nil
		}),
		cteCond("G017", 459, ModeWarn, "CT-e complementar exige o grupo infCteComp", func(inf map[string]any) bool {
			return cteS(sub(inf, "ide"), "tpCTe") == "1" && len(list(inf, "infCteComp")) == 0
		}),
	)
	for _, t := range []struct {
		id    string
		cstat int
		tag   string
	}{{"G018", 460, "rem"}, {"G019", 461, "exped"}, {"G020", 462, "receb"}, {"G021", 463, "dest"}} {
		t := t
		rs = append(rs, cteCond(t.id, t.cstat, ModeWarn, "tomador indicado como "+t.tag+" mas o grupo não existe", func(inf map[string]any) bool {
			tag, ok := cteTomador(inf)
			return ok && tag == t.tag && sub(inf, t.tag) == nil
		}))
	}
	rs = append(rs,
		cteCond("G022", 481, ModeWarn, "indIEToma=1 exige IE do tomador diferente de ISENTO", func(inf map[string]any) bool {
			tag, ok := cteTomador(inf)
			ie := cteS(cteActor(inf, tag), "IE")
			return ok && cteS(sub(inf, "ide"), "indIEToma") == "1" && (ie == "" || ie == "ISENTO")
		}),
		cteCond("G023", 482, ModeWarn, "indIEToma=2 exige IE do tomador igual a ISENTO", func(inf map[string]any) bool {
			tag, ok := cteTomador(inf)
			return ok && cteS(sub(inf, "ide"), "indIEToma") == "2" && cteS(cteActor(inf, tag), "IE") != "ISENTO"
		}),
		cteCond("G024", 617, ModeWarn, "tomador isento de IE em UF que não permite", func(inf map[string]any) bool {
			tag, ok := cteTomador(inf)
			return ok && cteS(sub(inf, "ide"), "indIEToma") == "2" &&
				cteIn(cteActorUF(inf, tag), "AM", "BA", "CE", "GO", "MG", "MS", "MT", "PE", "RN", "SE", "SP", "PI", "PA", "RJ")
		}),
		cteCond("G076", 503, ModeWarn, "CT-e substituto deve ter tipo de emissão normal", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return cteS(ide, "tpCTe") == "3" && cteS(ide, "tpEmis") != "1"
		}),
		cteCond("G077", 505, ModeWarn, "CT-e substituto exige o grupo infCteSub", func(inf map[string]any) bool {
			return cteS(sub(inf, "ide"), "tpCTe") == "3" && sub(inf, "infCTeNorm", "infCteSub") == nil
		}),
		cteR("G078", 847, ModeWarn, func(inf map[string]any) []string {
			k := cteS(sub(inf, "infCTeNorm", "infCteSub"), "chCte")
			if r := cteKeyReason(k, "57", 2009); cteS(sub(inf, "ide"), "tpCTe") == "3" && k != "" && r != "" {
				return cteMsg("chave do CT-e substituído: %s", r)
			}
			return nil
		}),
		cteCond("G218", 902, ModeWarn, "infSolicNFF só pode existir no Regime Especial da NFF (tpEmis 3)", func(inf map[string]any) bool {
			return sub(inf, "infSolicNFF") != nil && !cteNFF(inf)
		}),
		cteCond("G105", 229, ModeWarn, "IE do emitente deve ser informada", func(inf map[string]any) bool {
			e := sub(inf, "emit")
			paaMEI := sub(inf, "infPAA") != nil && cteS(sub(inf, "ide"), "procEmi") == "4" && cteS(e, "CRT") == "4"
			return !cteNFF(inf) && !paaMEI && strings.Trim(cteS(e, "IE"), "0") == ""
		}),
		cteCond("G104", 207, ModeBlock, "CNPJ do emitente zerado, nulo ou com dígito inválido", func(inf map[string]any) bool {
			return !cteNFF(inf) && !cteCNPJOK(cteS(sub(inf, "emit"), "CNPJ"))
		}),
		cteCond("G112", 712, ModeWarn, "município do emitente não pertence à UF informada", func(inf map[string]any) bool {
			return !cteNFF(inf) && cteMunUFBad(sub(inf, "emit", "enderEmit"))
		}),
		cteCond("G178", 493, ModeWarn, "município de envio não pertence à UFEnv", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return cteMunBad(cteS(ide, "cMunEnv"), cteS(ide, "UFEnv"))
		}),
		cteCond("G180", 456, ModeWarn, "município de início da prestação não pertence à UFIni", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return cteMunBad(cteS(ide, "cMunIni"), cteS(ide, "UFIni"))
		}),
		cteCond("G182", 414, ModeWarn, "município de término da prestação não pertence à UFFim", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return cteMunBad(cteS(ide, "cMunFim"), cteS(ide, "UFFim"))
		}),
		cteCond("G050", 519, ModeWarn, "primeiro dígito do CFOP incompatível com as UFs da prestação", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			cf := cteS(ide, "CFOP")
			ini, fim := cteS(ide, "UFIni"), cteS(ide, "UFFim")
			if cf == "" || ini == "" || fim == "" {
				return false
			}
			want := "6"
			switch {
			case fim == "EX":
				want = "7"
			case ini == fim:
				want = "5"
			}
			return cf[:1] != want
		}),
		cteCond("G051", 524, ModeWarn, "UF do emitente difere da de início da prestação: CFOP deve ser 5932 ou 6932", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			ini, fim, uf := cteS(ide, "UFIni"), cteS(ide, "UFFim"), cteActorUF(inf, "emit")
			return !cteNFF(inf) && uf != "" && uf != ini && ini != "EX" && fim != "EX" && !cteIn(cteS(ide, "CFOP"), "5932", "6932")
		}),
		cteCond("G052", 908, ModeWarn, "UF do emitente igual à de início da prestação: CFOP não pode ser 5932 ou 6932", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			ini, fim, uf := cteS(ide, "UFIni"), cteS(ide, "UFFim"), cteActorUF(inf, "emit")
			return !cteNFF(inf) && uf != "" && uf == ini && ini != "EX" && fim != "EX" && cteIn(cteS(ide, "CFOP"), "5932", "6932")
		}),
		// Processo de emissão por PAA (item 3.1; as regras não têm id próprio no MOC).
		cteCond("PAA-910", 910, ModeWarn, "com infPAA o procEmi deve ser 4", func(inf map[string]any) bool {
			return sub(inf, "infPAA") != nil && cteS(sub(inf, "ide"), "procEmi") != "4"
		}),
		cteCond("PAA-911", 911, ModeWarn, "sem infPAA o procEmi não pode ser 4", func(inf map[string]any) bool {
			return sub(inf, "infPAA") == nil && cteS(sub(inf, "ide"), "procEmi") == "4"
		}),
		cteCond("PAA-912", 912, ModeWarn, "série fora da faixa do PAA (970-979 para CPF, 980-989 para CNPJ)", func(inf map[string]any) bool {
			if sub(inf, "infPAA") == nil {
				return false
			}
			s := cteS(sub(inf, "ide"), "serie")
			if cteS(sub(inf, "emit"), "CPF") != "" {
				return s < "970" || s > "979" || len(s) != 3
			}
			return s < "980" || s > "989" || len(s) != 3
		}),
		cteCond("PAA-913", 913, ModeWarn, "com infPAA o CRT do emitente deve ser 1 ou 4", func(inf map[string]any) bool {
			return sub(inf, "infPAA") != nil && !cteIn(cteS(sub(inf, "emit"), "CRT"), "1", "4")
		}),
		cteCond("PAA01", 914, ModeBlock, "CNPJ do PAA zerado ou com dígito inválido", func(inf map[string]any) bool {
			p := sub(inf, "infPAA")
			return p != nil && !cteCNPJOK(cteS(p, "CNPJPAA"))
		}),
	)
	return rs
}

// cteMunBad: as 2 primeiras posições do cMun não batem com a UF (EX e valores incompletos não se aplicam).
func cteMunBad(cMun, uf string) bool {
	if len(cMun) != 7 || uf == "" || uf == "EX" {
		return false
	}
	return cteUF[cMun[:2]] != uf
}

func cteMunUFBad(ender map[string]any) bool {
	return ender != nil && cteMunBad(cteS(ender, "cMun"), cteS(ender, "UF"))
}

// --- atores: CNPJ/CPF e município ---

func cteActorRules() []Rule {
	var rs []Rule
	for _, a := range []struct {
		tag, cnpj, cpf, mun string
		c1, c2, c3          int
	}{
		{"rem", "G114", "G115", "G116", 415, 416, 418},
		{"dest", "G122", "G123", "G124", 208, 237, 424},
		{"exped", "G132", "G133", "G134", 428, 429, 431},
		{"receb", "G140", "G141", "G142", 436, 437, 439},
		{"toma4", "G148", "G149", "G150", 444, 445, 447},
	} {
		a := a
		rs = append(rs,
			cteCond(a.cnpj, a.c1, ModeBlock, "CNPJ de "+a.tag+" zerado ou com dígito inválido", func(inf map[string]any) bool {
				n := cteActor(inf, a.tag)
				return cteS(n, "CNPJ") != "" && !cteCNPJOK(cteS(n, "CNPJ"))
			}),
			cteCond(a.cpf, a.c2, ModeBlock, "CPF de "+a.tag+" zerado, repetido ou com dígito inválido", func(inf map[string]any) bool {
				n := cteActor(inf, a.tag)
				return cteS(n, "CPF") != "" && !cteCPFOK(cteS(n, "CPF"))
			}),
			cteCond(a.mun, a.c3, ModeWarn, "município de "+a.tag+" não pertence à UF informada", func(inf map[string]any) bool {
				return cteMunUFBad(sub(cteActor(inf, a.tag), cteEnder[a.tag]))
			}),
		)
	}
	// autXML (G193 a G196).
	rs = append(rs,
		cteR("G193", 699, ModeBlock, func(inf map[string]any) []string {
			var out []string
			for i, a := range list(inf, "autXML") {
				if v := cteS(a, "CNPJ"); v != "" && !cteCNPJOK(v) {
					out = append(out, fmt.Sprintf("autXML %d: CNPJ inválido", i+1))
				}
			}
			return out
		}),
		cteR("G194", 700, ModeBlock, func(inf map[string]any) []string {
			var out []string
			for i, a := range list(inf, "autXML") {
				if v := cteS(a, "CPF"); v != "" && !cteCPFOK(v) {
					out = append(out, fmt.Sprintf("autXML %d: CPF inválido", i+1))
				}
			}
			return out
		}),
		cteR("G195", 715, ModeWarn, func(inf map[string]any) []string {
			var ids []string
			for _, a := range list(inf, "autXML") {
				ids = append(ids, cteDoc(a))
			}
			if d := cteDup(ids); d != "" {
				return cteMsg("autXML duplicado: %s", d)
			}
			return nil
		}),
		cteR("G196", 828, ModeWarn, func(inf map[string]any) []string {
			actors := map[string]bool{}
			for _, t := range []string{"emit", "rem", "exped", "receb", "dest", "toma4"} {
				if d := cteDoc(cteActor(inf, t)); d != "" {
					actors[d] = true
				}
			}
			var out []string
			for _, a := range list(inf, "autXML") {
				if d := cteDoc(a); d != "" && actors[d] {
					out = append(out, "autXML repete ator do CT-e: "+d)
				}
			}
			return out
		}),
		cteCond("G210", 836, ModeBlock, "CNPJ do responsável técnico inválido", func(inf map[string]any) bool {
			r := sub(inf, "infRespTec")
			return r != nil && !cteCNPJOK(cteS(r, "CNPJ"))
		}),
	)
	return rs
}

// --- documentos transportados, docAnt, tipo de serviço ---

func cteDocRules() []Rule {
	servRedespInt := func(inf map[string]any) bool { return cteIn(cteS(sub(inf, "ide"), "tpServ"), "3", "4") }
	rs := []Rule{
		cteCond("G027", 581, ModeWarn, "vCarga deve ser informado (modal diferente de dutoviário)", func(inf map[string]any) bool {
			return cteNormal(inf) && cteS(sub(inf, "ide"), "modal") != "06" && cteS(sub(inf, "infCTeNorm", "infCarga"), "vCarga") == ""
		}),
		cteCond("G036", 693, ModeWarn, "infDoc deve ser informado", func(inf map[string]any) bool {
			return cteNormal(inf) && !servRedespInt(inf) && sub(inf, "infCTeNorm", "infDoc") == nil
		}),
		cteCond("G037", 694, ModeWarn, "infDoc não deve ser informado em redespacho intermediário ou serviço vinculado", func(inf map[string]any) bool {
			return cteNormal(inf) && servRedespInt(inf) && sub(inf, "infCTeNorm", "infDoc") != nil
		}),
		cteCond("G045a", 601, ModeWarn, "mais de 2000 documentos em infDoc", func(inf map[string]any) bool {
			d := sub(inf, "infCTeNorm", "infDoc")
			return cteNormal(inf) && len(list(d, "infNF"))+len(list(d, "infNFe"))+len(list(d, "infOutros"))+len(list(d, "infDCe")) > 2000
		}),
		cteR("G039", 527, ModeWarn, func(inf map[string]any) []string {
			if d := cteDup(cteNFeKeys(inf)); cteNormal(inf) && d != "" {
				return cteMsg("chave de NF-e duplicada: %s", d)
			}
			return nil
		}),
		cteR("G040", 843, ModeWarn, func(inf map[string]any) []string {
			if !cteNormal(inf) {
				return nil
			}
			for _, k := range cteNFeKeys(inf) {
				if r := cteKeyReason(k, "55", 2005); r != "" {
					return cteMsg("chave de NF-e %s: %s", k, r)
				}
			}
			return nil
		}),
		cteR("G045b", 375, ModeWarn, func(inf map[string]any) []string {
			var ks []string
			for _, n := range list(sub(inf, "infCTeNorm", "infDoc"), "infDCe") {
				ks = append(ks, cteS(n, "chave"))
			}
			if d := cteDup(ks); d != "" {
				return cteMsg("chave de DC-e duplicada: %s", d)
			}
			return nil
		}),
		cteR("G045c", 376, ModeWarn, func(inf map[string]any) []string {
			for _, n := range list(sub(inf, "infCTeNorm", "infDoc"), "infDCe") {
				if r := cteKeyReason(cteS(n, "chave"), "62", 2025); r != "" {
					return cteMsg("chave de DC-e: %s", r)
				}
			}
			return nil
		}),
		cteCond("G053", 469, ModeWarn, "remetente deve ser informado", func(inf map[string]any) bool {
			return !servRedespInt(inf) && sub(inf, "rem") == nil
		}),
		cteCond("G054", 470, ModeWarn, "destinatário deve ser informado", func(inf map[string]any) bool {
			return !servRedespInt(inf) && sub(inf, "dest") == nil
		}),
		cteCond("G055", 474, ModeWarn, "expedidor deve ser informado em redespacho intermediário ou serviço vinculado", func(inf map[string]any) bool {
			return servRedespInt(inf) && sub(inf, "exped") == nil
		}),
		cteCond("G056", 475, ModeWarn, "recebedor deve ser informado em redespacho intermediário ou serviço vinculado", func(inf map[string]any) bool {
			return servRedespInt(inf) && sub(inf, "receb") == nil
		}),
		cteCond("G057", 651, ModeWarn, "serviço vinculado a multimodal exige chCTeMultimodal", func(inf map[string]any) bool {
			return cteNormal(inf) && cteS(sub(inf, "ide"), "tpServ") == "4" &&
				len(list(sub(inf, "infCTeNorm", "infServVinc"), "infCTeMultimodal")) == 0
		}),
		cteCond("G058", 814, ModeWarn, "infServVinc só é permitido no serviço vinculado a multimodal", func(inf map[string]any) bool {
			return cteS(sub(inf, "ide"), "tpServ") != "4" && sub(inf, "infCTeNorm", "infServVinc") != nil
		}),
		cteCond("G059", 521, ModeWarn, "docAnt deve ser informado em subcontratação, redespacho ou redespacho intermediário", func(inf map[string]any) bool {
			return cteNormal(inf) && cteIn(cteS(sub(inf, "ide"), "tpServ"), "1", "2", "3") && sub(inf, "infCTeNorm", "docAnt") == nil
		}),
		cteCond("G060", 747, ModeWarn, "docAnt não pode existir com tipo de serviço normal", func(inf map[string]any) bool {
			return cteS(sub(inf, "ide"), "tpServ") == "0" && sub(inf, "infCTeNorm", "docAnt") != nil
		}),
		cteR("G061", 543, ModeWarn, func(inf map[string]any) []string {
			if d := cteDup(cteDocAntKeys(inf)); d != "" {
				return cteMsg("chave de CT-e duplicada em docAnt: %s", d)
			}
			return nil
		}),
		cteR("G065", 844, ModeWarn, func(inf map[string]any) []string {
			if !cteNormal(inf) {
				return nil
			}
			for _, k := range cteDocAntKeys(inf) {
				if r := cteKeyReason(k, "57", 2009); r != "" {
					return cteMsg("chave de docAnt %s: %s", k, r)
				}
			}
			return nil
		}),
		cteR("G070", 714, ModeWarn, func(inf map[string]any) []string {
			if d := cteDup(cteMultimodalKeys(inf)); d != "" {
				return cteMsg("chave de CT-e multimodal duplicada: %s", d)
			}
			return nil
		}),
		cteR("G071", 845, ModeWarn, func(inf map[string]any) []string {
			if !cteNormal(inf) {
				return nil
			}
			for _, k := range cteMultimodalKeys(inf) {
				if r := cteKeyReason(k, "57", 2009); r != "" {
					return cteMsg("chave multimodal %s: %s", k, r)
				}
			}
			return nil
		}),
		cteR("G161", 849, ModeWarn, func(inf map[string]any) []string {
			if cteS(sub(inf, "ide"), "tpCTe") != "1" {
				return nil
			}
			for _, c := range list(inf, "infCteComp") {
				if r := cteKeyReason(cteS(c, "chCTe"), "57", 2009); r != "" {
					return cteMsg("chave do CT-e complementado: %s", r)
				}
			}
			return nil
		}),
		cteR("G177", 907, ModeWarn, func(inf map[string]any) []string {
			var ks []string
			for _, c := range list(inf, "infCteComp") {
				ks = append(ks, cteS(c, "chCTe"))
			}
			if d := cteDup(ks); d != "" {
				return cteMsg("chave complementada repetida: %s", d)
			}
			return nil
		}),
	}
	return rs
}

func cteMultimodalKeys(inf map[string]any) []string {
	var out []string
	for _, m := range list(sub(inf, "infCTeNorm", "infServVinc"), "infCTeMultimodal") {
		out = append(out, cteS(m, "chCTeMultimodal"))
	}
	return out
}

// --- valores e ICMS ---

func cteValueRules() []Rule {
	return []Rule{
		cteCond("G046", 650, ModeWarn, "valor total do serviço acima de R$ 9.999.999,99", func(inf map[string]any) bool {
			v, ok := num(sub(inf, "vPrest"), "vTPrest")
			return cteNormal(inf) && ok && v > 9999999.99+0.0001
		}),
		cteR("G047", 675, ModeWarn, func(inf map[string]any) []string {
			icms := sub(inf, "imp", "ICMS")
			var out []string
			for _, g := range []struct{ grp, bc, p, v string }{
				{"ICMS00", "vBC", "pICMS", "vICMS"}, {"ICMS20", "vBC", "pICMS", "vICMS"}, {"ICMS90", "vBC", "pICMS", "vICMS"},
				{"ICMSOutraUF", "vBCOutraUF", "pICMSOutraUF", "vICMSOutraUF"},
			} {
				n := sub(icms, g.grp)
				bc, ok1 := num(n, g.bc)
				p, ok2 := num(n, g.p)
				v, ok3 := num(n, g.v)
				if ok1 && ok2 && ok3 && math.Abs(bc*p/100-v) > cteTol {
					out = append(out, fmt.Sprintf("%s: valor do ICMS %.2f difere de vBC x alíquota (%.2f)", g.grp, v, bc*p/100))
				}
			}
			return out
		}),
		cteCond("G048", 531, ModeWarn, "vRec maior que vTPrest", func(inf map[string]any) bool {
			p := sub(inf, "vPrest")
			rec, ok1 := num(p, "vRec")
			tot, ok2 := num(p, "vTPrest")
			return ok1 && ok2 && rec > tot+0.0001
		}),
	}
}

// --- CT-e globalizado (G197 a G208) ---

func cteGlobalRules() []Rule {
	glob := func(inf map[string]any) bool { return cteS(sub(inf, "ide"), "indGlobalizado") == "1" }
	// idsNFe: CNPJ/CPF do emitente de cada NF-e (posições 6 a 19 da chave).
	idsNFe := func(inf map[string]any) []string {
		var out []string
		for _, k := range cteNFeKeys(inf) {
			if len(k) == 44 {
				out = append(out, k[6:20])
			}
		}
		return out
	}
	distinct := func(xs []string) int {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return len(m)
	}
	tom := func(inf map[string]any) string { t, _ := cteTomador(inf); return t }
	prod := func(inf map[string]any) bool { return cteS(sub(inf, "ide"), "tpAmb") == "1" }
	return []Rule{
		cteCond("G197", 743, ModeWarn, "CT-e globalizado exige UF de início igual à de fim", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return glob(inf) && cteS(ide, "UFIni") != cteS(ide, "UFFim")
		}),
		cteCond("G198", 722, ModeWarn, "CT-e globalizado exige tomador remetente ou destinatário", func(inf map[string]any) bool {
			return glob(inf) && !cteIn(tom(inf), "rem", "dest")
		}),
		cteCond("G199", 723, ModeWarn, "CT-e globalizado exige infNFe", func(inf map[string]any) bool {
			return glob(inf) && cteNormal(inf) && len(cteNFeKeys(inf)) == 0
		}),
		cteCond("G200", 724, ModeWarn, "CT-e globalizado com tomador destinatário exige NF-e de ao menos 5 remetentes", func(inf map[string]any) bool {
			return glob(inf) && cteNormal(inf) && tom(inf) == "dest" && distinct(idsNFe(inf)) < 5
		}),
		cteCond("G201", 744, ModeWarn, "CT-e globalizado com tomador remetente exige NF-e do mesmo emitente", func(inf map[string]any) bool {
			return glob(inf) && cteNormal(inf) && tom(inf) == "rem" && distinct(idsNFe(inf)) > 1
		}),
		cteCond("G202", 725, ModeWarn, "remetente deve ser DIVERSOS no globalizado com tomador destinatário", func(inf map[string]any) bool {
			return glob(inf) && prod(inf) && tom(inf) == "dest" && cteS(sub(inf, "rem"), "xNome") != "DIVERSOS"
		}),
		cteCond("G203", 726, ModeWarn, "destinatário deve ser DIVERSOS no globalizado com tomador remetente", func(inf map[string]any) bool {
			return glob(inf) && prod(inf) && tom(inf) == "rem" && cteS(sub(inf, "dest"), "xNome") != "DIVERSOS"
		}),
		cteCond("G204", 727, ModeWarn, "CNPJ do remetente deve ser o do emitente no globalizado com tomador destinatário", func(inf map[string]any) bool {
			return glob(inf) && tom(inf) == "dest" && cteS(sub(inf, "rem"), "CNPJ") != cteS(sub(inf, "emit"), "CNPJ")
		}),
		cteCond("G205", 728, ModeWarn, "CNPJ do destinatário deve ser o do emitente no globalizado com tomador remetente", func(inf map[string]any) bool {
			return glob(inf) && tom(inf) == "rem" && cteS(sub(inf, "dest"), "CNPJ") != cteS(sub(inf, "emit"), "CNPJ")
		}),
		cteCond("G206", 737, ModeWarn, "CT-e globalizado com tomador remetente exige ao menos 5 NF-e", func(inf map[string]any) bool {
			return glob(inf) && cteNormal(inf) && tom(inf) == "rem" && len(cteNFeKeys(inf)) < 5
		}),
		cteCond("G207", 729, ModeWarn, "NF-e de diversos remetentes em CT-e não globalizado", func(inf map[string]any) bool {
			return !glob(inf) && cteNormal(inf) && distinct(idsNFe(inf)) > 1
		}),
		cteCond("G208", 730, ModeWarn, "DIVERSOS só é permitido no CT-e globalizado", func(inf map[string]any) bool {
			return !glob(inf) && (cteS(sub(inf, "rem"), "xNome") == "DIVERSOS" || cteS(sub(inf, "dest"), "xNome") == "DIVERSOS")
		}),
	}
}

// --- reforma tributária: compras governamentais, IBS/CBS, Simples Nacional, antecipação, multa e juros ---

func cteRTCRules() []Rule {
	gov := func(inf map[string]any) map[string]any { return sub(inf, "ide", "gCompraGov") }
	gibs := func(inf map[string]any) map[string]any { return sub(inf, "imp", "IBSCBS", "gIBSCBS") }
	refs := func(inf map[string]any) []string { return cteRefKeys(gov(inf)) }
	rs := []Rule{
		cteCond("CGOV-002", 1004, ModeWarn, "tpOperGov=1 não admite refDFeAnt", func(inf map[string]any) bool {
			return cteS(gov(inf), "tpOperGov") == "1" && len(refs(inf)) > 0
		}),
		cteCond("CGOV-003", 1005, ModeWarn, "tpOperGov=2 exige exatamente um refDFeAnt", func(inf map[string]any) bool {
			return cteS(gov(inf), "tpOperGov") == "2" && len(refs(inf)) != 1
		}),
		cteCond("CGOV-004", 1006, ModeWarn, "tpOperGov=3 exige ao menos um refDFeAnt", func(inf map[string]any) bool {
			return cteS(gov(inf), "tpOperGov") == "3" && len(refs(inf)) == 0
		}),
		cteCond("CGOV-005", 1007, ModeWarn, "tpOperGov=4 não admite refDFeAnt", func(inf map[string]any) bool {
			return cteS(gov(inf), "tpOperGov") == "4" && len(refs(inf)) > 0
		}),
		cteR("CGOV-006", 1008, ModeWarn, func(inf map[string]any) []string {
			ks := refs(inf)
			for _, k := range ks {
				if r := cteKeyReason(k, "", 2000); r != "" {
					return cteMsg("refDFeAnt %s: %s", k, r)
				}
			}
			return nil
		}),
		cteR("CGOV-001", 347, ModeWarn, func(inf map[string]any) []string {
			g, b := gov(inf), gibs(inf)
			if g == nil || b == nil || cteIn(cteS(g, "tpEnteGov"), "5", "6") {
				return nil
			}
			uf, _ := num(sub(b, "gIBSUF"), "pIBSUF")
			mun, _ := num(sub(b, "gIBSMun"), "pIBSMun")
			cbs, _ := num(sub(b, "gCBS"), "pCBS")
			y := cteEmiYear(inf)
			var bad []string
			switch cteS(g, "tpEnteGov") {
			case "1":
				if uf != 0 || mun != 0 {
					bad = append(bad, "pIBSUF e pIBSMun")
				}
			case "2", "3":
				if mun != 0 {
					bad = append(bad, "pIBSMun")
				}
				if y >= 2033 && cbs != 0 {
					bad = append(bad, "pCBS")
				}
			case "4":
				if uf != 0 {
					bad = append(bad, "pIBSUF")
				}
				if y >= 2033 && cbs != 0 {
					bad = append(bad, "pCBS")
				}
			}
			if len(bad) > 0 {
				return cteMsg("compra governamental exige alíquota zero em %s", strings.Join(bad, ", "))
			}
			return nil
		}),
		cteCond("IBSCBS-049", 398, ModeWarn, "compra governamental exige gTribCompraGov", func(inf map[string]any) bool {
			return gov(inf) != nil && gibs(inf) != nil && sub(gibs(inf), "gTribCompraGov") == nil
		}),
		cteCond("IBSCBS-050", 399, ModeWarn, "gTribCompraGov sem compra governamental", func(inf map[string]any) bool {
			return gov(inf) == nil && sub(gibs(inf), "gTribCompraGov") != nil
		}),
		cteCond("IBSCBS-051", 400, ModeWarn, "soma de gTribCompraGov difere de vIBSUF + vIBSMun + vCBS", func(inf map[string]any) bool {
			b := gibs(inf)
			t := sub(b, "gTribCompraGov")
			if t == nil {
				return false
			}
			g := func(m map[string]any, k string) float64 { v, _ := num(m, k); return v }
			a := g(t, "vTribIBSUF") + g(t, "vTribIBSMun") + g(t, "vTribCBS")
			c := g(sub(b, "gIBSUF"), "vIBSUF") + g(sub(b, "gIBSMun"), "vIBSMun") + g(sub(b, "gCBS"), "vCBS")
			return math.Abs(a-c) > cteTol
		}),
		cteCond("IBSCBS-022b", 317, ModeWarn, "vIBS difere de vIBSUF + vIBSMun", func(inf map[string]any) bool {
			b := gibs(inf)
			t, ok := num(b, "vIBS")
			uf, ok1 := num(sub(b, "gIBSUF"), "vIBSUF")
			mun, ok2 := num(sub(b, "gIBSMun"), "vIBSMun")
			return ok && ok1 && ok2 && math.Abs(t-uf-mun) > cteTol
		}),
		cteCond("IBSCBS-360", 360, ModeWarn, "vTotDFe deve ser informado quando existe IBSCBS", func(inf map[string]any) bool {
			return sub(inf, "imp", "IBSCBS") != nil && cteS(sub(inf, "imp"), "vTotDFe") == ""
		}),
		cteCond("IBSCBS-365", 365, ModeWarn, "vTotDFe difere de vTPrest (+ IBS + CBS a partir de 2027)", func(inf map[string]any) bool {
			imp := sub(inf, "imp")
			tot, ok := num(imp, "vTotDFe")
			v, ok2 := num(sub(inf, "vPrest"), "vTPrest")
			y := cteEmiYear(inf)
			if !ok || !ok2 || y == 0 {
				return false
			}
			if y >= 2027 {
				ibs, _ := num(gibs(inf), "vIBS")
				cbs, _ := num(sub(gibs(inf), "gCBS"), "vCBS")
				v += ibs + cbs
			}
			return math.Abs(tot-v) > cteTol
		}),
		cteCond("IBSCBS-1021", 1021, ModeWarn, "gALCZFMCBS exige ISUFEmit do emitente", func(inf map[string]any) bool {
			return sub(gibs(inf), "gCBS", "gALCZFMCBS") != nil && cteS(sub(inf, "emit"), "ISUFEmit") == ""
		}),
	}
	rs = append(rs, cteIBSEntityRules(gov, gibs)...)
	rs = append(rs, cteRegularRules(gibs)...)
	rs = append(rs, cteSNRules()...)
	rs = append(rs,
		cteCond("NT2026.004-1055", 1055, ModeWarn, "indDoacao=1 exige emitente e tomador de bases diferentes", func(inf map[string]any) bool {
			tag, ok := cteTomador(inf)
			if !ok || cteS(sub(inf, "imp", "IBSCBS"), "indDoacao") != "1" {
				return false
			}
			e, t := cteDoc(sub(inf, "emit")), cteDoc(cteActor(inf, tag))
			return e != "" && cteDocBase(e) == cteDocBase(t)
		}),
		cteCond("NT2026.004-1072", 1072, ModeWarn, "CT-e de multa e juros deve usar ICMS CST 41 (ICMS45)", func(inf map[string]any) bool {
			return sub(inf, "ide", "gMultaJuros") != nil && cteS(sub(inf, "imp", "ICMS", "ICMS45"), "CST") != "41"
		}),
		cteCond("NT2026.004-1051", 1051, ModeWarn, "gICMSPrevistoPagtoAntecip só em compra governamental ou pagamento antecipado", func(inf map[string]any) bool {
			return sub(inf, "imp", "ICMS", "gICMSPrevistoPagtoAntecip") != nil && gov(inf) == nil && cteS(sub(inf, "ide"), "tpPagAnt") == ""
		}),
		cteCond("NT2026.004-1052", 1052, ModeWarn, "gICMSPrevistoPagtoAntecip informado indevidamente", func(inf map[string]any) bool {
			ide := sub(inf, "ide")
			return sub(inf, "imp", "ICMS", "gICMSPrevistoPagtoAntecip") != nil && (gov(inf) != nil || cteS(ide, "tpPagAnt") != "") &&
				cteS(ide, "tpPagAnt") != "1" && cteS(gov(inf), "tpOperGov") != "4"
		}),
		cteCond("ANT-1026", 1026, ModeWarn, "gPagAntecipado só com tpPagAnt=3", func(inf map[string]any) bool {
			t := cteS(sub(inf, "ide"), "tpPagAnt")
			return (t == "" || t == "1") && sub(inf, "ide", "gPagAntecipado") != nil
		}),
		cteCond("ANT-1027", 1027, ModeWarn, "tpPagAnt=3 exige gPagAntecipado", func(inf map[string]any) bool {
			return cteS(sub(inf, "ide"), "tpPagAnt") == "3" && sub(inf, "ide", "gPagAntecipado") == nil
		}),
	)
	return rs
}

// cteIBSEntityRules gera as regras de alíquota efetiva, diferimento, valor e teto de IBS UF, IBS Municipal e CBS.
func cteIBSEntityRules(gov, gibs func(map[string]any) map[string]any) []Rule {
	var rs []Rule
	for _, e := range []struct {
		grp, p, v               string
		efet, dif, val, cap     string
		cEfet, cDif, cVal, cCap int
	}{
		{"gIBSUF", "pIBSUF", "vIBSUF", "009b", "013", "014", "013a", 390, 320, 318, 395},
		{"gIBSMun", "pIBSMun", "vIBSMun", "017b", "021", "022", "022a", 392, 325, 323, 396},
		{"gCBS", "pCBS", "vCBS", "024b", "028", "029", "028a", 394, 329, 327, 397},
	} {
		e := e
		// rate: alíquota usada nos cálculos (a efetiva quando há gRed).
		rate := func(inf map[string]any) (float64, bool) {
			n := sub(gibs(inf), e.grp)
			if n == nil {
				return 0, false
			}
			if r := sub(n, "gRed"); r != nil {
				return num(r, "pAliqEfet")
			}
			return num(n, e.p)
		}
		bc := func(inf map[string]any) (float64, bool) { return num(gibs(inf), "vBC") }
		rs = append(rs,
			cteCond("IBSCBS-"+e.efet, e.cEfet, ModeWarn, e.grp+": pAliqEfet calculada incorretamente", func(inf map[string]any) bool {
				n := sub(gibs(inf), e.grp)
				red := sub(n, "gRed")
				p, ok1 := num(n, e.p)
				pr, ok2 := num(red, "pRedAliq")
				ef, ok3 := num(red, "pAliqEfet")
				if !ok1 || !ok2 || !ok3 {
					return false
				}
				exp := p * (1 - pr/100)
				if g := gov(inf); g != nil {
					if rd, ok := num(g, "pRedutor"); ok {
						exp *= 1 - rd/100
					}
				}
				return math.Abs(exp-ef) > 0.00011
			}),
			cteCond("IBSCBS-"+e.dif, e.cDif, ModeWarn, e.grp+": vDif difere de vBC x alíquota x pDif", func(inf map[string]any) bool {
				d := sub(gibs(inf), e.grp, "gDif")
				r, ok1 := rate(inf)
				b, ok2 := bc(inf)
				vd, ok3 := num(d, "vDif")
				pd, ok4 := num(d, "pDif")
				return ok1 && ok2 && ok3 && ok4 && math.Abs(b*r/100*pd/100-vd) > cteTol
			}),
			cteCond("IBSCBS-"+e.val, e.cVal, ModeWarn, e.grp+": valor difere de vBC x alíquota - vDif - vDevTrib", func(inf map[string]any) bool {
				n := sub(gibs(inf), e.grp)
				r, ok1 := rate(inf)
				b, ok2 := bc(inf)
				v, ok3 := num(n, e.v)
				vd, _ := num(sub(n, "gDif"), "vDif")
				vv, _ := num(sub(n, "gDevTrib"), "vDevTrib")
				return ok1 && ok2 && ok3 && math.Abs(b*r/100-vd-vv-v) > cteTol
			}),
			cteCond("IBSCBS-"+e.cap, e.cCap, ModeWarn, e.grp+": vDif + vDevTrib acima de vBC x alíquota", func(inf map[string]any) bool {
				n := sub(gibs(inf), e.grp)
				r, ok1 := rate(inf)
				b, ok2 := bc(inf)
				vd, _ := num(sub(n, "gDif"), "vDif")
				vv, _ := num(sub(n, "gDevTrib"), "vDevTrib")
				return ok1 && ok2 && vd+vv > b*r/100+cteTol
			}),
		)
	}
	return rs
}

// cteRegularRules: valor da tributação regular = vBC x alíquota efetiva (033 a 035).
func cteRegularRules(gibs func(map[string]any) map[string]any) []Rule {
	var rs []Rule
	for _, r := range []struct {
		id, p, v string
		cstat    int
	}{
		{"033", "pAliqEfetRegIBSUF", "vTribRegIBSUF", 333},
		{"034", "pAliqEfetRegIBSMun", "vTribRegIBSMun", 334},
		{"035", "pAliqEfetRegCBS", "vTribRegCBS", 335},
	} {
		r := r
		rs = append(rs, cteCond("IBSCBS-"+r.id, r.cstat, ModeWarn, r.v+" difere de vBC x "+r.p, func(inf map[string]any) bool {
			g := sub(gibs(inf), "gTribRegular")
			b, ok1 := num(gibs(inf), "vBC")
			p, ok2 := num(g, r.p)
			v, ok3 := num(g, r.v)
			return ok1 && ok2 && ok3 && math.Abs(b*p/100-v) > cteTol
		}))
	}
	return rs
}

// cteSNRules: CRT x tpRegimeIBSCBS e os grupos gIBSCBS/gTribSN do Simples Nacional (item 3.12).
func cteSNRules() []Rule {
	st := func(inf map[string]any) (crt, reg string, ibs, sn map[string]any, ok bool) {
		g := sub(inf, "imp", "IBSCBS")
		e := sub(inf, "emit")
		return cteS(e, "CRT"), cteS(e, "tpRegimeIBSCBS"), sub(g, "gIBSCBS"), sub(g, "gTribSN"), g != nil
	}
	// onlyIBS: regras de grupos exigem o IBSCBS informado (IBSCBS-001 do MOC, que o exige, ainda não vigora).
	return []Rule{
		cteCond("SN-1090", 1090, ModeWarn, "CRT 1 ou 2 com tpRegimeIBSCBS informado deve usar 1", func(inf map[string]any) bool {
			crt, reg, _, _, _ := st(inf)
			return cteIn(crt, "1", "2") && reg != "" && reg != "1"
		}),
		cteCond("SN-1091", 1091, ModeWarn, "CRT 3 com tpRegimeIBSCBS informado deve usar 2, 3 ou 4", func(inf map[string]any) bool {
			crt, reg, _, _, _ := st(inf)
			return crt == "3" && reg != "" && !cteIn(reg, "2", "3", "4")
		}),
		cteCond("SN-1092", 1092, ModeWarn, "CRT 4 (MEI) não admite tpRegimeIBSCBS", func(inf map[string]any) bool {
			crt, reg, _, _, _ := st(inf)
			return crt == "4" && reg != ""
		}),
		cteCond("SN-1100", 1100, ModeWarn, "CRT 1 sem tpRegimeIBSCBS não admite gIBSCBS", func(inf map[string]any) bool {
			crt, reg, ibs, _, ok := st(inf)
			return ok && crt == "1" && reg == "" && ibs != nil
		}),
		cteCond("SN-1101", 1101, ModeWarn, "CRT 1 sem tpRegimeIBSCBS exige gTribSN com gIBSSN e gCBSSN", func(inf map[string]any) bool {
			crt, reg, _, sn, ok := st(inf)
			return ok && crt == "1" && reg == "" && (sn == nil || sub(sn, "gIBSSN") == nil || sub(sn, "gCBSSN") == nil)
		}),
		cteCond("SN-1102", 1102, ModeWarn, "gTribSN não deve ser informado com regime regular", func(inf map[string]any) bool {
			crt, reg, _, sn, ok := st(inf)
			return ok && sn != nil && ((cteIn(crt, "1", "2") && reg == "1") || (crt == "3" && reg == "4"))
		}),
		cteCond("SN-1103", 1103, ModeWarn, "CRT 2 sem tpRegimeIBSCBS exige gTribSN com gCBSSN", func(inf map[string]any) bool {
			crt, reg, _, sn, ok := st(inf)
			return ok && crt == "2" && reg == "" && (sn == nil || sub(sn, "gCBSSN") == nil)
		}),
		cteCond("SN-1104", 1104, ModeWarn, "CRT 2 sem tpRegimeIBSCBS não admite gIBSSN", func(inf map[string]any) bool {
			crt, reg, _, sn, ok := st(inf)
			return ok && crt == "2" && reg == "" && sub(sn, "gIBSSN") != nil
		}),
		cteCond("SN-1107", 1107, ModeWarn, "CRT 4 (MEI) não admite gIBSCBS nem gTribSN", func(inf map[string]any) bool {
			crt, _, ibs, sn, ok := st(inf)
			return ok && crt == "4" && (ibs != nil || sn != nil)
		}),
	}
}

// cteRefKeys devolve as chaves de gCompraGov/refDFeAnt (repetição de escalar: string, []string ou []any).
func cteRefKeys(g map[string]any) []string {
	switch t := g["refDFeAnt"].(type) {
	case string:
		return []string{t}
	case []string:
		return t
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
