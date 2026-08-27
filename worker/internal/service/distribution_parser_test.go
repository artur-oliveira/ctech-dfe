package service

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// gzipB64 compresses s with gzip and base64-encodes the result — mirrors the SEFAZ docZip format.
func gzipB64(s string) string {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// ---------------------------------------------------------------------------
// determineSchemaType
// ---------------------------------------------------------------------------

func TestDetermineSchemaType_KnownSchemas(t *testing.T) {
	cases := []struct{ schema, want string }{
		{"resNFe_v1.01.xsd", SchemaResNFe},
		{"procNFe_v4.00.xsd", SchemaProcNFe},
		{"resCTe_v1.00.xsd", SchemaResCTe},
		{"cteProc_v3.00.xsd", SchemaProcCTe},
		{"cteOSProc_v1.00.xsd", SchemaProcCTeOS},
		{"GTVeProc_v1.00.xsd", SchemaProcGTVe},
		{"cteSimpProc_v1.00.xsd", SchemaProcCTeSimp},
		{"procMDFe_v3.00.xsd", SchemaProcMDFe},
		{"resMDFe_v1.00.xsd", SchemaResMDFe},
		{"resEvento_v1.01.xsd", SchemaResEvento},
		{"procEventoNFe_v1.00.xsd", SchemaProcEventoNFe},
		{"procEventoCTe_v1.00.xsd", SchemaProcEventoCTe},
		{"procEventoMDFe_v1.00.xsd", SchemaProcEventoMDFe},
	}
	for _, tc := range cases {
		if got := determineSchemaType(tc.schema); got != tc.want {
			t.Errorf("determineSchemaType(%q) = %q, want %q", tc.schema, got, tc.want)
		}
	}
}

func TestDetermineSchemaType_Unknown(t *testing.T) {
	if got := determineSchemaType("unknownSchema_v1.00.xsd"); got != "unknown" {
		t.Errorf("expected 'unknown', got %q", got)
	}
}

// procEventoNFe must match before procNFe because schemaOrder processes event schemas first.
func TestDetermineSchemaType_ProcEventoPriorityOverProcNFe(t *testing.T) {
	if got := determineSchemaType("procEventoNFe_v1.00.xsd"); got != SchemaProcEventoNFe {
		t.Errorf("expected %q, got %q", SchemaProcEventoNFe, got)
	}
}

// ---------------------------------------------------------------------------
// decompressDocZip
// ---------------------------------------------------------------------------

func TestDecompressDocZip_RoundTrip(t *testing.T) {
	original := "<resNFe>test content</resNFe>"
	compressed := gzipB64(original)
	got, err := decompressDocZip(compressed)
	if err != nil {
		t.Fatalf("decompressDocZip: %v", err)
	}
	if got != original {
		t.Errorf("got %q, want %q", got, original)
	}
}

func TestDecompressDocZip_LargeXML(t *testing.T) {
	original := strings.Repeat("<item>data</item>", 1000)
	got, err := decompressDocZip(gzipB64(original))
	if err != nil {
		t.Fatalf("decompressDocZip large: %v", err)
	}
	if got != original {
		t.Errorf("round-trip failed for large XML")
	}
}

func TestDecompressDocZip_InvalidBase64(t *testing.T) {
	_, err := decompressDocZip("not!valid!base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecompressDocZip_ValidBase64ButNotGzip(t *testing.T) {
	_, err := decompressDocZip(base64.StdEncoding.EncodeToString([]byte("not gzip content")))
	if err == nil {
		t.Error("expected error for non-gzip data")
	}
}

// ---------------------------------------------------------------------------
// parseXMLBytes / findText / findEl / findAllEls
// ---------------------------------------------------------------------------

func TestParseXMLBytes_ReturnsRoot(t *testing.T) {
	raw := `<root xmlns="http://example.com"><child>hello</child></root>`
	root, err := parseXMLBytes([]byte(raw))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	if root == nil {
		t.Fatal("root is nil")
	}
	if root.Local != "root" {
		t.Errorf("root.Local = %q, want 'root'", root.Local)
	}
	if len(root.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(root.Children))
	}
}

func TestFindText_DirectChild(t *testing.T) {
	raw := `<root xmlns="http://www.portalfiscal.inf.br/nfe">
		<chNFe>35250512345678000195550010000000011000000011</chNFe>
	</root>`
	root, _ := parseXMLBytes([]byte(raw))
	got := findText(root, nsNFe, "chNFe")
	if got != "35250512345678000195550010000000011000000011" {
		t.Errorf("findText = %q", got)
	}
}

func TestFindText_NestedElement(t *testing.T) {
	raw := `<root xmlns="http://www.portalfiscal.inf.br/nfe">
		<a><b><chNFe>KEYVALUE</chNFe></b></a>
	</root>`
	root, _ := parseXMLBytes([]byte(raw))
	if got := findText(root, nsNFe, "chNFe"); got != "KEYVALUE" {
		t.Errorf("findText nested = %q, want KEYVALUE", got)
	}
}

func TestFindText_MultipleLocals_CNPJ(t *testing.T) {
	raw := `<root xmlns="http://www.portalfiscal.inf.br/nfe"><CNPJ>12345678000195</CNPJ></root>`
	root, _ := parseXMLBytes([]byte(raw))
	if got := findText(root, nsNFe, "CNPJ", "CPF"); got != "12345678000195" {
		t.Errorf("findText CNPJ = %q", got)
	}
}

func TestFindText_MultipleLocals_FallsThroughToCPF(t *testing.T) {
	raw := `<root xmlns="http://www.portalfiscal.inf.br/nfe"><CPF>12345678909</CPF></root>`
	root, _ := parseXMLBytes([]byte(raw))
	if got := findText(root, nsNFe, "CNPJ", "CPF"); got != "12345678909" {
		t.Errorf("findText CPF fallthrough = %q", got)
	}
}

func TestFindText_Missing_ReturnsEmpty(t *testing.T) {
	raw := `<root xmlns="http://www.portalfiscal.inf.br/nfe"><other>val</other></root>`
	root, _ := parseXMLBytes([]byte(raw))
	if got := findText(root, nsNFe, "chNFe"); got != "" {
		t.Errorf("findText missing = %q, want empty", got)
	}
}

func TestFindText_NilElement_ReturnsEmpty(t *testing.T) {
	if got := findText(nil, nsNFe, "chNFe"); got != "" {
		t.Errorf("findText(nil) = %q, want empty", got)
	}
}

func TestFindEl_FindsNestedElement(t *testing.T) {
	raw := `<root xmlns="http://www.portalfiscal.inf.br/nfe">
		<infProt><nProt>99</nProt></infProt>
	</root>`
	root, _ := parseXMLBytes([]byte(raw))
	el := findEl(root, nsNFe, "infProt")
	if el == nil {
		t.Fatal("findEl returned nil")
	}
	if el.Local != "infProt" {
		t.Errorf("Local = %q, want infProt", el.Local)
	}
}

func TestFindEl_NilRoot_ReturnsNil(t *testing.T) {
	if findEl(nil, nsNFe, "infProt") != nil {
		t.Error("findEl(nil) should return nil")
	}
}

func TestFindAllEls_ReturnsAllMatches(t *testing.T) {
	raw := `<root xmlns="http://www.portalfiscal.inf.br/nfe">
		<det><prod><cProd>001</cProd></prod></det>
		<det><prod><cProd>002</cProd></prod></det>
		<det><prod><cProd>003</cProd></prod></det>
	</root>`
	root, _ := parseXMLBytes([]byte(raw))
	dets := findAllEls(root, nsNFe, "det")
	if len(dets) != 3 {
		t.Errorf("findAllEls returned %d elements, want 3", len(dets))
	}
}

func TestFindAllEls_NilRoot_ReturnsNil(t *testing.T) {
	if got := findAllEls(nil, nsNFe, "det"); got != nil {
		t.Error("findAllEls(nil) should return nil")
	}
}

// ---------------------------------------------------------------------------
// ufCode
// ---------------------------------------------------------------------------

func TestUfCode_AllKnownStates(t *testing.T) {
	cases := []struct{ uf, code string }{
		{"AC", "12"}, {"AL", "27"}, {"AP", "16"}, {"AM", "13"},
		{"BA", "29"}, {"CE", "23"}, {"DF", "53"}, {"ES", "32"},
		{"GO", "52"}, {"MA", "21"}, {"MT", "51"}, {"MS", "50"},
		{"MG", "31"}, {"PA", "15"}, {"PB", "25"}, {"PR", "41"},
		{"PE", "26"}, {"PI", "22"}, {"RJ", "33"}, {"RN", "24"},
		{"RS", "43"}, {"RO", "11"}, {"RR", "14"}, {"SC", "42"},
		{"SP", "35"}, {"SE", "28"}, {"TO", "17"},
	}
	for _, tc := range cases {
		if got := ufCode(tc.uf); got != tc.code {
			t.Errorf("ufCode(%q) = %q, want %q", tc.uf, got, tc.code)
		}
	}
}

func TestUfCode_UnknownDefaultsSP(t *testing.T) {
	for _, uf := range []string{"XX", "", "AN", "00"} {
		if got := ufCode(uf); got != "35" {
			t.Errorf("ufCode(%q) = %q, want 35 (default)", uf, got)
		}
	}
}

// ---------------------------------------------------------------------------
// akSerieNumber
// ---------------------------------------------------------------------------

func TestAkSerieNumber_Standard44Char(t *testing.T) {
	// positions 22-24 = serie "001", positions 25-33 = nNF "000000001"
	accessKey := "35250512345678000195550010000000011000000011"
	serie, number := akSerieNumber(accessKey)
	if serie != 1 {
		t.Errorf("serie = %d, want 1", serie)
	}
	if number != 1 {
		t.Errorf("number = %d, want 1", number)
	}
}

func TestAkSerieNumber_ShortKey_ReturnsZero(t *testing.T) {
	serie, number := akSerieNumber("short")
	if serie != 0 || number != 0 {
		t.Errorf("expected (0,0) for short key, got (%d,%d)", serie, number)
	}
}

// ---------------------------------------------------------------------------
// fmtDhManifest
// ---------------------------------------------------------------------------

func TestFmtDhManifest_UTCOffset(t *testing.T) {
	t1 := time.Date(2025, 6, 9, 14, 30, 0, 0, time.UTC)
	got := fmtDhManifest(t1)
	if !strings.HasPrefix(got, "2025-06-09T14:30:00") {
		t.Errorf("fmtDhManifest = %q, want date prefix 2025-06-09T14:30:00", got)
	}
	if !strings.Contains(got, "+00:00") {
		t.Errorf("fmtDhManifest = %q, want +00:00 for UTC", got)
	}
}

func TestFmtDhManifest_NegativeOffset(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	t1 := time.Date(2025, 6, 9, 14, 30, 0, 0, loc)
	got := fmtDhManifest(t1)
	if !strings.Contains(got, "-03:00") {
		t.Errorf("fmtDhManifest = %q, want -03:00 suffix", got)
	}
}

func TestFmtDhManifest_PositiveOffset(t *testing.T) {
	loc := time.FixedZone("IST", 5*3600+30*60)
	t1 := time.Date(2025, 6, 9, 14, 30, 0, 0, loc)
	got := fmtDhManifest(t1)
	if !strings.Contains(got, "+05:30") {
		t.Errorf("fmtDhManifest = %q, want +05:30 suffix", got)
	}
}

// ---------------------------------------------------------------------------
// genULID
// ---------------------------------------------------------------------------

// TestgenULID_Format valida o tamanho exato de 26 caracteres do ULID string.
func TestGenULID_Format(t *testing.T) {
	id := genULID()

	// Um ULID no formato string tem exatamente 26 caracteres (Base32).
	if len(id) != 26 {
		t.Fatalf("ULID deve ter exatamente 26 caracteres, mas tem %d: %q", len(id), id)
	}
}

// TestgenULID_CrockfordBase32 valida se a string usa apenas o alfabeto Crockford Base32.
// Ele exclui explicitamente as letras I, L, O e U para evitar ambiguidade visual.
func TestGenULID_CrockfordBase32(t *testing.T) {
	id := genULID()

	// Regex aceita apenas números e letras válidas no padrão Crockford (Maiúsculas)
	isValidULID := regexp.MustCompile("^[0-9A-HJKMNP-TV-Z]{26}$").MatchString

	if !isValidULID(id) {
		t.Errorf("ULID contém caracteres inválidos para o padrão Crockford Base32: %q", id)
	}

}

// TestgenULID_Uppercase valida se a string retornada segue o padrão oficial de maiúsculas.
func TestGenULID_Uppercase(t *testing.T) {
	id := genULID()
	if id != strings.ToUpper(id) {
		t.Errorf("O ULID padrão deve ser retornado em letras maiúsculas: %q", id)
	}
}

// TestgenULID_Uniqueness garante a unicidade em gerações em lote.
func TestGenULID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for range 100000 {
		id := genULID()
		if seen[id] {
			t.Fatalf("ULID duplicado detectado: %q", id)
		}
		seen[id] = true
	}
}

