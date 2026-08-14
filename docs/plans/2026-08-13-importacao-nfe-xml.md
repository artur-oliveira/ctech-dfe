# Importação de NF-e/NFC-e por XML Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Permitir que o usuário importe uma NF-e/NFC-e a partir de um arquivo XML (`nfeProc` ou `NFe`), validando vínculo com a organização (emit > dest > transp), confirmando o digest contra uma consulta protocolo real na SEFAZ, e persistindo o documento como se tivesse chegado pela distribuição.

**Architecture:** Endpoint `api` assíncrono (multipart → valida tamanho/raiz/cota → staging S3 → SQS) processado por um novo job `import_xml` no `worker` (mesma fila `distribution` existente), que classifica emit/dest/transp, chama `NfeConsultaProtocolo` via `go-dfe` (primeiro uso real desta operação), compara digest, monta o `nfeProc` final quando necessário, persiste e notifica via WS. Frontend adiciona botões de upload em NF-e (aba Distribuição) e NFC-e (discreto).

**Tech Stack:** Go (api, worker, go-dfe), Next.js/TypeScript (ui), DynamoDB, S3, SQS, SNS.

**Spec:** `docs/specs/2026-08-13-importacao-nfe-xml.md`

## Global Constraints

- Doc types suportados: apenas `nfe` e `nfce`.
- Raiz XML aceita: `nfeProc` ou `NFe`. Qualquer outra raiz é rejeição de negócio (sem retry).
- Prioridade de classificação/vínculo, sempre nesta ordem: `emit` → `Incoming=0` (emitida); senão `dest` → `Incoming=1` (destinada); senão `transp.transporta` → `Incoming=2` (transportada); nenhum bate → rejeição.
- Rejeições de negócio (raiz inválida, sem vínculo, nota já completa, divergência de digest, `cStat` de rejeição SEFAZ) NUNCA são retentadas — worker retorna `nil`, não `error`.
- Erros de rede/timeout na consulta protocolo retornam `error` (deixa SQS retry/DLQ tratar normalmente).
- Todo commit de código relevante deve atualizar `DOCS.md` e/ou `CONDUCT.md` no mesmo commit (Mandatory Documentation Policy).
- `go test ./... -race` (api, worker) e `npx eslint src --ext .ts,.tsx` + `npm test` (ui) precisam passar antes de qualquer commit.

---

### Task 1: go-dfe — exportar `BuildXMLFragment`

O worker precisa serializar um dict JSON-shaped (o `protNFe` retornado pela consulta protocolo) de volta para um fragmento XML, reusando exatamente a mesma lógica de `xmlops.BuildXML` (convenção `@attr`/`#text`/xsdorder). `xmlops` é um pacote `internal/` de `go-dfe` — inacessível fora do módulo `go-dfe` — então precisa de um wrapper exportado.

**Files:**
- Create: `go-dfe/xmlfragment.go`
- Test: `go-dfe/xmlfragment_test.go`

**Interfaces:**
- Produces: `dfe.BuildXMLFragment(body map[string]any, rootTag, xmlns string) ([]byte, error)` — usado pelo worker na Task 5.

- [ ] **Step 1: Escrever o teste que falha**

```go
// go-dfe/xmlfragment_test.go
package dfe

import "strings"

import "testing"

func TestBuildXMLFragment_ProtNFe(t *testing.T) {
	body := map[string]any{
		"@versao": "4.00",
		"infProt": map[string]any{
			"tpAmb":    "2",
			"chNFe":    "22260811647612000197550000000000501454670090",
			"dhRecbto": "2026-08-08T17:05:06-03:00",
			"nProt":    "322260000016670",
			"digVal":   "cKFyNtF4cg+d63/SRv0ezXGoef8=",
			"cStat":    "100",
			"xMotivo":  "Autorizado o uso da NF-e",
		},
	}
	out, err := BuildXMLFragment(body, "protNFe", "http://www.portalfiscal.inf.br/nfe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	xml := string(out)
	if !strings.Contains(xml, `<protNFe`) || !strings.Contains(xml, `versao="4.00"`) {
		t.Fatalf("missing root/attr: %s", xml)
	}
	if !strings.Contains(xml, `xmlns="http://www.portalfiscal.inf.br/nfe"`) {
		t.Fatalf("missing xmlns: %s", xml)
	}
	if !strings.Contains(xml, "<digVal>cKFyNtF4cg+d63/SRv0ezXGoef8=</digVal>") {
		t.Fatalf("missing digVal: %s", xml)
	}
}

func TestBuildXMLFragment_UnknownServiceStillBuilds(t *testing.T) {
	// BuildXMLFragment não faz lookup em dfe.Implements — é só serialização,
	// deve funcionar independente de qualquer gate de promoção.
	out, err := BuildXMLFragment(map[string]any{"#text": "x"}, "foo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "<foo>x</foo>" {
		t.Fatalf("got %s", out)
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd go-dfe && go test ./... -run TestBuildXMLFragment` (falha: `BuildXMLFragment` não existe)

- [ ] **Step 3: Implementar**

```go
// go-dfe/xmlfragment.go
package dfe

import "gopkg.aoctech.app/dfe/go-dfe/internal/xmlops"

// BuildXMLFragment serializes body into an XML fragment with rootTag as its
// root element, reusing xmlops.BuildXML's dict<->XML convention (@attr,
// #text, xsdorder-based child ordering). Exported for callers outside this
// module — worker/internal/service/distribution.go needs it to rebuild the
// nfeProc document from the protNFe dict a consulta protocolo response
// carries (see docs/specs/2026-08-13-importacao-nfe-xml.md).
func BuildXMLFragment(body map[string]any, rootTag, xmlns string) ([]byte, error) {
	return xmlops.BuildXML(body, rootTag, xmlns)
}
```

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd go-dfe && CGO_ENABLED=0 GOARCH=arm64 go build ./... && go test ./... -run TestBuildXMLFragment -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-dfe/xmlfragment.go go-dfe/xmlfragment_test.go
git commit -m "feat(go-dfe): export BuildXMLFragment for worker XML-import join"
```

---

### Task 2: worker — corrigir ambiguidade `Incoming=0` em `persistIncoming`

`DocFields.Incoming` usa `0` tanto para "não setado" (todo código existente trata 0 como 1/destinada, ver `distribution_parser.go:74`) quanto — a partir desta feature — para "emitida" de verdade. Sem um flag explícito, `persistIncoming` (`distribution.go:724-727`) sobrescreveria silenciosamente `Incoming=0` (emitida) para `1` (destinada), quebrando a prioridade emit>dest>transp exigida pelo spec. Correção mínima: um campo `IncomingSet bool`, aditivo — nenhum caller existente o seta, então nenhum comportamento atual muda.

**Files:**
- Modify: `worker/internal/service/distribution_parser.go:74` (campo `Incoming` no struct `DocFields`)
- Modify: `worker/internal/service/distribution.go:724-727` (`persistIncoming`)
- Test: `worker/internal/service/distribution_test.go`

**Interfaces:**
- Consumes: `mockDistDynamo`, `newDistSvc(dynm, s3m, lamm, snsm, cfg)` (helpers já existentes em `distribution_test.go`).
- Produces: `DocFields.IncomingSet bool` — usado pela Task 6 (`runImportXML`) para setar `Incoming=0` sem ser coagido a `1`.

- [ ] **Step 1: Escrever o teste que falha**

```go
// worker/internal/service/distribution_test.go (adicionar)
func TestPersistIncoming_ExplicitZero_IsNotCoercedToOne(t *testing.T) {
	dynm := &mockDistDynamo{}
	svc := newDistSvc(dynm, &mockS3{}, &mockLambda{}, &mockSNS{}, testConfig())

	fields := DocFields{
		AccessKey:   "22260811647612000197550000000000501454670090",
		Incoming:    0,
		IncomingSet: true,
	}
	svc.persistIncoming(context.Background(), "hom#CNPJ_11647612000197", fields, docTypeConfigs["nfe"])

	put := dynm.lastPutItem("dfe_test_nfes") // nome de tabela conforme testConfig().TablePrefix
	if put == nil {
		t.Fatal("expected a PutItem call")
	}
	got := put.Item["incoming"].(*types.AttributeValueMemberN).Value
	if got != "0" {
		t.Fatalf("expected incoming=0, got %s", got)
	}
}

func TestPersistIncoming_UnsetZero_StillDefaultsToOne(t *testing.T) {
	// Regressão: comportamento existente para todo caller que NÃO seta
	// IncomingSet continua tratando Incoming==0 como "não informado" -> 1.
	dynm := &mockDistDynamo{}
	svc := newDistSvc(dynm, &mockS3{}, &mockLambda{}, &mockSNS{}, testConfig())

	fields := DocFields{AccessKey: "22260811647612000197550000000000501454670090"}
	svc.persistIncoming(context.Background(), "hom#CNPJ_11647612000197", fields, docTypeConfigs["nfe"])

	put := dynm.lastPutItem("dfe_test_nfes")
	if put == nil {
		t.Fatal("expected a PutItem call")
	}
	got := put.Item["incoming"].(*types.AttributeValueMemberN).Value
	if got != "1" {
		t.Fatalf("expected incoming=1 (default), got %s", got)
	}
}
```

Se `mockDistDynamo` não tiver `lastPutItem(table string) *dynamodb.PutItemInput` nem `testConfig()` não existir, adicione-os no mesmo passo seguindo o padrão dos outros helpers de `distribution_test.go` (`newDistSvc`, `configItem`, etc.) — não são placeholders, apenas pequenos helpers de teste faltantes; implemente-os antes de rodar.

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd worker && go test ./internal/service/... -run TestPersistIncoming -v`
Expected: `TestPersistIncoming_ExplicitZero_IsNotCoercedToOne` FAIL (recebe "1", não "0"); o outro já passa hoje.

- [ ] **Step 3: Implementar**

```go
// distribution_parser.go:74 — adicionar campo logo após Incoming
	Incoming    int  // 0 means unset; callers treat 0 as 1, unless IncomingSet
	IncomingSet bool // true when Incoming was explicitly computed (e.g. import-by-XML emitida=0)
```

