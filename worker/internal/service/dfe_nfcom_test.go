package service

import (
	"context"
	"testing"
)

// A NFCom segue o caminho genérico do DfeService: a tabela vem da mensagem e cStat 100 autoriza.
func TestProcess_NFComEmission_UsesNFComTable(t *testing.T) {
	dynm := &mockDynamo{}
	msg := baseMsg
	msg.DocType = docTypeNFCom
	msg.TableName = "nfcoms"
	msg.S3Prefix = "nfcom"
	msg.SefazService = "NFComRecepcao"
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("100", "Autorizado o uso da NFCom", "135260000000001")}, dynm)

	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(dynm.updates) != 1 || dynm.updates[0].table != "dev_nfcoms" || dynm.updates[0].status != StatusAuthorized {
		t.Fatalf("updates = %+v", dynm.updates)
	}
}

// Cancelamento 110111 homologado (135) cancela a NFCom e marca o evento com sucesso.
func TestProcess_NFComCancellation_MarksDocCancelled(t *testing.T) {
	dynm := &mockDynamo{}
	msg := cancelMsg
	msg.DocType = docTypeNFCom
	msg.TableName = "nfcoms"
	msg.S3Prefix = "nfcom"
	msg.SefazService = "NFComRecepcaoEvento"
	msg.EventsTableName = new("nfcom_events")
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("135", "Evento registrado e vinculado a NFCom", "prot-1")}, dynm)

	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(dynm.updates) != 2 {
		t.Fatalf("updates = %+v", dynm.updates)
	}
	if dynm.updates[0].table != "dev_nfcoms" || dynm.updates[0].status != StatusCancelled {
		t.Errorf("doc update = %+v", dynm.updates[0])
	}
	if dynm.updates[1].table != "dev_nfcom_events" || dynm.updates[1].status != EventStatusSuccess {
		t.Errorf("event update = %+v", dynm.updates[1])
	}
}

// Uma duplicidade de autorização (204) da NFCom é reconciliada pela consulta de situação do próprio documento.
func TestConsultaSpecs_NFCom(t *testing.T) {
	got, ok := consultaSpecs[docTypeNFCom]
	if !ok {
		t.Fatal("NFCom sem consultaSpec: a duplicidade 204 não seria reconciliada")
	}
	want := consultaSpec{"NFComConsulta", "consSitNFCom", "1.00", nsNFCom, "chNFCom", "protNFCom", "retConsSitNFCom"}
	if got != want {
		t.Errorf("consultaSpec = %+v, want %+v", got, want)
	}
}

// cStat 100 sem protocolo não autoriza a NFCom (sem nProt o cancelamento nunca seria possível): erro retentável.
func TestProcess_NFComAuthorizedWithoutProtocolIsNotSaved(t *testing.T) {
	dynm := &mockDynamo{}
	msg := baseMsg
	msg.DocType = docTypeNFCom
	msg.TableName = "nfcoms"
	msg.S3Prefix = "nfcom"
	msg.SefazService = "NFComRecepcao"
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("100", "Autorizado o uso da NFCom", "")}, dynm)

	if err := svc.Process(context.Background(), msg); err == nil {
		t.Fatal("esperava erro retentável para cStat 100 sem protocolo")
	}
	for _, u := range dynm.updates {
		if u.status == StatusAuthorized {
			t.Fatalf("NFCom sem protocolo foi marcada autorizada: %+v", u)
		}
	}
}
