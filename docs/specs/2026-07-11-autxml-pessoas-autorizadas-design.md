# autXML — Pessoas Autorizadas a Visualizar XML — Design

## Contexto

SEFAZ permite até 10 pessoas (CPF ou CNPJ + nome) autorizadas a acessar o XML de uma NF-e/CT-e via
`autXML` (`<xs:element name="autXML">`, filho de `infNFe`, entre `entrega` e `det` —
`py-dfe/py_dfe/xmlops/xsd_order.py:40,84-85`, shape `["CNPJ", "CPF"]` — só o documento, sem nome no XML; o nome é
metadado interno nosso). Zero implementação hoje em `api/`/`ui/` (confirmado por grep — nenhuma ocorrência de `autXML`/
`authorized` fora da ordenação XSD).

É uma configuração de **organização** (o emitente decide quem sempre pode ver os XMLs que ele emite), não uma escolha
por emissão — mesmo tratamento em toda NF-e daquela organização.

## Mudanças

### Modelo

`organizations` ganha atributo `authorized_xml_viewers`: lista de até 10
`{cpf_cnpj: string (dígitos), name: string}`. Sem tabela nova, sem GSI novo.

### Backend

`api/internal/api/v1/dto.go`:

```go
type AuthorizedViewerBody struct {
    CpfOrCnpj string `json:"cpf_or_cnpj" validate:"required,cpfcnpj"`
    Name      string `json:"name" validate:"required,min=2,max=60"`
}
```

`api/internal/services/organizations.go` — dois métodos novos, reaproveitando o
`Update` genérico já existente (que já faz fetch→merge→diff→TransactWrite+audit, ver
`organizations.go:58-104`):

```go
const maxAuthorizedViewers = 10

func (s *OrganizationService) AddAuthorizedViewer(ctx context.Context, orgPK string, v AuthorizedViewerBody, userID, userName string) (map[string]types.AttributeValue, error) {
    current, err := s.repo.GetOrganization(ctx, orgPK)
    // ... NotFound se nil
    viewers := extractAuthorizedViewers(current) // []AuthorizedViewerBody
    if len(viewers) >= maxAuthorizedViewers {
        return nil, problem.BadRequest("limite de 10 pessoas autorizadas atingido")
    }
    for _, existing := range viewers {
        if existing.CpfOrCnpj == normalizeDoc(v.CpfOrCnpj) {
            return nil, problem.Conflict("CPF/CNPJ já autorizado")
        }
    }
    viewers = append(viewers, v)
    return s.Update(ctx, orgPK, map[string]any{"authorized_xml_viewers": viewers}, userID, userName)
}

func (s *OrganizationService) RemoveAuthorizedViewer(ctx context.Context, orgPK, cpfCnpj, userID, userName string) (map[string]types.AttributeValue, error) {
    // fetch, filter out matching cpf_cnpj, s.Update(...)
}
```

Rotas (`api/internal/api/v1/organizations.go`):

- `POST /organizations/:org_pk/authorized-viewers` → `AddAuthorizedViewer`
- `DELETE /organizations/:org_pk/authorized-viewers/:cpf_cnpj` → `RemoveAuthorizedViewer`

Endpoints dedicados (não um replace de array inteiro) porque a tarefa pede "cadastrar / remover / atualizar" — semântica
de item, não de documento inteiro; e cada operação vira uma linha de audit log própria (`AuditActionUpdate` sobre o
recurso organização, diff mostra só o item que mudou).

### Builder (`api/internal/services/nfes/builders_doc.go`)

```go
func buildAutXML(org map[string]any) []map[string]any {
    viewers, _ := org["authorized_xml_viewers"].([]any)
    if len(viewers) == 0 {
        return nil
    }
    out := make([]map[string]any, 0, len(viewers))
    for _, v := range viewers {
        vm, _ := v.(map[string]any)
        doc := anyStr(vm, "cpf_cnpj", "")
        entry := map[string]any{}
        if len(doc) == 14 {
            entry["CNPJ"] = doc
        } else {
            entry["CPF"] = doc
        }
        out = append(out, entry)
    }
    return out
}
```

Incluído em `BuildEnviNFe`'s `infNFe` dict sob a chave `autXML` sempre que não-vazio (não condicionado a nenhum campo do
`NfeEmitBody` — vem só da organização). Nenhuma mudança no py-dfe.

### Frontend

- `ui/src/lib/schemas/organizations.ts` (ou novo `authorized-viewers.ts`): schema com dedup de CPF/CNPJ (mesmo padrão de
  `state_registrations` UF-dup em `entity.ts:60-64`) e cap de 10.
  `ui/src/lib/types/api.ts`: `AuthorizedViewer`, `OrganizationOut.authorized_xml_viewers?`.
  `ui/src/lib/api/client.ts`: `addAuthorizedViewer(orgPk, body)`, `removeAuthorizedViewer(orgPk, cpfCnpj)`.
- Nova seção na página de organização (`ui/src/app/organizations/edit/page.tsx` ou componente dedicado
  `AuthorizedViewersSection.tsx`): lista atual + form CPF/CNPJ+nome + adicionar/remover, contador "X/10", erro inline se
  duplicado (o back já rejeita com 409, front também valida antes de submeter pra evitar round-trip desnecessário).

## Testes

| Mudança                  | Teste                                                                                                   |
|--------------------------|---------------------------------------------------------------------------------------------------------|
| `AddAuthorizedViewer`    | Unit: rejeita 11º item (400), rejeita CPF/CNPJ duplicado (409), aceita item válido                      |
| `RemoveAuthorizedViewer` | Unit: remove existente, 404 se organização não existe, no-op silencioso se CPF/CNPJ não estava na lista |
| `buildAutXML`            | Unit: `builders_doc_test.go` — lista vazia → chave omitida; CNPJ vs CPF routing correto                 |
| Endpoints                | Integration: `POST`/`DELETE /organizations/:pk/authorized-viewers` fim-a-fim contra DynamoDB Local      |
| Front dedup              | Unit: schema rejeita CPF/CNPJ já presente na lista                                                      |

## Impacto cross-project

- **api**: `dto.go`, `services/organizations.go`, `api/v1/organizations.go`,
  `services/nfes/builders_doc.go`.
- **ui**: novo componente de seção, `types/api.ts`, `client.ts`, `query-keys.ts`.
- **cdk**: nenhuma mudança.
- **py-dfe**: nenhuma mudança.
- **DOCS.md**: novo endpoint na tabela de Organizations; nota no corpo de emissão de NF-e sobre
  `autXML` ser automático (não é campo do payload de emissão).