```go
// distribution.go:724-727
	incoming := fields.Incoming
	if incoming == 0 && !fields.IncomingSet {
		incoming = 1
	}
```

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd worker && go test ./internal/service/... -run TestPersistIncoming -v`
Expected: PASS (ambos os testes)

- [ ] **Step 5: Rodar toda a suíte do pacote (regressão)**

Run: `cd worker && go test ./internal/service/... -race`
Expected: PASS — nenhum outro teste quebra (todo caller existente não seta `IncomingSet`, comportamento idêntico).

- [ ] **Step 6: Atualizar CONDUCT.md**

Adicionar em `CONDUCT.md` uma nota curta: `DocFields.Incoming == 0` é ambíguo (não-setado vs. emitida) — sempre setar `IncomingSet=true` junto quando `Incoming=0` for um valor real, não um default.

- [ ] **Step 7: Commit**

```bash
git add worker/internal/service/distribution_parser.go worker/internal/service/distribution.go worker/internal/service/distribution_test.go CONDUCT.md
git commit -m "fix(worker): disambiguate Incoming=0 (emitida) from unset via IncomingSet"
```

---

### Task 3: worker — doc_type `nfce`, campo `StagingKey` e dispatch do job `import_xml`

**Files:**
- Modify: `worker/internal/service/distribution_parser.go` (`docTypeConfigs` map, ~linha 65-100)
- Modify: `worker/internal/service/distribution.go:144-151` (`DistributionMessage`), `:168-184` (`Process`)
- Test: `worker/internal/service/distribution_test.go`

**Interfaces:**
- Consumes: `docTypeConfig` struct (já existente).
- Produces: `docTypeConfigs["nfce"]`; `DistributionMessage.StagingKey string`; `Process` despacha `"import_xml"` para `s.runImportXML(ctx, msg.OrgPK, msg.DocType, msg.StagingKey, dtcfg)` (assinatura que a Task 6 implementa — nesta task, um stub que retorna `nil` é suficiente para fechar o teste de dispatch).

- [ ] **Step 1: Escrever o teste que falha**

```go
// worker/internal/service/distribution_test.go (adicionar)
func TestDistProcess_ImportXML_DispatchesToRunImportXML(t *testing.T) {
	dynm := &mockDistDynamo{}
	svc := newDistSvc(dynm, &mockS3{}, &mockLambda{}, &mockSNS{}, testConfig())

	err := svc.Process(context.Background(), DistributionMessage{
		JobType:    "import_xml",
		OrgPK:      "CNPJ_11647612000197",
		DocType:    "nfce",
		StagingKey: "nfce-import-staging/hom/CNPJ_11647612000197/abc.xml",
	})
	// stub de runImportXML nesta task apenas confirma que "nfce" resolve um
	// dtcfg válido e o job não cai em "unknown doc_type"/"unknown job_type".
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd worker && go test ./internal/service/... -run TestDistProcess_ImportXML -v`
Expected: FAIL (doc_type "nfce" não existe em `docTypeConfigs`, log "unknown doc_type" e retorno antecipado — o teste como está não distingue isso sem uma asserção mais forte; troque a asserção por checar que `dynm`/`s3m` não foi chamado E que nenhum log de doc_type desconhecido — na prática, para esta task, basta confirmar que compila e que `docTypeConfigs["nfce"]` existe: `if _, ok := docTypeConfigs["nfce"]; !ok { t.Fatal("nfce doc type config missing") }` antes do `Process`).

- [ ] **Step 3: Implementar**

```go
// distribution_parser.go — adicionar ao lado de "nfe" em docTypeConfigs
	"nfce": {
		// import_xml é o único job_type válido para nfce hoje — NFC-e nunca
		// passa por dist_nsu/cons_nsu/cons_ch_nfe (sem distribuição SEFAZ), e
		// a API nunca enfileira esses job_types para doc_type=nfce.
		uf:                "AN",
		xmlns:             nsNFe,
		version:           "1.01",
		configTableSuffix: "nfce_configs",
		docTable:          "nfces",
		eventsTable:       "nfce_events",
	},
```

```go
// distribution.go:144-151
type DistributionMessage struct {
	JobType    string `json:"job_type"`
	OrgPK      string `json:"org_pk"`
	DocType    string `json:"doc_type"`
	Trigger    string `json:"trigger"`
	NSU        *int   `json:"nsu,omitempty"`
	AccessKey  string `json:"access_key,omitempty"`
	StagingKey string `json:"staging_key,omitempty"`
}
```

```go
// distribution.go:168-184 — adicionar case antes do default
	case "import_xml":
		return s.runImportXML(ctx, msg.OrgPK, msg.DocType, msg.StagingKey, dtcfg)
```

Stub temporário (substituído na Task 6):

```go
func (s *DistributionService) runImportXML(ctx context.Context, orgPK, docType, stagingKey string, dtcfg docTypeConfig) error {
	return nil
}
```

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd worker && go test ./internal/service/... -run TestDistProcess_ImportXML -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/internal/service/distribution_parser.go worker/internal/service/distribution.go worker/internal/service/distribution_test.go
git commit -m "feat(worker): add nfce doc_type config and import_xml job dispatch stub"
```

---

### Task 4: worker — classificação emit > dest > transp

**Files:**
- Modify: `worker/internal/service/distribution_parser.go` (nova função + const de namespace)
- Test: `worker/internal/service/distribution_parser_test.go` (criar se não existir, ou adicionar ao arquivo de teste do parser existente)

**Interfaces:**
- Produces: `type importClassification struct { Incoming int; AccessKey string }` e `classifyImportXML(root *xmlEl, cnpj string) (importClassification, bool)` — `ok=false` quando nenhum CNPJ/CPF bate (rejeição). Usado pela Task 6.

- [ ] **Step 1: Escrever o teste que falha**

```go
// worker/internal/service/distribution_parser_test.go
package service

import "testing"

const sampleNfeProcXML = `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><NFe><infNFe Id="NFe22260811647612000197550000000000501454670090"><emit><CNPJ>11647612000197</CNPJ></emit><dest><CNPJ>22222222000122</CNPJ></dest></infNFe></NFe></nfeProc>`

func TestClassifyImportXML_EmitMatch_IsEmitida(t *testing.T) {
	root, err := parseXMLBytes([]byte(sampleNfeProcXML))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := classifyImportXML(root, "11647612000197")
	if !ok || got.Incoming != 0 {
		t.Fatalf("expected Incoming=0 (emitida), got %+v ok=%v", got, ok)
	}
}

func TestClassifyImportXML_DestMatch_WhenEmitDiffers_IsDestinada(t *testing.T) {
	root, _ := parseXMLBytes([]byte(sampleNfeProcXML))
	got, ok := classifyImportXML(root, "22222222000122")
	if !ok || got.Incoming != 1 {
		t.Fatalf("expected Incoming=1 (destinada), got %+v ok=%v", got, ok)
	}
}

func TestClassifyImportXML_TranspMatch_WhenEmitAndDestDiffer_IsTransportada(t *testing.T) {
	xml := `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe"><NFe><infNFe Id="NFe1"><emit><CNPJ>11647612000197</CNPJ></emit><dest><CNPJ>22222222000122</CNPJ></dest><transp><transporta><CNPJ>33333333000199</CNPJ></transporta></transp></infNFe></NFe></nfeProc>`
	root, _ := parseXMLBytes([]byte(xml))
	got, ok := classifyImportXML(root, "33333333000199")
	if !ok || got.Incoming != 2 {
		t.Fatalf("expected Incoming=2 (transportada), got %+v ok=%v", got, ok)
	}
}

func TestClassifyImportXML_NoMatch_IsRejected(t *testing.T) {
	root, _ := parseXMLBytes([]byte(sampleNfeProcXML))
	_, ok := classifyImportXML(root, "99999999000100")
	if ok {
		t.Fatal("expected ok=false when no party matches org CNPJ")
	}
}

