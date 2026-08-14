package service

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
)

// Schema type constants — mirror distribution_parser.py.
const (
	SchemaProcEventoNFe  = "procEventoNFe"
	SchemaProcEventoCTe  = "procEventoCTe"
	SchemaProcEventoMDFe = "procEventoMDFe"
	SchemaProcNFe        = "procNFe"
	SchemaProcCTe        = "cteProc"
	SchemaProcCTeOS      = "cteOSProc"
	SchemaProcGTVe       = "GTVeProc"
	SchemaProcCTeSimp    = "cteSimpProc"
	SchemaProcMDFe       = "procMDFe"
	SchemaResNFe         = "resNFe"
	SchemaResCTe         = "resCTe"
	SchemaResMDFe        = "resMDFe"
	SchemaResEvento      = "resEvento"
)

var schemaOrder = []string{
	SchemaProcEventoNFe, SchemaProcEventoCTe, SchemaProcEventoMDFe,
	SchemaProcNFe, SchemaProcCTe, SchemaProcCTeOS, SchemaProcGTVe, SchemaProcCTeSimp, SchemaProcMDFe,
	SchemaResNFe, SchemaResCTe, SchemaResMDFe, SchemaResEvento,
}

var ufCodes = map[string]string{
	"AC": "12", "AL": "27", "AP": "16", "AM": "13", "BA": "29", "CE": "23",
	"DF": "53", "ES": "32", "GO": "52", "MA": "21", "MT": "51", "MS": "50",
	"MG": "31", "PA": "15", "PB": "25", "PR": "41", "PE": "26", "PI": "22",
	"RJ": "33", "RN": "24", "RS": "43", "RO": "11", "RR": "14", "SC": "42",
	"SP": "35", "SE": "28", "TO": "17",
}

const (
	nsNFe     = "http://www.portalfiscal.inf.br/nfe"
	nsCTe     = "http://www.portalfiscal.inf.br/cte"
	nsMDFe    = "http://www.portalfiscal.inf.br/mdfe"
	nsXMLDSig = "http://www.w3.org/2000/09/xmldsig#"
)

func ufCode(uf string) string {
	if c, ok := ufCodes[uf]; ok {
		return c
	}
	return "35"
}

// DocFields is the parsed result of a docZip entry.
type DocFields struct {
	SchemaType     string
	AccessKey      string
	EmitCPFCNPJ    string
	EmitName       string
	DestCPFCNPJ    string
	DestName       string
	Total          string
	SefazStatus    string
	SefazMotive    string
	SefazProtocol  string
	DHEmi          string
	DigestValue    string
	Status         string
	Incoming       int  // 0 means unset; callers treat 0 as 1, unless IncomingSet
	IncomingSet    bool // true when Incoming was explicitly computed (e.g. import-by-XML emitida=0)
	Year           int
	Month          int
	Day            int
	Serie          int
	Number         int
	CreatedAt      string
	EventType      string
	SequenceNumber string
	DHEvento       string
	XMLS3Key       string // filled in by the caller after S3 upload
	Products       []map[string]string
	Payments       []map[string]string
	// EmitDetails / DestDetails carry the counterparty's nested person object
	// (addresses, contacts, state_registrations, fantasy_name, crt) so it can be
	// persisted to organization_persons. nil when the party has no extra data.
	EmitDetails map[string]any
	DestDetails map[string]any
}

// importClassification is the result of classifyImportXML: which relation
// (emit/dest/transp) the org has to the uploaded document, and its access key.
type importClassification struct {
	Incoming  int
	AccessKey string
}

