// Tests for the xsdorder table — ported from py-dfe's
// tests/unit/test_xsd_order.py and tests/unit/test_xsd_order_comprehensive.py.
//
// The Python tests build a dict with keys in WRONG order and assert the
// resulting XML (via to_xml_bytes, which consults XSD_ORDER) has children in
// the correct XSD-defined sequence. Since the Go builder package that will
// consult this table is being written in parallel, these tests instead
// assert directly against Resolve/Lookup — the same data, checked the same
// way (relative order of a set of names within the registered slice), without
// depending on the not-yet-written builder.
package xsdorder

import "testing"

// indexOf mirrors the Python tests' `kids.index(x)` pattern.
func indexOf(order []string, name string) int {
	for i, n := range order {
		if n == name {
			return i
		}
	}
	return -1
}

// assertBefore fails unless a appears before b in order.
func assertBefore(t *testing.T, order []string, a, b string) {
	t.Helper()
	ia, ib := indexOf(order, a), indexOf(order, b)
	if ia == -1 {
		t.Fatalf("%q not found in %v", a, order)
	}
	if ib == -1 {
		t.Fatalf("%q not found in %v", b, order)
	}
	if !(ia < ib) {
		t.Errorf("expected %q before %q, got order %v", a, b, order)
	}
}

// assertOrderIsCorrect mirrors the Python _order_is_correct helper: every
// name in expected that's present in order must appear in the same relative
// sequence as expected (extra names in order are ignored).
func assertOrderIsCorrect(t *testing.T, order []string, expected []string) {
	t.Helper()
	positions := make(map[string]int, len(order))
	for i, n := range order {
		positions[n] = i
	}
	var present []string
	for _, n := range expected {
		if _, ok := positions[n]; ok {
			present = append(present, n)
		}
	}
	for i := 1; i < len(present); i++ {
		if positions[present[i-1]] > positions[present[i]] {
			t.Errorf("expected relative order %v, got positions %v within %v", expected, positions, order)
			return
		}
	}
}

func mustResolve(t *testing.T, parentTag, tag string) []string {
	t.Helper()
	order, ok := Resolve(parentTag, tag)
	if !ok {
		t.Fatalf("Resolve(%q, %q): no entry found", parentTag, tag)
	}
	return order
}

// ---------------------------------------------------------------------------
// NF-e — infNFe (test_xsd_order.py TestInfNFe)
// ---------------------------------------------------------------------------

func TestInfNFe(t *testing.T) {
	order := mustResolve(t, "", "infNFe")
	assertBefore(t, order, "ide", "emit")
	assertBefore(t, order, "emit", "dest")
	assertBefore(t, order, "dest", "det")
	assertBefore(t, order, "det", "total")
	assertBefore(t, order, "total", "transp")
	assertBefore(t, order, "transp", "pag")
	assertBefore(t, order, "pag", "infAdic")
}

// ---------------------------------------------------------------------------
// NF-e — ide (test_xsd_order.py TestIdeNFe)
// ---------------------------------------------------------------------------

func TestIdeNFe(t *testing.T) {
	order := mustResolve(t, "infNFe", "ide")
	if order[0] != "cUF" {
		t.Errorf("expected cUF first, got %v", order)
	}
	assertBefore(t, order, "nNF", "dhEmi")
	assertBefore(t, order, "tpAmb", "finNFe")
	assertBefore(t, order, "procEmi", "verProc")
}

// ---------------------------------------------------------------------------
// NF-e — emit: CNPJ/CPF first (test_xsd_order.py TestEmitNFe)
// ---------------------------------------------------------------------------

func TestEmitNFe(t *testing.T) {
	order := mustResolve(t, "infNFe", "emit")
	if order[0] != "CNPJ" {
		t.Errorf("expected CNPJ first, got %v", order)
	}
	assertBefore(t, order, "CPF", "xNome") // CPF and CNPJ share rank 0/1, both before xNome
	assertBefore(t, order, "xNome", "xFant")
	assertBefore(t, order, "enderEmit", "IE")
	assertBefore(t, order, "IE", "CRT")
}

// ---------------------------------------------------------------------------
// NF-e — dest (test_xsd_order.py TestDestNFe)
// ---------------------------------------------------------------------------

func TestDestNFe(t *testing.T) {
	order := mustResolve(t, "infNFe", "dest")
	assertBefore(t, order, "CNPJ", "xNome")
	assertBefore(t, order, "xNome", "indIEDest")
}

// ---------------------------------------------------------------------------
// NF-e — prod (test_xsd_order.py TestProd)
// ---------------------------------------------------------------------------

func TestProd(t *testing.T) {
	order := mustResolve(t, "", "prod")
	if order[0] != "cProd" {
		t.Errorf("expected cProd first, got %v", order)
	}
	assertOrderIsCorrect(t, order, []string{"cProd", "cEAN", "xProd", "NCM", "CFOP"})
	assertBefore(t, order, "vProd", "cEANTrib")
	assertBefore(t, order, "vDesc", "indTot")
}

// ---------------------------------------------------------------------------
// NF-e — imposto (test_xsd_order.py TestImposto)
// ---------------------------------------------------------------------------

func TestImposto(t *testing.T) {
	order := mustResolve(t, "", "imposto")
	assertBefore(t, order, "ICMS", "PIS")
	assertBefore(t, order, "PIS", "COFINS")
	assertBefore(t, order, "COFINS", "IBSCBS")
	assertBefore(t, order, "IS", "IBSCBS")
}

// ---------------------------------------------------------------------------
// IBSCBS (TTribNFe) (test_xsd_order.py TestIBSCBS)
// ---------------------------------------------------------------------------

func TestIBSCBS(t *testing.T) {
	order := mustResolve(t, "", "IBSCBS")
	if order[0] != "CST" {
		t.Errorf("expected CST first, got %v", order)
	}
	assertBefore(t, order, "cClassTrib", "gIBSCBS")
}

// ---------------------------------------------------------------------------
// gIBSCBS inner (TCIBS), scoped under IBSCBS (test_xsd_order.py TestGIBSCBS)
// ---------------------------------------------------------------------------

func TestGIBSCBS(t *testing.T) {
	// Mirrors builder.py's ancestor-narrowing: resolving "gIBSCBS" with
	// parentTag "IBSCBS" must hit the "IBSCBS:gIBSCBS" specific key.
	order := mustResolve(t, "IBSCBS", "gIBSCBS")
	if order[0] != "vBC" {
		t.Errorf("expected vBC first, got %v", order)
	}
	assertBefore(t, order, "gIBSUF", "gIBSMun")
	assertBefore(t, order, "gIBSMun", "vIBS")
	if order[len(order)-1] != "gCBS" {
		t.Errorf("expected gCBS last, got %v", order)
	}

	ufOrder := mustResolve(t, "", "gIBSUF")
	assertBefore(t, ufOrder, "pIBSUF", "vIBSUF")

	munOrder := mustResolve(t, "", "gIBSMun")
	assertBefore(t, munOrder, "pIBSMun", "vIBSMun")

	cbsOrder := mustResolve(t, "", "gCBS")
	assertBefore(t, cbsOrder, "pCBS", "vCBS")
}

// ---------------------------------------------------------------------------
// ICMS groups (test_xsd_order.py TestICMSGroups)
// ---------------------------------------------------------------------------

func TestICMSGroups(t *testing.T) {
	assertBefore(t, mustResolve(t, "", "ICMS40"), "orig", "CST")
	assertBefore(t, mustResolve(t, "", "ICMSSN102"), "orig", "CSOSN")
	assertBefore(t, mustResolve(t, "", "ICMSSN101"), "CSOSN", "pCredSN")
	assertBefore(t, mustResolve(t, "", "ICMS00"), "CST", "modBC")
	assertBefore(t, mustResolve(t, "", "ICMSSN500"), "orig", "CSOSN")
}

// ---------------------------------------------------------------------------
// PIS / COFINS (test_xsd_order.py TestPISCOFINS)
// ---------------------------------------------------------------------------

func TestPISCOFINS(t *testing.T) {
	if o := mustResolve(t, "", "PISAliq"); o[0] != "CST" {
		t.Errorf("PISAliq: expected CST first, got %v", o)
	}
	if o := mustResolve(t, "", "PISNT"); o[0] != "CST" {
		t.Errorf("PISNT: expected CST first, got %v", o)
	}
	if o := mustResolve(t, "", "COFINSAliq"); o[0] != "CST" {
		t.Errorf("COFINSAliq: expected CST first, got %v", o)
	}
	assertBefore(t, mustResolve(t, "", "PISOutr"), "CST", "vBC")
}

// ---------------------------------------------------------------------------
// ICMSTot / total (test_xsd_order.py TestICMSTot, TestTotal)
// ---------------------------------------------------------------------------

func TestICMSTot(t *testing.T) {
	order := mustResolve(t, "", "ICMSTot")
	if order[0] != "vBC" {
		t.Errorf("expected vBC first, got %v", order)
	}
	assertBefore(t, order, "vProd", "vFrete")
	assertBefore(t, order, "vNF", "vTotTrib")
	assertBefore(t, order, "vDesc", "vII")
}

func TestTotal(t *testing.T) {
	order := mustResolve(t, "", "total")
	if order[0] != "ICMSTot" {
		t.Errorf("expected ICMSTot first, got %v", order)
	}
	assertBefore(t, order, "ISTot", "IBSCBSTot")
	assertBefore(t, order, "ICMSTot", "IBSCBSTot")
}

// ---------------------------------------------------------------------------
// transp / vol, pag / detPag (test_xsd_order.py TestTransp, TestPag)
// ---------------------------------------------------------------------------

func TestTransp(t *testing.T) {
	order := mustResolve(t, "", "transp")
	if order[0] != "modFrete" {
		t.Errorf("expected modFrete first, got %v", order)
	}
	assertBefore(t, order, "modFrete", "vol")
}

func TestPag(t *testing.T) {
	assertBefore(t, mustResolve(t, "", "pag"), "detPag", "vTroco")
	assertBefore(t, mustResolve(t, "", "detPag"), "tPag", "vPag")
}

// ---------------------------------------------------------------------------
// Service request elements (test_xsd_order.py TestServiceElements)
// ---------------------------------------------------------------------------

func TestServiceElements(t *testing.T) {
	if o := mustResolve(t, "", "consStatServ"); !equalSlices(o, []string{"tpAmb", "cUF", "xServ"}) {
		t.Errorf("consStatServ: got %v", o)
	}
	assertOrderIsCorrect(t, mustResolve(t, "", "consSitNFe"), []string{"tpAmb", "xServ", "chNFe"})

	infInut := mustResolve(t, "inutNFe", "infInut")
	if infInut[0] != "tpAmb" {
		t.Errorf("inutNFe:infInut: expected tpAmb first, got %v", infInut)
	}
	if infInut[len(infInut)-1] != "xJust" {
		t.Errorf("inutNFe:infInut: expected xJust last, got %v", infInut)
	}

	distDFeInt := mustResolve(t, "", "distDFeInt")
	if distDFeInt[0] != "tpAmb" {
		t.Errorf("distDFeInt: expected tpAmb first, got %v", distDFeInt)
	}
	assertBefore(t, distDFeInt, "CNPJ", "distNSU")
}

// ---------------------------------------------------------------------------
// Events (NF-e) (test_xsd_order.py TestEventosNFe)
// ---------------------------------------------------------------------------

func TestEventosNFe(t *testing.T) {
	assertBefore(t, mustResolve(t, "", "envEvento"), "idLote", "evento")

	infEvento := mustResolve(t, "evento", "infEvento")
	if infEvento[0] != "cOrgao" {
		t.Errorf("evento:infEvento: expected cOrgao first, got %v", infEvento)
	}
	assertBefore(t, infEvento, "chNFe", "dhEvento")
	if infEvento[len(infEvento)-1] != "detEvento" {
		t.Errorf("evento:infEvento: expected detEvento last, got %v", infEvento)
	}

	detEvento := mustResolve(t, "", "detEvento")
	if detEvento[0] != "descEvento" {
		t.Errorf("detEvento: expected descEvento first, got %v", detEvento)
	}
	assertBefore(t, detEvento, "nProt", "xJust")
}

// ---------------------------------------------------------------------------
// CT-e — emit/ide disambiguation (test_xsd_order.py TestEmitCTe, TestIdeCTe)
// ---------------------------------------------------------------------------

func TestEmitCTe(t *testing.T) {
	order := mustResolve(t, "infCte", "emit")
	if order[0] != "CNPJ" {
		t.Errorf("infCte:emit: expected CNPJ first, got %v", order)
	}
	// CT-e specific: IE before xNome (opposite of NF-e, where xNome is before IE).
	assertBefore(t, order, "IE", "xNome")
	assertBefore(t, order, "xNome", "enderEmit")
}

func TestIdeCTe(t *testing.T) {
	order := mustResolve(t, "infCte", "ide")
	if order[0] != "cUF" {
		t.Errorf("infCte:ide: expected cUF first, got %v", order)
	}
	assertBefore(t, order, "CFOP", "mod")
}

// ---------------------------------------------------------------------------
// MDF-e — emit/ide disambiguation (test_xsd_order.py TestEmitMDFe, TestIdeMDFe)
// ---------------------------------------------------------------------------

func TestEmitMDFe(t *testing.T) {
	order := mustResolve(t, "infMDFe", "emit")
	if order[0] != "CNPJ" {
		t.Errorf("infMDFe:emit: expected CNPJ first, got %v", order)
	}
	assertBefore(t, order, "IE", "xNome")
}

func TestIdeMDFe(t *testing.T) {
	order := mustResolve(t, "infMDFe", "ide")
	if order[0] != "cUF" {
		t.Errorf("infMDFe:ide: expected cUF first, got %v", order)
	}
	assertBefore(t, order, "tpAmb", "mod")
}

// ---------------------------------------------------------------------------
// ConsStatServ CT-e / MDF-e (test_xsd_order.py TestConsStatServ)
// ---------------------------------------------------------------------------

func TestConsStatServCteMdfe(t *testing.T) {
	if o := mustResolve(t, "", "consStatServCTe"); !equalSlices(o, []string{"tpAmb", "cUF", "xServ"}) {
		t.Errorf("consStatServCTe: got %v", o)
	}
	if o := mustResolve(t, "", "consStatServMDFe"); !equalSlices(o, []string{"tpAmb", "xServ"}) {
		t.Errorf("consStatServMDFe: got %v", o)
	}
}

// ---------------------------------------------------------------------------
// enderEmit / enderDest (test_xsd_order.py TestEndereco)
// ---------------------------------------------------------------------------

func TestEndereco(t *testing.T) {
	emit := mustResolve(t, "", "enderEmit")
	if emit[0] != "xLgr" {
		t.Errorf("enderEmit: expected xLgr first, got %v", emit)
	}
	assertBefore(t, emit, "nro", "xBairro")

	dest := mustResolve(t, "", "enderDest")
	assertBefore(t, dest, "UF", "CEP")
}

// ---------------------------------------------------------------------------
// eventoCTe (test_xsd_order.py TestEventoCTe)
// ---------------------------------------------------------------------------

