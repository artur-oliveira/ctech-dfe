package mdfes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// fakeCiotRepo answers GetItem from a local httptest server (never AWS): row is the DynamoDB JSON item or "".
func fakeCiotRepo(t *testing.T, row string) *repositories.CiotRepository {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if row == "" {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"Item":` + row + `}`))
	}))
	t.Cleanup(srv.Close)
	db := dynamodb.New(dynamodb.Options{Region: "us-east-1", BaseEndpoint: aws.String(srv.URL),
		Credentials: credentials.NewStaticCredentialsProvider("x", "x", ""), RetryMaxAttempts: 1})
	return repositories.NewCiotRepository(db, &config.Config{TablePrefix: "t"})
}

func ciotRow(extra string) string {
	return `{"status":{"S":"declared"},"code":{"S":"123456789012"},"carrier_doc":{"S":"12345678000190"}` + extra + `}`
}

func TestResolveCIOTs(t *testing.T) {
	ctx := t.Context()
	cases := []struct {
		name, row, wantErr string
		in                 []MdfeCIOT
		wantLinks          int
	}{
		{"declared row", ciotRow(""), "", []MdfeCIOT{{CiotID: "A"}}, 1},
		{"external only", "", "", []MdfeCIOT{{CIOT: "123456789012", ResponsibleDoc: "12345678909"}}, 0},
		{"external incomplete", "", "informe o CIOT declarado", []MdfeCIOT{{CIOT: "123456789012"}}, 0},
		{"both forms", "", "não os dois", []MdfeCIOT{{CiotID: "A", CIOT: "123456789012"}}, 0},
		{"missing row", "", "não encontrado", []MdfeCIOT{{CiotID: "A"}}, 0},
		{"already linked", ciotRow(`,"mdfe_access_key":{"S":"x"}`), "já está vinculado", []MdfeCIOT{{CiotID: "A"}}, 0},
		{"operation in flight", ciotRow(`,"pending_operation":{"S":"cancel"}`), "em andamento", []MdfeCIOT{{CiotID: "A"}}, 0},
		{"cancelled", strings.Replace(ciotRow(""), "declared", "cancelled", 1), "declarado", []MdfeCIOT{{CiotID: "A"}}, 0},
		{"non numeric code", strings.Replace(ciotRow(""), "123456789012", "ABCDEFGHIJKL", 1), "12 dígitos", []MdfeCIOT{{CiotID: "A"}}, 0},
		{"same code twice", ciotRow(""), "mais de uma vez", []MdfeCIOT{{CiotID: "A"}, {CIOT: "123456789012", ResponsibleDoc: "12345678909"}}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &MdfeService{ciotRepo: fakeCiotRepo(t, c.row)}
			out, links, err := s.resolveCIOTs(ctx, "prod#ORG", c.in)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want %q", err, c.wantErr)
				}
				return
			}
			if err != nil || len(links) != c.wantLinks || len(out) != len(c.in) {
				t.Fatalf("out=%v links=%v err=%v", out, links, err)
			}
			if c.wantLinks == 1 && (out[0].CIOT != "123456789012" || out[0].ResponsibleDoc != "12345678000190") {
				t.Fatalf("resolved = %+v", out[0])
			}
		})
	}
}