// classifyImportXML checks org membership in emit > dest > transp priority
// order against cnpj (org's own CPF/CNPJ), exactly this sequence — see
// docs/specs/2026-08-13-importacao-nfe-xml.md. ok=false means no relation
// to the org was found and the import must be rejected. This is import-XML
// specific: extractProcNFe (used by the normal distribution flow) never
// produces Incoming=0, since SEFAZ distribution never hands an org back a
// document it emitted itself.
func classifyImportXML(root *xmlEl, cnpj string) (importClassification, bool) {
	accessKey := findText(root, nsNFe, "chNFe")
	if accessKey == "" {
		// A bare NFe (no protNFe protocol wrapper yet) never carries a chNFe
		// element — the access key only exists as infNFe's Id attribute.
		if infNFe := findEl(root, nsNFe, "infNFe"); infNFe != nil {
			accessKey = infNFe.Attrs["Id"]
		}
	}
	if strings.HasPrefix(accessKey, "NFe") {
		accessKey = accessKey[3:]
	}

	emit := findEl(root, nsNFe, "emit")
	dest := findEl(root, nsNFe, "dest")
	transp := findEl(root, nsNFe, "transporta")

	orgDoc := onlyDigits(cnpj)
	emitDoc := onlyDigits(findText(emit, nsNFe, "CNPJ", "CPF"))
	destDoc := onlyDigits(findText(dest, nsNFe, "CNPJ", "CPF"))
	transpDoc := onlyDigits(findText(transp, nsNFe, "CNPJ", "CPF"))

	switch {
	case emitDoc != "" && emitDoc == orgDoc:
		return importClassification{Incoming: 0, AccessKey: accessKey}, true
	case destDoc != "" && destDoc == orgDoc:
		return importClassification{Incoming: 1, AccessKey: accessKey}, true
	case transpDoc != "" && transpDoc == orgDoc:
		return importClassification{Incoming: 2, AccessKey: accessKey}, true
	default:
		return importClassification{AccessKey: accessKey}, false
	}
}

// validImportRoot reports whether root's tag is an accepted XML-import root
// (nfeProc — with protocol — or bare NFe — signed but not yet queried).
func validImportRoot(root *xmlEl) bool {
	if root == nil {
		return false
	}
	return root.Local == "nfeProc" || root.Local == "NFe"
}

// compareImportDigests validates the SEFAZ-returned digVal (from a
// consulta protocolo response) against the uploaded XML's own digest(s):
// for nfeProc, BOTH the uploaded protNFe/infProt/digVal and the uploaded
// Signature/SignedInfo/Reference/DigestValue must match; for a bare NFe
// (no protocol yet), only the Signature DigestValue is compared. See
// docs/specs/2026-08-13-importacao-nfe-xml.md.
func compareImportDigests(root *xmlEl, sefazDigVal string) bool {
	if sefazDigVal == "" {
		return false
	}
	sigDigVal := ""
	if sig := findEl(root, nsXMLDSig, "Signature"); sig != nil {
		sigDigVal = findText(sig, nsXMLDSig, "DigestValue")
	}
	if sigDigVal == "" || sigDigVal != sefazDigVal {
		return false
	}
	if root.Local == "nfeProc" {
		uploadedProtDigVal := findText(root, nsNFe, "digVal")
		return uploadedProtDigVal != "" && uploadedProtDigVal == sefazDigVal
	}
	return true
}

// buildFinalNfeProc returns the canonical nfeProc document to persist. When
// the uploaded root is already nfeProc, originalXML is returned unchanged.
// When the uploaded root is a bare NFe (no protocol), it wraps the original
// NFe bytes verbatim (the signature depends on exact byte content — it is
// never re-serialized) together with the protNFe fragment built from
// protNFeDict (the dict a consulta protocolo response carries) via
// godfe.BuildXMLFragment, adding the nfe namespace, mirroring the reference
// file used to design this feature.
func buildFinalNfeProc(originalXML []byte, root *xmlEl, protNFeDict map[string]any) ([]byte, error) {
	if root.Local == "nfeProc" {
		return originalXML, nil
	}
	nfeBytes := bytes.TrimSpace(originalXML)
	if idx := bytes.Index(nfeBytes, []byte("?>")); bytes.HasPrefix(nfeBytes, []byte("<?xml")) && idx >= 0 {
		nfeBytes = bytes.TrimSpace(nfeBytes[idx+2:])
	}
	protFragment, err := godfe.BuildXMLFragment(protNFeDict, "protNFe", nsNFe)
	if err != nil {
		return nil, fmt.Errorf("build protNFe fragment: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?><nfeProc versao="4.00" xmlns="` + nsNFe + `">`)
	buf.Write(nfeBytes)
	buf.Write(protFragment)
	buf.WriteString(`</nfeProc>`)
	return buf.Bytes(), nil
}

// buildPersonDetails extracts the nested person object (addresses, contacts,
// state_registrations, fantasy_name, crt) from an emit/dest party element,
// mirroring the api person model. Only present fields are included; returns nil
// when the party carries nothing useful.
func buildPersonDetails(party *xmlEl, ns string) map[string]any {
	if party == nil {
		return nil
	}
	details := map[string]any{}

	if fant := findText(party, ns, "xFant"); fant != "" {
		details["fantasy_name"] = fant
	}
	if crt := findText(party, ns, "CRT"); crt != "" {
		if n, err := strconv.Atoi(crt); err == nil {
			details["crt"] = n
		}
	}

	uf := ""
	if ender := findEnderEl(party, ns); ender != nil {
		addr := map[string]any{}
		setIfNotEmpty(addr, "street", findText(ender, ns, "xLgr"))
		setIfNotEmpty(addr, "number", findText(ender, ns, "nro"))
		setIfNotEmpty(addr, "complement", findText(ender, ns, "xCpl"))
		setIfNotEmpty(addr, "neighborhood", findText(ender, ns, "xBairro"))
		setIfNotEmpty(addr, "city_ibge_code", findText(ender, ns, "cMun"))
		setIfNotEmpty(addr, "city", findText(ender, ns, "xMun"))
		uf = findText(ender, ns, "UF")
		setIfNotEmpty(addr, "state_federation", uf)
		setIfNotEmpty(addr, "postal_code", findText(ender, ns, "CEP"))
		if len(addr) > 0 {
			details["addresses"] = []any{addr}
		}
	}

	// contacts: phone from <fone> (inside the address), email from <email>.
	var phones []any
	var emails []any
	if fone := findText(party, ns, "fone"); fone != "" {
		phones = append(phones, fone)
	}
	if email := findText(party, ns, "email"); email != "" {
		emails = append(emails, email)
	}
	if len(phones) > 0 || len(emails) > 0 {
		details["contacts"] = map[string]any{"phones": phones, "emails": emails}
	}

	// state_registrations: IE paired with the address UF.
	if ie := findText(party, ns, "IE"); ie != "" {
		details["state_registrations"] = []any{map[string]any{
			"uf":                 uf,
			"state_registration": ie,
		}}
	}

	if len(details) == 0 {
		return nil
	}
	return details
}

// findEnderEl recursively finds the first child whose local name starts with
// "ender" (enderEmit, enderDest, enderReme, enderToma, ...).
func findEnderEl(el *xmlEl, space string) *xmlEl {
	if el == nil {
		return nil
	}
	if el.Space == space && strings.HasPrefix(el.Local, "ender") {
		return el
	}
	for _, c := range el.Children {
		if found := findEnderEl(c, space); found != nil {
			return found
		}
	}
	return nil
}

func setIfNotEmpty(m map[string]any, key, val string) {
	if val != "" {
		m[key] = val
	}
}

// xmlEl is an in-memory XML element tree node.
type xmlEl struct {
	Space    string
	Local    string
	Text     string
	Attrs    map[string]string // unprefixed attribute local name -> value
	Children []*xmlEl
}

func parseXMLBytes(data []byte) (*xmlEl, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	var stack []*xmlEl
	var root *xmlEl

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml decode: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			el := &xmlEl{Space: t.Name.Space, Local: t.Name.Local}
			if len(t.Attr) > 0 {
				el.Attrs = make(map[string]string, len(t.Attr))
				for _, a := range t.Attr {
					el.Attrs[a.Name.Local] = a.Value
				}
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, el)
			} else {
				root = el
			}
			stack = append(stack, el)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				s := strings.TrimSpace(string(t))
				if s != "" {
					stack[len(stack)-1].Text += s
				}
			}
		}
	}
	return root, nil
}