func TestClassifyImportXML_EmitTakesPriorityOverDestAndTransp(t *testing.T) {
	// Mesmo CNPJ aparecendo como dest E transp — emit continua ganhando se
	// bater primeiro na prioridade (aqui emit não bate, mas dest e transp
	// batem com o mesmo CNPJ: dest deve vencer, nunca transp).
	xml := `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe"><NFe><infNFe Id="NFe1"><emit><CNPJ>11647612000197</CNPJ></emit><dest><CNPJ>22222222000122</CNPJ></dest><transp><transporta><CNPJ>22222222000122</CNPJ></transporta></transp></infNFe></NFe></nfeProc>`
	root, _ := parseXMLBytes([]byte(xml))
	got, ok := classifyImportXML(root, "22222222000122")
	if !ok || got.Incoming != 1 {
		t.Fatalf("expected dest (Incoming=1) to win over transp, got %+v ok=%v", got, ok)
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd worker && go test ./internal/service/... -run TestClassifyImportXML -v`
Expected: FAIL (`classifyImportXML` não existe)

- [ ] **Step 3: Implementar**

```go
// distribution_parser.go
type importClassification struct {
	Incoming  int
	AccessKey string
}

// classifyImportXML checks org membership in emit > dest > transp priority
// order against cnpj (org's own CPF/CNPJ), exactly this sequence — see
// docs/specs/2026-08-13-importacao-nfe-xml.md. ok=false means no relation
// to the org was found and the import must be rejected. This is import-XML
// specific: extractProcNFe (used by the normal distribution flow) never
// produces Incoming=0, since SEFAZ distribution never hands an org back a
// document it emitted itself.
func classifyImportXML(root *xmlEl, cnpj string) (importClassification, bool) {
	accessKey := findText(root, nsNFe, "chNFe", "Id")
	if strings.HasPrefix(accessKey, "NFe") {
		accessKey = accessKey[3:]
	}

	emit := findEl(root, nsNFe, "emit")
	dest := findEl(root, nsNFe, "dest")
	transp := findEl(root, nsNFe, "transporta")

	orgDoc := onlyDigits(cnpj)
	emitDoc := onlyDigits(findText(emit, nsNFe, "CNPJ", "CPF"))
	destDoc := onlyDigits(findText(dest, nsNFe, "CNPJ", "CPF"))
	transpDoc := onlyDigits(findText(transp, nsNFe, "CNPJ", "CPF"))

	switch {
	case emitDoc != "" && emitDoc == orgDoc:
		return importClassification{Incoming: 0, AccessKey: accessKey}, true
	case destDoc != "" && destDoc == orgDoc:
		return importClassification{Incoming: 1, AccessKey: accessKey}, true
	case transpDoc != "" && transpDoc == orgDoc:
		return importClassification{Incoming: 2, AccessKey: accessKey}, true
	default:
		return importClassification{AccessKey: accessKey}, false
	}
}
```

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd worker && go test ./internal/service/... -run TestClassifyImportXML -v`
Expected: PASS (todos os 5 casos)

- [ ] **Step 5: Commit**

```bash
git add worker/internal/service/distribution_parser.go worker/internal/service/distribution_parser_test.go
git commit -m "feat(worker): classify XML-import documents by emit>dest>transp priority"
```

---

### Task 5: worker — validação de raiz, comparação de digest e montagem do `nfeProc` final

**Files:**
- Modify: `worker/internal/service/distribution_parser.go` (novo const `nsXMLDSig`, novas funções)
- Test: `worker/internal/service/distribution_parser_test.go`

**Interfaces:**
- Consumes: `godfe.BuildXMLFragment` (Task 1), `xmlEl`/`findEl`/`findText`/`parseXMLBytes` (existentes).
- Produces:
  - `validImportRoot(root *xmlEl) bool` — `true` só para raiz `nfeProc` ou `NFe`.
  - `compareImportDigests(root *xmlEl, sefazDigVal string) bool`.
  - `buildFinalNfeProc(originalXML []byte, root *xmlEl, protNFeDict map[string]any) ([]byte, error)` — retorna `originalXML` inalterado quando `root.Local == "nfeProc"`; monta e retorna o `nfeProc` sintetizado quando `root.Local == "NFe"`.

- [ ] **Step 1: Escrever o teste que falha**

Usar o arquivo de referência do usuário como fixture: copie-o para
`worker/internal/service/testdata/nfeproc_sample.xml` (mesmo conteúdo de
`/home/artur/Downloads/22260811647612000197550000000000501454670090.xml`).

```go
// worker/internal/service/distribution_parser_test.go (adicionar)
import (
	"os"
	"strings"
	"testing"
)

func loadSampleNfeProc(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/nfeproc_sample.xml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestValidImportRoot_AcceptsNfeProcAndBareNFe(t *testing.T) {
	nfeProcRoot, _ := parseXMLBytes(loadSampleNfeProc(t))
	if !validImportRoot(nfeProcRoot) {
		t.Fatal("expected nfeProc root to be valid")
	}
	bareRoot, _ := parseXMLBytes([]byte(`<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe1"></infNFe></NFe>`))
	if !validImportRoot(bareRoot) {
		t.Fatal("expected bare NFe root to be valid")
	}
	otherRoot, _ := parseXMLBytes([]byte(`<resNFe xmlns="http://www.portalfiscal.inf.br/nfe"></resNFe>`))
	if validImportRoot(otherRoot) {
		t.Fatal("expected resNFe root to be rejected")
	}
}

func TestCompareImportDigests_NfeProc_AllThreeMustMatch(t *testing.T) {
	root, _ := parseXMLBytes(loadSampleNfeProc(t))
	const matchingDigest = "cKFyNtF4cg+d63/SRv0ezXGoef8="
	if !compareImportDigests(root, matchingDigest) {
		t.Fatal("expected match: fixture's protNFe/digVal and Signature/DigestValue are both this value")
	}
	if compareImportDigests(root, "different-digest") {
		t.Fatal("expected mismatch to be rejected")
	}
}

func TestCompareImportDigests_BareNFe_ComparesOnlySignatureDigest(t *testing.T) {
	full := string(loadSampleNfeProc(t))
	// Extrai só o <NFe>...</NFe> (sem protNFe) para simular upload sem protocolo.
	start := strings.Index(full, "<NFe>")
	end := strings.Index(full, "</NFe>") + len("</NFe>")
	bareXML := full[start:end]
	root, err := parseXMLBytes([]byte(bareXML))
	if err != nil {
		t.Fatal(err)
	}
	const sigDigest = "cKFyNtF4cg+d63/SRv0ezXGoef8=" // mesmo valor no fixture (Signature/DigestValue)
	if !compareImportDigests(root, sigDigest) {
		t.Fatal("expected match against Signature/DigestValue")
	}
	if compareImportDigests(root, "different-digest") {
		t.Fatal("expected mismatch to be rejected")
	}
}

func TestBuildFinalNfeProc_NfeProcRoot_ReturnsUnchanged(t *testing.T) {
	original := loadSampleNfeProc(t)
	root, _ := parseXMLBytes(original)
	out, err := buildFinalNfeProc(original, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(original) {
		t.Fatal("expected nfeProc input to pass through unchanged")
	}
}

func TestBuildFinalNfeProc_BareNFe_JoinsProtNFe(t *testing.T) {
	full := string(loadSampleNfeProc(t))
	start := strings.Index(full, "<NFe>")
	end := strings.Index(full, "</NFe>") + len("</NFe>")
	bareXML := []byte(full[start:end])
	root, err := parseXMLBytes(bareXML)
	if err != nil {
		t.Fatal(err)
	}
	protNFeDict := map[string]any{
		"@versao": "4.00",
		"infProt": map[string]any{
			"tpAmb":    "2",
			"chNFe":    "22260811647612000197550000000000501454670090",
			"dhRecbto": "2026-08-08T17:05:06-03:00",
			"nProt":    "322260000016670",
			"digVal":   "cKFyNtF4cg+d63/SRv0ezXGoef8=",
			"cStat":    "100",
			"xMotivo":  "Autorizado o uso da NF-e",
		},
	}
	out, err := buildFinalNfeProc(bareXML, root, protNFeDict)
	if err != nil {
		t.Fatal(err)
	}
	joinedRoot, err := parseXMLBytes(out)
	if err != nil {
		t.Fatalf("joined output is not valid xml: %v\n%s", err, out)
	}
	if joinedRoot.Local != "nfeProc" {
		t.Fatalf("expected joined root to be nfeProc, got %s", joinedRoot.Local)
	}
	if findText(joinedRoot, nsNFe, "digVal") != "cKFyNtF4cg+d63/SRv0ezXGoef8=" {
		t.Fatalf("joined protNFe digVal missing/wrong: %s", out)
	}
	if findText(joinedRoot, nsNFe, "chNFe", "Id") == "" {
		t.Fatal("expected original NFe content (chNFe/Id) preserved in joined output")
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd worker && go test ./internal/service/... -run 'TestValidImportRoot|TestCompareImportDigests|TestBuildFinalNfeProc' -v`
Expected: FAIL (funções não existem)

- [ ] **Step 3: Implementar**

```go
// distribution_parser.go
const nsXMLDSig = "http://www.w3.org/2000/09/xmldsig#"

// validImportRoot reports whether root's tag is an accepted XML-import root
// (nfeProc — with protocol — or bare NFe — signed but not yet queried).
func validImportRoot(root *xmlEl) bool {
	if root == nil {
		return false
	}
	return root.Local == "nfeProc" || root.Local == "NFe"
}

// compareImportDigests validates the SEFAZ-returned digVal (from a
// consulta protocolo response) against the uploaded XML's own digest(s):
// for nfeProc, BOTH the uploaded protNFe/infProt/digVal and the uploaded
// Signature/SignedInfo/Reference/DigestValue must match; for a bare NFe
// (no protocol yet), only the Signature DigestValue is compared. See
// docs/specs/2026-08-13-importacao-nfe-xml.md.
func compareImportDigests(root *xmlEl, sefazDigVal string) bool {
	if sefazDigVal == "" {
		return false
	}
	sigDigVal := ""
	if sig := findEl(root, nsXMLDSig, "Signature"); sig != nil {
		sigDigVal = findText(sig, nsXMLDSig, "DigestValue")
	}
	if sigDigVal == "" || sigDigVal != sefazDigVal {
		return false
	}
	if root.Local == "nfeProc" {
		uploadedProtDigVal := findText(root, nsNFe, "digVal")
		return uploadedProtDigVal != "" && uploadedProtDigVal == sefazDigVal
	}
	return true
}

// buildFinalNfeProc returns the canonical nfeProc document to persist. When
// the uploaded root is already nfeProc, originalXML is returned unchanged.
// When the uploaded root is a bare NFe (no protocol), it wraps the original
// NFe bytes verbatim (the signature depends on exact byte content — it is
// never re-serialized) together with the protNFe fragment built from
// protNFeDict (the dict a consulta protocolo response carries) via
// dfe.BuildXMLFragment, adding the nfe namespace, mirroring the reference
// file used to design this feature.
func buildFinalNfeProc(originalXML []byte, root *xmlEl, protNFeDict map[string]any) ([]byte, error) {
	if root.Local == "nfeProc" {
		return originalXML, nil
	}
	nfeBytes := bytes.TrimSpace(originalXML)
	if idx := bytes.Index(nfeBytes, []byte("?>")); bytes.HasPrefix(nfeBytes, []byte("<?xml")) && idx >= 0 {
		nfeBytes = bytes.TrimSpace(nfeBytes[idx+2:])
	}
	protFragment, err := godfe.BuildXMLFragment(protNFeDict, "protNFe", nsNFe)
	if err != nil {
		return nil, fmt.Errorf("build protNFe fragment: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?><nfeProc versao="4.00" xmlns="` + nsNFe + `">`)
	buf.Write(nfeBytes)
	buf.Write(protFragment)
	buf.WriteString(`</nfeProc>`)
	return buf.Bytes(), nil
}
```

`godfe` já está importado neste pacote (`worker/internal/service/godfe_shadow.go`, mesmo `package service`) — sem novo import necessário.

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd worker && go test ./internal/service/... -run 'TestValidImportRoot|TestCompareImportDigests|TestBuildFinalNfeProc' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add worker/internal/service/distribution_parser.go worker/internal/service/distribution_parser_test.go worker/internal/service/testdata/nfeproc_sample.xml
git commit -m "feat(worker): validate XML-import root, compare digests, join bare-NFe protocol"
```

---

### Task 6: worker — `runImportXML` (orquestração completa)

**Files:**
- Modify: `worker/internal/service/distribution.go` (substitui o stub da Task 3; novo `buildConsultaProtocoloPayload`, `notifyImportFailure`, `downloadStaging`/`deleteStaging` se ainda não existir um helper de download genérico)
- Test: `worker/internal/service/distribution_test.go`

**Interfaces:**
- Consumes: `classifyImportXML`, `validImportRoot`, `compareImportDigests`, `buildFinalNfeProc` (Tasks 4-5), `extractProcNFe`, `persistIncoming`, `persistCounterparties`, `persistEvent`, `notifyResult`, `loadConfig`, `loadCert`, `loadOrg`, `getCertB64`, `checkConsQuota`, `invokePyDfe`, `extractCNPJ`, `extractUF` (todos já existentes em `distribution.go`).
- Produces: `runImportXML(ctx, orgPK, docType, stagingKey string, dtcfg docTypeConfig) error` (assinatura final — substitui o stub); `notifyImportFailure(ctx, orgPK, docType, accessKey, reason string)`.

- [ ] **Step 1: Escrever os testes que falham**

Reaproveitar `newDistSvc`/`mockDistDynamo`/`mockS3`/`mockSNS`/`configItem`/`certItem`/`orgItemWithUF` de `distribution_test.go`. `mockS3` precisa suportar `GetObject` retornando o XML de staging — se ainda não suportar, estenda o mock no mesmo passo (não é um placeholder: é o mesmo mock já usado por `getCertB64`/`downloadDocs`, só adicionando mais uma entrada ao mapa de conteúdo por chave).

```go
// worker/internal/service/distribution_test.go (adicionar)

func consultaProtocoloResp(cStat, digVal string, hasProtocol bool) []byte {
	protNFe := ""
	if hasProtocol {
		protNFe = fmt.Sprintf(`,"protNFe":{"@versao":"4.00","infProt":{"tpAmb":"2","chNFe":"22260811647612000197550000000000501454670090","dhRecbto":"2026-08-08T17:05:06-03:00","nProt":"322260000016670","digVal":"%s","cStat":"%s","xMotivo":"Autorizado o uso da NF-e"}}`, digVal, cStat)
	}
	body := fmt.Sprintf(`{"retConsSitNFe":{"tpAmb":"2","cStat":"%s","xMotivo":"ok"%s}}`, cStat, protNFe)
	payload := map[string]any{"statusCode": float64(200), "body": body}
	b, _ := json.Marshal(payload)
	return b
}

func TestRunImportXML_Happy_NfeProc_PersistsAsEmitida(t *testing.T) {
	dynm := &mockDistDynamo{items: map[string]map[string]types.AttributeValue{
		"dfe_test_organization_nfe_configs#CNPJ_11647612000197": configItem(2, "hom", "", ""),
	}}
	s3m := &mockS3{
		objects: map[string][]byte{
			"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml": loadSampleNfeProcForTest(t),
		},
	}
	lamm := &mockLambda{} // não deve ser chamado — go-dfe.Implements("nfe","NfeConsultaProtocolo")==true
	snsm := &mockSNS{}
	svc := newDistSvc(dynm, s3m, lamm, snsm, testConfig())
	// força o caminho go-dfe (Implements=true) a devolver a resposta simulada
	// em vez de tentar mTLS de verdade — mesmo padrão de
	// TestDistNSU_GoDfeCutover_SkipsLambdaEntirely: sobrescrever godfeCall.
	origCall := godfeCall
	godfeCall = func(ctx context.Context, req godfe.Request) (godfe.Response, error) {
		return godfe.Response{StatusCode: 200, Body: string(consultaProtocoloResp("100", "cKFyNtF4cg+d63/SRv0ezXGoef8=", true))}, nil
	}
	defer func() { godfeCall = origCall }()

	err := svc.runImportXML(context.Background(), "CNPJ_11647612000197", "nfe",
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml", docTypeConfigs["nfe"])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	put := dynm.lastPutItem("dfe_test_nfes")
	if put == nil {
		t.Fatal("expected nfes PutItem")
	}
	if got := put.Item["incoming"].(*types.AttributeValueMemberN).Value; got != "0" {
		t.Fatalf("expected incoming=0 (emitida — CNPJ do org bate com emit no fixture), got %s", got)
	}
	if len(snsm.published) == 0 {
		t.Fatal("expected notifyResult SNS publish on success")
	}
	if s3m.deleted["nfe-import-staging/hom/CNPJ_11647612000197/abc.xml"] != true {
		t.Fatal("expected staging object to be deleted after success")
	}
}

func TestRunImportXML_InvalidRoot_RejectsWithoutRetry(t *testing.T) {
	dynm := &mockDistDynamo{items: map[string]map[string]types.AttributeValue{
		"dfe_test_organization_nfe_configs#CNPJ_11647612000197": configItem(2, "hom", "", ""),
	}}
	s3m := &mockS3{objects: map[string][]byte{
		"nfe-import-staging/hom/CNPJ_11647612000197/bad.xml": []byte(`<resNFe xmlns="http://www.portalfiscal.inf.br/nfe"></resNFe>`),
	}}
	snsm := &mockSNS{}
	svc := newDistSvc(dynm, s3m, &mockLambda{}, snsm, testConfig())

	err := svc.runImportXML(context.Background(), "CNPJ_11647612000197", "nfe",
		"nfe-import-staging/hom/CNPJ_11647612000197/bad.xml", docTypeConfigs["nfe"])
	if err != nil {
		t.Fatalf("business rejection must return nil, not error: %v", err)
	}
	if len(snsm.published) == 0 {
		t.Fatal("expected a failure notification to be published")
	}
	if s3m.deleted["nfe-import-staging/hom/CNPJ_11647612000197/bad.xml"] != true {
		t.Fatal("expected staging object to be deleted after rejection")
	}
}

func TestRunImportXML_NoOrgMatch_RejectsWithoutRetry(t *testing.T) {
	dynm := &mockDistDynamo{items: map[string]map[string]types.AttributeValue{
		"dfe_test_organization_nfe_configs#CNPJ_99999999000100": configItem(2, "hom", "", ""),
	}}
	s3m := &mockS3{objects: map[string][]byte{
		"nfe-import-staging/hom/CNPJ_99999999000100/abc.xml": loadSampleNfeProcForTest(t),
	}}
	svc := newDistSvc(dynm, s3m, &mockLambda{}, &mockSNS{}, testConfig())

	err := svc.runImportXML(context.Background(), "CNPJ_99999999000100", "nfe",
		"nfe-import-staging/hom/CNPJ_99999999000100/abc.xml", docTypeConfigs["nfe"])
	if err != nil {
		t.Fatalf("business rejection must return nil, not error: %v", err)
	}
	if dynm.lastPutItem("dfe_test_nfes") != nil {
		t.Fatal("no document should be persisted when no party matches the org")
	}
}

func TestRunImportXML_DigestMismatch_RejectsWithoutRetry(t *testing.T) {
	dynm := &mockDistDynamo{items: map[string]map[string]types.AttributeValue{
		"dfe_test_organization_nfe_configs#CNPJ_11647612000197": configItem(2, "hom", "", ""),
	}}
	s3m := &mockS3{objects: map[string][]byte{
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml": loadSampleNfeProcForTest(t),
	}}
	origCall := godfeCall
	godfeCall = func(ctx context.Context, req godfe.Request) (godfe.Response, error) {
		return godfe.Response{StatusCode: 200, Body: string(consultaProtocoloResp("100", "digest-que-nao-bate", true))}, nil
	}
	defer func() { godfeCall = origCall }()
	svc := newDistSvc(dynm, s3m, &mockLambda{}, &mockSNS{}, testConfig())

	err := svc.runImportXML(context.Background(), "CNPJ_11647612000197", "nfe",
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml", docTypeConfigs["nfe"])
	if err != nil {
		t.Fatalf("business rejection must return nil, not error: %v", err)
	}
	if dynm.lastPutItem("dfe_test_nfes") != nil {
		t.Fatal("no document should be persisted on digest mismatch")
	}
}

func TestRunImportXML_AlreadyCompleteDocument_RejectsWithoutRetry(t *testing.T) {
	dynm := &mockDistDynamo{items: map[string]map[string]types.AttributeValue{
		"dfe_test_organization_nfe_configs#CNPJ_11647612000197": configItem(2, "hom", "", ""),
		"dfe_test_nfes#hom#CNPJ_11647612000197#22260811647612000197550000000000501454670090": {
			"pk":       dynS("pk", "hom#CNPJ_11647612000197"),
			"sk":       dynS("sk", "22260811647612000197550000000000501454670090"),
			"products": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
		},
	}}
	s3m := &mockS3{objects: map[string][]byte{
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml": loadSampleNfeProcForTest(t),
	}}
	svc := newDistSvc(dynm, s3m, &mockLambda{}, &mockSNS{}, testConfig())

	err := svc.runImportXML(context.Background(), "CNPJ_11647612000197", "nfe",
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml", docTypeConfigs["nfe"])
	if err != nil {
		t.Fatalf("business rejection must return nil, not error: %v", err)
	}
}

func TestRunImportXML_SefazBusinessRejection_NotRetried(t *testing.T) {
	dynm := &mockDistDynamo{items: map[string]map[string]types.AttributeValue{
		"dfe_test_organization_nfe_configs#CNPJ_11647612000197": configItem(2, "hom", "", ""),
	}}
	s3m := &mockS3{objects: map[string][]byte{
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml": loadSampleNfeProcForTest(t),
	}}
	origCall := godfeCall
	godfeCall = func(ctx context.Context, req godfe.Request) (godfe.Response, error) {
		return godfe.Response{StatusCode: 200, Body: string(consultaProtocoloResp("217", "", false))}, nil // 217: NF-e não consta na SEFAZ
	}
	defer func() { godfeCall = origCall }()
	svc := newDistSvc(dynm, s3m, &mockLambda{}, &mockSNS{}, testConfig())

	err := svc.runImportXML(context.Background(), "CNPJ_11647612000197", "nfe",
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml", docTypeConfigs["nfe"])
	if err != nil {
		t.Fatalf("SEFAZ business rejection must return nil (no retry): %v", err)
	}
}

func TestRunImportXML_NetworkError_ReturnsErrorForRetry(t *testing.T) {
	dynm := &mockDistDynamo{items: map[string]map[string]types.AttributeValue{
		"dfe_test_organization_nfe_configs#CNPJ_11647612000197": configItem(2, "hom", "", ""),
	}}
	s3m := &mockS3{objects: map[string][]byte{
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml": loadSampleNfeProcForTest(t),
	}}
	origCall := godfeCall
	godfeCall = func(ctx context.Context, req godfe.Request) (godfe.Response, error) {
		return godfe.Response{}, errors.New("connection reset")
	}
	defer func() { godfeCall = origCall }()
	svc := newDistSvc(dynm, s3m, &mockLambda{}, &mockSNS{}, testConfig())

	err := svc.runImportXML(context.Background(), "CNPJ_11647612000197", "nfe",
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml", docTypeConfigs["nfe"])
	if err == nil {
		t.Fatal("network/timeout error must be returned so SQS retries")
	}
}

func TestRunImportXML_DuplicateMessage_IsIdempotent(t *testing.T) {
	// Mesma mensagem processada duas vezes não deve duplicar o documento —
	// a segunda chamada encontra o documento já persistido (com produtos) e
	// rejeita como "já completa" em vez de tentar persistir de novo.
	dynm := &mockDistDynamo{items: map[string]map[string]types.AttributeValue{
		"dfe_test_organization_nfe_configs#CNPJ_11647612000197": configItem(2, "hom", "", ""),
	}}
	s3m := &mockS3{objects: map[string][]byte{
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml": loadSampleNfeProcForTest(t),
	}}
	origCall := godfeCall
	godfeCall = func(ctx context.Context, req godfe.Request) (godfe.Response, error) {
		return godfe.Response{StatusCode: 200, Body: string(consultaProtocoloResp("100", "cKFyNtF4cg+d63/SRv0ezXGoef8=", true))}, nil
	}
	defer func() { godfeCall = origCall }()
	svc := newDistSvc(dynm, s3m, &mockLambda{}, &mockSNS{}, testConfig())

	if err := svc.runImportXML(context.Background(), "CNPJ_11647612000197", "nfe",
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml", docTypeConfigs["nfe"]); err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	firstPutCount := dynm.putCount("dfe_test_nfes")

	// re-adiciona o objeto de staging (a primeira chamada o deletou) para
	// simular a mesma mensagem SQS entregue de novo.
	s3m.objects["nfe-import-staging/hom/CNPJ_11647612000197/abc.xml"] = loadSampleNfeProcForTest(t)

	if err := svc.runImportXML(context.Background(), "CNPJ_11647612000197", "nfe",
		"nfe-import-staging/hom/CNPJ_11647612000197/abc.xml", docTypeConfigs["nfe"]); err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if dynm.putCount("dfe_test_nfes") != firstPutCount {
		t.Fatal("duplicate message must not persist the document a second time")
	}
}
```

`loadSampleNfeProcForTest` é o mesmo `loadSampleNfeProc` da Task 5 (renomeie para reuso entre os dois arquivos de teste, ou mantenha um só helper compartilhado em `distribution_test.go`). `mockS3.deleted map[string]bool` e `mockDistDynamo.putCount(table string) int` são pequenos acréscimos aos mocks existentes, no mesmo espírito de `lastPutItem` da Task 2 — implemente-os neste passo.

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd worker && go test ./internal/service/... -run TestRunImportXML -v`
Expected: FAIL (stub da Task 3 retorna `nil` sem persistir/notificar nada)

- [ ] **Step 3: Implementar**

```go
// distribution.go — substitui o stub da Task 3

func (s *DistributionService) runImportXML(ctx context.Context, orgPK, docType, stagingKey string, dtcfg docTypeConfig) error {
	configTable := fmt.Sprintf("%s_organization_%s", s.cfg.TablePrefix, dtcfg.configTableSuffix)
	cfg, err := s.loadConfig(ctx, orgPK, configTable)
	if err != nil || cfg == nil {
		slog.Warn("import_xml: no config found", "org_pk", orgPK, "doc_type", docType)
		return nil
	}
	environment := attrN(cfg, "environment", 2)
	envPrefix := envHom
	sefazEnv := sefazEnvHom
	if environment == 1 {
		envPrefix = envProd
		sefazEnv = sefazEnvProd
	}
	if !s.checkConsQuota(ctx, orgPK, configTable, cfg, envPrefix, time.Now().UTC()) {
		slog.Warn("import_xml quota exceeded — dropping duplicate job", "org_pk", orgPK)
		return nil
	}

	xmlBytes, err := s.downloadStaging(ctx, stagingKey)
	if err != nil {
		return fmt.Errorf("download staging xml: %w", err)
	}

	root, parseErr := parseXMLBytes(xmlBytes)
	if parseErr != nil || !validImportRoot(root) {
		s.notifyImportFailure(ctx, orgPK, docType, "", "XML inválido: raiz deve ser nfeProc ou NFe")
		s.deleteStaging(ctx, stagingKey)
		return nil
	}

	cnpj := extractCNPJ(orgPK)
	class, ok := classifyImportXML(root, cnpj)
	if !ok {
		s.notifyImportFailure(ctx, orgPK, docType, class.AccessKey, "XML não pertence à organização")
		s.deleteStaging(ctx, stagingKey)
		return nil
	}

	docPK := envPrefix + "#" + orgPK
	docTable := s.cfg.TablePrefix + "_" + dtcfg.docTable
	existing, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(docTable),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: docPK},
			"sk": &types.AttributeValueMemberS{Value: class.AccessKey},
		},
	})
	if err == nil && existing != nil && existing.Item != nil {
		if _, hasProducts := existing.Item["products"]; hasProducts {
			s.notifyImportFailure(ctx, orgPK, docType, class.AccessKey, "NF-e já existe completa")
			s.deleteStaging(ctx, stagingKey)
			return nil
		}
	}

	cert, err := s.loadCert(ctx, orgPK, dtcfg.configTableSuffix)
	if err != nil || cert == nil {
		return nil
	}
	certB64, err := s.getCertB64(ctx, attrS(cert, "s3_key"))
	if err != nil {
		return err
	}
	certPassword := attrS(cert, "password")
	uf := dtcfg.uf

	payload := s.buildConsultaProtocoloPayload(cnpj, certB64, certPassword, uf, sefazEnv, docType, class.AccessKey)
	resp, err := s.invokePyDfe(ctx, payload)
	if err != nil {
		return fmt.Errorf("consulta protocolo: %w", err)
	}
	if int(getFloat(resp, "statusCode")) != 200 {
		s.notifyImportFailure(ctx, orgPK, docType, class.AccessKey, "falha na consulta protocolo SEFAZ")
		s.deleteStaging(ctx, stagingKey)
		return nil
	}

	var respBody map[string]any
	if b, ok := resp["body"].(string); ok {
		_ = json.Unmarshal([]byte(b), &respBody)
	}
	ret := asMap(respBody, "retConsSitNFe")
	cStat := mapStr(ret, "cStat", "")
	if cStat != "100" && cStat != "150" {
		s.notifyImportFailure(ctx, orgPK, docType, class.AccessKey,
			fmt.Sprintf("SEFAZ rejeitou a consulta: %s", mapStr(ret, "xMotivo", cStat)))
		s.deleteStaging(ctx, stagingKey)
		return nil
	}

	protNFeDict := asMap(ret, "protNFe")
	sefazDigVal := mapStr(asMap(protNFeDict, "infProt"), "digVal", "")
	if !compareImportDigests(root, sefazDigVal) {
		s.notifyImportFailure(ctx, orgPK, docType, class.AccessKey, "divergência de assinatura entre o XML enviado e a SEFAZ")
		s.deleteStaging(ctx, stagingKey)
		return nil
	}

	finalXML, err := buildFinalNfeProc(xmlBytes, root, protNFeDict)
	if err != nil {
		return fmt.Errorf("build final nfeProc: %w", err)
	}
	finalRoot, err := parseXMLBytes(finalXML)
	if err != nil {
		return fmt.Errorf("parse final nfeProc: %w", err)
	}

	fields := extractProcNFe(finalRoot, cnpj)
	fields.Incoming = class.Incoming
	fields.IncomingSet = true

	docS3Key := fmt.Sprintf("%s/%s/%s/%s.xml", docType, envPrefix, orgPK, class.AccessKey)
	if _, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.DocumentsBucket),
		Key:         aws.String(docS3Key),
		Body:        bytes.NewReader(finalXML),
		ContentType: aws.String("application/xml"),
	}); err != nil {
		return fmt.Errorf("upload final xml: %w", err)
	}
	fields.XMLS3Key = docS3Key

	s.persistIncoming(ctx, docPK, fields, dtcfg)
	if err := s.persistCounterparties(ctx, orgPK, cnpj, fields); err != nil {
		return err
	}
	for _, ev := range asSlice(ret, "procEventoNFe") {
		evMap, _ := ev.(map[string]any)
		if evMap == nil {
			continue
		}
		infEvento := asMap(evMap, "infEvento")
		s.persistEvent(ctx, DocFields{
			AccessKey:      class.AccessKey,
			EventType:      mapStr(infEvento, "tpEvento", ""),
			SequenceNumber: mapStr(infEvento, "nSeqEvento", "1"),
			SefazStatus:    mapStr(infEvento, "cStat", ""),
			SefazMotive:    mapStr(infEvento, "xMotivo", ""),
			SefazProtocol:  mapStr(infEvento, "nProt", ""),
			DHEvento:       mapStr(infEvento, "dhEvento", ""),
			XMLS3Key:       docS3Key,
		}, dtcfg)
	}

	s.notifyResult(ctx, orgPK, fields, 0, dtcfg)
	s.deleteStaging(ctx, stagingKey)
	return nil
}

