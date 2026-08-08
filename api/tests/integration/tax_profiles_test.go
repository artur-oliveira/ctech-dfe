//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func taxProfileFields(name string, cfops ...string) map[string]types.AttributeValue {
	list := make([]types.AttributeValue, 0, len(cfops))
	for _, c := range cfops {
		list = append(list, &types.AttributeValueMemberS{Value: c})
	}
	return map[string]types.AttributeValue{
		"name":               &types.AttributeValueMemberS{Value: name},
		"cfops":              &types.AttributeValueMemberL{Value: list},
		"pis":                &types.AttributeValueMemberS{Value: "01"},
		"cofins":             &types.AttributeValueMemberS{Value: "01"},
		"ibs_cbs_cst":        &types.AttributeValueMemberS{Value: "000"},
		"ibs_cbs_class_trib": &types.AttributeValueMemberS{Value: "000001"},
		"ibs_uf_aliq":        &types.AttributeValueMemberS{Value: "8.0000"},
		"ibs_mun_aliq":       &types.AttributeValueMemberS{Value: "1.0000"},
		"cbs_aliq":           &types.AttributeValueMemberS{Value: "9.0000"},
	}
}

func avString(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func TestTaxProfile_CRUD(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := taxProfileSvc.Create(ctx, orgPK,
		taxProfileFields("Venda de mercadoria", "5102", "6102"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := avString(created, "sk")
	if !strings.HasPrefix(sk, repositories.SKPrefixTaxProfile) {
		t.Fatalf("sk = %q, want prefix %q", sk, repositories.SKPrefixTaxProfile)
	}

	got, err := taxProfileSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if avString(got, "name") != "Venda de mercadoria" {
		t.Errorf("name = %q", avString(got, "name"))
	}

	// The bare id (no prefix) must resolve to the same row — routes take
	// whichever form the client kept.
	bare := strings.TrimPrefix(sk, repositories.SKPrefixTaxProfile)
	if _, err := taxProfileSvc.Get(ctx, orgPK, bare); err != nil {
		t.Errorf("Get by bare id: %v", err)
	}

	if _, err := taxProfileSvc.Update(ctx, orgPK, sk,
		map[string]any{"name": "Venda para revenda"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, _ := taxProfileSvc.Get(ctx, orgPK, sk)
	if avString(after, "name") != "Venda para revenda" {
		t.Errorf("name after update = %q", avString(after, "name"))
	}

	if err := taxProfileSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := taxProfileSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("expected 404 after delete, got %d", problemStatus(err))
	}
}

func TestTaxProfile_ListAndNamePrefix(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	for _, name := range []string{"Venda interna", "Venda interestadual", "Devolução de compra"} {
		if _, err := taxProfileSvc.Create(ctx, orgPK,
			taxProfileFields(name, "5102"), "test-user", "Test User"); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	all, err := taxProfileSvc.List(ctx, orgPK, repositories.OrgEntityListOpts{Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all.Items) != 3 {
		t.Errorf("List returned %d profiles, want 3", len(all.Items))
	}

	vendas, err := taxProfileSvc.List(ctx, orgPK, repositories.OrgEntityListOpts{NamePrefix: "Venda", Limit: 50})
	if err != nil {
		t.Fatalf("List by name prefix: %v", err)
	}
	if len(vendas.Items) != 2 {
		t.Errorf("name prefix \"Venda\" returned %d profiles, want 2", len(vendas.Items))
	}
}

// Registries are org-scoped like every other cadastro.
func TestTaxProfile_IsolatedPerOrganization(t *testing.T) {
	ctx := context.Background()
	orgA := "CNPJ_" + randomCNPJ()
	orgB := "CNPJ_" + randomCNPJ()

	created, err := taxProfileSvc.Create(ctx, orgA, taxProfileFields("Só da A", "5102"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create in org A: %v", err)
	}
	if _, err := taxProfileSvc.Get(ctx, orgB, avString(created, "sk")); problemStatus(err) != 404 {
		t.Errorf("org B reached org A's profile (status %d)", problemStatus(err))
	}
}

// BatchGet é o caminho que a emissão usa: um round trip para todos os perfis
// referenciados pelos produtos da nota, nunca um Get por item dentro do laço.
func TestTaxProfile_BatchGet(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	var sks []string
	for _, name := range []string{"Perfil A", "Perfil B"} {
		created, err := taxProfileSvc.Create(ctx, orgPK, taxProfileFields(name, "5102"), "test-user", "Test User")
		if err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
		sks = append(sks, avString(created, "sk"))
	}

	// Ids repetidos e um inexistente: repetido colapsa, inexistente some do
	// resultado em vez de virar erro — quem chama decide se falta é problema.
	rows, err := taxProfileRepo.BatchGet(ctx, orgPK, []string{sks[0], sks[1], sks[0], "TAXPROFILE_nao-existe"})
	if err != nil {
		t.Fatalf("BatchGet: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("BatchGet devolveu %d perfis, esperado 2", len(rows))
	}
	if avString(rows[sks[0]], "name") != "Perfil A" {
		t.Errorf("perfil A = %q", avString(rows[sks[0]], "name"))
	}
	if _, ok := rows["TAXPROFILE_nao-existe"]; ok {
		t.Error("perfil inexistente não pode aparecer no resultado")
	}
}

// Um perfil de outra organização não pode ser alcançado pelo BatchGet.
func TestTaxProfile_BatchGetIsOrgScoped(t *testing.T) {
	ctx := context.Background()
	orgA := "CNPJ_" + randomCNPJ()
	orgB := "CNPJ_" + randomCNPJ()

	created, err := taxProfileSvc.Create(ctx, orgA, taxProfileFields("Só da A", "5102"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rows, err := taxProfileRepo.BatchGet(ctx, orgB, []string{avString(created, "sk")})
	if err != nil {
		t.Fatalf("BatchGet: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("org B alcançou %d perfis da org A", len(rows))
	}
}
