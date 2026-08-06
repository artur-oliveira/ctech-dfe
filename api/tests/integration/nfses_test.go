//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfses"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"
)

// idDPSLen é o comprimento fixo do identificador da DPS (TSIdDPS): "DPS" +
// cLocEmi(7) + tpInsc(1) + inscFederal(14) + serie(5) + nDPS(15).
const idDPSLen = 45

// eventFiscoOnly não tem constante no go-dfe de propósito: 105104 é privativo
// do fisco e o pacote só nomeia o que o contribuinte pode emitir. O teste usa o
// literal justamente para provar que o conjunto é fechado.
const eventFiscoOnly = "105104"

const testUserID, testUserName = "test-user", "Test User"

// problemType complementa problemStatus: a regra de erro da API é o Problem
// RFC 7807 inteiro, não só o status.
func problemType(err error) string {
	var pe *problem.Problem
	if errors.As(err, &pe) {
		return pe.Type
	}
	return ""
}

// seedNfseOrg cria organização, config, certificado e um serviço do catálogo —
// o mínimo para uma emissão. Os itens de cadastro são gravados direto porque o
// grupo `nfse` (reg_trib) não faz parte da superfície CRUD dos serviços.
func seedNfseOrg(t *testing.T, withRegTrib bool) (orgPK, serviceID string) {
	t.Helper()
	ctx := context.Background()
	orgPK = "CNPJ_" + randomCNPJ()

	org := map[string]types.AttributeValue{
		"pk":       &types.AttributeValueMemberS{Value: orgPK},
		"name":     &types.AttributeValueMemberS{Value: "Prestador Teste LTDA"},
		"cpf_cnpj": &types.AttributeValueMemberS{Value: services.StripPKPrefix(orgPK)},
	}
	if withRegTrib {
		org["nfse"] = &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"im": &types.AttributeValueMemberS{Value: "123456"},
			"reg_trib": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"op_simp_nac":  &types.AttributeValueMemberN{Value: "1"},
				"reg_esp_trib": &types.AttributeValueMemberN{Value: "0"},
			}},
		}}
	}
	putItem(t, tablePrefix+"_organizations", org)

	putItem(t, tablePrefix+"_organization_certificates", map[string]types.AttributeValue{
		"pk":       &types.AttributeValueMemberS{Value: orgPK},
		"sk":       &types.AttributeValueMemberS{Value: "CERTIFICATE_test"},
		"s3_key":   &types.AttributeValueMemberS{Value: orgPK + "/cert.pfx"},
		"password": &types.AttributeValueMemberS{Value: "unused-in-this-harness"},
	})

	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, nfseConfigFields(), testUserID, testUserName); err != nil {
		t.Fatalf("Upsert config: %v", err)
	}

	svc, err := serviceRepo.Create(ctx, orgPK, map[string]types.AttributeValue{
		"description":         &types.AttributeValueMemberS{Value: "Consultoria em TI"},
		"code":                &types.AttributeValueMemberS{Value: "S001"},
		"trib_nacional_code":  &types.AttributeValueMemberS{Value: "010101"},
		"trib_municipal_code": &types.AttributeValueMemberS{Value: "0101"},
		"value":               &types.AttributeValueMemberS{Value: "1000.00"},
		"iss": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"trib_issqn": &types.AttributeValueMemberN{Value: "1"},
			"tax_rate":   &types.AttributeValueMemberS{Value: "5.00"},
		}},
	})
	if err != nil {
		t.Fatalf("Create service: %v", err)
	}
	return orgPK, svc["sk"].(*types.AttributeValueMemberS).Value
}

func putItem(t *testing.T, table string, item map[string]types.AttributeValue) {
	t.Helper()
	if _, err := db.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(table), Item: item,
	}); err != nil {
		t.Fatalf("PutItem %s: %v", table, err)
	}
}