// buildConsultaProtocoloPayload builds the generic Request payload shape
// (see mapToDfeRequest/invokePyDfe) for a one-off NfeConsultaProtocolo call —
// distinct from buildPayload, whose "service" is fixed to dtcfg.sefazService
// (the doc type's *distribution* service, not consulta protocolo).
func (s *DistributionService) buildConsultaProtocoloPayload(cnpj, certB64, certPassword, uf, sefazEnv, docType, accessKey string) map[string]any {
	environmentStr := "2"
	if sefazEnv == sefazEnvProd {
		environmentStr = "1"
	}
	return map[string]any{
		"cnpj":                 cnpj,
		"certificate_b64":      certB64,
		"certificate_password": certPassword,
		"uf":                   uf,
		"environment":          sefazEnv,
		"doc_type":             docType,
		"service":              "NfeConsultaProtocolo",
		"validate_schema":      false,
		"max_retries":          2,
		"body": map[string]any{
			"consSitNFe": map[string]any{
				"@versao": "4.00",
				"@xmlns":  nsNFe,
				"tpAmb":   environmentStr,
				"xServ":   "CONSULTAR",
				"chNFe":   accessKey,
			},
		},
	}
}

// notifyImportFailure publishes a business-rejection notification for the
// XML-import flow — the failure counterpart to notifyResult (success only).
// The frontend's useRealtimeUpdates.ts shows an error toast for this type.
func (s *DistributionService) notifyImportFailure(ctx context.Context, orgPK, docType, accessKey, reason string) {
	if s.cfg.ResultsTopicARN == "" || s.sns == nil {
		return
	}
	msg, _ := json.Marshal(map[string]any{
		"type":       "import_xml_failed",
		"org_pk":     orgPK,
		"doc_type":   docType,
		"access_key": accessKey,
		"reason":     reason,
	})
	if _, err := s.sns.Publish(ctx, snsInput(s.cfg.ResultsTopicARN, string(msg))); err != nil {
		slog.Warn("failed to notify import failure", "access_key", accessKey, "err", err)
	}
}

