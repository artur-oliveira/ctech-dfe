package service

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/worker/internal/config"
)

func TestParseNfseResponse_Authorized(t *testing.T) {
	out := parseNfseResponse(map[string]any{
		fieldChaveAcesso: "99999999999999999999999999999999999999999999999999",
		fieldIDDPS:       "DPS123",
		fieldNfseXML:     "<NFSe/>",
		fieldDpsXML:      "<DPS/>",
	})
	if out.Status != StatusAuthorized {
		t.Errorf("Status = %q, esperado %q", out.Status, StatusAuthorized)
	}
	if out.AccessKey == "" || out.NFSeXML == "" || out.DPSXML == "" {
		t.Errorf("campos perdidos: %+v", out)
	}
}

// TestHandleNfseResponse_AuthorizedWithoutXMLFailsClosed garante que o status
// authorized nunca seja persistido sem o documento fiscal válido no S3.
func TestHandleNfseResponse_AuthorizedWithoutXMLFailsClosed(t *testing.T) {
	s3m, dynm := &mockS3{}, &mockDynamo{}
	svc := New(Clients{S3: s3m, Dynamo: dynm}, testCfg)

	err := svc.handleNfseResponse(context.Background(), nfseMsg, map[string]any{
		fieldChaveAcesso: "99999999999999999999999999999999999999999999999999",
		fieldDpsXML:      "<DPS/>",
	})
	if !errors.Is(err, errNfseAuthorizedWithoutXML) {
		t.Fatalf("erro = %v, esperado %v", err, errNfseAuthorizedWithoutXML)
	}
	if len(s3m.putCalls) != 0 {
		t.Errorf("não deve gravar artefatos sem NFS-e válida, gravou %v", s3m.putCalls)
	}
	if len(dynm.updates) != 0 {
		t.Errorf("não deve autorizar documento sem XML, updates = %+v", dynm.updates)
	}
}

func TestHandleNfseResponse_AuthorizedWithoutDPSFailsClosed(t *testing.T) {
	s3m, dynm := &mockS3{}, &mockDynamo{}
	svc := New(Clients{S3: s3m, Dynamo: dynm}, testCfg)

	err := svc.handleNfseResponse(context.Background(), nfseMsg, map[string]any{
		fieldChaveAcesso: "99999999999999999999999999999999999999999999999999",
		fieldNfseXML:     "<NFSe/>",
	})
	if !errors.Is(err, errNfseAuthorizedWithoutDPS) {
		t.Fatalf("erro = %v, esperado %v", err, errNfseAuthorizedWithoutDPS)
	}
	if len(s3m.putCalls) != 0 {
		t.Errorf("não deve gravar artefato parcial, gravou %v", s3m.putCalls)
	}
	if len(dynm.updates) != 0 {
		t.Errorf("não deve autorizar documento sem DPS, updates = %+v", dynm.updates)
	}
}

func TestHandleNfseResponse_XMLUploadFailureDoesNotAuthorize(t *testing.T) {
	s3m, dynm := &mockS3{putErr: errors.New("s3 unavailable")}, &mockDynamo{}
	svc := New(Clients{S3: s3m, Dynamo: dynm}, testCfg)

	err := svc.handleNfseResponse(context.Background(), nfseMsg, map[string]any{
		fieldChaveAcesso: "99999999999999999999999999999999999999999999999999",
		fieldNfseXML:     "<NFSe/>",
		fieldDpsXML:      "<DPS/>",
	})
	if err == nil {
		t.Fatal("falha do S3 foi ignorada")
	}
	if len(dynm.updates) != 0 {
		t.Errorf("não deve autorizar sem persistir XML, updates = %+v", dynm.updates)
	}
}