// patchNfse escreve atributos direto na linha, simulando o que o worker grava
// no desfecho (status, access_key) sem subir o worker.
func patchNfse(t *testing.T, pk, sk string, attrs map[string]types.AttributeValue) {
	t.Helper()
	names := map[string]string{}
	values := map[string]types.AttributeValue{}
	sets := make([]string, 0, len(attrs))
	i := 0
	for k, v := range attrs {
		alias := string(rune('a' + i))
		names["#"+alias] = k
		values[":"+alias] = v
		sets = append(sets, "#"+alias+" = :"+alias)
		i++
	}
	if _, err := db.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(tablePrefix + "_nfses"),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression:          aws.String("SET " + strings.Join(sets, ", ")),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	}); err != nil {
		t.Fatalf("UpdateItem nfses: %v", err)
	}
}

func emitBody(serviceID string) nfses.NfseEmitBody {
	return nfses.NfseEmitBody{
		TpEmit:     1,
		Competence: time.Now().Format("02/01/2006"),
		Service:    nfses.NfseServiceItem{ServiceID: serviceID},
	}
}

// emitAuthorized emite e força o desfecho autorizado, o estado que os eventos
// exigem: o pedido de registro é endereçado à chave de 50 dígitos.
func emitAuthorized(t *testing.T, orgPK, serviceID string) (idDPS, accessKey string) {
	t.Helper()
	item, err := nfseSvc.Emit(context.Background(), orgPK, emitBody(serviceID), testUserID, testUserName)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	idDPS = item["sk"].(*types.AttributeValueMemberS).Value
	pk := item["pk"].(*types.AttributeValueMemberS).Value
	accessKey = strings.Repeat("9", 50)
	patchNfse(t, pk, idDPS, map[string]types.AttributeValue{
		"status":     &types.AttributeValueMemberS{Value: nfses.StatusAuthorized},
		"access_key": &types.AttributeValueMemberS{Value: accessKey},
	})
	return idDPS, accessKey
}