// downloadStaging/deleteStaging wrap the S3 staging object created by the
// api layer (DistributionService.ImportXML) before enqueueing.
func (s *DistributionService) downloadStaging(ctx context.Context, key string) ([]byte, error) {
	out, err := s.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.cfg.DocumentsBucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *DistributionService) deleteStaging(ctx context.Context, key string) {
	if _, err := s.s3.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.DocumentsBucket), Key: aws.String(key)}); err != nil {
		slog.Warn("failed to delete staging object", "key", key, "err", err)
	}
}
```

Se a interface `S3Client` (usada por `DistributionService.s3`) ainda não expuser `GetObject`/`DeleteObject`, adicione-os à interface no mesmo passo (mesmo arquivo onde `S3Client` é declarado) — são métodos padrão do SDK `s3.Client`, só precisam aparecer na interface local.

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd worker && go test ./internal/service/... -run TestRunImportXML -v`
Expected: PASS (todos os 8 casos)

- [ ] **Step 5: Rodar toda a suíte do worker (regressão)**

Run: `cd worker && go test ./... -race`
Expected: PASS

- [ ] **Step 6: Atualizar DOCS.md**

Adicionar nota em `DOCS.md` (seção worker/distribuição): job `import_xml`, e que esta é a primeira chamada real de `NfeConsultaProtocolo` via `go-dfe` em produção (operação já estava em `dfe.Implements`, nunca invocada por nenhum caller até aqui).