// findText recursively finds the first element matching (space, local) and returns its Text.
// When multiple locals are given it tries each in order, returning the first non-empty match.
func findText(el *xmlEl, space string, locals ...string) string {
	if el == nil {
		return ""
	}
	for _, local := range locals {
		if v := findTextDFS(el, space, local); v != "" {
			return v
		}
	}
	return ""
}

func findTextDFS(el *xmlEl, space, local string) string {
	if el.Space == space && el.Local == local && el.Text != "" {
		return el.Text
	}
	for _, c := range el.Children {
		if v := findTextDFS(c, space, local); v != "" {
			return v
		}
	}
	return ""
}

// findEl recursively finds the first element matching (space, local).
func findEl(el *xmlEl, space, local string) *xmlEl {
	if el == nil {
		return nil
	}
	if el.Space == space && el.Local == local {
		return el
	}
	for _, c := range el.Children {
		if found := findEl(c, space, local); found != nil {
			return found
		}
	}
	return nil
}

// findAllEls recursively collects all elements matching (space, local).
func findAllEls(el *xmlEl, space, local string) []*xmlEl {
	if el == nil {
		return nil
	}
	var result []*xmlEl
	if el.Space == space && el.Local == local {
		result = append(result, el)
	}
	for _, c := range el.Children {
		result = append(result, findAllEls(c, space, local)...)
	}
	return result
}

func decompressDocZip(text string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", fmt.Errorf("base64: %w", err)
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("gzip open: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("gzip read: %w", err)
	}
	return string(out), nil
}

func determineSchemaType(schema string) string {
	for _, s := range schemaOrder {
		if strings.Contains(schema, s) {
			return s
		}
	}
	return "unknown"
}

func parseDH(dhEmi string) (year, month, day int) {
	t, err := time.Parse(time.RFC3339, dhEmi)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", dhEmi)
	}
	if err != nil {
		now := time.Now().UTC()
		return now.Year(), int(now.Month()), now.Day()
	}
	return t.Year(), int(t.Month()), t.Day()
}

func akSerieNumber(accessKey string) (serie, number int) {
	if len(accessKey) == 44 {
		rawSerie := accessKey[22:25]
		rawNumber := accessKey[25:34]
		if s, err := strconv.Atoi(rawSerie); err == nil {
			serie = s
		} else {
			serie = 1
		}
		if n, err := strconv.Atoi(rawNumber); err == nil {
			number = n
		}
	}
	return serie, number
}

func fmtDhManifest(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	h := offset / 3600
	m := (offset % 3600) / 60
	return t.Format("2006-01-02T15:04:05") + fmt.Sprintf("%s%02d:%02d", sign, h, m)
}

// extractResNFe parses a resNFe docZip element.
func extractResNFe(root *xmlEl) DocFields {
	accessKey := findText(root, nsNFe, "chNFe")
	dhEmi := findText(root, nsNFe, "dhEmi")
	year, month, day := parseDH(dhEmi)
	serie, number := akSerieNumber(accessKey)
	now := time.Now().UTC()
	return DocFields{
		SchemaType:  SchemaResNFe,
		AccessKey:   accessKey,
		EmitCPFCNPJ: findText(root, nsNFe, "CNPJ"),
		EmitName:    findText(root, nsNFe, "xNome"),
		Total:       findText(root, nsNFe, "vNF"),
		Status:      findText(root, nsNFe, "cSitNFe"),
		DHEmi:       dhEmi,
		DigestValue: findText(root, nsNFe, "digVal"),
		Incoming:    1,
		Year:        year,
		Month:       month,
		Day:         day,
		Serie:       serie,
		Number:      number,
		CreatedAt:   now.Format(time.RFC3339Nano),
	}
}