// ---------------------------------------------------------------------------
// extractResNFe
// ---------------------------------------------------------------------------

const resNFeXML = `<?xml version="1.0" encoding="UTF-8"?>
<resNFe xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.01">
  <chNFe>35250512345678000195550010000000011000000011</chNFe>
  <CNPJ>12345678000195</CNPJ>
  <xNome>EMPRESA EMITENTE LTDA</xNome>
  <dhEmi>2025-05-01T10:00:00-03:00</dhEmi>
  <tpNF>1</tpNF>
  <vNF>1500.00</vNF>
  <digVal>DIGESTVAL==</digVal>
  <cSitNFe>1</cSitNFe>
</resNFe>`

func TestExtractResNFe_AllFields(t *testing.T) {
	root, err := parseXMLBytes([]byte(resNFeXML))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	f := extractResNFe(root)

	if f.SchemaType != SchemaResNFe {
		t.Errorf("SchemaType = %q, want %q", f.SchemaType, SchemaResNFe)
	}
	if f.AccessKey != "35250512345678000195550010000000011000000011" {
		t.Errorf("AccessKey = %q", f.AccessKey)
	}
	if f.EmitCPFCNPJ != "12345678000195" {
		t.Errorf("EmitCPFCNPJ = %q", f.EmitCPFCNPJ)
	}
	if f.EmitName != "EMPRESA EMITENTE LTDA" {
		t.Errorf("EmitName = %q", f.EmitName)
	}
	if f.Total != "1500.00" {
		t.Errorf("Total = %q", f.Total)
	}
	if f.Status != "1" {
		t.Errorf("Status = %q", f.Status)
	}
	if f.DigestValue != "DIGESTVAL==" {
		t.Errorf("DigestValue = %q", f.DigestValue)
	}
	if f.Incoming != 1 {
		t.Errorf("Incoming = %d, want 1", f.Incoming)
	}
	if f.Year != 2025 || f.Month != 5 || f.Day != 1 {
		t.Errorf("date = %d-%02d-%02d, want 2025-05-01", f.Year, f.Month, f.Day)
	}
	if f.Serie != 1 {
		t.Errorf("Serie = %d, want 1 (from access key)", f.Serie)
	}
	if f.Number != 1 {
		t.Errorf("Number = %d, want 1 (from access key)", f.Number)
	}
}