- [ ] **Step 7: Commit**

```bash
git add worker/internal/service/distribution.go worker/internal/service/distribution_test.go DOCS.md
git commit -m "feat(worker): implement runImportXML — first real NfeConsultaProtocolo caller"
```

---

### Task 7: api — validação de upload (tamanho, doc_type, raiz)

**Files:**
- Create: `api/internal/services/import_xml_validation.go`
- Test: `api/internal/services/import_xml_validation_test.go`

**Interfaces:**
- Produces:
  - `const maxImportXMLSize = 1 << 20` (1 MiB)
  - `validImportDocType(docType string) bool` — só `nfe`/`nfce`.
  - `peekXMLRoot(xmlBytes []byte) (string, error)` — retorna o nome local do elemento raiz sem parse completo.

- [ ] **Step 1: Escrever o teste que falha**

```go
// api/internal/services/import_xml_validation_test.go
package services

import "testing"

func TestValidImportDocType(t *testing.T) {
	cases := map[string]bool{"nfe": true, "nfce": true, "cte": false, "mdfe": false, "nfse": false, "": false}
	for docType, want := range cases {
		if got := validImportDocType(docType); got != want {
			t.Errorf("validImportDocType(%q) = %v, want %v", docType, got, want)
		}
	}
}

func TestPeekXMLRoot_AcceptsAndRejects(t *testing.T) {
	cases := []struct {
		name    string
		xml     string
		want    string
		wantErr bool
	}{
		{"nfeProc", `<nfeProc xmlns="x"><NFe/></nfeProc>`, "nfeProc", false},
		{"bare NFe", `<NFe xmlns="x"><infNFe/></NFe>`, "NFe", false},
		{"other root", `<resNFe xmlns="x"/>`, "resNFe", false},
		{"malformed", `<nfeProc><NFe`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := peekXMLRoot([]byte(tc.xml))
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd api && go test ./internal/services/... -run 'TestValidImportDocType|TestPeekXMLRoot' -v`
Expected: FAIL (funções não existem)

- [ ] **Step 3: Implementar**

```go
// api/internal/services/import_xml_validation.go
package services

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// maxImportXMLSize bounds the multipart file accepted by
// POST /distributions/{doc_type}/import-xml — generous for a single NF-e/
// NFC-e document (typically a few KB to a few hundred KB), rejects anything
// clearly not a single fiscal document upload.
const maxImportXMLSize = 1 << 20 // 1 MiB

var importXMLDocTypes = map[string]bool{DocTypeNFe: true, DocTypeNFCe: true}

// validImportDocType restricts XML import to nfe/nfce — CT-e/MDF-e/NFS-e are
// out of scope for this feature (docs/specs/2026-08-13-importacao-nfe-xml.md).
func validImportDocType(docType string) bool {
	return importXMLDocTypes[docType]
}

// peekXMLRoot reads only the first StartElement token — no full parse — and
// returns its local name, so the caller can fail fast on an unsupported root
// before spending S3/SQS/SEFAZ-quota on an upload that will be rejected
// anyway. Full structural validation happens in the worker.
func peekXMLRoot(xmlBytes []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return "", fmt.Errorf("xml sem elemento raiz")
		}
		if err != nil {
			return "", fmt.Errorf("xml malformado: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name.Local, nil
		}
	}
}
```

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd api && go test ./internal/services/... -run 'TestValidImportDocType|TestPeekXMLRoot' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/import_xml_validation.go api/internal/services/import_xml_validation_test.go
git commit -m "feat(api): add doc_type/size/root validation for XML import"
```

---

### Task 8: api — expor `NfceConfig` em `DistributionService` (`fiscalCfg`)

`DistributionService.fiscalCfg(docType)` (`api/internal/services/distributions.go:86-98`) hoje só resolve `nfe`/`cte`/`mdfe`/`nfse` — não tem case para `nfce`, então retorna `nil`. `checkConsQuota` (usado pela Task 9) chama `s.fiscalCfg(docType).Get(ctx, orgPK)` incondicionalmente; para `docType="nfce"` isso chamaria um método num ponteiro `nil`, quebrando em runtime assim que a importação de NFC-e por XML fosse usada. `repositories.NfceConfigRepository`/`NewNfceConfigRepository` já existem (`api/internal/repositories/fiscal_configs.go:26-34`, já injetado via fx para `NfceConfigService` — `api/internal/app/app.go:62,91`) — só falta passar essa mesma instância para `DistributionService`.

**Files:**
- Modify: `api/internal/services/distributions.go` (struct `DistributionService`, `NewDistributionService`, `fiscalCfg`)
- Modify: `api/internal/app/app.go:277-301` (`newDistributionService`)
- Test: `api/internal/services/distributions_test.go`

**Interfaces:**
- Consumes: `*repositories.NfceConfigRepository` (já existe).
- Produces: `s.fiscalCfg(DocTypeNFCe)` resolve para `&s.NfceConfig.FiscalConfigRepository` em vez de `nil`.

- [ ] **Step 1: Escrever o teste que falha**

```go
// api/internal/services/distributions_test.go (adicionar)
func TestFiscalCfg_NFCe_ResolvesNonNil(t *testing.T) {
	nfceRepo := &repositories.NfceConfigRepository{}
	svc := &DistributionService{NfceConfig: nfceRepo}
	if got := svc.fiscalCfg(DocTypeNFCe); got == nil {
		t.Fatal("expected fiscalCfg(nfce) to resolve to a non-nil repository")
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd api && go test ./internal/services/... -run TestFiscalCfg_NFCe -v`
Expected: FAIL (`DistributionService` não tem campo `NfceConfig`, não compila)

- [ ] **Step 3: Implementar**

```go
// distributions.go — struct DistributionService: adicionar campo
	NfceConfig        *repositories.NfceConfigRepository
```

```go
// distributions.go — NewDistributionService: adicionar parâmetro (logo após NfeConfig)
func NewDistributionService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	NfeConfig *repositories.NfeConfigRepository,
	NfceConfig *repositories.NfceConfigRepository,
	CteConfig *repositories.CteConfigRepository,
	MdfeConfig *repositories.MdfeConfigRepository,
	NfseConfig *repositories.NfseConfigRepository,
	nfeDist *repositories.NFeDistributionRepository,
	cteDist *repositories.CTeDistributionRepository,
	mdfeDist *repositories.MDFeDistributionRepository,
	nfseDist *repositories.NfseDistributionRepository,
	clients *awsclient.Clients,
	queueURL, bucketDocs, bucketCerts, sefazFunctionName string,
) *DistributionService {
	return &DistributionService{
		orgRepo: orgRepo, certRepo: certRepo,
		NfeConfig: NfeConfig, NfceConfig: NfceConfig, CteConfig: CteConfig, MdfeConfig: MdfeConfig, NfseConfig: NfseConfig,
		nfeDist: nfeDist, cteDist: cteDist, mdfeDist: mdfeDist, nfseDist: nfseDist,
		clients:           clients,
		queueURL:          queueURL,
		bucketDocs:        bucketDocs,
		bucketCerts:       bucketCerts,
		sefazFunctionName: sefazFunctionName,
	}
}
```

```go
// distributions.go — fiscalCfg: adicionar case
func (s *DistributionService) fiscalCfg(docType string) *repositories.FiscalConfigRepository {
	switch docType {
	case DocTypeNFe:
		return &s.NfeConfig.FiscalConfigRepository
	case DocTypeNFCe:
		return &s.NfceConfig.FiscalConfigRepository
	case DocTypeCTe:
		return &s.CteConfig.FiscalConfigRepository
	case DocTypeMDFe:
		return &s.MdfeConfig.FiscalConfigRepository
	case DocTypeNfse:
		return &s.NfseConfig.FiscalConfigRepository
	}
	return nil
}
```

```go
// api/internal/app/app.go:277-301 — newDistributionService: adicionar parâmetro e repassar
func newDistributionService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	NfeConfig *repositories.NfeConfigRepository,
	NfceConfig *repositories.NfceConfigRepository,
	CteConfig *repositories.CteConfigRepository,
	MdfeConfig *repositories.MdfeConfigRepository,
	NfseConfig *repositories.NfseConfigRepository,
	nfeDist *repositories.NFeDistributionRepository,
	cteDist *repositories.CTeDistributionRepository,
	mdfeDist *repositories.MDFeDistributionRepository,
	nfseDist *repositories.NfseDistributionRepository,
	clients *awsclient.Clients,
	cfg *config.Config,
) *services.DistributionService {
	return services.NewDistributionService(
		orgRepo, certRepo,
		NfeConfig, NfceConfig, CteConfig, MdfeConfig, NfseConfig,
		nfeDist, cteDist, mdfeDist, nfseDist,
		clients,
		cfg.DistributionQueueURL,
		cfg.S3BucketDocuments,
		cfg.S3BucketCerts,
		cfg.SefazFunctionName,
	)
}
```

`*repositories.NfceConfigRepository` já é fornecido ao container fx (`app.go:62`, `repositories.NewNfceConfigRepository`) — fx injeta o novo parâmetro de `newDistributionService` por tipo automaticamente, sem precisar registrar um novo provider.

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd api && go build ./... && go test ./internal/services/... -run TestFiscalCfg_NFCe -v`
Expected: compila; PASS

- [ ] **Step 5: Rodar toda a suíte da api (regressão)**