// extractProcNFe parses a procNFe docZip element.
func extractProcNFe(root *xmlEl, cnpj string) DocFields {
	accessKey := findText(root, nsNFe, "chNFe", "Id")
	if strings.HasPrefix(accessKey, "NFe") {
		accessKey = accessKey[3:]
	}

	emit := findEl(root, nsNFe, "emit")
	dest := findEl(root, nsNFe, "dest")
	totalEl := findEl(root, nsNFe, "ICMSTot")
	prot := findEl(root, nsNFe, "infProt")
	transp := findEl(root, nsNFe, "transporta")
	ide := findEl(root, nsNFe, "ide")

	emitDoc := findText(emit, nsNFe, "CNPJ", "CPF")
	emitName := findText(emit, nsNFe, "xNome")
	destDoc := findText(dest, nsNFe, "CNPJ", "CPF")
	destName := findText(dest, nsNFe, "xNome")
	vNF := findText(totalEl, nsNFe, "vNF")
	sefazProtocol := findText(prot, nsNFe, "nProt")
	sefazStatus := findText(prot, nsNFe, "cStat")
	sefazMotive := findText(prot, nsNFe, "xMotivo")

	incoming := 1
	if transp != nil {
		transpDoc := findText(transp, nsNFe, "CNPJ", "CPF")
		if transpDoc == cnpj && destDoc != cnpj {
			incoming = 2
		}
	}

	dhEmi := findText(ide, nsNFe, "dhEmi")
	serieStr := findText(ide, nsNFe, "serie")
	numberStr := findText(ide, nsNFe, "nNF")
	year, month, day := parseDH(dhEmi)
	now := time.Now().UTC()

	serie, _ := strconv.Atoi(serieStr)
	if serie == 0 {
		serie = 1
	}
	number, _ := strconv.Atoi(numberStr)

	return DocFields{
		SchemaType:    SchemaProcNFe,
		AccessKey:     accessKey,
		Incoming:      incoming,
		EmitCPFCNPJ:   emitDoc,
		EmitName:      emitName,
		DestCPFCNPJ:   destDoc,
		DestName:      destName,
		Total:         vNF,
		SefazStatus:   sefazStatus,
		SefazMotive:   sefazMotive,
		SefazProtocol: sefazProtocol,
		Serie:         serie,
		Number:        number,
		DHEmi:         dhEmi,
		Year:          year,
		Month:         month,
		Day:           day,
		CreatedAt:     now.Format(time.RFC3339Nano),
		Products:      extractProductsNFe(root),
		Payments:      extractPaymentsNFe(root),
		EmitDetails:   buildPersonDetails(emit, nsNFe),
		DestDetails:   buildPersonDetails(dest, nsNFe),
	}
}

func extractProductsNFe(root *xmlEl) []map[string]string {
	var products []map[string]string
	for _, det := range findAllEls(root, nsNFe, "det") {
		prod := findEl(det, nsNFe, "prod")
		if prod == nil {
			continue
		}
		products = append(products, map[string]string{
			"product_code": findText(prod, nsNFe, "cProd"),
			"description":  findText(prod, nsNFe, "xProd"),
			"ncm":          findText(prod, nsNFe, "NCM"),
			"cfop":         findText(prod, nsNFe, "CFOP"),
			"unit":         findText(prod, nsNFe, "uCom"),
			"quantity":     findText(prod, nsNFe, "qCom"),
			"unit_value":   findText(prod, nsNFe, "vUnCom"),
			"discount":     orDefault(findText(prod, nsNFe, "vDesc"), "0.00"),
			"total":        findText(prod, nsNFe, "vProd"),
		})
	}
	return products
}

