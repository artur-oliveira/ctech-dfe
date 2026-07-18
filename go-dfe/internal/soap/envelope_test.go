package soap

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"testing"
)

const samplePayload = `<consStatServ versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe"><tpAmb>2</tpAmb></consStatServ>`

// syntheticResponse builds a minimal SOAP 1.2 response envelope wrapping
// innerXML in resultElem, the way SEFAZ's real responses wrap their payload
// in the doc_type's Result element (constants.SOAPElements[docType].Result).
func syntheticResponse(resultElem, innerXML string) []byte {
	return []byte(`<soap12:Envelope xmlns:soap12="` + soap12NS + `">` +
		`<soap12:Body><` + resultElem + ` xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NfeStatusServico4">` +
		innerXML + `</` + resultElem + `></soap12:Body></soap12:Envelope>`)
}

func TestBuild_UnwrappedBodyService(t *testing.T) {
	b, err := NewBuilder("nfe", "SP", "NfeStatusServico")
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	envelope, err := b.Build([]byte(samplePayload), false, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want := `<nfeDadosMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeStatusServico4">` + samplePayload + `</nfeDadosMsg>`
	if !bytes.Contains(envelope, []byte(want)) {
		t.Fatalf("envelope missing unwrapped body %s, got: %s", want, envelope)
	}
	if bytes.Contains(envelope, []byte("soap12:Header")) {
		t.Fatalf("envelope should have no header when includeHeader=false, got: %s", envelope)
	}

	ct := b.ContentType()
	if !bytes.Contains([]byte(ct), []byte("NFeStatusServico4")) || !bytes.Contains([]byte(ct), []byte("nfeStatusServicoNF")) {
		t.Fatalf("unexpected content type: %s", ct)
	}

	// Parse round trip: a synthetic SEFAZ response wrapping retConsStatServ in
	// nfeResultMsg (constants.SOAPElements["nfe"].Result) should unwrap back
	// to that element.
	inner := `<retConsStatServ versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe"><cStat>107</cStat></retConsStatServ>`
	resp := syntheticResponse("nfeResultMsg", inner)

	result, err := ParseResult("nfe", resp)
	if err != nil {
		t.Fatalf("ParseResult: %v", err)
	}
	if !bytes.Contains(result, []byte("nfeResultMsg")) || !bytes.Contains(result, []byte(inner)) {
		t.Fatalf("ParseResult did not return the nfeResultMsg element, got: %s", result)
	}
}

func TestBuild_GzipPayload(t *testing.T) {
	b, err := NewBuilder("nfe", "SP", "NfeStatusServico")
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	envelope, err := b.Build([]byte(samplePayload), true, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	start := bytes.Index(envelope, []byte(`nfeDadosMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeStatusServico4">`))
	if start == -1 {
		t.Fatalf("nfeDadosMsg open tag not found: %s", envelope)
	}
	start += len(`nfeDadosMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeStatusServico4">`)
	end := bytes.Index(envelope[start:], []byte(`</nfeDadosMsg>`))
	if end == -1 {
		t.Fatalf("nfeDadosMsg close tag not found: %s", envelope)
	}
	encoded := envelope[start : start+end]

	gzBytes, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(gzBytes))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	decompressed, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}
	if string(decompressed) != samplePayload {
		t.Fatalf("gzip round trip mismatch: got %q want %q", decompressed, samplePayload)
	}
}

func TestBuild_WrappedBodyService(t *testing.T) {
	b, err := NewBuilder("nfe", "SP", "NFeDistribuicaoDFe")
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	envelope, err := b.Build([]byte(samplePayload), false, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ns := "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe"
	want := `<nfeDistDFeInteresse xmlns="` + ns + `"><nfeDadosMsg>` + samplePayload + `</nfeDadosMsg></nfeDistDFeInteresse>`
	if !bytes.Contains(envelope, []byte(want)) {
		t.Fatalf("envelope missing wrapped body %s, got: %s", want, envelope)
	}

	ct := b.ContentType()
	if !bytes.Contains([]byte(ct), []byte("nfeDistDFeInteresse")) {
		t.Fatalf("unexpected content type: %s", ct)
	}

	inner := `<retDistDFeInt versao="1.01" xmlns="http://www.portalfiscal.inf.br/nfe"><cStat>138</cStat></retDistDFeInt>`
	resp := syntheticResponse("nfeResultMsg", inner)
	result, err := ParseResult("nfe", resp)
	if err != nil {
		t.Fatalf("ParseResult: %v", err)
	}
	if !bytes.Contains(result, []byte(inner)) {
		t.Fatalf("ParseResult did not return inner payload, got: %s", result)
	}
}

func TestBuild_MTNfeConsultaCadastroOverride(t *testing.T) {
	b, err := NewBuilder("nfe", "MT", "NfeConsultaCadastro")
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	envelope, err := b.Build([]byte(samplePayload), false, true)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ns := "http://www.portalfiscal.inf.br/nfe/wsdl/CadConsultaCadastro4"
	wantBody := `<consultaCadastro xmlns="` + ns + `"><nfeDadosMsg>` + samplePayload + `</nfeDadosMsg></consultaCadastro>`
	if !bytes.Contains(envelope, []byte(wantBody)) {
		t.Fatalf("envelope missing MT wrapped body override %s, got: %s", wantBody, envelope)
	}

	// SOAPHeaderVersionOverride["NfeConsultaCadastro"] = "2.00" must win over
	// SchemaVersion["nfe"] = "4.00", and cUF must be MT's IBGE code (51).
	wantHeader := `<nfeCabecMsg xmlns="` + ns + `"><cUF>51</cUF><versaoDados>2.00</versaoDados></nfeCabecMsg>`
	if !bytes.Contains(envelope, []byte(wantHeader)) {
		t.Fatalf("envelope missing header with version override %s, got: %s", wantHeader, envelope)
	}
}

func TestNewBuilder_UnknownServiceErrors(t *testing.T) {
	if _, err := NewBuilder("nfe", "SP", "UnknownService"); err == nil {
		t.Fatal("expected error for unknown service, got nil")
	}
}

func TestParseResult_MissingBodyErrors(t *testing.T) {
	if _, err := ParseResult("nfe", []byte(`<root><other/></root>`)); err == nil {
		t.Fatal("expected error for missing SOAP Body, got nil")
	}
}

func TestParseResult_FallsBackToFirstChildWhenResultNameAbsent(t *testing.T) {
	// No element named "nfeResultMsg" anywhere in the response — ParseResult
	// should fall back to Body's first child, mirroring py-dfe's extract_body.
	resp := []byte(`<soap12:Envelope xmlns:soap12="` + soap12NS + `">` +
		`<soap12:Body><someOtherWrapper><cStat>107</cStat></someOtherWrapper></soap12:Body></soap12:Envelope>`)

	result, err := ParseResult("nfe", resp)
	if err != nil {
		t.Fatalf("ParseResult: %v", err)
	}
	if !bytes.Contains(result, []byte("someOtherWrapper")) {
		t.Fatalf("expected fallback to first child, got: %s", result)
	}
}
