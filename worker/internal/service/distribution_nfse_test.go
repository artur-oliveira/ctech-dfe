package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func TestParseNfseDistResponse_LoteComDocumentos(t *testing.T) {
	batch := parseNfseDistResponse(map[string]any{
		"status_distribuicao": "DOCUMENTOS_LOCALIZADOS",
		"distribuicao": []any{
			map[string]any{
				"nsu": float64(11), "chave_acesso": "3526",
				"tipo_documento": "NFSE", "xml": "<NFSe/>",
			},
			map[string]any{
				"nsu": float64(12), "chave_acesso": "3526",
				"tipo_documento": "EVENTO", "tipo_evento": "101101", "xml": "<evento/>",
			},
		},
	})

	if batch.Status != "DOCUMENTOS_LOCALIZADOS" {
		t.Errorf("Status = %q", batch.Status)
	}
	if len(batch.Items) != 2 {
		t.Fatalf("Items = %d, esperado 2", len(batch.Items))
	}
	if batch.Items[0].NSU != 11 || batch.Items[1].NSU != 12 {
		t.Errorf("NSUs = %d, %d", batch.Items[0].NSU, batch.Items[1].NSU)
	}
	if batch.Items[1].TipoEvento != "101101" {
		t.Errorf("TipoEvento = %q", batch.Items[1].TipoEvento)
	}
}

func TestParseNfseDistResponse_LoteVazio(t *testing.T) {
	batch := parseNfseDistResponse(map[string]any{
		"status_distribuicao": "NENHUM_DOCUMENTO_LOCALIZADO",
	})
	if len(batch.Items) != 0 {
		t.Errorf("lote vazio veio com %d itens", len(batch.Items))
	}
}

func TestMaxNSUOf(t *testing.T) {
	if got := maxNSUOf([]nfseDistItem{{NSU: 11}, {NSU: 43}, {NSU: 27}}); got != 43 {
		t.Errorf("maxNSUOf = %d, esperado 43", got)
	}
	if got := maxNSUOf(nil); got != 0 {
		t.Errorf("maxNSUOf(nil) = %d, esperado 0", got)
	}
}

func TestBuildNfseDistPayload(t *testing.T) {
	p := buildNfseDistPayload("12345678000199", "Y2VydA==", "senha", sefazEnvHom, "nacional", 42)

	if p["doc_type"] != docTypeNfse {
		t.Errorf("doc_type = %v", p["doc_type"])
	}
	if p["service"] != serviceNFSeDistribuicao {
		t.Errorf("service = %v", p["service"])
	}
	if p["uf"] != "" {
		t.Errorf("uf deveria ser vazia em NFS-e, veio %v", p["uf"])
	}
	body, ok := p["body"].(map[string]any)
	if !ok {
		t.Fatalf("body não é map: %T", p["body"])
	}
	if _, exists := body["distDFeInt"]; exists {
		t.Error("payload de NFS-e não pode conter distDFeInt (não é SOAP)")
	}
	if body["nsu"] != int64(42) {
		t.Errorf("nsu = %v", body["nsu"])
	}
	if body["cnpj_consulta"] != "12345678000199" {
		t.Errorf("cnpj_consulta = %v", body["cnpj_consulta"])
	}
	if body["provider"] != "nacional" {
		t.Errorf("provider = %v", body["provider"])
	}
}

// TestMapToDfeRequest_NfseSemUF: NFS-e é competência municipal e viaja sem UF
// (api/internal/services/nfses/emit.go). Se o guard de UF vazia derrubasse o
// payload, a distribuição cairia no py-dfe, que não implementa NFS-e.
func TestMapToDfeRequest_NfseSemUF(t *testing.T) {
	req, ok := mapToDfeRequest(buildNfseDistPayload("12345678000199", "Y2VydA==", "senha", sefazEnvHom, "nacional", 1))
	if !ok {
		t.Fatal("payload de NFS-e rejeitado por mapToDfeRequest")
	}
	if req.Service != serviceNFSeDistribuicao {
		t.Errorf("Service = %q", req.Service)
	}
	if _, ok := mapToDfeRequest(map[string]any{
		"doc_type": "nfe", "service": "NFeDistribuicaoDFe", "body": map[string]any{},
	}); ok {
		t.Error("NF-e sem UF deveria continuar sendo rejeitada")
	}
}

// ---------------------------------------------------------------------------
// runNfseDistNSU
// ---------------------------------------------------------------------------

// nfseConfigItem monta o item de organization_nfse_configs com o cursor de NSU.
func nfseConfigItem(provider string, nsu int) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: testOrgPK},
		"environment": &types.AttributeValueMemberN{Value: "2"},
		"provider":    &types.AttributeValueMemberS{Value: provider},
		"hom_nsu":     &types.AttributeValueMemberN{Value: strconv.Itoa(nsu)},
	}
}

// nfseDistResp monta a resposta do go-dfe/py-dfe para um lote do ADN.
func nfseDistResp(items []map[string]any) []byte {
	status := "NENHUM_DOCUMENTO_LOCALIZADO"
	body := map[string]any{}
	if len(items) > 0 {
		status = "DOCUMENTOS_LOCALIZADOS"
		body["distribuicao"] = items
	}
	body["status_distribuicao"] = status
	bodyBytes, _ := json.Marshal(body)
	payload, _ := json.Marshal(map[string]any{"statusCode": 200, "body": string(bodyBytes)})
	return payload
}

func TestRunNfseDistNSU_PersisteLoteEAvancaCursor(t *testing.T) {
	dynm := &mockDistDynamo{gets: []getResult{
		{item: nfseConfigItem("nacional", 10)}, {item: orgItemWithUF("SP")},
	}}
	dynm.queries = []queryResult{{items: []map[string]types.AttributeValue{certItem()}}}
	s3m := certS3()
	lamm := &mockLambda{payloads: [][]byte{
		nfseDistResp([]map[string]any{
			{"nsu": 11, "chave_acesso": testAK, "tipo_documento": "NFSE", "xml": "<NFSe/>"},
			{"nsu": 12, "chave_acesso": testAK, "tipo_documento": "EVENTO", "tipo_evento": "101101", "xml": "<evento/>"},
		}),
		nfseDistResp(nil),
	}}

	svc := newDistSvc(dynm, s3m, lamm, &mockSNS{}, distCfg)
	if err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: docTypeNfse, Trigger: "scheduler",
	}); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if lamm.calls != 2 {
		t.Errorf("chamadas ao ADN = %d, esperado 2 (lote + lote vazio)", lamm.calls)
	}

	wantKeys := []string{
		"nfse-distribution/hom/" + testOrgPK + "/NSU_000000000000011.xml",
		"nfse-distribution/hom/" + testOrgPK + "/NSU_000000000000012.xml",
	}
	if len(s3m.putCalls) != len(wantKeys) {
		t.Fatalf("puts = %v, esperado %v", s3m.putCalls, wantKeys)
	}
	for i, want := range wantKeys {
		if s3m.putCalls[i] != want {
			t.Errorf("put %d = %q, want %q", i, s3m.putCalls[i], want)
		}
	}

	if len(dynm.putCalls) != 2 {
		t.Fatalf("registros de distribuição = %d, esperado 2", len(dynm.putCalls))
	}
	first := dynm.putCalls[0]
	if *first.TableName != "dev_nfse_distributions" {
		t.Errorf("tabela = %q", *first.TableName)
	}
	if pk, _ := first.Item["pk"].(*types.AttributeValueMemberS); pk == nil || pk.Value != "hom#"+testOrgPK {
		t.Errorf("pk = %v, esperado hom#%s", first.Item["pk"], testOrgPK)
	}
	if nsu, _ := first.Item["nsu"].(*types.AttributeValueMemberN); nsu == nil || nsu.Value != "11" {
		t.Errorf("nsu = %v", first.Item["nsu"])
	}
	if ev, _ := dynm.putCalls[1].Item["event_type"].(*types.AttributeValueMemberS); ev == nil || ev.Value != "101101" {
		t.Errorf("event_type = %v", dynm.putCalls[1].Item["event_type"])
	}

	// Último UpdateItem é o cursor: hom_nsu = 12 (maior NSU do lote).
	if len(dynm.updateCalls) == 0 {
		t.Fatal("cursor não foi atualizado")
	}
	last := dynm.updateCalls[len(dynm.updateCalls)-1]
	if got, _ := last.ExpressionAttributeValues[":nsu"].(*types.AttributeValueMemberN); got == nil || got.Value != "12" {
		t.Errorf("cursor = %v, esperado 12", last.ExpressionAttributeValues[":nsu"])
	}
}

// TestRunNfseDistNSU_ProviderAbrasf_NoOp: só o Sistema Nacional tem ADN.
func TestRunNfseDistNSU_ProviderAbrasf_NoOp(t *testing.T) {
	dynm := &mockDistDynamo{gets: []getResult{{item: nfseConfigItem("abrasf204", 0)}}}
	lamm := &mockLambda{}
	svc := newDistSvc(dynm, certS3(), lamm, &mockSNS{}, distCfg)

	if err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: docTypeNfse, Trigger: "scheduler",
	}); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 0 {
		t.Errorf("abrasf204 não distribui, mas houve %d chamadas", lamm.calls)
	}
}

// TestRunNfseDistNSU_S3UploadFailure_DoesNotAdvanceCursor is a regression
// test: persistNfseIncoming used to swallow the S3 PutObject error (just
// logged it) and the loop advanced the NSU cursor regardless. Since the ADN
// only serves documents strictly after the requested NSU, advancing past a
// document whose XML never made it to S3 lost that document permanently —
// the next cycle would never ask for it again. The fix propagates the S3
// error so the cycle aborts before writing the distribution record or
// moving the cursor; the next cycle retries the same NSU.
func TestRunNfseDistNSU_S3UploadFailure_DoesNotAdvanceCursor(t *testing.T) {
	dynm := &mockDistDynamo{gets: []getResult{
		{item: nfseConfigItem("nacional", 10)}, {item: orgItemWithUF("SP")},
	}}
	dynm.queries = []queryResult{{items: []map[string]types.AttributeValue{certItem()}}}
	s3m := certS3()
	s3m.putErr = errors.New("s3 unavailable")
	lamm := &mockLambda{payloads: [][]byte{
		nfseDistResp([]map[string]any{
			{"nsu": 11, "chave_acesso": testAK, "tipo_documento": "NFSE", "xml": "<NFSe/>"},
		}),
	}}

	svc := newDistSvc(dynm, s3m, lamm, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: docTypeNfse, Trigger: "scheduler",
	})
	if err == nil {
		t.Fatal("esperado erro por falha de upload S3, veio nil")
	}
	if len(dynm.putCalls) != 0 {
		t.Errorf("registro de distribuição não deveria ser gravado com upload S3 falho, houve %d", len(dynm.putCalls))
	}
	// updateCalls[0] é o claim do slot (trigger=scheduler); o cursor (segundo
	// UpdateItem) não deve acontecer.
	if len(dynm.updateCalls) != 1 {
		t.Errorf("cursor de NSU não deveria avançar com upload S3 falho, houve %d UpdateItem", len(dynm.updateCalls))
	}
}

// TestRunNfseDistNSU_UpdateNSUFailure_Propagates is a regression test: the
// cursor UpdateItem error used to be discarded (`_ = s.updateNSU(...)`), so a
// DynamoDB failure right after persisting a batch went unnoticed — the loop
// kept using its in-memory currentNSU for the next ADN request while the
// persisted cursor silently stayed behind.
func TestRunNfseDistNSU_UpdateNSUFailure_Propagates(t *testing.T) {
	dynm := &mockDistDynamo{
		gets: []getResult{
			{item: nfseConfigItem("nacional", 10)}, {item: orgItemWithUF("SP")},
		},
		updateErrs: []error{nil, errors.New("dynamodb unavailable")}, // [0]=claim ok, [1]=cursor update fails
	}
	dynm.queries = []queryResult{{items: []map[string]types.AttributeValue{certItem()}}}
	lamm := &mockLambda{payloads: [][]byte{
		nfseDistResp([]map[string]any{
			{"nsu": 11, "chave_acesso": testAK, "tipo_documento": "NFSE", "xml": "<NFSe/>"},
		}),
	}}

	svc := newDistSvc(dynm, certS3(), lamm, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: docTypeNfse, Trigger: "scheduler",
	})
	if err == nil {
		t.Fatal("esperado erro por falha no UpdateItem do cursor, veio nil")
	}
}

// TestRunNfseDistNSU_RealInProcessPath exercises the actual production route
// (godfeImplements=true, godfeCall wired) instead of this file's mockLambda,
// which every other test in this package uses (see distribution_test.go's
// package-level init forcing godfeImplements=false). That masking is why the
// int64 NSU / dispatch.go intOf mismatch shipped undetected: no test ever
// sent a native (non-JSON-decoded) body through the real in-process call.
// This test stubs godfeCall directly and asserts the NSU cursor reaches it
// as int64, proving the path this package's other NFS-e tests never touch.
func TestRunNfseDistNSU_RealInProcessPath(t *testing.T) {
	origImplements, origCall := godfeImplements, godfeCall
	defer func() { godfeImplements, godfeCall = origImplements, origCall }()

	var gotNSU any
	godfeImplements = func(docType, service string) bool {
		return docType == docTypeNfse && service == serviceNFSeDistribuicao
	}
	godfeCall = func(_ context.Context, req godfe.Request) (godfe.Response, error) {
		gotNSU = req.Body[nfse.BodyKeyNSU]
		body, _ := json.Marshal(map[string]any{"status_distribuicao": "NENHUM_DOCUMENTO_LOCALIZADO"})
		return godfe.Response{StatusCode: 200, Body: string(body)}, nil
	}

	dynm := &mockDistDynamo{gets: []getResult{
		{item: nfseConfigItem("nacional", 10)}, {item: orgItemWithUF("SP")},
	}}
	dynm.queries = []queryResult{{items: []map[string]types.AttributeValue{certItem()}}}
	lam := &mockLambda{}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)

	if err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: docTypeNfse, Trigger: "scheduler",
	}); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lam.calls != 0 {
		t.Errorf("py-dfe Lambda não deveria ser chamado (go-dfe implementa nfse dist), houve %d chamadas", lam.calls)
	}
	if nsu, ok := gotNSU.(int64); !ok || nsu != 11 {
		t.Errorf("req.Body[%q] = %v (%T), esperado int64(11)", nfse.BodyKeyNSU, gotNSU, gotNSU)
	}
}