// ---------------------------------------------------------------------------
// extractProcNFe
// ---------------------------------------------------------------------------

const procNFeXML = `<?xml version="1.0" encoding="UTF-8"?>
<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00">
  <NFe>
    <infNFe Id="NFe35250512345678000195550010000000011000000011">
      <ide>
        <dhEmi>2025-05-01T10:00:00-03:00</dhEmi>
        <serie>1</serie>
        <nNF>1</nNF>
      </ide>
      <emit>
        <CNPJ>12345678000195</CNPJ>
        <xNome>EMITENTE SA</xNome>
      </emit>
      <dest>
        <CNPJ>98765432000188</CNPJ>
        <xNome>DESTINATARIO LTDA</xNome>
      </dest>
      <det nItem="1">
        <prod>
          <cProd>001</cProd>
          <xProd>PRODUTO TESTE</xProd>
          <NCM>01012100</NCM>
          <CFOP>6101</CFOP>
          <uCom>UN</uCom>
          <qCom>10.0000</qCom>
          <vUnCom>150.0000</vUnCom>
          <vDesc>0.00</vDesc>
          <vProd>1500.00</vProd>
        </prod>
      </det>
      <total>
        <ICMSTot>
          <vNF>1500.00</vNF>
        </ICMSTot>
      </total>
      <pag>
        <detPag>
          <tPag>01</tPag>
          <vPag>1500.00</vPag>
        </detPag>
      </pag>
    </infNFe>
  </NFe>
  <protNFe>
    <infProt>
      <chNFe>35250512345678000195550010000000011000000011</chNFe>
      <nProt>135250512345678</nProt>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso da NF-e</xMotivo>
    </infProt>
  </protNFe>
</nfeProc>`

const richEmitXML = `<emit xmlns="http://www.portalfiscal.inf.br/nfe">
  <CNPJ>98765432000188</CNPJ>
  <xNome>FORNECEDOR SA</xNome>
  <xFant>FORNECEDOR</xFant>
  <enderEmit>
    <xLgr>RUA TESTE</xLgr>
    <nro>100</nro>
    <xCpl>SALA 1</xCpl>
    <xBairro>CENTRO</xBairro>
    <cMun>3550308</cMun>
    <xMun>SAO PAULO</xMun>
    <UF>SP</UF>
    <CEP>01001000</CEP>
    <fone>1133334444</fone>
  </enderEmit>
  <IE>123456789</IE>
  <CRT>3</CRT>
</emit>`

func TestBuildPersonDetails_FullParty(t *testing.T) {
	root, err := parseXMLBytes([]byte(richEmitXML))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	d := buildPersonDetails(root, nsNFe)
	if d == nil {
		t.Fatal("buildPersonDetails returned nil for a rich party")
	}

	if d["fantasy_name"] != "FORNECEDOR" {
		t.Errorf("fantasy_name = %v", d["fantasy_name"])
	}
	if d["crt"] != 3 {
		t.Errorf("crt = %v (%T), want int 3", d["crt"], d["crt"])
	}

	addrs, ok := d["addresses"].([]any)
	if !ok || len(addrs) != 1 {
		t.Fatalf("addresses = %v, want 1 entry", d["addresses"])
	}
	addr := addrs[0].(map[string]any)
	for k, want := range map[string]string{
		"street": "RUA TESTE", "number": "100", "complement": "SALA 1",
		"neighborhood": "CENTRO", "city_ibge_code": "3550308", "city": "SAO PAULO",
		"state_federation": "SP", "postal_code": "01001000",
	} {
		if addr[k] != want {
			t.Errorf("address[%q] = %v, want %q", k, addr[k], want)
		}
	}

	contacts := d["contacts"].(map[string]any)
	if phones := contacts["phones"].([]any); len(phones) != 1 || phones[0] != "1133334444" {
		t.Errorf("contacts.phones = %v", contacts["phones"])
	}

	regs := d["state_registrations"].([]any)
	reg := regs[0].(map[string]any)
	if reg["uf"] != "SP" || reg["state_registration"] != "123456789" {
		t.Errorf("state_registrations[0] = %v", reg)
	}
}

func TestBuildPersonDetails_EmptyParty_ReturnsNil(t *testing.T) {
	root, _ := parseXMLBytes([]byte(`<emit xmlns="http://www.portalfiscal.inf.br/nfe"><CNPJ>98765432000188</CNPJ><xNome>X</xNome></emit>`))
	if d := buildPersonDetails(root, nsNFe); d != nil {
		t.Errorf("expected nil for party without address/contacts/IE, got %v", d)
	}
}

func TestExtractProcNFe_AllFields(t *testing.T) {
	root, err := parseXMLBytes([]byte(procNFeXML))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	f := extractProcNFe(root, "98765432000188")

	if f.SchemaType != SchemaProcNFe {
		t.Errorf("SchemaType = %q, want %q", f.SchemaType, SchemaProcNFe)
	}
	if f.AccessKey != "35250512345678000195550010000000011000000011" {
		t.Errorf("AccessKey = %q", f.AccessKey)
	}
	if f.EmitCPFCNPJ != "12345678000195" {
		t.Errorf("EmitCPFCNPJ = %q", f.EmitCPFCNPJ)
	}
	if f.EmitName != "EMITENTE SA" {
		t.Errorf("EmitName = %q", f.EmitName)
	}
	if f.DestCPFCNPJ != "98765432000188" {
		t.Errorf("DestCPFCNPJ = %q", f.DestCPFCNPJ)
	}
	if f.DestName != "DESTINATARIO LTDA" {
		t.Errorf("DestName = %q", f.DestName)
	}
	if f.SefazStatus != "100" {
		t.Errorf("SefazStatus = %q", f.SefazStatus)
	}
	if f.SefazMotive != "Autorizado o uso da NF-e" {
		t.Errorf("SefazMotive = %q", f.SefazMotive)
	}
	if f.SefazProtocol != "135250512345678" {
		t.Errorf("SefazProtocol = %q", f.SefazProtocol)
	}
	if f.Serie != 1 {
		t.Errorf("Serie = %d, want 1", f.Serie)
	}
	if f.Number != 1 {
		t.Errorf("Number = %d, want 1", f.Number)
	}
	if len(f.Products) != 1 {
		t.Fatalf("Products len = %d, want 1", len(f.Products))
	}
	if f.Products[0]["description"] != "PRODUTO TESTE" {
		t.Errorf("Products[0].description = %q", f.Products[0]["description"])
	}
	if f.Products[0]["cfop"] != "6101" {
		t.Errorf("Products[0].cfop = %q", f.Products[0]["cfop"])
	}
	if f.Products[0]["discount"] != "0.00" {
		t.Errorf("Products[0].discount = %q, want 0.00 (default)", f.Products[0]["discount"])
	}
	if len(f.Payments) != 1 {
		t.Fatalf("Payments len = %d, want 1", len(f.Payments))
	}
	if f.Payments[0]["payment_type"] != "01" {
		t.Errorf("Payments[0].payment_type = %q", f.Payments[0]["payment_type"])
	}
	if f.Payments[0]["value"] != "1500.00" {
		t.Errorf("Payments[0].value = %q", f.Payments[0]["value"])
	}
	if f.Incoming != 1 {
		t.Errorf("Incoming = %d, want 1", f.Incoming)
	}
}

func TestExtractProcNFe_StripNFePrefix(t *testing.T) {
	// When chNFe starts with "NFe", the extractor must strip the prefix.
	root, _ := parseXMLBytes([]byte(procNFeXML))
	f := extractProcNFe(root, "98765432000188")
	if strings.HasPrefix(f.AccessKey, "NFe") {
		t.Errorf("AccessKey must not have NFe prefix: %q", f.AccessKey)
	}
}