func extractPaymentsNFe(root *xmlEl) []map[string]string {
	var payments []map[string]string
	for _, detPag := range findAllEls(root, nsNFe, "detPag") {
		tPag := findText(detPag, nsNFe, "tPag")
		if tPag == "" {
			continue
		}
		payments = append(payments, map[string]string{
			"payment_type": tPag,
			"value":        findText(detPag, nsNFe, "vPag"),
		})
	}
	return payments
}

func extractResCTe(root *xmlEl) DocFields {
	accessKey := findText(root, nsCTe, "chCTe")
	dhEmi := findText(root, nsCTe, "dhEmi")
	year, month, day := parseDH(dhEmi)
	serie, number := akSerieNumber(accessKey)
	now := time.Now().UTC()
	return DocFields{
		SchemaType:  SchemaResCTe,
		AccessKey:   accessKey,
		EmitCPFCNPJ: findText(root, nsCTe, "CNPJ"),
		EmitName:    findText(root, nsCTe, "xNome"),
		Total:       findText(root, nsCTe, "vCT"),
		Status:      findText(root, nsCTe, "cSitCTe"),
		DHEmi:       dhEmi,
		Incoming:    1,
		Year:        year,
		Month:       month,
		Day:         day,
		Serie:       serie,
		Number:      number,
		CreatedAt:   now.Format(time.RFC3339Nano),
	}
}

func extractProcCTe(root *xmlEl) DocFields {
	accessKey := findText(root, nsCTe, "chCTe")
	prot := findEl(root, nsCTe, "infProt")
	ide := findEl(root, nsCTe, "ide")
	emit := findEl(root, nsCTe, "emit")
	toma := findEl(root, nsCTe, "toma4")
	if toma == nil {
		toma = findEl(root, nsCTe, "toma3")
	}

	emitDoc := findText(emit, nsCTe, "CNPJ", "CPF")
	emitName := findText(emit, nsCTe, "xNome")
	destDoc := findText(toma, nsCTe, "CNPJ", "CPF")
	destName := findText(toma, nsCTe, "xNome")
	vCT := findText(root, nsCTe, "vTPrest")
	sefazProtocol := findText(prot, nsCTe, "nProt")
	sefazStatus := findText(prot, nsCTe, "cStat")
	sefazMotive := findText(prot, nsCTe, "xMotivo")
	dhEmi := findText(ide, nsCTe, "dhEmi")
	serieStr := findText(ide, nsCTe, "serie")
	numberStr := findText(ide, nsCTe, "nCT")

	year, month, day := parseDH(dhEmi)
	now := time.Now().UTC()
	serie, _ := strconv.Atoi(serieStr)
	if serie == 0 {
		serie = 1
	}
	number, _ := strconv.Atoi(numberStr)

	return DocFields{
		SchemaType:    SchemaProcCTe,
		AccessKey:     accessKey,
		Incoming:      1,
		EmitCPFCNPJ:   emitDoc,
		EmitName:      emitName,
		DestCPFCNPJ:   destDoc,
		DestName:      destName,
		Total:         vCT,
		SefazStatus:   sefazStatus,
		SefazMotive:   sefazMotive,
		SefazProtocol: sefazProtocol,
		Serie:         serie,
		Number:        number,
		DHEmi:         dhEmi,
		Year:          year,
		Month:         month,
		Day:           day,
		CreatedAt:     now.Format(time.RFC3339Nano),
		EmitDetails:   buildPersonDetails(emit, nsCTe),
		DestDetails:   buildPersonDetails(toma, nsCTe),
	}
}