// TestParseNfseResponse_RejectedIsTerminal: o ADN pode devolver 200 com a lista
// "erros" preenchida. É rejeição de negócio — terminal, nunca retry — e o
// código do fisco tem que sobreviver até o motivo gravado.
func TestParseNfseResponse_RejectedIsTerminal(t *testing.T) {
	out := parseNfseResponse(map[string]any{
		fieldErros: []any{map[string]any{"codigo": "E123", "descricao": "cTribNac inválido"}},
	})
	if out.Status != StatusRejected {
		t.Errorf("Status = %q, esperado %q", out.Status, StatusRejected)
	}
	if out.Motivo != "E123 - cTribNac inválido" {
		t.Errorf("Motivo = %q, o código e a descrição do fisco têm que ser preservados", out.Motivo)
	}
}

func TestParseNfseResponse_Event(t *testing.T) {
	out := parseNfseResponse(map[string]any{fieldEventoXML: "<evento/>"})
	if out.EventoXML != "<evento/>" {
		t.Errorf("EventoXML = %q", out.EventoXML)
	}
	if out.Status != StatusAuthorized {
		t.Errorf("evento aceito é terminal de sucesso, veio %q", out.Status)
	}
}

func TestIsNfse(t *testing.T) {
	if !isNfse(docTypeNfse) {
		t.Error("isNfse(nfse) = false")
	}
	if isNfse("nfe") {
		t.Error("isNfse(nfe) = true")
	}
}