func TestNfse(t *testing.T) {
	ctx := context.Background()

	t.Run("EmitPersisteComSKIDDPS", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)

		item, err := nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if err != nil {
			t.Fatalf("Emit: %v", err)
		}

		idDPS := item["sk"].(*types.AttributeValueMemberS).Value
		if len(idDPS) != idDPSLen {
			t.Errorf("len(id_dps) = %d, esperado %d (%s)", len(idDPS), idDPSLen, idDPS)
		}
		if !strings.HasPrefix(idDPS, "DPS") {
			t.Errorf("id_dps = %s, esperado prefixo DPS", idDPS)
		}
		// A chave de acesso só existe na resposta do fisco: gravar vazio aqui
		// poluiria a access-key-index.
		if _, ok := item["access_key"]; ok {
			t.Error("access_key gravada na emissão")
		}
		if s := item["status"].(*types.AttributeValueMemberS).Value; s != nfses.StatusPending {
			t.Errorf("status = %s, esperado %s", s, nfses.StatusPending)
		}

		pk := item["pk"].(*types.AttributeValueMemberS).Value
		stored, err := nfseRepo.Get(ctx, pk, idDPS)
		if err != nil || stored == nil {
			t.Fatalf("linha não persistida: %v", err)
		}
	})

	t.Run("EmitEnfileiraOutbox", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)

		item, err := nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if err != nil {
			t.Fatalf("Emit: %v", err)
		}
		idDPS := item["sk"].(*types.AttributeValueMemberS).Value
		operationID := item["operation_id"].(*types.AttributeValueMemberS).Value
		if want := repositories.TableNfses + "#" + idDPS; operationID != want {
			t.Errorf("operation_id = %s, esperado %s", operationID, want)
		}

		out, err := db.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(tablePrefix + "_worker_outbox"),
			Key: map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: operationID},
				"sk": &types.AttributeValueMemberS{Value: "command"},
			},
		})
		if err != nil {
			t.Fatalf("GetItem outbox: %v", err)
		}
		if out.Item == nil {
			t.Fatal("comando do worker não foi gravado na mesma transação")
		}
		payload := out.Item["payload"].(*types.AttributeValueMemberS).Value
		// O worker resolve a linha por AccessKey; em NFS-e esse campo carrega
		// o id_dps, que é a SK da tabela nfses.
		if !strings.Contains(payload, idDPS) {
			t.Errorf("payload do outbox não referencia o id_dps: %s", payload)
		}
	})

	t.Run("EmitDuplicadoRejeita", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)

		first, err := nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if err != nil {
			t.Fatalf("primeira emissão: %v", err)
		}
		pk := first["pk"].(*types.AttributeValueMemberS).Value

		// A numeração é reservada na mesma transação do Put, então o conflito
		// real é uma linha já existente no id_dps que a próxima emissão vai
		// calcular. Reproduz o retry concorrente sem corrida no teste.
		next, err := nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if err != nil {
			t.Fatalf("segunda emissão: %v", err)
		}
		idDPS := next["sk"].(*types.AttributeValueMemberS).Value
		patchNfse(t, pk, idDPS, map[string]types.AttributeValue{
			"user_name": &types.AttributeValueMemberS{Value: "original intocado"},
		})
		// Recua o contador para que a próxima emissão recalcule o mesmo id_dps.
		if _, err := db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(tablePrefix + "_organization_nfse_configs"),
			Key: map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: orgPK},
			},
			UpdateExpression: aws.String("SET hom_current_number = :n"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":n": &types.AttributeValueMemberN{Value: "1"},
			},
		}); err != nil {
			t.Fatalf("recuar contador: %v", err)
		}

		_, err = nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if problemStatus(err) != 409 {
			t.Fatalf("status = %d (%v), esperado 409", problemStatus(err), err)
		}
		if typ := problemType(err); typ != problem.TypeConflict {
			t.Errorf("type = %s, esperado %s", typ, problem.TypeConflict)
		}

		stored, err := nfseRepo.Get(ctx, pk, idDPS)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if u := stored["user_name"].(*types.AttributeValueMemberS).Value; u != "original intocado" {
			t.Errorf("user_name = %s — a transação rejeitada sobrescreveu o item original", u)
		}
	})

	t.Run("EmitSemRegTribRejeita", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, false)

		_, err := nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if problemStatus(err) != 400 {
			t.Fatalf("status = %d (%v), esperado 400", problemStatus(err), err)
		}
		if typ := problemType(err); typ != problem.TypeBadRequest {
			t.Errorf("type = %s, esperado %s", typ, problem.TypeBadRequest)
		}
		if !strings.Contains(err.Error(), "reg_trib") {
			t.Errorf("detalhe não cita o campo: %v", err)
		}
	})

	t.Run("GetNfsePorIDDPSEPorChave", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)
		idDPS, accessKey := emitAuthorized(t, orgPK, serviceID)

		byID, err := nfseSvc.GetNfse(ctx, orgPK, idDPS)
		if err != nil {
			t.Fatalf("GetNfse por id_dps: %v", err)
		}
		byKey, err := nfseSvc.GetNfse(ctx, orgPK, accessKey)
		if err != nil {
			t.Fatalf("GetNfse por chave: %v", err)
		}
		if byID["sk"].(*types.AttributeValueMemberS).Value != byKey["sk"].(*types.AttributeValueMemberS).Value {
			t.Error("id_dps e chave de acesso resolveram linhas diferentes")
		}
	})

	t.Run("ListNfsesFiltraPorCompetencia", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)

		atual, err := nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if err != nil {
			t.Fatalf("Emit atual: %v", err)
		}
		antiga, err := nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if err != nil {
			t.Fatalf("Emit antiga: %v", err)
		}
		pk := antiga["pk"].(*types.AttributeValueMemberS).Value
		patchNfse(t, pk, antiga["sk"].(*types.AttributeValueMemberS).Value, map[string]types.AttributeValue{
			"year":  &types.AttributeValueMemberN{Value: "1999"},
			"month": &types.AttributeValueMemberN{Value: "1"},
		})

		now := time.Now()
		year, month := now.Year(), int(now.Month())
		res, err := nfseSvc.ListNfses(ctx, orgPK, repositories.NfseListOpts{Year: &year, Month: &month})
		if err != nil {
			t.Fatalf("ListNfses: %v", err)
		}
		if len(res.Items) != 1 {
			t.Fatalf("itens = %d, esperado 1 — o filtro de competência não foi aplicado", len(res.Items))
		}
		if got, want := res.Items[0]["sk"].(*types.AttributeValueMemberS).Value,
			atual["sk"].(*types.AttributeValueMemberS).Value; got != want {
			t.Errorf("sk = %s, esperado %s", got, want)
		}
	})

	t.Run("EventoCancelamentoExigeMotivo", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)
		idDPS, _ := emitAuthorized(t, orgPK, serviceID)

		_, err := nfseSvc.SendEvent(ctx, orgPK, idDPS, nfses.NfseEventBody{
			EventType: nfse.EventCancelamento,
		}, testUserID, testUserName)
		if problemStatus(err) != 400 {
			t.Fatalf("status = %d (%v), esperado 400", problemStatus(err), err)
		}
		if !strings.Contains(err.Error(), "reason_code") {
			t.Errorf("detalhe não cita reason_code: %v", err)
		}
	})

	t.Run("EventoFiscoRejeitado", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)
		idDPS, _ := emitAuthorized(t, orgPK, serviceID)

		_, err := nfseSvc.SendEvent(ctx, orgPK, idDPS, nfses.NfseEventBody{
			EventType: eventFiscoOnly,
		}, testUserID, testUserName)
		if problemStatus(err) != 400 {
			t.Fatalf("status = %d (%v), esperado 400", problemStatus(err), err)
		}
		if !strings.Contains(err.Error(), "contribuinte") {
			t.Errorf("detalhe não explica que o evento é do fisco: %v", err)
		}
	})

	t.Run("SubstituicaoNaoEhEvento", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)
		idDPS, _ := emitAuthorized(t, orgPK, serviceID)

		_, err := nfseSvc.SendEvent(ctx, orgPK, idDPS, nfses.NfseEventBody{
			EventType: nfse.EventCancelamentoPorSubst,
		}, testUserID, testUserName)
		if problemStatus(err) != 400 {
			t.Fatalf("status = %d (%v), esperado 400", problemStatus(err), err)
		}
		if !strings.Contains(err.Error(), "/substitute") {
			t.Errorf("detalhe não aponta a rota de substituição: %v", err)
		}
	})

	t.Run("EventoExigeNfseAutorizada", func(t *testing.T) {
		orgPK, serviceID := seedNfseOrg(t, true)
		item, err := nfseSvc.Emit(ctx, orgPK, emitBody(serviceID), testUserID, testUserName)
		if err != nil {
			t.Fatalf("Emit: %v", err)
		}

		_, err = nfseSvc.SendEvent(ctx, orgPK, item["sk"].(*types.AttributeValueMemberS).Value,
			nfses.NfseEventBody{
				EventType: nfse.EventCancelamento, ReasonCode: "1", ReasonDescription: "erro",
			}, testUserID, testUserName)
		if problemStatus(err) != 400 {
			t.Fatalf("status = %d (%v), esperado 400", problemStatus(err), err)
		}
	})

	t.Run("ParametrosMunicipaisValidamAridade", func(t *testing.T) {
		orgPK, _ := seedNfseOrg(t, true)

		// A aridade é validada antes de qualquer ida ao ADN — se não fosse, o
		// teste falharia por rede, não por 400.
		_, err := nfseSvc.MunicipalParameters(ctx, orgPK, nacional.ParamConvenio, []string{"a", "b"})
		if problemStatus(err) != 400 {
			t.Fatalf("status = %d (%v), esperado 400", problemStatus(err), err)
		}
		_, err = nfseSvc.MunicipalParameters(ctx, orgPK, "inexistente", []string{"a"})
		if problemStatus(err) != 400 {
			t.Fatalf("status = %d (%v), esperado 400", problemStatus(err), err)
		}
	})
}