func TestExtractProcNFe_TransportadoraIncoming(t *testing.T) {
	// If the org is the transportadora (not the dest), incoming = 2.
	xml := `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe">
		<NFe><infNFe Id="NFe35250512345678000195550010000000011000000011">
			<ide><dhEmi>2025-01-01T00:00:00-03:00</dhEmi><serie>1</serie><nNF>1</nNF></ide>
			<emit><CNPJ>11111111000111</CNPJ><xNome>EMIT</xNome></emit>
			<dest><CNPJ>22222222000122</CNPJ><xNome>DEST</xNome></dest>
			<transporta><CNPJ>99999999000199</CNPJ></transporta>
			<total><ICMSTot><vNF>100</vNF></ICMSTot></total>
		</infNFe></NFe>
		<protNFe><infProt>
			<chNFe>35250512345678000195550010000000011000000011</chNFe>
			<cStat>100</cStat><xMotivo>Autorizado</xMotivo><nProt>1</nProt>
		</infProt></protNFe>
	</nfeProc>`
	root, _ := parseXMLBytes([]byte(xml))
	// cnpj is the transportadora's CNPJ and dest is different → incoming=2.
	f := extractProcNFe(root, "99999999000199")
	if f.Incoming != 2 {
		t.Errorf("Incoming = %d, want 2 for transportadora when dest differs", f.Incoming)
	}
}

func TestExtractProcNFe_MultipleProducts(t *testing.T) {
	xml := `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe">
		<NFe><infNFe Id="NFe35250512345678000195550010000000011000000011">
			<ide><dhEmi>2025-01-01T00:00:00-03:00</dhEmi><serie>1</serie><nNF>1</nNF></ide>
			<emit><CNPJ>12345678000195</CNPJ><xNome>E</xNome></emit>
			<dest><CNPJ>98765432000188</CNPJ><xNome>D</xNome></dest>
			<det nItem="1"><prod><cProd>A</cProd><xProd>Prod A</xProd><NCM>1</NCM><CFOP>1</CFOP><uCom>UN</uCom><qCom>1</qCom><vUnCom>10</vUnCom><vProd>10</vProd></prod></det>
			<det nItem="2"><prod><cProd>B</cProd><xProd>Prod B</xProd><NCM>2</NCM><CFOP>2</CFOP><uCom>KG</uCom><qCom>5</qCom><vUnCom>20</vUnCom><vProd>100</vProd></prod></det>
			<total><ICMSTot><vNF>110</vNF></ICMSTot></total>
			<pag><detPag><tPag>01</tPag><vPag>110</vPag></detPag></pag>
		</infNFe></NFe>
		<protNFe><infProt>
			<chNFe>35250512345678000195550010000000011000000011</chNFe>
			<cStat>100</cStat><xMotivo>OK</xMotivo><nProt>1</nProt>
		</infProt></protNFe>
	</nfeProc>`
	root, _ := parseXMLBytes([]byte(xml))
	f := extractProcNFe(root, "98765432000188")
	if len(f.Products) != 2 {
		t.Errorf("Products len = %d, want 2", len(f.Products))
	}
}

// ---------------------------------------------------------------------------
// extractResCTe
// ---------------------------------------------------------------------------

const resCTeXML = `<?xml version="1.0" encoding="UTF-8"?>
<resCTe xmlns="http://www.portalfiscal.inf.br/cte" versao="1.00">
  <chCTe>35250512345678000195570010000000421000000042</chCTe>
  <CNPJ>12345678000195</CNPJ>
  <xNome>TRANSPORTADORA SA</xNome>
  <dhEmi>2025-05-01T10:00:00-03:00</dhEmi>
  <vCT>850.00</vCT>
  <cSitCTe>1</cSitCTe>
</resCTe>`

func TestExtractResCTe_AllFields(t *testing.T) {
	root, err := parseXMLBytes([]byte(resCTeXML))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	f := extractResCTe(root)

	if f.SchemaType != SchemaResCTe {
		t.Errorf("SchemaType = %q, want %q", f.SchemaType, SchemaResCTe)
	}
	if f.AccessKey != "35250512345678000195570010000000421000000042" {
		t.Errorf("AccessKey = %q", f.AccessKey)
	}
	if f.EmitCPFCNPJ != "12345678000195" {
		t.Errorf("EmitCPFCNPJ = %q", f.EmitCPFCNPJ)
	}
	if f.EmitName != "TRANSPORTADORA SA" {
		t.Errorf("EmitName = %q", f.EmitName)
	}
	if f.Total != "850.00" {
		t.Errorf("Total = %q", f.Total)
	}
	if f.Status != "1" {
		t.Errorf("Status = %q", f.Status)
	}
	if f.Incoming != 1 {
		t.Errorf("Incoming = %d, want 1", f.Incoming)
	}
}

// ---------------------------------------------------------------------------
// extractProcCTe
// ---------------------------------------------------------------------------

const procCTeXML = `<?xml version="1.0" encoding="UTF-8"?>
<cteProc xmlns="http://www.portalfiscal.inf.br/cte" versao="3.00">
  <CTe>
    <infCte>
      <ide>
        <dhEmi>2025-05-01T10:00:00-03:00</dhEmi>
        <serie>1</serie>
        <nCT>42</nCT>
      </ide>
      <emit>
        <CNPJ>12345678000195</CNPJ>
        <xNome>TRANSPORTADORA SA</xNome>
      </emit>
      <infCTeNorm>
        <toma4>
          <CNPJ>98765432000188</CNPJ>
          <xNome>TOMADOR LTDA</xNome>
        </toma4>
      </infCTeNorm>
    </infCte>
  </CTe>
  <protCTe>
    <infProt>
      <chCTe>35250512345678000195570010000000421000000042</chCTe>
      <nProt>135250512345678</nProt>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso do CT-e</xMotivo>
    </infProt>
  </protCTe>
  <vTPrest>850.00</vTPrest>
</cteProc>`

