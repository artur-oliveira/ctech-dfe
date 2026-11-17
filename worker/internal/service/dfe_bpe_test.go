package service

import (
	"context"
	"testing"
)

func bpeMsg(variantService string) WorkerMessage {
	m := baseMsg
	m.DocType = docTypeBPe
	m.TableName = "bpes"
	m.S3Prefix = "bpe"
	m.SefazService = variantService
	return m
}

// As três variantes seguem o caminho genérico: cStat 100 com protocolo grava na tabela bpes.
func TestProcess_BPeEmission_AllVariants(t *testing.T) {
	for _, svcName := range []string{"BPeRecepcao", "BPeRecepcaoTM", "BPeRecepcaoTA"} {
		dynm := &mockDynamo{}
		svc := newSvc(certS3(), &mockLambda{payload: invokeResp("100", "Autorizado o uso do BP-e", "135260000000001")}, dynm)
		if err := svc.Process(context.Background(), bpeMsg(svcName)); err != nil {
			t.Fatalf("%s: %v", svcName, err)
		}
		if len(dynm.updates) != 1 || dynm.updates[0].table != "dev_bpes" || dynm.updates[0].status != StatusAuthorized {
			t.Fatalf("%s: updates = %+v", svcName, dynm.updates)
		}
	}
}

// cStat 100 sem protocolo não autoriza o BP-e: sem nProt o cancelamento nunca seria possível.
func TestProcess_BPeAuthorizedWithoutProtocolIsNotSaved(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("100", "Autorizado o uso do BP-e", "")}, dynm)
	if err := svc.Process(context.Background(), bpeMsg("BPeRecepcao")); err == nil {
		t.Fatal("esperava erro retentável para cStat 100 sem protocolo")
	}
	for _, u := range dynm.updates {
		if u.status == StatusAuthorized {
			t.Fatalf("BP-e sem protocolo foi marcado autorizado: %+v", u)
		}
	}
}

// Cancelamento 110111 homologado cancela o bilhete; não embarque 110115 só marca o evento (o documento segue autorizado).
func TestProcess_BPeEvents(t *testing.T) {
	cancel := cancelMsg
	cancel.DocType, cancel.TableName, cancel.S3Prefix = docTypeBPe, "bpes", "bpe"
	cancel.SefazService, cancel.EventsTableName = "BPeRecepcaoEvento", new("bpe_events")
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("135", "Evento registrado e vinculado ao BP-e", "prot-1")}, dynm)
	if err := svc.Process(context.Background(), cancel); err != nil {
		t.Fatal(err)
	}
	if len(dynm.updates) != 2 || dynm.updates[0].table != "dev_bpes" || dynm.updates[0].status != StatusCancelled ||
		dynm.updates[1].table != "dev_bpe_events" || dynm.updates[1].status != EventStatusSuccess {
		t.Fatalf("cancelamento: %+v", dynm.updates)
	}

	noShow := cancel
	noShow.EventType = new("110115")
	dynm = &mockDynamo{}
	svc = newSvc(certS3(), &mockLambda{payload: invokeResp("135", "Evento registrado e vinculado ao BP-e", "prot-2")}, dynm)
	if err := svc.Process(context.Background(), noShow); err != nil {
		t.Fatal(err)
	}
	for _, u := range dynm.updates {
		if u.table == "dev_bpes" {
			t.Fatalf("não embarque não deve alterar o documento: %+v", u)
		}
	}
}

func TestConsultaSpecs_BPe(t *testing.T) {
	got, ok := consultaSpecs[docTypeBPe]
	if !ok {
		t.Fatal("BP-e sem consultaSpec: a duplicidade 204 não seria reconciliada")
	}
	want := consultaSpec{"BPeConsulta", "consSitBPe", "1.00", nsBPe, "chBPe", "protBPe", "retConsSitBPe"}
	if got != want {
		t.Errorf("consultaSpec = %+v, want %+v", got, want)
	}
}