func extractResMDFe(root *xmlEl) DocFields {
	accessKey := findText(root, nsMDFe, "chMDFe")
	dhEmi := findText(root, nsMDFe, "dhEmi")
	year, month, day := parseDH(dhEmi)
	serie, number := akSerieNumber(accessKey)
	now := time.Now().UTC()
	return DocFields{
		SchemaType:  SchemaResMDFe,
		AccessKey:   accessKey,
		EmitCPFCNPJ: findText(root, nsMDFe, "CNPJ"),
		EmitName:    findText(root, nsMDFe, "xNome"),
		Status:      findText(root, nsMDFe, "cSitMDFe"),
		DHEmi:       dhEmi,
		Incoming:    1,
		Year:        year,
		Month:       month,
		Day:         day,
		Serie:       serie,
		Number:      number,
		CreatedAt:   now.Format(time.RFC3339Nano),
	}
}

func extractProcMDFe(root *xmlEl) DocFields {
	accessKey := findText(root, nsMDFe, "chMDFe")
	prot := findEl(root, nsMDFe, "infProt")
	ide := findEl(root, nsMDFe, "ide")
	emit := findEl(root, nsMDFe, "emit")

	emitDoc := findText(emit, nsMDFe, "CNPJ", "CPF")
	emitName := findText(emit, nsMDFe, "xNome")
	sefazProtocol := findText(prot, nsMDFe, "nProt")
	sefazStatus := findText(prot, nsMDFe, "cStat")
	sefazMotive := findText(prot, nsMDFe, "xMotivo")
	dhEmi := findText(ide, nsMDFe, "dhEmi")
	serieStr := findText(ide, nsMDFe, "serie")
	numberStr := findText(ide, nsMDFe, "nMDF")

	year, month, day := parseDH(dhEmi)
	now := time.Now().UTC()
	serie, _ := strconv.Atoi(serieStr)
	if serie == 0 {
		serie = 1
	}
	number, _ := strconv.Atoi(numberStr)

	return DocFields{
		SchemaType:    SchemaProcMDFe,
		AccessKey:     accessKey,
		Incoming:      1,
		EmitCPFCNPJ:   emitDoc,
		EmitName:      emitName,
		SefazStatus:   sefazStatus,
		SefazMotive:   sefazMotive,
		SefazProtocol: sefazProtocol,
		Serie:         serie,
		Number:        number,
		DHEmi:         dhEmi,
		Year:          year,
		Month:         month,
		Day:           day,
		CreatedAt:     now.Format(time.RFC3339Nano),
	}
}

func extractResEvento(root *xmlEl, docType string) DocFields {
	ns := nsNFe
	switch docType {
	case "cte":
		ns = nsCTe
	case "mdfe":
		ns = nsMDFe
	}
	akTag := map[string]string{"nfe": "chNFe", "cte": "chCTe", "mdfe": "chMDFe"}
	tag := akTag[docType]
	if tag == "" {
		tag = "chNFe"
	}
	return DocFields{
		SchemaType:     SchemaResEvento,
		AccessKey:      findText(root, ns, tag),
		EventType:      findText(root, ns, "tpEvento"),
		SequenceNumber: findText(root, ns, "nSeqEvento"),
		SefazStatus:    findText(root, ns, "cStat"),
		SefazMotive:    findText(root, ns, "xMotivo"),
		SefazProtocol:  findText(root, ns, "nProt"),
		DHEvento:       findText(root, ns, "dhEvento"),
	}
}

// extractDoc dispatches to the correct extractor based on schema type.
func extractDoc(schemaType string, root *xmlEl, docType, cnpj string) DocFields {
	switch schemaType {
	case SchemaResNFe:
		return extractResNFe(root)
	case SchemaProcNFe:
		return extractProcNFe(root, cnpj)
	case SchemaResCTe:
		return extractResCTe(root)
	case SchemaProcCTe:
		return extractProcCTe(root)
	case SchemaResMDFe:
		return extractResMDFe(root)
	case SchemaProcMDFe:
		return extractProcMDFe(root)
	case SchemaResEvento, SchemaProcEventoNFe, SchemaProcEventoCTe, SchemaProcEventoMDFe:
		return extractResEvento(root, docType)
	default:
		return DocFields{}
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