func TestEventoCTe(t *testing.T) {
	order := mustResolve(t, "eventoCTe", "infEvento")
	if order[0] != "cOrgao" {
		t.Errorf("eventoCTe:infEvento: expected cOrgao first, got %v", order)
	}
	assertBefore(t, order, "chCTe", "dhEvento")
}

// ---------------------------------------------------------------------------
// enviNFe / NFe (test_xsd_order.py TestEnviNFe, TestNFe)
// ---------------------------------------------------------------------------

func TestEnviNFe(t *testing.T) {
	order := mustResolve(t, "", "enviNFe")
	assertBefore(t, order, "idLote", "NFe")
	assertBefore(t, order, "indSinc", "NFe")
}

func TestNFeRoot(t *testing.T) {
	order := mustResolve(t, "", "NFe")
	assertBefore(t, order, "infNFe", "infNFeSupl")
}

// =============================================================================
// Comprehensive full-order assertions
// (ported from test_xsd_order_comprehensive.py — one full-sequence check per
// major root/webservice, rather than the file's exhaustive per-field set)
// =============================================================================

func TestInfNFeFullOrder(t *testing.T) {
	order := mustResolve(t, "", "infNFe")
	assertOrderIsCorrect(t, order, []string{
		"ide", "emit", "avulsa", "dest", "retirada", "entrega", "autXML",
		"det", "total", "transp", "cobr", "pag",
		"infIntermed", "infAdic", "exporta", "compra", "cana",
		"infRespTec", "infSolicNFF",
	})
	if order[len(order)-1] != "infSolicNFF" {
		t.Errorf("infNFe: expected infSolicNFF last, got %v", order)
	}
}

func TestIdeNFeFullOrder(t *testing.T) {
	order := mustResolve(t, "infNFe", "ide")
	assertOrderIsCorrect(t, order, []string{
		"cUF", "cNF", "natOp", "mod", "serie", "nNF",
		"dhEmi", "dhSaiEnt", "dPrevEntrega", "tpNF", "idDest", "cMunFG", "cMunFGIBS",
		"tpImp", "tpEmis", "cDV", "tpAmb", "finNFe",
		"tpNFDebito", "tpNFCredito",
		"indFinal", "indPres", "indIntermed", "procEmi", "verProc",
		"dhCont", "xJust",
	})
	assertBefore(t, order, "dhEmi", "dhSaiEnt")
	assertBefore(t, order, "dPrevEntrega", "tpNF")
	assertBefore(t, order, "cMunFG", "cMunFGIBS")
	assertBefore(t, order, "finNFe", "tpNFDebito")
	assertBefore(t, order, "verProc", "xJust")
}

func TestEnviCTeFullOrder(t *testing.T) {
	order := mustResolve(t, "", "infCte")
	assertOrderIsCorrect(t, order, []string{
		"ide", "compl", "emit", "rem", "exped", "receb", "dest", "vPrest",
		"imp", "infCteNorm", "infCteSub", "infCteComp", "autXML",
		"infRespTec", "infSolicNFF",
	})
}

func TestEnviMDFeFullOrder(t *testing.T) {
	order := mustResolve(t, "", "infMDFe")
	assertOrderIsCorrect(t, order, []string{
		"ide", "emit", "infModal", "infDoc",
		"seg", "prodPred", "tot", "lacres", "autXML",
		"infAdic", "infRespTec", "infSolicNFF", "infPAA",
	})
}

func TestInfMDFeIdeFullOrder(t *testing.T) {
	order := mustResolve(t, "infMDFe", "ide")
	assertOrderIsCorrect(t, order, []string{
		"cUF", "tpAmb", "tpEmit", "tpTransp", "mod", "serie", "nMDF",
		"cMDF", "cDV", "modal", "dhEmi", "tpEmis",
		"procEmi", "verProc", "UFIni", "UFFim",
		"infMunCarrega", "infPercurso",
		"dhIniViagem", "indCanalVerde", "indCarregaPosterior",
	})
}