// TestIsCancellationEvent_Nfse: o código de cancelamento da NFS-e (101101) não
// é o da NF-e (110111). Sem esta regra o documento ficaria "authorized" depois
// de um cancelamento aceito pelo fisco.
func TestIsCancellationEvent_Nfse(t *testing.T) {
	cases := []struct {
		docType, eventType string
		want               bool
	}{
		{docTypeNfse, nfseCancellationEvent, true},
		{docTypeNfse, "101103", false},
		{"nfe", nfseCancellationEvent, false},
		{"nfe", cancellationEvent, true},
	}
	for _, c := range cases {
		if got := isCancellationEvent(c.docType, &c.eventType); got != c.want {
			t.Errorf("isCancellationEvent(%q, %q) = %v, want %v", c.docType, c.eventType, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// handleNfseResponse — persistência
// ---------------------------------------------------------------------------

var nfseMsg = WorkerMessage{
	DocPK:            "prod#CNPJ_12345678000195",
	AccessKey:        "3550308123456780001952024000000001", // id_dps, não a chave
	ExpectedFileName: "3550308123456780001952024000000001",
	TableName:        "nfses",
	S3Prefix:         "nfse",
	CNPJ:             "12345678000195",
	DocType:          docTypeNfse,
	SefazService:     "recepcao",
}

func strAttrValue(u capturedUpdate, name string) string {
	v, ok := u.values[":"+name].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return v.Value
}

// TestHandleNfseResponse_Document: a SK da linha é o id_dps; a chave de acesso
// devolvida pelo fisco entra como atributo. Trocar os dois cria item órfão.
func TestHandleNfseResponse_Document(t *testing.T) {
	s3m, dynm, snsm := &mockS3{}, &mockDynamo{}, &mockSNS{}
	svc := New(Clients{S3: s3m, Dynamo: dynm, SNS: snsm}, &config.Config{
		TablePrefix: "dev", DocumentsBucket: "docs-bucket", ResultsTopicARN: "arn:topic",
	})

	err := svc.handleNfseResponse(context.Background(), nfseMsg, map[string]any{
		fieldChaveAcesso: "99999999999999999999999999999999999999999999999999",
		fieldNfseXML:     "<NFSe/>",
		fieldDpsXML:      "<DPS/>",
	})
	if err != nil {
		t.Fatalf("handleNfseResponse: %v", err)
	}

	wantKeys := []string{
		"nfse/prod/CNPJ_12345678000195/3550308123456780001952024000000001.xml",
		"nfse/prod/CNPJ_12345678000195/3550308123456780001952024000000001_dps.xml",
	}
	if len(s3m.putCalls) != len(wantKeys) {
		t.Fatalf("puts = %v, esperado %v", s3m.putCalls, wantKeys)
	}
	for i, want := range wantKeys {
		if s3m.putCalls[i] != want {
			t.Errorf("put %d = %q, want %q", i, s3m.putCalls[i], want)
		}
	}

	if len(dynm.updates) != 1 {
		t.Fatalf("updates = %d, esperado 1", len(dynm.updates))
	}
	u := dynm.updates[0]
	if u.table != "dev_nfses" || u.status != StatusAuthorized {
		t.Errorf("update = %+v", u)
	}
	if got := strAttrValue(u, "access_key"); got != "99999999999999999999999999999999999999999999999999" {
		t.Errorf("access_key = %q", got)
	}
	if got := strAttrValue(u, "xml_s3_key"); got != wantKeys[0] {
		t.Errorf("xml_s3_key = %q", got)
	}
	if got := strAttrValue(u, "dps_xml_s3_key"); got != wantKeys[1] {
		t.Errorf("dps_xml_s3_key = %q", got)
	}
	if len(snsm.calls) != 1 {
		t.Errorf("notificações = %d, esperado 1", len(snsm.calls))
	}
}

// TestHandleNfseResponse_RejectedNoUpload: rejeição de negócio é terminal e não
// gera XML nenhum — só status e motivo.
func TestHandleNfseResponse_RejectedNoUpload(t *testing.T) {
	s3m, dynm := &mockS3{}, &mockDynamo{}
	svc := New(Clients{S3: s3m, Dynamo: dynm}, testCfg)

	err := svc.handleNfseResponse(context.Background(), nfseMsg, map[string]any{
		fieldErros: []any{map[string]any{fieldCodigo: "E123", fieldDescricao: "cTribNac inválido"}},
	})
	if err != nil {
		t.Fatalf("handleNfseResponse: %v", err)
	}
	if len(s3m.putCalls) != 0 {
		t.Errorf("rejeição não deve gravar XML, gravou %v", s3m.putCalls)
	}
	if len(dynm.updates) != 1 || dynm.updates[0].status != StatusFailed {
		t.Fatalf("updates = %+v, esperado status %q", dynm.updates, StatusFailed)
	}
	if got := strAttrValue(dynm.updates[0], "sefaz_motive"); got != "E123 - cTribNac inválido" {
		t.Errorf("sefaz_motive = %q", got)
	}
}

// TestHandleNfseResponse_CancellationEvent: cancelamento aceito grava o XML do
// evento e reverte a NFS-e para cancelled.
func TestHandleNfseResponse_CancellationEvent(t *testing.T) {
	s3m, dynm, snsm := &mockS3{}, &mockDynamo{}, &mockSNS{}
	svc := New(Clients{S3: s3m, Dynamo: dynm, SNS: snsm}, &config.Config{
		TablePrefix: "dev", DocumentsBucket: "docs-bucket", ResultsTopicARN: "arn:topic",
	})

	msg := nfseMsg
	msg.ExpectedFileName = nfseMsg.AccessKey + "_101101_001"
	msg.EventsTableName = strPtr("nfse_events")
	msg.EventType = strPtr(nfseCancellationEvent)
	msg.EventSK = strPtr("EV01")

	if err := svc.handleNfseResponse(context.Background(), msg, map[string]any{
		fieldEventoXML: "<evento/>",
	}); err != nil {
		t.Fatalf("handleNfseResponse: %v", err)
	}

	wantKey := "nfse/prod/CNPJ_12345678000195/3550308123456780001952024000000001_101101_001.xml"
	if len(s3m.putCalls) != 1 || s3m.putCalls[0] != wantKey {
		t.Fatalf("puts = %v, esperado [%s]", s3m.putCalls, wantKey)
	}
	if len(dynm.updates) != 2 {
		t.Fatalf("updates = %+v, esperado documento + evento", dynm.updates)
	}
	if dynm.updates[0].table != "dev_nfses" || dynm.updates[0].status != StatusCancelled {
		t.Errorf("documento = %+v, esperado dev_nfses/%s", dynm.updates[0], StatusCancelled)
	}
	if dynm.updates[1].table != "dev_nfse_events" || dynm.updates[1].status != EventStatusSuccess {
		t.Errorf("evento = %+v", dynm.updates[1])
	}
	if len(snsm.calls) != 1 {
		t.Errorf("notificações = %d, esperado 1 (do evento)", len(snsm.calls))
	}
}