func TestExtractProcCTe_AllFields(t *testing.T) {
	root, err := parseXMLBytes([]byte(procCTeXML))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	f := extractProcCTe(root)

	if f.SchemaType != SchemaProcCTe {
		t.Errorf("SchemaType = %q, want %q", f.SchemaType, SchemaProcCTe)
	}
	if f.EmitCPFCNPJ != "12345678000195" {
		t.Errorf("EmitCPFCNPJ = %q", f.EmitCPFCNPJ)
	}
	if f.EmitName != "TRANSPORTADORA SA" {
		t.Errorf("EmitName = %q", f.EmitName)
	}
	if f.DestCPFCNPJ != "98765432000188" {
		t.Errorf("DestCPFCNPJ = %q", f.DestCPFCNPJ)
	}
	if f.Total != "850.00" {
		t.Errorf("Total = %q", f.Total)
	}
	if f.SefazStatus != "100" {
		t.Errorf("SefazStatus = %q", f.SefazStatus)
	}
	if f.SefazProtocol != "135250512345678" {
		t.Errorf("SefazProtocol = %q", f.SefazProtocol)
	}
	if f.Number != 42 {
		t.Errorf("Number = %d, want 42", f.Number)
	}
}

// ---------------------------------------------------------------------------
// extractResMDFe
// ---------------------------------------------------------------------------

const resMDFeXML = `<?xml version="1.0" encoding="UTF-8"?>
<resMDFe xmlns="http://www.portalfiscal.inf.br/mdfe" versao="1.00">
  <chMDFe>35250512345678000195580010000000071000000007</chMDFe>
  <CNPJ>12345678000195</CNPJ>
  <xNome>TRANSPORTADORA SA</xNome>
  <dhEmi>2025-05-01T10:00:00-03:00</dhEmi>
  <cSitMDFe>1</cSitMDFe>
</resMDFe>`

func TestExtractResMDFe_AllFields(t *testing.T) {
	root, err := parseXMLBytes([]byte(resMDFeXML))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	f := extractResMDFe(root)

	if f.SchemaType != SchemaResMDFe {
		t.Errorf("SchemaType = %q, want %q", f.SchemaType, SchemaResMDFe)
	}
	if f.AccessKey != "35250512345678000195580010000000071000000007" {
		t.Errorf("AccessKey = %q", f.AccessKey)
	}
	if f.EmitCPFCNPJ != "12345678000195" {
		t.Errorf("EmitCPFCNPJ = %q", f.EmitCPFCNPJ)
	}
	if f.Status != "1" {
		t.Errorf("Status = %q", f.Status)
	}
	if f.Incoming != 1 {
		t.Errorf("Incoming = %d, want 1", f.Incoming)
	}
}

// ---------------------------------------------------------------------------
// extractProcMDFe
// ---------------------------------------------------------------------------

const procMDFeXML = `<?xml version="1.0" encoding="UTF-8"?>
<procMDFe xmlns="http://www.portalfiscal.inf.br/mdfe" versao="3.00">
  <MDFe>
    <infMDFe>
      <ide>
        <dhEmi>2025-05-01T10:00:00-03:00</dhEmi>
        <serie>1</serie>
        <nMDF>7</nMDF>
      </ide>
      <emit>
        <CNPJ>12345678000195</CNPJ>
        <xNome>TRANSPORTADORA SA</xNome>
      </emit>
    </infMDFe>
  </MDFe>
  <protMDFe>
    <infProt>
      <chMDFe>35250512345678000195580010000000071000000007</chMDFe>
      <nProt>135250512345678</nProt>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso do MDF-e</xMotivo>
    </infProt>
  </protMDFe>
</procMDFe>`

func TestExtractProcMDFe_AllFields(t *testing.T) {
	root, err := parseXMLBytes([]byte(procMDFeXML))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	f := extractProcMDFe(root)

	if f.SchemaType != SchemaProcMDFe {
		t.Errorf("SchemaType = %q, want %q", f.SchemaType, SchemaProcMDFe)
	}
	if f.EmitCPFCNPJ != "12345678000195" {
		t.Errorf("EmitCPFCNPJ = %q", f.EmitCPFCNPJ)
	}
	if f.EmitName != "TRANSPORTADORA SA" {
		t.Errorf("EmitName = %q", f.EmitName)
	}
	if f.SefazStatus != "100" {
		t.Errorf("SefazStatus = %q", f.SefazStatus)
	}
	if f.SefazProtocol != "135250512345678" {
		t.Errorf("SefazProtocol = %q", f.SefazProtocol)
	}
	if f.Number != 7 {
		t.Errorf("Number = %d, want 7", f.Number)
	}
}

// ---------------------------------------------------------------------------
// extractResEvento
// ---------------------------------------------------------------------------

const resEventoNFeXML = `<?xml version="1.0" encoding="UTF-8"?>
<retEnvEvento xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.00">
  <retEvento versao="1.00">
    <infEvento>
      <chNFe>35250512345678000195550010000000011000000011</chNFe>
      <tpEvento>110111</tpEvento>
      <nSeqEvento>1</nSeqEvento>
      <cStat>135</cStat>
      <xMotivo>Evento registrado e vinculado a NF-e</xMotivo>
      <nProt>135250512345678</nProt>
      <dhRegEvento>2025-05-01T10:00:00-03:00</dhRegEvento>
    </infEvento>
  </retEvento>
</retEnvEvento>`

func TestExtractResEvento_NFe(t *testing.T) {
	root, err := parseXMLBytes([]byte(resEventoNFeXML))
	if err != nil {
		t.Fatalf("parseXMLBytes: %v", err)
	}
	f := extractResEvento(root, "nfe")

	if f.SchemaType != SchemaResEvento {
		t.Errorf("SchemaType = %q, want %q", f.SchemaType, SchemaResEvento)
	}
	if f.AccessKey != "35250512345678000195550010000000011000000011" {
		t.Errorf("AccessKey = %q", f.AccessKey)
	}
	if f.EventType != "110111" {
		t.Errorf("EventType = %q, want 110111", f.EventType)
	}
	if f.SequenceNumber != "1" {
		t.Errorf("SequenceNumber = %q, want 1", f.SequenceNumber)
	}
	if f.SefazStatus != "135" {
		t.Errorf("SefazStatus = %q, want 135", f.SefazStatus)
	}
	if f.SefazMotive != "Evento registrado e vinculado a NF-e" {
		t.Errorf("SefazMotive = %q", f.SefazMotive)
	}
	if f.SefazProtocol != "135250512345678" {
		t.Errorf("SefazProtocol = %q", f.SefazProtocol)
	}
}

func TestExtractResEvento_CTe(t *testing.T) {
	xml := `<retEnvEvento xmlns="http://www.portalfiscal.inf.br/cte">
		<retEvento><infEvento>
			<chCTe>35250512345678000195570010000000421000000042</chCTe>
			<tpEvento>110111</tpEvento>
			<nSeqEvento>1</nSeqEvento>
			<cStat>135</cStat>
			<xMotivo>Evento registrado</xMotivo>
			<nProt>1</nProt>
		</infEvento></retEvento>
	</retEnvEvento>`
	root, _ := parseXMLBytes([]byte(xml))
	f := extractResEvento(root, "cte")
	if f.AccessKey != "35250512345678000195570010000000421000000042" {
		t.Errorf("CTe event AccessKey = %q", f.AccessKey)
	}
}

// ---------------------------------------------------------------------------
// extractDoc dispatcher
// ---------------------------------------------------------------------------

func TestExtractDoc_DispatchesAllSchemas(t *testing.T) {
	cases := []struct {
		schemaType string
		xml        string
		docType    string
	}{
		{SchemaResNFe, resNFeXML, "nfe"},
		{SchemaProcNFe, procNFeXML, "nfe"},
		{SchemaResCTe, resCTeXML, "cte"},
		{SchemaProcCTe, procCTeXML, "cte"},
		{SchemaResMDFe, resMDFeXML, "mdfe"},
		{SchemaProcMDFe, procMDFeXML, "mdfe"},
		{SchemaResEvento, resEventoNFeXML, "nfe"},
	}
	for _, tc := range cases {
		root, _ := parseXMLBytes([]byte(tc.xml))
		f := extractDoc(tc.schemaType, root, tc.docType, "12345678000195")
		if f.SchemaType != tc.schemaType {
			t.Errorf("[%s] SchemaType = %q, want %q", tc.schemaType, f.SchemaType, tc.schemaType)
		}
	}
}

func TestExtractDoc_UnknownSchema_ReturnsEmpty(t *testing.T) {
	root, _ := parseXMLBytes([]byte("<root/>"))
	f := extractDoc("unknown", root, "nfe", "")
	if f.AccessKey != "" || f.SchemaType != "" {
		t.Errorf("expected empty DocFields for unknown schema, got %+v", f)
	}
}

// ---------------------------------------------------------------------------
// classifyImportXML
// ---------------------------------------------------------------------------

const sampleNfeProcXML = `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><NFe><infNFe Id="NFe22260811647612000197550000000000501454670090"><emit><CNPJ>11647612000197</CNPJ></emit><dest><CNPJ>22222222000122</CNPJ></dest></infNFe></NFe></nfeProc>`

func TestClassifyImportXML_EmitMatch_IsEmitida(t *testing.T) {
	root, err := parseXMLBytes([]byte(sampleNfeProcXML))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := classifyImportXML(root, "11647612000197")
	if !ok || got.Incoming != 0 {
		t.Fatalf("expected Incoming=0 (emitida), got %+v ok=%v", got, ok)
	}
}

func TestClassifyImportXML_DestMatch_WhenEmitDiffers_IsDestinada(t *testing.T) {
	root, _ := parseXMLBytes([]byte(sampleNfeProcXML))
	got, ok := classifyImportXML(root, "22222222000122")
	if !ok || got.Incoming != 1 {
		t.Fatalf("expected Incoming=1 (destinada), got %+v ok=%v", got, ok)
	}
}

func TestClassifyImportXML_TranspMatch_WhenEmitAndDestDiffer_IsTransportada(t *testing.T) {
	xml := `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe"><NFe><infNFe Id="NFe1"><emit><CNPJ>11647612000197</CNPJ></emit><dest><CNPJ>22222222000122</CNPJ></dest><transp><transporta><CNPJ>33333333000199</CNPJ></transporta></transp></infNFe></NFe></nfeProc>`
	root, _ := parseXMLBytes([]byte(xml))
	got, ok := classifyImportXML(root, "33333333000199")
	if !ok || got.Incoming != 2 {
		t.Fatalf("expected Incoming=2 (transportada), got %+v ok=%v", got, ok)
	}
}

func TestClassifyImportXML_NoMatch_IsRejected(t *testing.T) {
	root, _ := parseXMLBytes([]byte(sampleNfeProcXML))
	_, ok := classifyImportXML(root, "99999999000100")
	if ok {
		t.Fatal("expected ok=false when no party matches org CNPJ")
	}
}

func TestClassifyImportXML_EmitTakesPriorityOverDestAndTransp(t *testing.T) {
	// Mesmo CNPJ aparecendo como dest E transp — dest deve vencer, nunca transp
	// (emit não bate aqui, então dest é o primeiro na prioridade que bate).
	xml := `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe"><NFe><infNFe Id="NFe1"><emit><CNPJ>11647612000197</CNPJ></emit><dest><CNPJ>22222222000122</CNPJ></dest><transp><transporta><CNPJ>22222222000122</CNPJ></transporta></transp></infNFe></NFe></nfeProc>`
	root, _ := parseXMLBytes([]byte(xml))
	got, ok := classifyImportXML(root, "22222222000122")
	if !ok || got.Incoming != 1 {
		t.Fatalf("expected dest (Incoming=1) to win over transp, got %+v ok=%v", got, ok)
	}
}

