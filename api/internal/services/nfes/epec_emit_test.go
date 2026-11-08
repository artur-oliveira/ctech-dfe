package nfes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	godfe "gopkg.aoctech.app/dfe/go-dfe"

	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

func epecEnviNFe(now time.Time) map[string]any {
	return map[string]any{"enviNFe": map[string]any{"NFe": map[string]any{"infNFe": map[string]any{
		"ide":   map[string]any{"tpNF": "1", "dhEmi": fmtDhEmi(now), "tpEmis": "4"},
		"emit":  map[string]any{"CNPJ": "11222333000181", "IE": "111111111111"},
		"dest":  map[string]any{"CNPJ": "99888777000166", "enderDest": map[string]any{"UF": "RJ"}},
		"total": map[string]any{"ICMSTot": map[string]any{"vNF": "100.00", "vICMS": "18.00", "vST": "0.00"}},
	}}}}
}

func epecService(eg services.EgressCaller) *NfeService {
	cfg := &config.Config{TablePrefix: "dev"}
	return (&NfeService{
		tech:      TechData{Version: "ctech-1"},
		workerSvc: services.NewWorkerService(nil, "", "dev"),
		eventRepo: repositories.NewDocumentEventRepository(nil, cfg, "nfe"),
	}).WithEgress(eg)
}

func epecAuthMsg() services.WorkerMessage {
	return services.WorkerMessage{
		DocPK: "prod#CNPJ_1", AccessKey: epecKey, TableName: "nfes", S3Prefix: "nfe", CNPJ: "11222333000181", UF: "SP",
		SefazEnvironment: SefazEnvHom, CertS3Key: "c", CertPassword: "p", DocType: "nfe", SefazService: "NFeAutorizacao",
		BillingOrganizationID: "org-1", BillingMeter: "nfe",
	}
}

func TestResolveNFeEmissionEPEC(t *testing.T) {
	cfg := map[string]types.AttributeValue{
		repositories.AttrEmissionType:      &types.AttributeValueMemberS{Value: repositories.EmissionEPEC},
		repositories.AttrContingencySince:  &types.AttributeValueMemberS{Value: "2026-11-08T10:00:00-03:00"},
		repositories.AttrContingencyReason: &types.AttributeValueMemberS{Value: "SEFAZ de origem fora do ar"},
	}
	m, auth, err := resolveNFeEmission(cfg, "SP")
	if err != nil || auth != "" || m.TpEmis != TpEmisEPEC || m.Validate() != nil {
		t.Fatalf("mode=%+v auth=%q err=%v", m, auth, err)
	}
	// EPEC is not a SVC note: its cancellation goes to the UF's own SEFAZ.
	if svcAuthorizerForKey(epecKey) != "" {
		t.Fatal("an EPEC key must not route to a SVC")
	}
}

// build -> sign-only -> event -> awaiting (not armed): the API half of the EPEC flow with a fake egress.
func TestStageEPECSignsOnlyQueuesEventAndDoesNotArm(t *testing.T) {
	now := time.Now()
	body, _ := json.Marshal(map[string]string{"xml": "<enviNFe><Signature/></enviNFe>"})
	eg := &fakeEgress{resp: godfe.Response{StatusCode: 200, Body: string(body)}}
	item := map[string]types.AttributeValue{}
	st, err := epecService(eg).stageEPEC(context.Background(), item, epecEnviNFe(now), epecAuthMsg(), "CNPJ_1", 2, "u", "User", now)
	if err != nil {
		t.Fatal(err)
	}
	if eg.got.Service != services.EgressServiceSignOnly || eg.got.DocType != "nfe" {
		t.Fatalf("egress must be asked to sign only: %+v", eg.got)
	}
	if got := services.ContingencySignedXML(item); got != "<enviNFe><Signature/></enviNFe>" {
		t.Fatalf("signed xml not stored: %q", got)
	}
	if _, armed := item[services.AttrContingencySweep]; armed {
		t.Fatal("the document must not be armed before the EPEC event is registered")
	}
	if item[services.AttrContingencyState].(*types.AttributeValueMemberS).Value != services.ContingencyAwaitingEPEC {
		t.Fatalf("state: %v", item[services.AttrContingencyState])
	}
	if len(st.Tx) != 2 || st.Tx[0].Put == nil || st.Tx[1].Put == nil {
		t.Fatalf("want outbox + event row in the document transaction: %+v", st.Tx)
	}
	var ev services.WorkerMessage
	payload := st.Tx[0].Put.Item["payload"].(*types.AttributeValueMemberS).Value
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatal(err)
	}
	inf := ev.Body.(map[string]any)["envEvento"].(map[string]any)["evento"].(map[string]any)["infEvento"].(map[string]any)
	if ev.SefazService != "RecepcaoEvento" || ev.UF != "AN" || inf["cOrgao"] != "91" || inf["tpEvento"] != TpEventoEPEC ||
		*ev.EventType != TpEventoEPEC || *ev.EventsTableName != "nfe_events" || ev.BillingOrganizationID != "org-1" {
		t.Fatalf("event message: %+v", ev)
	}
	det := inf["detEvento"].(map[string]any)
	if det["vNF"] != "100.00" || det["IE"] != "111111111111" || det["dest"].(map[string]any)["UF"] != "RJ" {
		t.Fatalf("summary does not follow the note: %v", det)
	}
	if *ev.EventSK != st.Tx[1].Put.Item["sk"].(*types.AttributeValueMemberS).Value {
		t.Fatal("the message must point at the event row written in the same transaction")
	}
}

func TestStageEPECStopsWhenSignOnlyFails(t *testing.T) {
	eg := &fakeEgress{resp: godfe.Response{StatusCode: 400, Body: `{"detail":"x"}`}}
	item := map[string]types.AttributeValue{}
	if _, err := epecService(eg).stageEPEC(context.Background(), item, epecEnviNFe(time.Now()), epecAuthMsg(), "CNPJ_1", 2, "u", "U", time.Now()); err == nil {
		t.Fatal("sign-only failure must abort the emission")
	}
	if len(item) != 0 {
		t.Fatalf("nothing may be stored: %v", item)
	}
}

func TestEPECPreValidation(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		mut  func(*EPECSummary)
		code string
	}{
		{"ok", func(*EPECSummary) {}, ""},
		{"PR refused (121)", func(e *EPECSummary) { e.AccessKey = "41" + e.AccessKey[2:] }, CodeEPECUFRefused},
		{"PB refused (121)", func(e *EPECSummary) { e.AccessKey = "25" + e.AccessKey[2:] }, CodeEPECUFRefused},
		{"dhEmi older than a day (228)", func(e *EPECSummary) { e.IssuedAt = now.Add(-25 * time.Hour) }, CodeEPECStale},
		{"normal key", func(e *EPECSummary) { e.AccessKey = e.AccessKey[:34] + "1" + e.AccessKey[35:] }, CodeEPECInvalid},
	}
	for _, c := range cases {
		e := epecOK()
		e.IssuedAt = now
		c.mut(&e)
		err := e.ValidateAt(now)
		if (err == nil) != (c.code == "") || (err != nil && !strings.Contains(err.Error(), "")) {
			t.Errorf("%s: err=%v", c.name, err)
		}
		if c.code != "" {
			b, _ := json.Marshal(err)
			if !strings.Contains(string(b), c.code) {
				t.Errorf("%s: want code %s in %s", c.name, c.code, b)
			}
		}
	}
}