Run: `cd api && go test ./... -race`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add api/internal/services/distributions.go api/internal/app/app.go api/internal/services/distributions_test.go
git commit -m "fix(api): wire NfceConfig into DistributionService.fiscalCfg"
```

---

### Task 9: api — `ImportXML` service, rota e OpenAPI

**Files:**
- Modify: `api/internal/services/distributions.go` (novo método `ImportXML`)
- Modify: `api/internal/api/v1/distributions.go` (nova rota)
- Modify: `api/internal/api/v1/openapi/*.yaml` (novo path — usar o arquivo onde `distributions` já está documentado)
- Modify: `DOCS.md`
- Test: `api/internal/services/distributions_test.go`

**Interfaces:**
- Consumes: `validImportDocType`, `peekXMLRoot`, `maxImportXMLSize` (Task 7); `s.checkConsQuota` (já existente, agora funcional para `nfce` graças à Task 8); `readOptionalUpload` (`api/internal/api/v1/helpers.go:69-85`).
- Produces: `DistributionService.ImportXML(ctx, orgPK, docType string, xmlBytes []byte) (map[string]any, error)`; rota `POST /distributions/:doc_type/import-xml`.

`awsclient.Clients` (`api/internal/awsclient/*.go:22-29`) expõe os clientes AWS reais (`*s3.Client`, `*sqs.Client`), sem interface de mock — os testes existentes para `EnqueueSync`/`EnqueueLookupByKey` (as duas funções mais parecidas com `ImportXML`) por isso também não têm teste unitário para o trecho que fala com S3/SQS, só para as validações puras que retornam antes disso (ver `distributions_test.go` atual: só testa `validateDistDocType`/`validateSefazDistDocType`). `ImportXML` segue a mesma postura: as 3 rejeições de validação (doc_type, tamanho, raiz) são unit-testáveis com `&DistributionService{}` vazio, porque retornam antes de tocar `s.clients`/`s.checkConsQuota`; o caminho de sucesso (staging + enqueue) é verificado manualmente (Step 6) e pelos testes de paridade do OpenAPI (Step 7), não por unit test — não é uma lacuna desta task, é a mesma cobertura que o código irmão já tem hoje.

- [ ] **Step 1: Escrever o teste que falha (validações)**

```go
// api/internal/services/distributions_test.go (adicionar)
func TestImportXML_InvalidDocType_Rejected(t *testing.T) {
	svc := &DistributionService{}
	_, err := svc.ImportXML(context.Background(), "CNPJ_11647612000197", "cte", []byte(`<nfeProc/>`))
	if err == nil {
		t.Fatal("expected error for unsupported doc_type")
	}
}

func TestImportXML_FileTooLarge_Rejected(t *testing.T) {
	svc := &DistributionService{}
	big := make([]byte, maxImportXMLSize+1)
	_, err := svc.ImportXML(context.Background(), "CNPJ_11647612000197", "nfe", big)
	if err == nil {
		t.Fatal("expected error for oversized upload")
	}
}

func TestImportXML_InvalidRoot_Rejected(t *testing.T) {
	svc := &DistributionService{}
	_, err := svc.ImportXML(context.Background(), "CNPJ_11647612000197", "nfe", []byte(`<resNFe xmlns="x"/>`))
	if err == nil {
		t.Fatal("expected error for unsupported root element")
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd api && go test ./internal/services/... -run TestImportXML -v`
Expected: FAIL (`ImportXML` não existe)

- [ ] **Step 3: Implementar o service**

```go
// api/internal/services/distributions.go (adicionar)

// ImportXML validates an uploaded NF-e/NFC-e XML, stages it in S3, and
// enqueues an "import_xml" distribution job — the worker
// (runImportXML, worker/internal/service/distribution.go) does the actual
// classification/digest-check/persistence. See
// docs/specs/2026-08-13-importacao-nfe-xml.md.
func (s *DistributionService) ImportXML(ctx context.Context, orgPK, docType string, xmlBytes []byte) (map[string]any, error) {
	if !validImportDocType(docType) {
		return nil, problem.BadRequest("doc_type inválido para importação por XML: " + docType)
	}
	if len(xmlBytes) == 0 {
		return nil, problem.BadRequest("arquivo XML vazio")
	}
	if len(xmlBytes) > maxImportXMLSize {
		return nil, problem.PayloadTooLarge("arquivo XML excede o limite de 1 MiB")
	}
	root, err := peekXMLRoot(xmlBytes)
	if err != nil || (root != "nfeProc" && root != "NFe") {
		return nil, problem.BadRequest("XML inválido: raiz deve ser nfeProc ou NFe")
	}
	if s.queueURL == "" {
		return nil, problem.BadRequest("fila de distribuição não configurada")
	}
	if err := s.checkConsQuota(ctx, orgPK, docType); err != nil {
		return nil, err
	}

	// staging não precisa de env (hom/prod) no path — é uma área de espera
	// efêmera; o worker (runImportXML) já resolve o ambiente de novo a
	// partir do fiscal config ao processar o job.
	stagingKey := fmt.Sprintf("%s-import-staging/%s/%s.xml", docType, orgPK, uuid.NewString())
	if _, err := s.clients.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketDocs),
		Key:         aws.String(stagingKey),
		Body:        bytes.NewReader(xmlBytes),
		ContentType: aws.String("application/xml"),
	}); err != nil {
		return nil, problem.InternalServer("falha ao enviar XML: " + err.Error())
	}

	msg := map[string]any{
		"job_type":     "import_xml",
		"org_pk":       orgPK,
		"doc_type":     docType,
		"staging_key":  stagingKey,
		"trigger":      "user",
		"triggered_at": time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(msg)
	if _, err := s.clients.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		return nil, problem.InternalServer("failed to enqueue import: " + err.Error())
	}
	return map[string]any{"status": "enqueued"}, nil
}
```

`checkConsQuota(ctx, orgPK, docType)` já existe (usado por `EnqueueLookupByKey`); `problem.PayloadTooLarge` — se não existir ainda no pacote `problem`, adicione-o (413) seguindo o padrão de `problem.BadRequest`/`problem.TooManyRequests`. Import `"github.com/google/uuid"` (já é dependência do módulo, usado no worker).

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd api && go test ./internal/services/... -run TestImportXML -v`
Expected: PASS (os 3 casos de rejeição)

- [ ] **Step 5: Implementar a rota**

```go
// api/internal/api/v1/distributions.go (adicionar dentro de RegisterDistributions)

	// POST /distributions/{doc_type}/import-xml
	g.Post("/:doc_type/import-xml", perm.RequireDynamic("create.%s_distributions", "doc_type"), func(c fiber.Ctx) error {
		xmlBytes, err := readOptionalUpload(c, "file")
		if err != nil {
			return sendProblem(c, err)
		}
		if xmlBytes == nil {
			return sendProblem(c, problem.BadRequest("arquivo XML obrigatório (campo \"file\")"))
		}
		result, err := svc.ImportXML(c.Context(), middleware.GetOrgPK(c), c.Params("doc_type"), xmlBytes)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusAccepted).JSON(result)
	})
```

- [ ] **Step 6: Rodar e confirmar que os testes de serviço passam**

Run: `cd api && go test ./internal/services/... -run 'TestImportXML' -v`
Expected: PASS

- [ ] **Step 7: Atualizar OpenAPI e confirmar paridade**

Adicionar `POST /distributions/{doc_type}/import-xml` no arquivo YAML onde os demais paths de `distributions` já estão documentados (`multipart/form-data`, campo `file` obrigatório tipo `string($binary)`, resposta `202` com `{"status":"enqueued"}`, erros `400`/`413` Problem JSON). O teste de paridade bidirecional já existente (`api/internal/api/v1/openapi_test.go`) falha automaticamente se a rota nova não estiver documentada, ou se a documentação ficar com um path que a rota não tem:

Run: `cd api && go test ./internal/api/v1/... -run 'TestOpenAPI_DocumentsEveryRegisteredRoute|TestOpenAPI_HasNoStaleOperations|TestOpenAPI_SpecLoads' -v`
Expected: PASS

- [ ] **Step 8: Atualizar DOCS.md**

Adicionar o novo contrato `POST /distributions/{doc_type}/import-xml` (`doc_type` ∈ {nfe, nfce}) em `DOCS.md`.

- [ ] **Step 9: Verificar manualmente o caminho feliz (staging + enqueue)**

Como o caminho de sucesso não é coberto por unit test (ver nota no início desta task), suba a stack local/homolog e confirme manualmente: `curl -F "file=@nota.xml" http://localhost:PORT/v1.0/distributions/nfe/import-xml -H "Authorization: Bearer <token>" -H "Dfe-Organization-Pk: <org_pk>"` retorna `202 {"status":"enqueued"}`, e que o objeto aparece em `s3://<bucket>/nfe-import-staging/<org_pk>/<uuid>.xml`.

- [ ] **Step 10: Commit**

```bash
git add api/internal/services/distributions.go api/internal/api/v1/distributions.go api/internal/api/v1/openapi/*.yaml api/internal/services/distributions_test.go DOCS.md
git commit -m "feat(api): add POST /distributions/{doc_type}/import-xml endpoint"
```

---

### Task 10: ui — `apiClient.importXML` e notificação de falha via WS

**Files:**
- Modify: `ui/src/lib/api/client.ts`
- Modify: `ui/src/lib/hooks/useRealtimeUpdates.ts`
- Test: `ui/src/lib/api/__tests__/distributions-client.test.ts` (arquivo já existe — testa `importNfeByKey`; adicionar o novo caso ao mesmo `describe`)

**Interfaces:**
- Produces: `apiClient.importXML(docType: 'nfe' | 'nfce', file: File): Promise<{status: string}>`.
- Consumes: `axios` instance (`this.http`), mesmo padrão multipart que `createOrganization` já usa (`headers: {'Content-Type': undefined}`) — distinto do wrapper privado `post` (2 args) usado por `importNfeByKey`/`syncDistributions`, porque multipart precisa do terceiro argumento de config.

- [ ] **Step 1: Escrever o teste que falha**

```ts
// ui/src/lib/api/__tests__/distributions-client.test.ts (adicionar ao describe existente)
it('importXML posts multipart with the file field to the doc_type-specific route', async () => {
  const httpPriv = apiClient as unknown as {
    http: { post: (url: string, body?: unknown, config?: unknown) => Promise<{ data: unknown }> }
  }
  const spy = vi.spyOn(httpPriv.http, 'post').mockResolvedValue({data: {status: 'enqueued'}})

  const file = new File(['<nfeProc/>'], 'nota.xml', {type: 'application/xml'})
  const result = await apiClient.importXML('nfe', file)

  expect(result).toEqual({status: 'enqueued'})
  expect(spy).toHaveBeenCalledWith(
    '/v1.0/distributions/nfe/import-xml',
    expect.any(FormData),
    {headers: {'Content-Type': undefined}},
  )
})
```

- [ ] **Step 2: Rodar e confirmar que falha**

Run: `cd ui && npm test -- distributions-client`
Expected: FAIL (`apiClient.importXML` não existe)

- [ ] **Step 3: Implementar**

```ts
// ui/src/lib/api/client.ts (adicionar perto de importNfeByKey/syncDistributions)

  // importXML uploads a NF-e/NFC-e XML file for async import (see
  // POST /distributions/{doc_type}/import-xml). Result arrives via WebSocket
  // (new_distribution_nfe on success, import_xml_failed on rejection).
  async importXML(docType: 'nfe' | 'nfce', file: File): Promise<{ status: string }> {
    const formData = new FormData()
    formData.append('file', file)
    return (await this.http.post<{ status: string }>(
      `/v1.0/distributions/${docType}/import-xml`,
      formData,
      {headers: {'Content-Type': undefined}},
    )).data
  }
```

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd ui && npm test -- distributions-client`
Expected: PASS

- [ ] **Step 5: Adicionar o caso de falha no WS handler**

```ts
// ui/src/lib/hooks/useRealtimeUpdates.ts — adicionar ao final de handleMessage, antes do fechamento do useCallback

    if (msg.type === 'import_xml_failed') {
      toast.error(msg.reason ? `Falha ao importar XML: ${msg.reason}` : 'Falha ao importar XML.')
    }
```

`RealtimeMessage`'s interface precisa do campo opcional `reason?: string` — adicionar junto ao `interface RealtimeMessage` (linha 23-38).

- [ ] **Step 6: Rodar ESLint e testes da ui**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npm test`
Expected: zero erros/warnings; PASS

- [ ] **Step 7: Commit**

```bash
git add ui/src/lib/api/client.ts ui/src/lib/hooks/useRealtimeUpdates.ts ui/src/lib/api/__tests__/distributions-client.test.ts
git commit -m "feat(ui): add apiClient.importXML and import_xml_failed toast handling"
```

---

### Task 11: ui — botão "Importar XML" na aba de Distribuição NF-e

Usar a skill `impeccable` para a construção visual deste componente (pedido explícito do usuário) — este task descreve o comportamento/dados necessários; a skill cuida do polish de shape/craft durante a implementação.

**Files:**
- Modify: `ui/src/app/nfe/page.tsx` (`NfeDistributionTab`, ~linhas 171-317)

**Interfaces:**
- Consumes: `apiClient.importXML('nfe', file)` (Task 10).

- [ ] **Step 1: Adicionar estado e mutation ao lado de `importMutation` existente**

```tsx
// dentro de NfeDistributionTab, ao lado de showImportModal/importKeyInput
  const [showImportXmlModal, setShowImportXmlModal] = useState(false)
  const [importXmlFile, setImportXmlFile] = useState<File | null>(null)

  const importXmlMutation = useMutation({
    mutationFn: () => {
      if (!importXmlFile) throw new Error('Selecione um arquivo XML.')
      return apiClient.importXML('nfe', importXmlFile)
    },
    onSuccess: () => {
      setShowImportXmlModal(false)
      setImportXmlFile(null)
      toast.info('Importação enfileirada. A NF-e aparecerá automaticamente quando processada.')
    },
    onError: (err: unknown) => {
      toast.error(err instanceof ApiError ? err.detail : 'Erro ao importar XML.')
    },
  })
```

- [ ] **Step 2: Adicionar o botão ao lado de "Importar NF-e" (linha ~239-248)**

```tsx
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setImportXmlFile(null)
              setShowImportXmlModal(true)
            }}
            className="text-brand-600 border-brand-200 hover:bg-brand-50"
          >
            Importar XML
          </Button>
```

- [ ] **Step 3: Adicionar o modal, ao lado do modal de importação por chave existente (~linha 290-315)**

```tsx
      <Modal
        isOpen={showImportXmlModal}
        title="Importar NF-e por XML"
        onClose={() => setShowImportXmlModal(false)}
        onSubmit={() => importXmlMutation.mutate()}
        submitLabel="Importar"
        cancelLabel="Cancelar"
        loading={importXmlMutation.isPending}
        submitDisabled={!importXmlFile}
      >
        <div className="space-y-2">
          <label htmlFor="import-xml-file" className="block text-sm font-medium text-gray-700">
            Arquivo XML
          </label>
          <input
            id="import-xml-file"
            type="file"
            accept=".xml,application/xml,text/xml"
            onChange={(e) => setImportXmlFile(e.target.files?.[0] ?? null)}
            className="w-full text-sm text-gray-600 file:mr-3 file:rounded-lg file:border-0 file:bg-brand-50 file:px-3 file:py-2 file:text-brand-700 file:text-sm"
          />
        </div>
      </Modal>
```

- [ ] **Step 4: Testar manualmente no navegador**

Rodar `npm run dev` em `ui/`, abrir a aba Distribuição de NF-e, confirmar que o botão "Importar XML" abre o modal, aceita apenas `.xml`, e que o submit chama o endpoint (checar Network tab: `POST /v1.0/distributions/nfe/import-xml`, `202`). Testar em viewport 375px (regra mobile-first do `ui/CLAUDE.md`).

- [ ] **Step 5: Rodar ESLint**

Run: `cd ui && npx eslint src --ext .ts,.tsx`
Expected: zero erros/warnings

- [ ] **Step 6: Commit**

```bash
git add ui/src/app/nfe/page.tsx
git commit -m "feat(ui): add Importar XML button to NF-e distribution tab"
```

---

### Task 12: ui — opção discreta de importação por XML na NFC-e

Usar a skill `impeccable` para o polish visual (pedido explícito do usuário). NFC-e não tem aba de distribuição (nunca recebe via SEFAZ) — a entrada é um botão pequeno/secundário na barra de ações da listagem, não um botão primário.

**Files:**
- Modify: `ui/src/app/nfce/page.tsx`

**Interfaces:**
- Consumes: `apiClient.importXML('nfce', file)` (Task 10); mesmo padrão de modal da Task 11 (considerar extrair um componente compartilhado `ImportXmlModal` em `ui/src/components/dfe/` se a duplicação entre nfe/page.tsx e nfce/page.tsx ficar maior que ~20 linhas idênticas — ver regra DRY do `ui/CLAUDE.md`).

- [ ] **Step 1: Extrair `ImportXmlModal` compartilhado (evita duplicar Task 11)**

```tsx
// ui/src/components/dfe/ImportXmlModal.tsx
'use client'

import {useState} from 'react'
import {useMutation} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient, ApiError} from '@/lib/api/client'
import {Modal} from '@/components/ui/modal'

export function ImportXmlModal({docType, isOpen, onClose}: {
  docType: 'nfe' | 'nfce'
  isOpen: boolean
  onClose: () => void
}) {
  const [file, setFile] = useState<File | null>(null)

  const mutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error('Selecione um arquivo XML.')
      return apiClient.importXML(docType, file)
    },
    onSuccess: () => {
      setFile(null)
      onClose()
      toast.info('Importação enfileirada. O documento aparecerá automaticamente quando processado.')
    },
    onError: (err: unknown) => {
      toast.error(err instanceof ApiError ? err.detail : 'Erro ao importar XML.')
    },
  })

  return (
    <Modal
      isOpen={isOpen}
      title={docType === 'nfe' ? 'Importar NF-e por XML' : 'Importar NFC-e por XML'}
      onClose={onClose}
      onSubmit={() => mutation.mutate()}
      submitLabel="Importar"
      cancelLabel="Cancelar"
      loading={mutation.isPending}
      submitDisabled={!file}
    >
      <div className="space-y-2">
        <label htmlFor={`import-xml-file-${docType}`} className="block text-sm font-medium text-gray-700">
          Arquivo XML
        </label>
        <input
          id={`import-xml-file-${docType}`}
          type="file"
          accept=".xml,application/xml,text/xml"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          className="w-full text-sm text-gray-600 file:mr-3 file:rounded-lg file:border-0 file:bg-brand-50 file:px-3 file:py-2 file:text-brand-700 file:text-sm"
        />
      </div>
    </Modal>
  )
}
```

- [ ] **Step 2: Reaproveitar em `nfe/page.tsx` (remove a duplicação da Task 11)**

Substituir o estado/mutation/modal adicionados na Task 11 por:

```tsx
  const [showImportXmlModal, setShowImportXmlModal] = useState(false)
  // ... botão "Importar XML" chama setShowImportXmlModal(true) como antes ...
```

```tsx
      <ImportXmlModal docType="nfe" isOpen={showImportXmlModal} onClose={() => setShowImportXmlModal(false)}/>
```

Remover `importXmlMutation`/`importXmlFile` e o `<Modal>` inline adicionados na Task 11.

- [ ] **Step 3: Adicionar o botão discreto na NFC-e**

Localizar a barra de ações da listagem de NFC-e (mesma região onde fica o botão "Nova NFC-e"/filtros, próximo ao topo de `NfceList`). Adicionar como link/botão secundário pequeno (não destacado), por exemplo ao lado dos filtros de ano/mês:

```tsx
  const [showImportXmlModal, setShowImportXmlModal] = useState(false)
```

```tsx
        <button
          type="button"
          onClick={() => setShowImportXmlModal(true)}
          className="text-xs text-gray-400 hover:text-gray-600 underline underline-offset-2"
        >
          Importar XML
        </button>
```

```tsx
      <ImportXmlModal docType="nfce" isOpen={showImportXmlModal} onClose={() => setShowImportXmlModal(false)}/>
```

- [ ] **Step 4: Testar manualmente no navegador**

`npm run dev`, abrir `/nfce`, confirmar que o link discreto abre o modal e o submit chama `POST /v1.0/distributions/nfce/import-xml`. Testar em 375px.

- [ ] **Step 5: Rodar ESLint**

Run: `cd ui && npx eslint src --ext .ts,.tsx`
Expected: zero erros/warnings

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/dfe/ImportXmlModal.tsx ui/src/app/nfe/page.tsx ui/src/app/nfce/page.tsx
git commit -m "feat(ui): add discreet XML import option to NFC-e, extract shared modal"
```

---

### Task 13: Verificação final cross-project

**Files:** nenhum (apenas execução/verificação)

- [ ] **Step 1: Rodar toda a suíte do worker**

Run: `cd worker && go test ./... -race`
Expected: PASS

- [ ] **Step 2: Rodar toda a suíte da api**

Run: `cd api && go test ./... -race`
Expected: PASS

- [ ] **Step 3: Rodar toda a suíte do go-dfe**

Run: `cd go-dfe && CGO_ENABLED=0 GOARCH=arm64 go build ./... && go test ./...`
Expected: PASS

- [ ] **Step 4: Rodar ESLint e testes da ui**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npm test`
Expected: zero erros/warnings; PASS

- [ ] **Step 5: Conferir Mandatory Documentation Policy**

Confirmar que `DOCS.md` tem: contrato `POST /distributions/{doc_type}/import-xml`, job `import_xml`, primeiro uso real de `NfeConsultaProtocolo`. Confirmar que `CONDUCT.md` tem: nota sobre `DocFields.Incoming`/`IncomingSet`, nota sobre a chave de staging `{doc_type}-import-staging/...`.

- [ ] **Step 6: Commit final (se sobrar algo solto)**

```bash
git status
# se houver mudanças de doc pendentes desta verificação:
git add DOCS.md CONDUCT.md
git commit -m "docs: finalize XML import documentation"
```