func TestInfDocOnlyInfMunDescarga(t *testing.T) {
	// NT change: infMunCarrega moved to "ide"; infDoc now has only infMunDescarga.
	order := mustResolve(t, "", "infDoc")
	if !equalSlices(order, []string{"infMunDescarga"}) {
		t.Errorf("infDoc: expected only infMunDescarga, got %v", order)
	}
}

func TestInfMDFeInfAdic(t *testing.T) {
	// MDF-e infAdic is a narrower subset than NF-e's generic infAdic.
	order := mustResolve(t, "infMDFe", "infAdic")
	if !equalSlices(order, []string{"infAdFisco", "infCpl"}) {
		t.Errorf("infMDFe:infAdic: got %v", order)
	}
}

func TestEvCancCTeFullOrder(t *testing.T) {
	order := mustResolve(t, "", "evCancCTe")
	if !equalSlices(order, []string{"descEvento", "nProt", "xJust"}) {
		t.Errorf("evCancCTe: got %v", order)
	}
}

func TestEvEncMDFeFullOrder(t *testing.T) {
	order := mustResolve(t, "", "evEncMDFe")
	assertOrderIsCorrect(t, order, []string{
		"descEvento", "nProt", "dtEnc", "cUF", "cMun", "indEncPorTerceiro",
	})
}

func TestEvCancMDFeFullOrder(t *testing.T) {
	order := mustResolve(t, "", "evCancMDFe")
	if !equalSlices(order, []string{"descEvento", "nProt", "xJust"}) {
		t.Errorf("evCancMDFe: got %v", order)
	}
}

func TestEvIncCondutorMDFeFullOrder(t *testing.T) {
	order := mustResolve(t, "", "evIncCondutorMDFe")
	if !equalSlices(order, []string{"descEvento", "condutor"}) {
		t.Errorf("evIncCondutorMDFe: got %v", order)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Resolve algorithm itself: ancestor-scoped narrowing (mirrors builder.py's
// _build_element lookup — multi-level ancestor path, and generic fallback).
// ---------------------------------------------------------------------------

func TestResolveAncestorNarrowing(t *testing.T) {
	// "infMDFe:emit:enderEmit" is registered 3-deep; resolving with the full
	// ancestor chain must find it rather than falling back to "enderEmit".
	order := mustResolve(t, "infMDFe:emit", "enderEmit")
	expected := []string{"xLgr", "nro", "xCpl", "xBairro", "cMun", "xMun", "CEP", "UF", "fone", "email"}
	if !equalSlices(order, expected) {
		t.Errorf("infMDFe:emit:enderEmit: got %v, want %v", order, expected)
	}
}

func TestResolveFallsBackToGeneric(t *testing.T) {
	// No "someUnknownParent:enderEmit" entry exists, so Resolve must fall
	// back to the plain "enderEmit" generic entry.
	order, ok := Resolve("someUnknownParent", "enderEmit")
	if !ok {
		t.Fatal("expected fallback to generic enderEmit entry")
	}
	generic, _ := Lookup("enderEmit")
	if !equalSlices(order, generic) {
		t.Errorf("expected fallback to match generic enderEmit, got %v vs %v", order, generic)
	}
}

func TestLookupMissingKey(t *testing.T) {
	if _, ok := Lookup("thisElementDoesNotExist"); ok {
		t.Error("expected ok=false for unregistered key")
	}
}

func TestTableKeyCount(t *testing.T) {
	// Sanity check: the merged table must have exactly as many keys as the
	// Python XSD_ORDER (213, verified by diffing a JSON dump of both against
	// each other during the port — see task report). This guards against an
	// accidental key loss/duplication regressing silently.
	const wantKeys = 213
	if got := len(Table); got != wantKeys {
		t.Errorf("Table has %d keys, want %d", got, wantKeys)
	}
}