// ---------------------------------------------------------------------------
// validImportRoot / compareImportDigests / buildFinalNfeProc
// ---------------------------------------------------------------------------

func loadSampleNfeProc(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/nfeproc_sample.xml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestValidImportRoot_AcceptsNfeProcAndBareNFe(t *testing.T) {
	nfeProcRoot, _ := parseXMLBytes(loadSampleNfeProc(t))
	if !validImportRoot(nfeProcRoot) {
		t.Fatal("expected nfeProc root to be valid")
	}
	bareRoot, _ := parseXMLBytes([]byte(`<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe1"></infNFe></NFe>`))
	if !validImportRoot(bareRoot) {
		t.Fatal("expected bare NFe root to be valid")
	}
	otherRoot, _ := parseXMLBytes([]byte(`<resNFe xmlns="http://www.portalfiscal.inf.br/nfe"></resNFe>`))
	if validImportRoot(otherRoot) {
		t.Fatal("expected resNFe root to be rejected")
	}
}

func TestExtractImportTpAmb_ReadsIdeTpAmb(t *testing.T) {
	root, _ := parseXMLBytes(loadSampleNfeProc(t))
	if got := extractImportTpAmb(root); got != "2" {
		t.Fatalf("expected tpAmb=2 (fixture is homologação), got %q", got)
	}
}

func TestExtractImportTpAmb_MissingIde_ReturnsEmpty(t *testing.T) {
	root, _ := parseXMLBytes([]byte(`<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe1"></infNFe></NFe>`))
	if got := extractImportTpAmb(root); got != "" {
		t.Fatalf("expected empty tpAmb when <ide> is absent, got %q", got)
	}
}

func TestCompareImportDigests_NfeProc_AllThreeMustMatch(t *testing.T) {
	root, _ := parseXMLBytes(loadSampleNfeProc(t))
	const matchingDigest = "cKFyNtF4cg+d63/SRv0ezXGoef8="
	if !compareImportDigests(root, matchingDigest) {
		t.Fatal("expected match: fixture's protNFe/digVal and Signature/DigestValue are both this value")
	}
	if compareImportDigests(root, "different-digest") {
		t.Fatal("expected mismatch to be rejected")
	}
}

func TestCompareImportDigests_BareNFe_ComparesOnlySignatureDigest(t *testing.T) {
	full := string(loadSampleNfeProc(t))
	// Extrai só o <NFe>...</NFe> (sem protNFe) para simular upload sem protocolo.
	start := strings.Index(full, "<NFe>")
	end := strings.Index(full, "</NFe>") + len("</NFe>")
	bareXML := full[start:end]
	root, err := parseXMLBytes([]byte(bareXML))
	if err != nil {
		t.Fatal(err)
	}
	const sigDigest = "cKFyNtF4cg+d63/SRv0ezXGoef8=" // mesmo valor no fixture (Signature/DigestValue)
	if !compareImportDigests(root, sigDigest) {
		t.Fatal("expected match against Signature/DigestValue")
	}
	if compareImportDigests(root, "different-digest") {
		t.Fatal("expected mismatch to be rejected")
	}
}

func TestBuildFinalNfeProc_NfeProcRoot_ReturnsUnchanged(t *testing.T) {
	original := loadSampleNfeProc(t)
	root, _ := parseXMLBytes(original)
	out, err := buildFinalNfeProc(original, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(original) {
		t.Fatal("expected nfeProc input to pass through unchanged")
	}
}

func TestBuildFinalNfeProc_BareNFe_JoinsProtNFe(t *testing.T) {
	full := string(loadSampleNfeProc(t))
	start := strings.Index(full, "<NFe>")
	end := strings.Index(full, "</NFe>") + len("</NFe>")
	bareXML := []byte(full[start:end])
	root, err := parseXMLBytes(bareXML)
	if err != nil {
		t.Fatal(err)
	}
	protNFeDict := map[string]any{
		"@versao": "4.00",
		"infProt": map[string]any{
			"tpAmb":    "2",
			"chNFe":    "22260811647612000197550000000000501454670090",
			"dhRecbto": "2026-08-08T17:05:06-03:00",
			"nProt":    "322260000016670",
			"digVal":   "cKFyNtF4cg+d63/SRv0ezXGoef8=",
			"cStat":    "100",
			"xMotivo":  "Autorizado o uso da NF-e",
		},
	}
	out, err := buildFinalNfeProc(bareXML, root, protNFeDict)
	if err != nil {
		t.Fatal(err)
	}
	joinedRoot, err := parseXMLBytes(out)
	if err != nil {
		t.Fatalf("joined output is not valid xml: %v\n%s", err, out)
	}
	if joinedRoot.Local != "nfeProc" {
		t.Fatalf("expected joined root to be nfeProc, got %s", joinedRoot.Local)
	}
	if findText(joinedRoot, nsNFe, "digVal") != "cKFyNtF4cg+d63/SRv0ezXGoef8=" {
		t.Fatalf("joined protNFe digVal missing/wrong: %s", out)
	}
	if findText(joinedRoot, nsNFe, "chNFe") == "" {
		t.Fatal("expected joined protNFe chNFe preserved")
	}
}

func TestClassifyImportXML_BareNFe_ExtractsAccessKeyFromIdAttribute(t *testing.T) {
	// Um NFe sem protocolo não tem elemento <chNFe> em lugar nenhum — só o
	// atributo Id de infNFe carrega o access key (confirmado no arquivo de
	// referência real). Sem isso, a consulta protocolo (Task 6) não teria
	// como saber qual chNFe consultar.
	root, err := parseXMLBytes([]byte(sampleNfeProcXML))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := classifyImportXML(root, "11647612000197")
	if !ok {
		t.Fatal("expected match")
	}
	if got.AccessKey != "22260811647612000197550000000000501454670090" {
		t.Fatalf("expected access key from infNFe/@Id (NFe prefix stripped), got %q", got.AccessKey)
	}
}
