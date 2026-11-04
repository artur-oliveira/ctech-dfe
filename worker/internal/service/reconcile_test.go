package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const (
	dupDigest   = "abc123digest="
	dupSigned   = `<mdfeProc versao="3.00" xmlns="http://www.portalfiscal.inf.br/mdfe"><MDFe><Signature><DigestValue>` + dupDigest + `</DigestValue></Signature></MDFe><protMDFe versao="3.00"><infProt><cStat>204</cStat><xMotivo>Duplicidade de MDFe</xMotivo></infProt></protMDFe></mdfeProc>`
	dupProtKept = `<nProt>135260000000001</nProt>`
)

func bodyPayload(body map[string]any) []byte {
	b, _ := json.Marshal(body)
	p, _ := json.Marshal(map[string]any{"statusCode": 200, "body": string(b)})
	return p
}

func duplicateAuthMsg() WorkerMessage {
	m := baseMsg
	m.DocType = docTypeMDFe
	m.TableName = "mdfes"
	m.S3Prefix = "mdfe"
	m.SefazService = "MDFeRecepcaoSinc"
	m.AccessKey = "35250512345678000195580010000000011000000019"
	return m
}

func authorizationDuplicated() []byte {
	return bodyPayload(map[string]any{
		"cStat": "204", "xMotivo": "Duplicidade de MDFe", "nProt": "",
		"@xml": dupSigned,
	})
}

func consultaAuthorized(digVal string) []byte {
	return bodyPayload(map[string]any{"retConsSitMDFe": map[string]any{
		"cStat": "100", "xMotivo": "Autorizado o uso do MDF-e",
		"protMDFe": map[string]any{"@versao": "3.00", "infProt": map[string]any{
			"cStat": "100", "xMotivo": "Autorizado o uso do MDF-e",
			"nProt": "135260000000001", "digVal": digVal,
		}},
	}})
}

// 204 com a consulta confirmando o mesmo digest: o documento vira autorizado com o
// protocolo vigente em vez de rejeitado.
func TestProcess_Duplicate204_ReconciledAsAuthorized(t *testing.T) {
	dynm := &mockDynamo{}
	lam := &mockLambda{payloads: [][]byte{authorizationDuplicated(), consultaAuthorized(dupDigest)}}
	svc := newSvc(certS3(), lam, dynm)

	if err := svc.Process(context.Background(), duplicateAuthMsg()); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lam.calls != 2 {
		t.Fatalf("egress calls = %d, want 2 (autorização + consulta)", lam.calls)
	}
	if got := dynm.updates[len(dynm.updates)-1].status; got != StatusAuthorized {
		t.Fatalf("status = %q, want authorized", got)
	}
}

// Digest diferente: outro documento ocupa a chave; segue rejeitado.
func TestProcess_Duplicate204_DigestMismatchStaysRejected(t *testing.T) {
	dynm := &mockDynamo{}
	lam := &mockLambda{payloads: [][]byte{authorizationDuplicated(), consultaAuthorized("outro-digest=")}}
	svc := newSvc(certS3(), lam, dynm)

	if err := svc.Process(context.Background(), duplicateAuthMsg()); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got := dynm.updates[len(dynm.updates)-1].status; got != StatusRejected {
		t.Fatalf("status = %q, want rejected", got)
	}
}

// Consulta que não devolve autorizado (ex.: 217 não consta na base): segue rejeitado.
func TestProcess_Duplicate204_NotAuthorizedStaysRejected(t *testing.T) {
	dynm := &mockDynamo{}
	notFound := bodyPayload(map[string]any{"retConsSitMDFe": map[string]any{"cStat": "217", "xMotivo": "MDF-e nao consta na base"}})
	lam := &mockLambda{payloads: [][]byte{authorizationDuplicated(), notFound}}
	svc := newSvc(certS3(), lam, dynm)

	if err := svc.Process(context.Background(), duplicateAuthMsg()); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got := dynm.updates[len(dynm.updates)-1].status; got != StatusRejected {
		t.Fatalf("status = %q, want rejected", got)
	}
}

// 539 (chave diferente) não reconcilia: uma só chamada ao egress, rejeitado.
func TestProcess_Duplicate539_NoReconcile(t *testing.T) {
	dynm := &mockDynamo{}
	lam := &mockLambda{payload: invokeResp("539", "Duplicidade de MDFe, com diferença na Chave de Acesso", "")}
	svc := newSvc(certS3(), lam, dynm)

	if err := svc.Process(context.Background(), duplicateAuthMsg()); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lam.calls != 1 {
		t.Fatalf("egress calls = %d, want 1", lam.calls)
	}
	if got := dynm.updates[len(dynm.updates)-1].status; got != StatusRejected {
		t.Fatalf("status = %q, want rejected", got)
	}
}

// O processado sai com o protocolo vigente no lugar do 204 e o documento intacto.
func TestReconcileDuplicate_RebuildsProcWithCurrentProtocol(t *testing.T) {
	var respBody map[string]any
	_ = json.Unmarshal([]byte(`{"cStat":"204","@xml":`+quote(dupSigned)+`}`), &respBody)
	lam := &mockLambda{payload: consultaAuthorized(dupDigest)}
	svc := newSvc(certS3(), lam, &mockDynamo{})

	rec, ok, err := svc.reconcileDuplicateAuthorization(context.Background(), duplicateAuthMsg(), respBody)
	if err != nil || !ok {
		t.Fatalf("reconcile ok=%v err=%v", ok, err)
	}
	proc := rec.respBody["@xml"].(string)
	if strings.Contains(proc, "<cStat>204</cStat>") || !strings.Contains(proc, dupProtKept) {
		t.Fatalf("protocolo não trocado: %s", proc)
	}
	if !strings.Contains(proc, "<MDFe><Signature><DigestValue>"+dupDigest+"</DigestValue></Signature></MDFe>") {
		t.Fatalf("documento assinado foi alterado: %s", proc)
	}
	if rec.cStat != "100" || rec.nProt != "135260000000001" {
		t.Fatalf("rec = %+v", rec)
	}
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }
