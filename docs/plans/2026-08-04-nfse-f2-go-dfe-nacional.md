# NFS-e — Fase F2: `go-dfe/nfse` — Provider Nacional — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Entregar em `go-dfe` toda a comunicação com o Sistema Nacional NFS-e — modelo neutro de documento, serialização e assinatura da DPS 1.01 (incluindo IBS/CBS), pedido de registro de evento, consultas, distribuição ADN, DANFSE e parâmetros municipais — acessível pelo mesmo `dfe.Call(ctx, Request)` que hoje serve NF-e/CT-e/MDF-e.

**Arquitetura:** NFS-e não é SOAP. O caminho `services.Client` (SOAP + `endpoints.Resolve` + `xmlops.BuildXML`) não é reusado; `dfe.Call` ganha um desvio antes dele para `docType == "nfse"`. Dentro de `nfse/`, o modelo neutro `nfse.Document` é serializado por structs `encoding/xml` — a ordem dos campos da struct É a ordem do XSD, o que dispensa a tabela `xsdorder` usada pelos outros doc types. Assinatura, carga de certificado e mTLS são os existentes, sem alteração: `xmlops.Sign` e `certificate.Load`.

**Tech Stack:** Go 1.26, `encoding/xml`, `compress/gzip`, `encoding/base64`, `net/http`, `crypto/tls`. Zero dependência nova.

**Spec:** `docs/specs/2026-08-04-nfse-design.md` §4 inteiro, §3.7 (consome as tabelas da F1), §9 (linhas `go-dfe`).

**Depende de:** F1 (pacote `go-dfe/nfse/tables`). Não depende de F3/F4.

## Global Constraints

- `CGO_ENABLED=0 GOARCH=arm64 go build ./...` limpo e `go test ./...` verde em `go-dfe/` antes de qualquer commit.
- Zero string mágica: nome de serviço, host, path, código de evento, namespace XML e versão de leiaute são constantes nomeadas em `nfse/constants.go` ou `nfse/nacional/endpoints.go`.
- `internal/xmlops/signer.go` NÃO é modificado. NFS-e usa `xmlops.Sign` como está (enveloped, RSA-SHA1, digest SHA-1, C14N 1.0).
- `internal/certificate/manager.go` NÃO é modificado, incluindo `InsecureSkipVerify: true`, que é deliberado.
- Erros do provider carregam código e descrição devolvidos pelo fisco. Nunca engolir mensagem de rejeição.
- Campo do modelo neutro sem equivalente no destino faz o adapter **falhar explicitamente** nomeando o campo — nunca descartar em silêncio (regra da spec §4.3, valerá para ABRASF na F5; o nacional cobre tudo).
- Nenhum commit leva certificado PFX, credencial AWS, CNPJ real ou dado de cliente real. CNPJs em teste são fictícios com DV válido.
- Nenhum commit leva trailer `Co-Authored-By: Claude`.
- Toda mudança de comportamento atualiza `DOCS.md` (seção go-dfe) e, se criar restrição durável, `CONDUCT.md` — no MESMO commit.
- Commits em Conventional Commit, sem emoji.

## Correção de spec que esta fase carrega

A tabela de ambientes da spec (§1, "Ambientes") registra a produção restrita do Sefin Nacional como
`https://sefin.producaorestrita.nfse.gov.br/SefinNacional`. A fonte primária
(`tmp/apis-prod-restrita-e-producao.txt`, linha 30) mostra o segmento `/API` só no ambiente restrito:

```
https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional/docs/index    (produção restrita)
https://sefin.nfse.gov.br/SefinNacional/docs/index                        (produção)
```

A tabela de endpoints da Task 2 segue a fonte primária. A Task 9 corrige a spec.

---

## Estrutura de arquivos

| Arquivo | Responsabilidade | Ação |
|---|---|---|
| `go-dfe/internal/constants/constants.go` | `DocTypeNFSE` + nomes de serviço NFS-e | Modificar |
| `go-dfe/nfse/document.go` | Modelo neutro `Document` e sub-structs | Criar |
| `go-dfe/nfse/document_test.go` | Round-trip JSON do modelo neutro | Criar |
| `go-dfe/nfse/result.go` | `Result`, `EventResult`, `Message`, `DistributionItem` | Criar |
| `go-dfe/nfse/provider.go` | Interface `Provider` + `Decode`/dispatch por serviço | Criar |
| `go-dfe/nfse/errors.go` | `FieldNotSupportedError`, `FiscalError` | Criar |
| `go-dfe/nfse/constants.go` | Serviços, tipos de evento, namespace, versões | Criar |
| `go-dfe/nfse/nacional/endpoints.go` | Hosts e paths por ambiente | Criar |
| `go-dfe/nfse/nacional/endpoints_test.go` | Resolução de URL | Criar |
| `go-dfe/nfse/nacional/dps.go` | Structs XML do DPS 1.01 + conversão do modelo neutro | Criar |
| `go-dfe/nfse/nacional/dps_ibscbs.go` | Grupo `IBSCBS` (`TCRTCInfoIBSCBS`) | Criar |
| `go-dfe/nfse/nacional/dps_test.go` | Ordem de elementos, campos opcionais, `Id` | Criar |
| `go-dfe/nfse/nacional/evento.go` | `pedRegEvento` + tipos `TE*` | Criar |
| `go-dfe/nfse/nacional/evento_test.go` | Serialização por tipo de evento | Criar |
| `go-dfe/nfse/nacional/transport.go` | gzip+base64, HTTP mTLS, envelopes JSON | Criar |
| `go-dfe/nfse/nacional/transport_test.go` | `httptest` — sucesso, erro fiscal, retry | Criar |
| `go-dfe/nfse/nacional/provider.go` | `Nacional` implementando `Provider` | Criar |
| `go-dfe/nfse/nacional/provider_test.go` | Emissão/evento/consulta ponta a ponta com `httptest` | Criar |
| `go-dfe/nfse/nacional/adn.go` | Distribuição NSU, DANFSE, parâmetros municipais | Criar |
| `go-dfe/nfse/nacional/adn_test.go` | Parsing das respostas do ADN | Criar |
| `go-dfe/nfse/testdata/*.xml` | XMLs golden | Criar |
| `go-dfe/dfe.go` | Desvio `docType == nfse` + `implemented` | Modificar |
| `go-dfe/dfe_test.go` | `Implements` para NFS-e | Modificar |
| `DOCS.md`, `CONDUCT.md`, `docs/specs/2026-08-04-nfse-design.md` | Documentação | Modificar |

---

### Task 1: Modelo neutro, constantes e erros (`go-dfe/nfse`)

Primeira porque tudo depende dos tipos. Nada de rede aqui.

**Files:**
- Create: `go-dfe/nfse/constants.go`, `go-dfe/nfse/document.go`, `go-dfe/nfse/result.go`, `go-dfe/nfse/errors.go`, `go-dfe/nfse/provider.go`
- Test: `go-dfe/nfse/document_test.go`
- Modify: `go-dfe/internal/constants/constants.go`

**Interfaces:**
- Consumes: `gopkg.aoctech.app/dfe/go-dfe/nfse/tables` (F1) — só nos testes desta task, para provar que um `cTribNac` inválido é rejeitado antes de virar XML.
- Produces:
  - `nfse.Document` e sub-structs (abaixo), todos com tags `json:"..."` — a `api` monta esse JSON em `dfe.Request.Body["document"]`.
  - `nfse.DecodeDocument(body map[string]any) (Document, error)`
  - `nfse.Provider` interface
  - `nfse.Result{ChaveAcesso, IDDPS, NFSeXML, DPSXML, EventoXML, Ambiente, VersaoAplicativo, DataHoraProcessamento string; Alertas, Erros []Message}`
  - `nfse.Message{Codigo, Descricao, Complemento string}`
  - `nfse.EventRequest{ChaveAcesso, TipoEvento string; NSeqEvento int; CNPJAutor, CPFAutor string; DhEvento time.Time; Motivo *EventMotivo; ChSubstituta string; CPFAgTrib string; IDEvManifRej string; TpAmb int; VerAplic string}`
  - `nfse.EventMotivo{Codigo, Descricao string}`
  - `nfse.EventFilter{ChaveAcesso, TipoEvento string; NSeqEvento int}`
  - `nfse.DistributionItem{NSU int64; ChaveAcesso, TipoDocumento, TipoEvento, XML, DataHoraGeracao string}`
  - `nfse.FieldNotSupportedError{Provider, Field string}` e `nfse.FiscalError{Status int; Messages []Message}`
  - constantes de serviço em `internal/constants`

**Contexto para quem implementa:** o modelo neutro é moldado no DPS 1.01, o leiaute mais rico (spec §4.2). A ordem dos campos das structs de `document.go` é irrelevante — quem serializa é `nacional/dps.go`. O que importa aqui é cobrir todos os grupos.

Referência exata do leiaute (`tmp/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01/tiposComplexos_v1.01.xsd`):

```
TCInfDPS  = tpAmb, dhEmi, verAplic, serie, nDPS, dCompet, tpEmit, cLocEmi,
            subst?, prest, toma?, interm?, serv, valores, IBSCBS?    (@Id = TSIdDPS)
TCInfoPrestador = CNPJ|CPF|NIF, cNaoNIF, CAEPF?, IM?, xNome?, end?, fone?, email?, regTrib
TCInfoPessoa    = CNPJ|CPF|NIF, cNaoNIF, CAEPF?, IM?, xNome, end?, fone?, email?
TCRegTrib       = opSimpNac, regApTribSN?, regEspTrib
TCEndereco      = (endNac | endExt), xLgr, nro, xCpl?, xBairro
TCServ          = locPrest, cServ, comExt?, obra?, atvEvento?, infoCompl?
TCCServ         = cTribNac, cTribMun?, xDescServ, cNBS?, cIntContrib?
TCInfoValores   = vServPrest, vDescCondIncond?, vDedRed?, trib
TCTribMunicipal = tribISSQN, cPaisResult?, tpImunidade?, exigSusp?, BM?, tpRetISSQN, pAliq?
TCTribFederal   = piscofins?, vRetCP?, vRetIRRF?, vRetCSLL?
TCRTCInfoIBSCBS = finNFSe, indFinal?, cIndOp, tpOper?, gRefNFSe?, tpEnteGov?,
                  indDest, dest?, imovel?, valores
```

- [ ] **Step 1: Escrever o teste que falha**

Crie `go-dfe/nfse/document_test.go`:

```go
package nfse

import (
	"encoding/json"
	"testing"
)

// A api monta o Document como JSON dentro de dfe.Request.Body["document"];
// DecodeDocument tem que reconstruir todos os grupos sem perda.
func TestDecodeDocument_RoundTrip(t *testing.T) {
	src := Document{
		Ambiente: 2, TpEmit: 1, Serie: "00001", Numero: 42,
		Competencia: "2026-08-01", CLocEmi: "2211001", VerAplic: "ctech-1.0",
		Prestador: Prestador{
			Pessoa:  Pessoa{CNPJ: "11222333000181", XNome: "Prestador Teste"},
			RegTrib: RegTrib{OpSimpNac: 1, RegEspTrib: 0},
		},
		Tomador: &Pessoa{CPF: "12345678909", XNome: "Tomador Teste"},
		Servico: Servico{
			LocPrest: LocPrest{CLocPrestacao: "2211001"},
			CServ:    CServ{CTribNac: "10101", XDescServ: "Análise de sistemas"},
		},
		Valores: Valores{VServPrest: VServPrest{VServ: "1000.00"},
			Trib: Tributacao{TribMun: TribMunicipal{TribISSQN: 1, TpRetISSQN: 1, PAliq: "2.00"}}},
	}

	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	got, err := DecodeDocument(body)
	if err != nil {
		t.Fatalf("DecodeDocument: %v", err)
	}
	if got.Prestador.Pessoa.CNPJ != src.Prestador.Pessoa.CNPJ {
		t.Errorf("CNPJ = %q, esperado %q", got.Prestador.Pessoa.CNPJ, src.Prestador.Pessoa.CNPJ)
	}
	if got.Tomador == nil || got.Tomador.CPF != "12345678909" {
		t.Errorf("tomador perdido no round-trip: %+v", got.Tomador)
	}
	if got.Servico.CServ.CTribNac != "10101" {
		t.Errorf("cTribNac = %q, esperado 10101", got.Servico.CServ.CTribNac)
	}
	if got.Valores.Trib.TribMun.PAliq != "2.00" {
		t.Errorf("pAliq = %q, esperado 2.00", got.Valores.Trib.TribMun.PAliq)
	}
}

func TestDecodeDocument_RejectsUnknownField(t *testing.T) {
	// Campo desconhecido é erro, não silêncio: um typo na api tem que
	// estourar aqui, não virar DPS incompleta aceita pelo fisco.
	_, err := DecodeDocument(map[string]any{"tp_emit": 1, "campo_inexistente": "x"})
	if err == nil {
		t.Fatal("esperado erro para campo desconhecido")
	}
}

func TestFieldNotSupportedError_Message(t *testing.T) {
	err := &FieldNotSupportedError{Provider: "abrasf204", Field: "IBSCBS"}
	want := `nfse: provider "abrasf204" não suporta o campo "IBSCBS"`
	if err.Error() != want {
		t.Errorf("Error() = %q, esperado %q", err.Error(), want)
	}
}
```

- [ ] **Step 2: Rodar o teste e ver falhar**

Run: `cd go-dfe && go test ./nfse/ -run 'TestDecodeDocument|TestFieldNotSupported' -v`
Expected: FAIL — `undefined: Document`, `undefined: DecodeDocument`.

- [ ] **Step 3: Escrever `constants.go`**

```go
// Package nfse é a camada NFS-e do go-dfe: modelo neutro de documento,
// interface de provider e tipos de resultado, compartilhados entre o provider
// nacional (REST+JSON, este pacote na F2) e o ABRASF 2.04 (SOAP, F5).
//
// Diferente de NF-e/CT-e/MDF-e, NFS-e não passa por internal/services nem por
// internal/soap — dfe.Call desvia para cá antes de montar um cliente SOAP.
package nfse

// Namespace e versão do leiaute nacional
// (tmp/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01/DPS_v1.01.xsd).
const (
	Namespace     = "http://www.sped.fazenda.gov.br/nfse"
	LayoutVersion = "1.01"
)

// Providers suportados. O valor vem de dfe.Request.Body["provider"].
const (
	ProviderNacional  = "nacional"
	ProviderAbrasf204 = "abrasf204"
)

// Tipos de evento que o contribuinte PODE emitir (Anexo II).
const (
	EventCancelamento              = "101101" // TE101101
	EventCancelamentoPorSubst      = "105102" // TE105102
	EventSolicAnaliseFiscalCanc    = "101103" // TE101103
	EventConfirmacaoPrestador      = "202201" // TE202201
	EventConfirmacaoTomador        = "203202" // TE203202
	EventConfirmacaoIntermediario  = "204203" // TE204203
	EventRejeicaoPrestador         = "202205" // TE202205
	EventRejeicaoTomador           = "203206" // TE203206
	EventRejeicaoIntermediario     = "204207" // TE204207
	EventAnulacaoRejeicao          = "205208" // TE205208
)

// ContribuinteEvents é o conjunto fechado do que este pacote serializa.
// Os demais tipos do XSD (105104, 105105, 205204, 305101-305103) são
// privativos do fisco/município e só chegam pela distribuição — nunca são
// emitidos por nós.
var ContribuinteEvents = map[string]bool{
	EventCancelamento: true, EventCancelamentoPorSubst: true,
	EventSolicAnaliseFiscalCanc: true, EventConfirmacaoPrestador: true,
	EventConfirmacaoTomador: true, EventConfirmacaoIntermediario: true,
	EventRejeicaoPrestador: true, EventRejeicaoTomador: true,
	EventRejeicaoIntermediario: true, EventAnulacaoRejeicao: true,
}
```

- [ ] **Step 4: Escrever `errors.go`**

```go
package nfse

import "fmt"

// FieldNotSupportedError é a falha explícita exigida pela spec §4.3: um
// campo presente no modelo neutro que o provider de destino não representa
// NUNCA é descartado em silêncio.
type FieldNotSupportedError struct {
	Provider string
	Field    string
}

func (e *FieldNotSupportedError) Error() string {
	return fmt.Sprintf("nfse: provider %q não suporta o campo %q", e.Provider, e.Field)
}

// FiscalError carrega a rejeição do fisco com código e descrição preservados.
// Status é o HTTP devolvido pela API nacional.
type FiscalError struct {
	Status   int
	Messages []Message
}

func (e *FiscalError) Error() string {
	if len(e.Messages) == 0 {
		return fmt.Sprintf("nfse: fisco retornou HTTP %d sem mensagens", e.Status)
	}
	return fmt.Sprintf("nfse: %s - %s", e.Messages[0].Codigo, e.Messages[0].Descricao)
}
```

- [ ] **Step 5: Escrever `document.go`**

Modelo neutro completo. Todos os campos monetários e de alíquota são `string` decimal — nunca `float64`, para não introduzir erro de arredondamento em valor fiscal.

```go
package nfse

import (
	"encoding/json"
	"fmt"
)

// Document é o modelo neutro de emissão, moldado no DPS 1.01.
type Document struct {
	Ambiente     int    `json:"ambiente"`      // TSTipoAmbiente: 1 produção, 2 homologação
	VerAplic     string `json:"ver_aplic"`     // identificação do nosso aplicativo
	TpEmit       int    `json:"tp_emit"`       // 1 prestador, 2 tomador, 3 intermediário
	MotivoEmisTI int    `json:"motivo_emis_ti,omitempty"` // obrigatório quando TpEmit != 1
	ChNFSeRej    string `json:"ch_nfse_rej,omitempty"`    // TpEmit != 1 e motivo == 4
	DhEmi        string `json:"dh_emi,omitempty"`         // RFC3339 UTC; vazio = agora
	Competencia  string `json:"competencia"`              // AAAA-MM-DD
	Serie        string `json:"serie"`
	Numero       int    `json:"numero"`
	CLocEmi      string `json:"c_loc_emi"` // IBGE 7 dígitos

	Substituicao  *Substituicao `json:"substituicao,omitempty"`
	Prestador     Prestador     `json:"prestador"`
	Tomador       *Pessoa       `json:"tomador,omitempty"`
	Intermediario *Pessoa       `json:"intermediario,omitempty"`
	Servico       Servico       `json:"servico"`
	Valores       Valores       `json:"valores"`
	IBSCBS        *IBSCBS       `json:"ibs_cbs,omitempty"`
}

type Substituicao struct {
	ChSubstda string `json:"ch_substda"`
	CMotivo   string `json:"c_motivo"`
	XMotivo   string `json:"x_motivo,omitempty"`
}

type Pessoa struct {
	CNPJ    string    `json:"cnpj,omitempty"`
	CPF     string    `json:"cpf,omitempty"`
	NIF     string    `json:"nif,omitempty"`
	CNaoNIF int       `json:"c_nao_nif,omitempty"`
	CAEPF   string    `json:"caepf,omitempty"`
	IM      string    `json:"im,omitempty"`
	XNome   string    `json:"x_nome,omitempty"`
	End     *Endereco `json:"endereco,omitempty"`
	Fone    string    `json:"fone,omitempty"`
	Email   string    `json:"email,omitempty"`
}

type Prestador struct {
	Pessoa  `json:",inline"`
	RegTrib RegTrib `json:"reg_trib"`
}

type RegTrib struct {
	OpSimpNac   int `json:"op_simp_nac"`
	RegApTribSN int `json:"reg_ap_trib_sn,omitempty"`
	RegEspTrib  int `json:"reg_esp_trib"`
}

// Endereco é a escolha endNac|endExt do TCEndereco, achatada: CMun preenchido
// significa endereço nacional; CPais preenchido significa exterior.
type Endereco struct {
	CMun        string `json:"c_mun,omitempty"`
	CEP         string `json:"cep,omitempty"`
	CPais       string `json:"c_pais,omitempty"`
	CEndPost    string `json:"c_end_post,omitempty"`
	XCidade     string `json:"x_cidade,omitempty"`
	XEstadoProv string `json:"x_estado_prov,omitempty"`
	XLgr        string `json:"x_lgr"`
	Nro         string `json:"nro"`
	XCpl        string `json:"x_cpl,omitempty"`
	XBairro     string `json:"x_bairro"`
}

type Servico struct {
	LocPrest  LocPrest   `json:"loc_prest"`
	CServ     CServ      `json:"c_serv"`
	ComExt    *ComExt    `json:"com_ext,omitempty"`
	Obra      *Obra      `json:"obra,omitempty"`
	AtvEvento *AtvEvento `json:"atv_evento,omitempty"`
	InfoCompl *InfoCompl `json:"info_compl,omitempty"`
}

type LocPrest struct {
	CLocPrestacao string `json:"c_loc_prestacao,omitempty"`
	CPaisPrestacao string `json:"c_pais_prestacao,omitempty"`
	OpConsumServ  int    `json:"op_consum_serv,omitempty"`
}

type CServ struct {
	CTribNac    string `json:"c_trib_nac"`
	CTribMun    string `json:"c_trib_mun,omitempty"`
	XDescServ   string `json:"x_desc_serv"`
	CNBS        string `json:"c_nbs,omitempty"`
	CIntContrib string `json:"c_int_contrib,omitempty"`
}

type ComExt struct {
	MdPrestacao      int    `json:"md_prestacao"`
	VincPrest        int    `json:"vinc_prest"`
	TpMoeda          string `json:"tp_moeda"`
	VServMoeda       string `json:"v_serv_moeda"`
	MecAFComexP      int    `json:"mec_af_comex_p,omitempty"`
	MecAFComexT      int    `json:"mec_af_comex_t,omitempty"`
	MovTempBens      int    `json:"mov_temp_bens,omitempty"`
	NDI              string `json:"n_di,omitempty"`
	NRE              string `json:"n_re,omitempty"`
	MdicMovTempBens  string `json:"mdic,omitempty"`
}

type Obra struct {
	CObra        string    `json:"c_obra,omitempty"`
	InscImobFisc string    `json:"insc_imob_fisc,omitempty"`
	CCIB         string    `json:"c_cib,omitempty"`
	End          *Endereco `json:"endereco,omitempty"`
}

type AtvEvento struct {
	XNome    string    `json:"x_nome"`
	DtIni    string    `json:"dt_ini,omitempty"`
	DtFim    string    `json:"dt_fim,omitempty"`
	IDAtvEvt string    `json:"id_atv_evt,omitempty"`
	End      *Endereco `json:"endereco,omitempty"`
}

type InfoCompl struct {
	IDDocTec  string `json:"id_doc_tec,omitempty"`
	DocRef    string `json:"doc_ref,omitempty"`
	XInfComp  string `json:"x_inf_comp,omitempty"`
	NPedido   string `json:"n_pedido,omitempty"`
	ItemPedido string `json:"item_pedido,omitempty"`
}

type Valores struct {
	VServPrest       VServPrest    `json:"v_serv_prest"`
	VDescCondIncond  *DescCondIncond `json:"v_desc_cond_incond,omitempty"`
	VDedRed          *DedRed       `json:"v_ded_red,omitempty"`
	Trib             Tributacao    `json:"trib"`
}

type VServPrest struct {
	VReceb string `json:"v_receb,omitempty"`
	VServ  string `json:"v_serv"`
}

type DescCondIncond struct {
	VDescIncond string `json:"v_desc_incond,omitempty"`
	VDescCond   string `json:"v_desc_cond,omitempty"`
}

type DedRed struct {
	PDR        string        `json:"p_dr,omitempty"`
	VDR        string        `json:"v_dr,omitempty"`
	Documentos []DedRedDoc   `json:"documentos,omitempty"`
}

type DedRedDoc struct {
	ChNFSe   string `json:"ch_nfse,omitempty"`
	ChNFe    string `json:"ch_nfe,omitempty"`
	NDocFisc string `json:"n_doc_fisc,omitempty"`
	NDoc     string `json:"n_doc,omitempty"`
	TpDedRed int    `json:"tp_ded_red,omitempty"`
	VDedRed  string `json:"v_ded_red,omitempty"`
	DtEmiDoc string `json:"dt_emi_doc,omitempty"`
}

type Tributacao struct {
	TribMun TribMunicipal `json:"trib_mun"`
	TribFed *TribFederal  `json:"trib_fed,omitempty"`
	TotTrib *TotTrib      `json:"tot_trib,omitempty"`
}

type TribMunicipal struct {
	TribISSQN   int        `json:"trib_issqn"`
	CPaisResult string     `json:"c_pais_result,omitempty"`
	TpImunidade int        `json:"tp_imunidade,omitempty"`
	ExigSusp    *ExigSusp  `json:"exig_susp,omitempty"`
	BM          *BenefMun  `json:"bm,omitempty"`
	TpRetISSQN  int        `json:"tp_ret_issqn"`
	PAliq       string     `json:"p_aliq,omitempty"`
}

type ExigSusp struct {
	TpSusp    int    `json:"tp_susp"`
	NProcesso string `json:"n_processo"`
}

type BenefMun struct {
	TBM   int    `json:"t_bm"`
	NBM   string `json:"n_bm"`
	VlRed string `json:"vl_red,omitempty"`
}

type TribFederal struct {
	CST        string `json:"cst,omitempty"`
	VBCPisCofins string `json:"v_bc_pis_cofins,omitempty"`
	PAliqPis   string `json:"p_aliq_pis,omitempty"`
	PAliqCofins string `json:"p_aliq_cofins,omitempty"`
	VPis       string `json:"v_pis,omitempty"`
	VCofins    string `json:"v_cofins,omitempty"`
	TpRetPisCofins int `json:"tp_ret_pis_cofins,omitempty"`
	VRetCP     string `json:"v_ret_cp,omitempty"`
	VRetIRRF   string `json:"v_ret_irrf,omitempty"`
	VRetCSLL   string `json:"v_ret_csll,omitempty"`
}

type TotTrib struct {
	IndTotTrib  int    `json:"ind_tot_trib"`
	PTotTribSN  string `json:"p_tot_trib_sn,omitempty"`
	VTotTribFed string `json:"v_tot_trib_fed,omitempty"`
	VTotTribEst string `json:"v_tot_trib_est,omitempty"`
	VTotTribMun string `json:"v_tot_trib_mun,omitempty"`
	PTotTribFed string `json:"p_tot_trib_fed,omitempty"`
	PTotTribEst string `json:"p_tot_trib_est,omitempty"`
	PTotTribMun string `json:"p_tot_trib_mun,omitempty"`
}

// IBSCBS espelha TCRTCInfoIBSCBS (reforma tributária).
type IBSCBS struct {
	FinNFSe   int             `json:"fin_nfse"`
	IndFinal  int             `json:"ind_final,omitempty"`
	CIndOp    string          `json:"c_ind_op"` // Anexo C, 6 dígitos
	TpOper    int             `json:"tp_oper,omitempty"`
	GRefNFSe  *RefNFSe        `json:"g_ref_nfse,omitempty"`
	TpEnteGov int             `json:"tp_ente_gov,omitempty"`
	IndDest   int             `json:"ind_dest"`
	Dest      *Pessoa         `json:"dest,omitempty"`
	Imovel    *Imovel         `json:"imovel,omitempty"`
	Valores   IBSCBSValores   `json:"valores"`
}

type RefNFSe struct {
	ChNFSe string `json:"ch_nfse"`
}

type Imovel struct {
	CIB          string `json:"cib,omitempty"`
	InscImobFisc string `json:"insc_imob_fisc,omitempty"`
	CMun         string `json:"c_mun,omitempty"`
}

type IBSCBSValores struct {
	CST         string `json:"cst"`
	CClassTrib  string `json:"c_class_trib"`
	VBC         string `json:"v_bc,omitempty"`
	GIBSUF      *IBSComponente `json:"g_ibs_uf,omitempty"`
	GIBSMun     *IBSComponente `json:"g_ibs_mun,omitempty"`
	GCBS        *IBSComponente `json:"g_cbs,omitempty"`
	GDif        *Diferimento   `json:"g_dif,omitempty"`
	GCredPres   *CredPresumido `json:"g_cred_pres,omitempty"`
	VTotIBS     string `json:"v_tot_ibs,omitempty"`
	VTotCBS     string `json:"v_tot_cbs,omitempty"`
}

type IBSComponente struct {
	PAliq    string `json:"p_aliq,omitempty"`
	VTrib    string `json:"v_trib,omitempty"`
	PRedAliq string `json:"p_red_aliq,omitempty"`
	VTribOp  string `json:"v_trib_op,omitempty"`
}

type Diferimento struct {
	PDif string `json:"p_dif,omitempty"`
	VDif string `json:"v_dif,omitempty"`
}

type CredPresumido struct {
	CCredPres string `json:"c_cred_pres,omitempty"`
	PCredPres string `json:"p_cred_pres,omitempty"`
	VCredPres string `json:"v_cred_pres,omitempty"`
}

// DecodeDocument converte o Body["document"] recebido em dfe.Request para o
// modelo neutro. Campo desconhecido é erro: um typo na api tem que estourar
// aqui e não virar uma DPS silenciosamente incompleta.
func DecodeDocument(body map[string]any) (Document, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return Document{}, fmt.Errorf("nfse: encode document body: %w", err)
	}
	dec := json.NewDecoder(bytesReader(raw))
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("nfse: decode document: %w", err)
	}
	return doc, nil
}
```

Adicione ao topo do arquivo o helper `bytesReader` (evita import de `bytes` em cada uso):

```go
func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
```

e o import `"bytes"`.

- [ ] **Step 6: Escrever `result.go` e `provider.go`**

```go
package nfse

import (
	"context"
	"time"
)

// Message é uma mensagem de processamento do fisco (MensagemProcessamento no
// Swagger do Sefin Nacional, tmp/nfse-sefin.json).
type Message struct {
	Codigo     string `json:"codigo"`
	Descricao  string `json:"descricao"`
	Complemento string `json:"complemento,omitempty"`
}

// Result é o retorno neutro de qualquer operação. Campos não pertinentes à
// operação ficam vazios.
type Result struct {
	ChaveAcesso           string    `json:"chave_acesso,omitempty"`
	IDDPS                 string    `json:"id_dps,omitempty"`
	NFSeXML               string    `json:"nfse_xml,omitempty"`
	DPSXML                string    `json:"dps_xml,omitempty"`
	EventoXML             string    `json:"evento_xml,omitempty"`
	PedRegEventoXML       string    `json:"ped_reg_evento_xml,omitempty"`
	Ambiente              int       `json:"ambiente,omitempty"`
	VersaoAplicativo      string    `json:"versao_aplicativo,omitempty"`
	DataHoraProcessamento string    `json:"data_hora_processamento,omitempty"`
	Alertas               []Message `json:"alertas,omitempty"`
	Erros                 []Message `json:"erros,omitempty"`
	Distribuicao          []DistributionItem `json:"distribuicao,omitempty"`
	StatusDistribuicao    string    `json:"status_distribuicao,omitempty"`
	PDF                   []byte    `json:"pdf,omitempty"`
	Parametros            map[string]any `json:"parametros,omitempty"`
}

type DistributionItem struct {
	NSU             int64  `json:"nsu"`
	ChaveAcesso     string `json:"chave_acesso,omitempty"`
	TipoDocumento   string `json:"tipo_documento,omitempty"`
	TipoEvento      string `json:"tipo_evento,omitempty"`
	XML             string `json:"xml,omitempty"`
	DataHoraGeracao string `json:"data_hora_geracao,omitempty"`
}

type EventMotivo struct {
	Codigo    string `json:"codigo"`
	Descricao string `json:"descricao,omitempty"`
}

type EventRequest struct {
	ChaveAcesso  string       `json:"chave_acesso"`
	TipoEvento   string       `json:"tipo_evento"`
	NSeqEvento   int          `json:"n_seq_evento"`
	TpAmb        int          `json:"tp_amb"`
	VerAplic     string       `json:"ver_aplic"`
	DhEvento     time.Time    `json:"dh_evento"`
	CNPJAutor    string       `json:"cnpj_autor,omitempty"`
	CPFAutor     string       `json:"cpf_autor,omitempty"`
	Motivo       *EventMotivo `json:"motivo,omitempty"`
	ChSubstituta string       `json:"ch_substituta,omitempty"`
	CPFAgTrib    string       `json:"cpf_ag_trib,omitempty"`
	IDEvManifRej string       `json:"id_ev_manif_rej,omitempty"`
}

type EventFilter struct {
	ChaveAcesso string `json:"chave_acesso"`
	TipoEvento  string `json:"tipo_evento,omitempty"`
	NSeqEvento  int    `json:"n_seq_evento,omitempty"`
}

// Provider é o contrato que nacional (F2) e abrasf204 (F5) implementam.
type Provider interface {
	Emit(ctx context.Context, doc Document) (Result, error)
	Event(ctx context.Context, ev EventRequest) (Result, error)
	QueryByKey(ctx context.Context, key string) (Result, error)
	QueryByDPSID(ctx context.Context, idDPS string) (Result, error)
	QueryEvents(ctx context.Context, f EventFilter) (Result, error)
}
```

- [ ] **Step 7: Adicionar as constantes de doc type e serviço**

Em `go-dfe/internal/constants/constants.go`, no bloco `DocType`:

```go
	DocTypeMDFE = "mdfe"
	DocTypeNFSE = "nfse"
```

E, logo abaixo do bloco de retry, um bloco novo:

```go
// Serviços NFS-e. Diferente dos demais doc types, não existem operações de
// WSDL aqui: o Sistema Nacional é REST e o nome do serviço só seleciona a
// rota dentro de nfse/nacional.
const (
	ServiceNFSeRecepcao            = "NFSeRecepcao"
	ServiceNFSeConsulta            = "NFSeConsulta"
	ServiceNFSeConsultaDPS         = "NFSeConsultaDPS"
	ServiceNFSeEvento              = "NFSeEvento"
	ServiceNFSeConsultaEvento      = "NFSeConsultaEvento"
	ServiceNFSeDistribuicao        = "NFSeDistribuicao"
	ServiceNFSeDANFSE              = "NFSeDANFSE"
	ServiceNFSeParametrosMunicipais = "NFSeParametrosMunicipais"
)
```

- [ ] **Step 8: Rodar os testes**

Run: `cd go-dfe && go test ./nfse/ -v`
Expected: PASS nos três testes.

- [ ] **Step 9: Build cross-compile**

Run: `cd go-dfe && CGO_ENABLED=0 GOARCH=arm64 go build ./...`
Expected: sem saída.

- [ ] **Step 10: Commit**

```bash
git add go-dfe/nfse go-dfe/internal/constants/constants.go
git commit -m "feat(nfse): modelo neutro, constantes e erros da camada NFS-e"
```

---

### Task 2: Tabela de endpoints do ambiente nacional

**Files:**
- Create: `go-dfe/nfse/nacional/endpoints.go`
- Test: `go-dfe/nfse/nacional/endpoints_test.go`

**Interfaces:**
- Consumes: `constants.EnvironmentProd` / `constants.EnvironmentHom` de `go-dfe/internal/constants`.
- Produces:
  - `nacional.ResolveBase(system, environment string) (string, error)`
  - constantes `nacional.SystemSefin`, `SystemADN`, `SystemDANFSE`, `SystemParametros`
  - constantes de path: `PathNFSe`, `PathNFSeByKey`, `PathDPS`, `PathEventos`, `PathEventoEspecifico`, `PathDistribuicaoNSU`, `PathEventosADN`, `PathDANFSE`, `PathParamAliquota`, `PathParamConvenio`, `PathParamBeneficio`, `PathParamRegimesEspeciais`, `PathParamRetencoes`

**Contexto:** hosts e paths saem de `tmp/apis-prod-restrita-e-producao.txt` (hosts) e dos Swaggers (`tmp/nfse-sefin.json`, `tmp/nfse-adn-contribuintes.json`, `tmp/nfse-danfse.json`, `tmp/nfse-parametros-municipais.json`). Rotas confirmadas:

| Sistema | Rota | Método |
|---|---|---|
| Sefin | `/nfse` | POST |
| Sefin | `/nfse/{chaveAcesso}` | GET |
| Sefin | `/dps/{id}` | GET, HEAD |
| Sefin | `/nfse/{chaveAcesso}/eventos` | POST |
| Sefin | `/nfse/{chaveAcesso}/eventos/{tipoEvento}/{numSeqEvento}` | GET |
| ADN contribuintes | `/DFe/{NSU}?cnpjConsulta=&lote=` | GET |
| ADN contribuintes | `/NFSe/{ChaveAcesso}/Eventos` | GET |
| DANFSE | `/{chaveAcesso}` | GET |
| Parametrização | `/{codigoMunicipio}/{codigoServico}/{competencia}/aliquota` | GET |
| Parametrização | `/{codigoMunicipio}/convenio` | GET |
| Parametrização | `/{codigoMunicipio}/{numeroBeneficio}/{competencia}/beneficio` | GET |
| Parametrização | `/{codigoMunicipio}/{codigoServico}/{competencia}/regimes_especiais` | GET |
| Parametrização | `/{codigoMunicipio}/{competencia}/retencoes` | GET |

Duas rotas do Swagger do Sefin retornam 501 lá e têm o serviço real em outro host: `GET /DANFSe` e `GET /ParametrosMunicipais`. Por isso `SystemDANFSE` e `SystemParametros` são sistemas próprios, não paths do Sefin.

O Swagger do Sefin **não** expõe listagem de todos os eventos de uma chave — só o evento específico por `{tipoEvento}/{numSeqEvento}`. A listagem completa é do ADN (`GET /NFSe/{ChaveAcesso}/Eventos`). `QueryEvents` roteia por isso (Task 6).

- [ ] **Step 1: Escrever o teste que falha**

Crie `go-dfe/nfse/nacional/endpoints_test.go`:

```go
package nacional

import "testing"

func TestResolveBase(t *testing.T) {
	cases := []struct {
		system, env, want string
	}{
		{SystemSefin, "prod", "https://sefin.nfse.gov.br/SefinNacional"},
		{SystemSefin, "hom", "https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional"},
		{SystemADN, "prod", "https://adn.nfse.gov.br/contribuintes"},
		{SystemADN, "hom", "https://adn.producaorestrita.nfse.gov.br/contribuintes"},
		{SystemDANFSE, "prod", "https://adn.nfse.gov.br/danfse"},
		{SystemDANFSE, "hom", "https://adn.producaorestrita.nfse.gov.br/danfse"},
		{SystemParametros, "prod", "https://adn.nfse.gov.br/parametrizacao"},
		{SystemParametros, "hom", "https://adn.producaorestrita.nfse.gov.br/parametrizacao"},
	}
	for _, c := range cases {
		got, err := ResolveBase(c.system, c.env)
		if err != nil {
			t.Fatalf("ResolveBase(%q,%q): %v", c.system, c.env, err)
		}
		if got != c.want {
			t.Errorf("ResolveBase(%q,%q) = %q, esperado %q", c.system, c.env, got, c.want)
		}
	}
}

func TestResolveBase_Unknown(t *testing.T) {
	if _, err := ResolveBase("inexistente", "prod"); err == nil {
		t.Error("esperado erro para sistema desconhecido")
	}
	if _, err := ResolveBase(SystemSefin, "staging"); err == nil {
		t.Error("esperado erro para ambiente desconhecido")
	}
}
```

- [ ] **Step 2: Rodar o teste e ver falhar**

Run: `cd go-dfe && go test ./nfse/nacional/ -run TestResolveBase -v`
Expected: FAIL — `undefined: ResolveBase`.

- [ ] **Step 3: Escrever `endpoints.go`**

```go
// Package nacional implementa o provider do Sistema Nacional NFS-e (Sefin
// Nacional): REST + JSON com payload XML gzip+base64, mTLS com o mesmo
// certificado A1 usado nos demais documentos fiscais.
package nacional

import (
	"fmt"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
)

// Sistemas do ambiente nacional. Cada um tem host próprio por ambiente.
const (
	SystemSefin      = "sefin"
	SystemADN        = "adn"
	SystemDANFSE     = "danfse"
	SystemParametros = "parametros"
)

// Paths, com placeholders no formato de fmt.Sprintf.
const (
	PathNFSe                  = "/nfse"
	PathNFSeByKey             = "/nfse/%s"
	PathDPS                   = "/dps/%s"
	PathEventos               = "/nfse/%s/eventos"
	PathEventoEspecifico      = "/nfse/%s/eventos/%s/%d"
	PathDistribuicaoNSU       = "/DFe/%d"
	PathEventosADN            = "/NFSe/%s/Eventos"
	PathDANFSE                = "/%s"
	PathParamAliquota         = "/%s/%s/%s/aliquota"
	PathParamConvenio         = "/%s/convenio"
	PathParamBeneficio        = "/%s/%s/%s/beneficio"
	PathParamRegimesEspeciais = "/%s/%s/%s/regimes_especiais"
	PathParamRetencoes        = "/%s/%s/retencoes"
)

// bases mapeia (sistema, ambiente) -> URL base.
//
// Fonte: tmp/apis-prod-restrita-e-producao.txt. Atenção ao segmento "/API"
// que existe SÓ na produção restrita do Sefin Nacional — a tabela de
// ambientes da spec omitia esse segmento; a fonte primária prevalece.
var bases = map[string]map[string]string{
	SystemSefin: {
		constants.EnvironmentProd: "https://sefin.nfse.gov.br/SefinNacional",
		constants.EnvironmentHom:  "https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional",
	},
	SystemADN: {
		constants.EnvironmentProd: "https://adn.nfse.gov.br/contribuintes",
		constants.EnvironmentHom:  "https://adn.producaorestrita.nfse.gov.br/contribuintes",
	},
	SystemDANFSE: {
		constants.EnvironmentProd: "https://adn.nfse.gov.br/danfse",
		constants.EnvironmentHom:  "https://adn.producaorestrita.nfse.gov.br/danfse",
	},
	SystemParametros: {
		constants.EnvironmentProd: "https://adn.nfse.gov.br/parametrizacao",
		constants.EnvironmentHom:  "https://adn.producaorestrita.nfse.gov.br/parametrizacao",
	},
}

// ResolveBase devolve a URL base de (system, environment).
func ResolveBase(system, environment string) (string, error) {
	envs, ok := bases[system]
	if !ok {
		return "", fmt.Errorf("nacional: sistema desconhecido %q", system)
	}
	base, ok := envs[environment]
	if !ok {
		return "", fmt.Errorf("nacional: ambiente desconhecido %q para o sistema %q", environment, system)
	}
	return base, nil
}
```

- [ ] **Step 4: Rodar os testes**

Run: `cd go-dfe && go test ./nfse/nacional/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go-dfe/nfse/nacional
git commit -m "feat(nfse): tabela de endpoints do ambiente nacional"
```

---

### Task 3: Serialização da DPS 1.01

O núcleo da fase. Structs `encoding/xml` cuja ordem de campos É a ordem do XSD.

**Files:**
- Create: `go-dfe/nfse/nacional/dps.go`
- Test: `go-dfe/nfse/nacional/dps_test.go`
- Create: `go-dfe/nfse/testdata/dps_minima.xml`

**Interfaces:**
- Consumes: `nfse.Document` e sub-structs (Task 1); `tables.IsValidTribNacional` e `tables.IsValidNBS` (F1).
- Produces:
  - `nacional.BuildDPS(doc nfse.Document, now time.Time) ([]byte, string, error)` — devolve o XML da DPS **sem assinatura** e o `idDPS` calculado.
  - `nacional.BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie string, nDPS int) string`

**Contexto:** a regra do `idDPS` está em `TSIdDPS` (`tiposSimples_v1.01.xsd:47`) — 45 caracteres:

```
"DPS" + cLocEmi(7) + tpInsc(1) + inscFederal(14) + serie(5) + nDPS(15)
```

`tpInsc` é `1` para CPF e `2` para CNPJ. `inscFederal` tem 14 posições: CPF completa com zeros à esquerda. `serie` completa com zeros à esquerda até 5. `nDPS` completa com zeros à esquerda até 15.

`infDPS` leva `Id="{idDPS}"` — é esse atributo que a assinatura referencia.

Formato de `dhEmi`: `TSDateTimeUTC`, ou seja `AAAA-MM-DDThh:mm:ss-03:00` ou com `Z`. Use UTC com sufixo `Z`, via `t.UTC().Format(time.RFC3339)`.

Elementos vazios não podem aparecer no XML — todos os campos opcionais são ponteiro ou têm `omitempty`. `encoding/xml` **não** respeita `omitempty` em struct aninhada não-ponteiro, por isso todo grupo opcional é ponteiro.

- [ ] **Step 1: Escrever o teste que falha**

Crie `go-dfe/nfse/nacional/dps_test.go`:

```go
package nacional

import (
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func minimalDoc() nfse.Document {
	return nfse.Document{
		Ambiente: 2, VerAplic: "ctech-1.0", TpEmit: 1,
		Competencia: "2026-08-01", Serie: "1", Numero: 42, CLocEmi: "2211001",
		Prestador: nfse.Prestador{
			Pessoa:  nfse.Pessoa{CNPJ: "11222333000181", IM: "123456"},
			RegTrib: nfse.RegTrib{OpSimpNac: 1, RegEspTrib: 0},
		},
		Tomador: &nfse.Pessoa{CPF: "12345678909", XNome: "Tomador Teste"},
		Servico: nfse.Servico{
			LocPrest: nfse.LocPrest{CLocPrestacao: "2211001"},
			CServ:    nfse.CServ{CTribNac: "10101", XDescServ: "Análise de sistemas"},
		},
		Valores: nfse.Valores{
			VServPrest: nfse.VServPrest{VServ: "1000.00"},
			Trib: nfse.Tributacao{
				TribMun: nfse.TribMunicipal{TribISSQN: 1, TpRetISSQN: 1, PAliq: "2.00"},
			},
		},
	}
}

func TestBuildIDDPS(t *testing.T) {
	got := BuildIDDPS("2211001", "2", "11222333000181", "1", 42)
	want := "DPS" + "2211001" + "2" + "11222333000181" + "00001" + "000000000000042"
	if got != want {
		t.Fatalf("BuildIDDPS = %q, esperado %q", got, want)
	}
	if len(got) != 45 {
		t.Errorf("len = %d, esperado 45 (TSIdDPS)", len(got))
	}
}

func TestBuildDPS_ElementOrder(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	xmlBytes, idDPS, err := BuildDPS(minimalDoc(), now)
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)

	if !strings.Contains(s, `Id="`+idDPS+`"`) {
		t.Errorf("infDPS sem Id=%q", idDPS)
	}
	if !strings.Contains(s, `xmlns="`+nfse.Namespace+`"`) {
		t.Errorf("namespace ausente")
	}
	// A ordem de TCInfDPS é normativa: tpAmb, dhEmi, verAplic, serie, nDPS,
	// dCompet, tpEmit, cLocEmi, [subst], prest, [toma], [interm], serv,
	// valores, [IBSCBS].
	order := []string{"<tpAmb>", "<dhEmi>", "<verAplic>", "<serie>", "<nDPS>",
		"<dCompet>", "<tpEmit>", "<cLocEmi>", "<prest>", "<toma>", "<serv>", "<valores>"}
	prev := -1
	for _, tag := range order {
		i := strings.Index(s, tag)
		if i < 0 {
			t.Fatalf("tag %s ausente no XML", tag)
		}
		if i < prev {
			t.Errorf("tag %s fora de ordem", tag)
		}
		prev = i
	}
}

func TestBuildDPS_OmitsEmptyOptionalGroups(t *testing.T) {
	xmlBytes, _, err := BuildDPS(minimalDoc(), time.Now())
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)
	for _, tag := range []string{"<subst>", "<interm>", "<IBSCBS>", "<comExt>",
		"<obra>", "<atvEvento>", "<vDedRed>", "<end>"} {
		if strings.Contains(s, tag) {
			t.Errorf("grupo opcional vazio %s não deveria aparecer", tag)
		}
	}
}

func TestBuildDPS_RejectsInvalidTribNacional(t *testing.T) {
	doc := minimalDoc()
	doc.Servico.CServ.CTribNac = "99999"
	if _, _, err := BuildDPS(doc, time.Now()); err == nil {
		t.Fatal("esperado erro para cTribNac inexistente no Anexo B")
	}
}

func TestBuildDPS_RequiresMotivoWhenTpEmitNotPrestador(t *testing.T) {
	doc := minimalDoc()
	doc.TpEmit = 2
	if _, _, err := BuildDPS(doc, time.Now()); err == nil {
		t.Fatal("esperado erro: cMotivoEmisTI é obrigatório quando tpEmit != 1")
	}
}
```

- [ ] **Step 2: Rodar o teste e ver falhar**

Run: `cd go-dfe && go test ./nfse/nacional/ -run TestBuildDPS -v`
Expected: FAIL — `undefined: BuildDPS`.

- [ ] **Step 3: Escrever `dps.go`**

```go
package nacional

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/tables"
)

// Tipos de inscrição federal usados no idDPS (TSIdDPS).
const (
	tpInscCPF  = "1"
	tpInscCNPJ = "2"
)

// Larguras fixas do idDPS.
const (
	widthInscFederal = 14
	widthSerie       = 5
	widthNDPS        = 15
)

// xmlDPS é a raiz do documento (DPS_v1.01.xsd:9 — elemento DPS, tipo TCDPS).
type xmlDPS struct {
	XMLName xml.Name   `xml:"DPS"`
	Xmlns   string     `xml:"xmlns,attr"`
	Versao  string     `xml:"versao,attr"`
	InfDPS  xmlInfDPS  `xml:"infDPS"`
}

// xmlInfDPS espelha TCInfDPS — a ordem dos campos É a ordem do XSD.
type xmlInfDPS struct {
	ID       string        `xml:"Id,attr"`
	TpAmb    int           `xml:"tpAmb"`
	DhEmi    string        `xml:"dhEmi"`
	VerAplic string        `xml:"verAplic"`
	Serie    string        `xml:"serie"`
	NDPS     int           `xml:"nDPS"`
	DCompet  string        `xml:"dCompet"`
	TpEmit   int           `xml:"tpEmit"`
	CMotivo  int           `xml:"cMotivoEmisTI,omitempty"`
	ChNFSeRej string       `xml:"chNFSeRej,omitempty"`
	CLocEmi  string        `xml:"cLocEmi"`
	Subst    *xmlSubst     `xml:"subst,omitempty"`
	Prest    xmlPrestador  `xml:"prest"`
	Toma     *xmlPessoa    `xml:"toma,omitempty"`
	Interm   *xmlPessoa    `xml:"interm,omitempty"`
	Serv     xmlServ       `xml:"serv"`
	Valores  xmlValores    `xml:"valores"`
	IBSCBS   *xmlIBSCBS    `xml:"IBSCBS,omitempty"`
}

type xmlSubst struct {
	ChSubstda string `xml:"chSubstda"`
	CMotivo   string `xml:"cMotivo"`
	XMotivo   string `xml:"xMotivo,omitempty"`
}

type xmlPrestador struct {
	CNPJ    string      `xml:"CNPJ,omitempty"`
	CPF     string      `xml:"CPF,omitempty"`
	NIF     string      `xml:"NIF,omitempty"`
	CNaoNIF int         `xml:"cNaoNIF,omitempty"`
	CAEPF   string      `xml:"CAEPF,omitempty"`
	IM      string      `xml:"IM,omitempty"`
	XNome   string      `xml:"xNome,omitempty"`
	End     *xmlEnd     `xml:"end,omitempty"`
	Fone    string      `xml:"fone,omitempty"`
	Email   string      `xml:"email,omitempty"`
	RegTrib xmlRegTrib  `xml:"regTrib"`
}

type xmlPessoa struct {
	CNPJ    string  `xml:"CNPJ,omitempty"`
	CPF     string  `xml:"CPF,omitempty"`
	NIF     string  `xml:"NIF,omitempty"`
	CNaoNIF int     `xml:"cNaoNIF,omitempty"`
	CAEPF   string  `xml:"CAEPF,omitempty"`
	IM      string  `xml:"IM,omitempty"`
	XNome   string  `xml:"xNome"`
	End     *xmlEnd `xml:"end,omitempty"`
	Fone    string  `xml:"fone,omitempty"`
	Email   string  `xml:"email,omitempty"`
}

type xmlRegTrib struct {
	OpSimpNac   int `xml:"opSimpNac"`
	RegApTribSN int `xml:"regApTribSN,omitempty"`
	RegEspTrib  int `xml:"regEspTrib"`
}

// xmlEnd espelha TCEndereco: escolha endNac|endExt e depois logradouro.
type xmlEnd struct {
	EndNac  *xmlEndNac `xml:"endNac,omitempty"`
	EndExt  *xmlEndExt `xml:"endExt,omitempty"`
	XLgr    string     `xml:"xLgr"`
	Nro     string     `xml:"nro"`
	XCpl    string     `xml:"xCpl,omitempty"`
	XBairro string     `xml:"xBairro"`
}

type xmlEndNac struct {
	CMun string `xml:"cMun"`
	CEP  string `xml:"CEP"`
}

type xmlEndExt struct {
	CPais       string `xml:"cPais"`
	CEndPost    string `xml:"cEndPost,omitempty"`
	XCidade     string `xml:"xCidade"`
	XEstProvReg string `xml:"xEstProvReg,omitempty"`
}

type xmlServ struct {
	LocPrest  xmlLocPrest   `xml:"locPrest"`
	CServ     xmlCServ      `xml:"cServ"`
	ComExt    *xmlComExt    `xml:"comExt,omitempty"`
	Obra      *xmlObra      `xml:"obra,omitempty"`
	AtvEvento *xmlAtvEvento `xml:"atvEvento,omitempty"`
	InfoCompl *xmlInfoCompl `xml:"infoCompl,omitempty"`
}

type xmlLocPrest struct {
	CLocPrestacao  string `xml:"cLocPrestacao,omitempty"`
	CPaisPrestacao string `xml:"cPaisPrestacao,omitempty"`
	OpConsumServ   int    `xml:"opConsumServ,omitempty"`
}

type xmlCServ struct {
	CTribNac    string `xml:"cTribNac"`
	CTribMun    string `xml:"cTribMun,omitempty"`
	XDescServ   string `xml:"xDescServ"`
	CNBS        string `xml:"cNBS,omitempty"`
	CIntContrib string `xml:"cIntContrib,omitempty"`
}

type xmlComExt struct {
	MdPrestacao int    `xml:"mdPrestacao"`
	VincPrest   int    `xml:"vincPrest"`
	TpMoeda     string `xml:"tpMoeda"`
	VServMoeda  string `xml:"vServMoeda"`
	MecAFComexP int    `xml:"mecAFComexP,omitempty"`
	MecAFComexT int    `xml:"mecAFComexT,omitempty"`
	MovTempBens int    `xml:"movTempBens,omitempty"`
	NDI         string `xml:"nDI,omitempty"`
	NRE         string `xml:"nRE,omitempty"`
	MdicMoeda   string `xml:"mdic,omitempty"`
}

type xmlObra struct {
	CObra        string  `xml:"cObra,omitempty"`
	InscImobFisc string  `xml:"inscImobFisc,omitempty"`
	CCIB         string  `xml:"cCIB,omitempty"`
	End          *xmlEnd `xml:"end,omitempty"`
}

type xmlAtvEvento struct {
	XNome    string  `xml:"xNome"`
	DtIni    string  `xml:"dtIni,omitempty"`
	DtFim    string  `xml:"dtFim,omitempty"`
	IDAtvEvt string  `xml:"idAtvEvt,omitempty"`
	End      *xmlEnd `xml:"end,omitempty"`
}

type xmlInfoCompl struct {
	IDDocTec   string `xml:"idDocTec,omitempty"`
	DocRef     string `xml:"docRef,omitempty"`
	XInfComp   string `xml:"xInfComp,omitempty"`
	NPedido    string `xml:"nPedido,omitempty"`
	ItemPedido string `xml:"itemPedido,omitempty"`
}

type xmlValores struct {
	VServPrest      xmlVServPrest      `xml:"vServPrest"`
	VDescCondIncond *xmlDescCondIncond `xml:"vDescCondIncond,omitempty"`
	VDedRed         *xmlDedRed         `xml:"vDedRed,omitempty"`
	Trib            xmlTrib            `xml:"trib"`
}

type xmlVServPrest struct {
	VReceb string `xml:"vReceb,omitempty"`
	VServ  string `xml:"vServ"`
}

type xmlDescCondIncond struct {
	VDescIncond string `xml:"vDescIncond,omitempty"`
	VDescCond   string `xml:"vDescCond,omitempty"`
}

type xmlDedRed struct {
	PDR      string          `xml:"pDR,omitempty"`
	VDR      string          `xml:"vDR,omitempty"`
	DocDedRed []xmlDedRedDoc `xml:"documentos>docDedRed,omitempty"`
}

type xmlDedRedDoc struct {
	ChNFSe   string `xml:"chNFSe,omitempty"`
	ChNFe    string `xml:"chNFe,omitempty"`
	NDocFisc string `xml:"nDocFisc,omitempty"`
	NDoc     string `xml:"nDoc,omitempty"`
	TpDedRed int    `xml:"tpDedRed,omitempty"`
	VDedRed  string `xml:"vDedRed,omitempty"`
	DtEmiDoc string `xml:"dtEmiDoc,omitempty"`
}

type xmlTrib struct {
	TribMun xmlTribMun  `xml:"tribMun"`
	TribFed *xmlTribFed `xml:"tribFed,omitempty"`
	TotTrib *xmlTotTrib `xml:"totTrib,omitempty"`
}

type xmlTribMun struct {
	TribISSQN   int           `xml:"tribISSQN"`
	CPaisResult string        `xml:"cPaisResult,omitempty"`
	TpImunidade int           `xml:"tpImunidade,omitempty"`
	ExigSusp    *xmlExigSusp  `xml:"exigSusp,omitempty"`
	BM          *xmlBenefMun  `xml:"BM,omitempty"`
	TpRetISSQN  int           `xml:"tpRetISSQN"`
	PAliq       string        `xml:"pAliq,omitempty"`
}

type xmlExigSusp struct {
	TpSusp    int    `xml:"tpSusp"`
	NProcesso string `xml:"nProcesso"`
}

type xmlBenefMun struct {
	TBM   int    `xml:"tBM"`
	NBM   string `xml:"nBM"`
	VlRed string `xml:"vlRed,omitempty"`
}

type xmlTribFed struct {
	PisCofins      *xmlPisCofins `xml:"piscofins,omitempty"`
	VRetCP         string        `xml:"vRetCP,omitempty"`
	VRetIRRF       string        `xml:"vRetIRRF,omitempty"`
	VRetCSLL       string        `xml:"vRetCSLL,omitempty"`
}

type xmlPisCofins struct {
	CST         string `xml:"CST"`
	VBCPisCofins string `xml:"vBCPisCofins,omitempty"`
	PAliqPis    string `xml:"pAliqPis,omitempty"`
	PAliqCofins string `xml:"pAliqCofins,omitempty"`
	VPis        string `xml:"vPis,omitempty"`
	VCofins     string `xml:"vCofins,omitempty"`
	TpRetPisCofins int `xml:"tpRetPisCofins,omitempty"`
}

type xmlTotTrib struct {
	VTotTrib *xmlVTotTrib `xml:"vTotTrib,omitempty"`
	PTotTrib *xmlPTotTrib `xml:"pTotTrib,omitempty"`
	IndTotTrib int        `xml:"indTotTrib,omitempty"`
	PTotTribSN string     `xml:"pTotTribSN,omitempty"`
}

type xmlVTotTrib struct {
	VTotTribFed string `xml:"vTotTribFed"`
	VTotTribEst string `xml:"vTotTribEst"`
	VTotTribMun string `xml:"vTotTribMun"`
}

type xmlPTotTrib struct {
	PTotTribFed string `xml:"pTotTribFed"`
	PTotTribEst string `xml:"pTotTribEst"`
	PTotTribMun string `xml:"pTotTribMun"`
}

// BuildIDDPS monta o identificador da DPS (TSIdDPS, 45 caracteres).
func BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie string, nDPS int) string {
	return "DPS" + cLocEmi + tpInsc +
		leftPad(inscFederal, widthInscFederal) +
		leftPad(serie, widthSerie) +
		leftPad(fmt.Sprintf("%d", nDPS), widthNDPS)
}

func leftPad(s string, width int) string {
	if len(s) >= width {
		return s[len(s)-width:]
	}
	return strings.Repeat("0", width-len(s)) + s
}

// BuildDPS serializa o modelo neutro na DPS 1.01, ainda SEM assinatura.
// Devolve o XML e o idDPS. now é injetado para tornar o teste determinístico;
// só é usado quando doc.DhEmi está vazio.
func BuildDPS(doc nfse.Document, now time.Time) ([]byte, string, error) {
	if err := validateDoc(doc); err != nil {
		return nil, "", err
	}

	tpInsc, inscFederal := tpInscCNPJ, doc.Prestador.CNPJ
	if doc.Prestador.CPF != "" {
		tpInsc, inscFederal = tpInscCPF, doc.Prestador.CPF
	}
	idDPS := BuildIDDPS(doc.CLocEmi, tpInsc, inscFederal, doc.Serie, doc.Numero)

	dhEmi := doc.DhEmi
	if dhEmi == "" {
		dhEmi = now.UTC().Format(time.RFC3339)
	}

	inf := xmlInfDPS{
		ID: idDPS, TpAmb: doc.Ambiente, DhEmi: dhEmi, VerAplic: doc.VerAplic,
		Serie: doc.Serie, NDPS: doc.Numero, DCompet: doc.Competencia,
		TpEmit: doc.TpEmit, CMotivo: doc.MotivoEmisTI, ChNFSeRej: doc.ChNFSeRej,
		CLocEmi: doc.CLocEmi,
		Prest:   toXMLPrestador(doc.Prestador),
		Toma:    toXMLPessoa(doc.Tomador),
		Interm:  toXMLPessoa(doc.Intermediario),
		Serv:    toXMLServ(doc.Servico),
		Valores: toXMLValores(doc.Valores),
		IBSCBS:  toXMLIBSCBS(doc.IBSCBS),
	}
	if doc.Substituicao != nil {
		inf.Subst = &xmlSubst{
			ChSubstda: doc.Substituicao.ChSubstda,
			CMotivo:   doc.Substituicao.CMotivo,
			XMotivo:   doc.Substituicao.XMotivo,
		}
	}

	out, err := xml.Marshal(xmlDPS{Xmlns: nfse.Namespace, Versao: nfse.LayoutVersion, InfDPS: inf})
	if err != nil {
		return nil, "", fmt.Errorf("nacional: serializar DPS: %w", err)
	}
	return out, idDPS, nil
}

// validateDoc cobre só as regras estruturais que impedem gerar um XML
// coerente. As regras de negócio fiscais ficam com o Sefin (spec §11).
func validateDoc(doc nfse.Document) error {
	if doc.Prestador.CNPJ == "" && doc.Prestador.CPF == "" && doc.Prestador.NIF == "" {
		return fmt.Errorf("nacional: prestador sem CNPJ, CPF ou NIF")
	}
	if doc.TpEmit != 1 && doc.MotivoEmisTI == 0 {
		return fmt.Errorf("nacional: cMotivoEmisTI é obrigatório quando tpEmit=%d", doc.TpEmit)
	}
	if !tables.IsValidTribNacional(doc.Servico.CServ.CTribNac) {
		return fmt.Errorf("nacional: cTribNac %q não consta no Anexo B", doc.Servico.CServ.CTribNac)
	}
	if c := doc.Servico.CServ.CNBS; c != "" && !tables.IsValidNBS(c) {
		return fmt.Errorf("nacional: cNBS %q não consta na NBS 2.0", c)
	}
	if doc.IBSCBS != nil && !tables.IsValidIndOp(doc.IBSCBS.CIndOp) {
		return fmt.Errorf("nacional: cIndOp %q não consta no Anexo C", doc.IBSCBS.CIndOp)
	}
	return nil
}

func toXMLEnd(e *nfse.Endereco) *xmlEnd {
	if e == nil {
		return nil
	}
	out := &xmlEnd{XLgr: e.XLgr, Nro: e.Nro, XCpl: e.XCpl, XBairro: e.XBairro}
	if e.CMun != "" {
		out.EndNac = &xmlEndNac{CMun: e.CMun, CEP: e.CEP}
	} else {
		out.EndExt = &xmlEndExt{CPais: e.CPais, CEndPost: e.CEndPost,
			XCidade: e.XCidade, XEstProvReg: e.XEstadoProv}
	}
	return out
}

func toXMLPrestador(p nfse.Prestador) xmlPrestador {
	return xmlPrestador{
		CNPJ: p.CNPJ, CPF: p.CPF, NIF: p.NIF, CNaoNIF: p.CNaoNIF,
		CAEPF: p.CAEPF, IM: p.IM, XNome: p.XNome, End: toXMLEnd(p.End),
		Fone: p.Fone, Email: p.Email,
		RegTrib: xmlRegTrib{OpSimpNac: p.RegTrib.OpSimpNac,
			RegApTribSN: p.RegTrib.RegApTribSN, RegEspTrib: p.RegTrib.RegEspTrib},
	}
}

func toXMLPessoa(p *nfse.Pessoa) *xmlPessoa {
	if p == nil {
		return nil
	}
	return &xmlPessoa{
		CNPJ: p.CNPJ, CPF: p.CPF, NIF: p.NIF, CNaoNIF: p.CNaoNIF,
		CAEPF: p.CAEPF, IM: p.IM, XNome: p.XNome, End: toXMLEnd(p.End),
		Fone: p.Fone, Email: p.Email,
	}
}

func toXMLServ(s nfse.Servico) xmlServ {
	out := xmlServ{
		LocPrest: xmlLocPrest{CLocPrestacao: s.LocPrest.CLocPrestacao,
			CPaisPrestacao: s.LocPrest.CPaisPrestacao, OpConsumServ: s.LocPrest.OpConsumServ},
		CServ: xmlCServ{CTribNac: s.CServ.CTribNac, CTribMun: s.CServ.CTribMun,
			XDescServ: s.CServ.XDescServ, CNBS: s.CServ.CNBS, CIntContrib: s.CServ.CIntContrib},
	}
	if c := s.ComExt; c != nil {
		out.ComExt = &xmlComExt{MdPrestacao: c.MdPrestacao, VincPrest: c.VincPrest,
			TpMoeda: c.TpMoeda, VServMoeda: c.VServMoeda, MecAFComexP: c.MecAFComexP,
			MecAFComexT: c.MecAFComexT, MovTempBens: c.MovTempBens,
			NDI: c.NDI, NRE: c.NRE, MdicMoeda: c.MdicMovTempBens}
	}
	if o := s.Obra; o != nil {
		out.Obra = &xmlObra{CObra: o.CObra, InscImobFisc: o.InscImobFisc,
			CCIB: o.CCIB, End: toXMLEnd(o.End)}
	}
	if a := s.AtvEvento; a != nil {
		out.AtvEvento = &xmlAtvEvento{XNome: a.XNome, DtIni: a.DtIni, DtFim: a.DtFim,
			IDAtvEvt: a.IDAtvEvt, End: toXMLEnd(a.End)}
	}
	if i := s.InfoCompl; i != nil {
		out.InfoCompl = &xmlInfoCompl{IDDocTec: i.IDDocTec, DocRef: i.DocRef,
			XInfComp: i.XInfComp, NPedido: i.NPedido, ItemPedido: i.ItemPedido}
	}
	return out
}

func toXMLValores(v nfse.Valores) xmlValores {
	out := xmlValores{
		VServPrest: xmlVServPrest{VReceb: v.VServPrest.VReceb, VServ: v.VServPrest.VServ},
		Trib: xmlTrib{TribMun: xmlTribMun{
			TribISSQN: v.Trib.TribMun.TribISSQN, CPaisResult: v.Trib.TribMun.CPaisResult,
			TpImunidade: v.Trib.TribMun.TpImunidade,
			TpRetISSQN:  v.Trib.TribMun.TpRetISSQN, PAliq: v.Trib.TribMun.PAliq,
		}},
	}
	if d := v.VDescCondIncond; d != nil {
		out.VDescCondIncond = &xmlDescCondIncond{VDescIncond: d.VDescIncond, VDescCond: d.VDescCond}
	}
	if d := v.VDedRed; d != nil {
		dr := &xmlDedRed{PDR: d.PDR, VDR: d.VDR}
		for _, doc := range d.Documentos {
			dr.DocDedRed = append(dr.DocDedRed, xmlDedRedDoc{
				ChNFSe: doc.ChNFSe, ChNFe: doc.ChNFe, NDocFisc: doc.NDocFisc,
				NDoc: doc.NDoc, TpDedRed: doc.TpDedRed, VDedRed: doc.VDedRed,
				DtEmiDoc: doc.DtEmiDoc,
			})
		}
		out.VDedRed = dr
	}
	if e := v.Trib.TribMun.ExigSusp; e != nil {
		out.Trib.TribMun.ExigSusp = &xmlExigSusp{TpSusp: e.TpSusp, NProcesso: e.NProcesso}
	}
	if b := v.Trib.TribMun.BM; b != nil {
		out.Trib.TribMun.BM = &xmlBenefMun{TBM: b.TBM, NBM: b.NBM, VlRed: b.VlRed}
	}
	if f := v.Trib.TribFed; f != nil {
		tf := &xmlTribFed{VRetCP: f.VRetCP, VRetIRRF: f.VRetIRRF, VRetCSLL: f.VRetCSLL}
		if f.CST != "" {
			tf.PisCofins = &xmlPisCofins{CST: f.CST, VBCPisCofins: f.VBCPisCofins,
				PAliqPis: f.PAliqPis, PAliqCofins: f.PAliqCofins,
				VPis: f.VPis, VCofins: f.VCofins, TpRetPisCofins: f.TpRetPisCofins}
		}
		out.Trib.TribFed = tf
	}
	if t := v.Trib.TotTrib; t != nil {
		tt := &xmlTotTrib{IndTotTrib: t.IndTotTrib, PTotTribSN: t.PTotTribSN}
		if t.VTotTribFed != "" {
			tt.VTotTrib = &xmlVTotTrib{VTotTribFed: t.VTotTribFed,
				VTotTribEst: t.VTotTribEst, VTotTribMun: t.VTotTribMun}
		}
		if t.PTotTribFed != "" {
			tt.PTotTrib = &xmlPTotTrib{PTotTribFed: t.PTotTribFed,
				PTotTribEst: t.PTotTribEst, PTotTribMun: t.PTotTribMun}
		}
		out.Trib.TotTrib = tt
	}
	return out
}
```

- [ ] **Step 4: Escrever `dps_ibscbs.go`**

```go
package nacional

import (
	"encoding/xml"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// xmlIBSCBS espelha TCRTCInfoIBSCBS: finNFSe, indFinal?, cIndOp, tpOper?,
// gRefNFSe?, tpEnteGov?, indDest, dest?, imovel?, valores.
type xmlIBSCBS struct {
	XMLName   xml.Name        `xml:"IBSCBS"`
	FinNFSe   int             `xml:"finNFSe"`
	IndFinal  int             `xml:"indFinal,omitempty"`
	CIndOp    string          `xml:"cIndOp"`
	TpOper    int             `xml:"tpOper,omitempty"`
	GRefNFSe  *xmlRefNFSe     `xml:"gRefNFSe,omitempty"`
	TpEnteGov int             `xml:"tpEnteGov,omitempty"`
	IndDest   int             `xml:"indDest"`
	Dest      *xmlPessoa      `xml:"dest,omitempty"`
	Imovel    *xmlImovel      `xml:"imovel,omitempty"`
	Valores   xmlIBSCBSValores `xml:"valores"`
}

type xmlRefNFSe struct {
	ChNFSe string `xml:"chNFSe"`
}

type xmlImovel struct {
	CIB          string `xml:"cIB,omitempty"`
	InscImobFisc string `xml:"inscImobFisc,omitempty"`
	CMun         string `xml:"cMun,omitempty"`
}

type xmlIBSCBSValores struct {
	CST        string            `xml:"CST"`
	CClassTrib string            `xml:"cClassTrib"`
	VBC        string            `xml:"vBC,omitempty"`
	GIBSUF     *xmlIBSComponente `xml:"gIBSUF,omitempty"`
	GIBSMun    *xmlIBSComponente `xml:"gIBSMun,omitempty"`
	GCBS       *xmlIBSComponente `xml:"gCBS,omitempty"`
	GDif       *xmlDiferimento   `xml:"gDif,omitempty"`
	GCredPres  *xmlCredPresumido `xml:"gCredPres,omitempty"`
	VTotIBS    string            `xml:"vTotIBS,omitempty"`
	VTotCBS    string            `xml:"vTotCBS,omitempty"`
}

type xmlIBSComponente struct {
	PAliq    string `xml:"pAliq,omitempty"`
	PRedAliq string `xml:"pRedAliq,omitempty"`
	VTribOp  string `xml:"vTribOp,omitempty"`
	VTrib    string `xml:"vTrib,omitempty"`
}

type xmlDiferimento struct {
	PDif string `xml:"pDif,omitempty"`
	VDif string `xml:"vDif,omitempty"`
}

type xmlCredPresumido struct {
	CCredPres string `xml:"cCredPres,omitempty"`
	PCredPres string `xml:"pCredPres,omitempty"`
	VCredPres string `xml:"vCredPres,omitempty"`
}

func toXMLIBSCBS(g *nfse.IBSCBS) *xmlIBSCBS {
	if g == nil {
		return nil
	}
	out := &xmlIBSCBS{
		FinNFSe: g.FinNFSe, IndFinal: g.IndFinal, CIndOp: g.CIndOp,
		TpOper: g.TpOper, TpEnteGov: g.TpEnteGov, IndDest: g.IndDest,
		Dest: toXMLPessoa(g.Dest),
		Valores: xmlIBSCBSValores{
			CST: g.Valores.CST, CClassTrib: g.Valores.CClassTrib, VBC: g.Valores.VBC,
			GIBSUF: toXMLComponente(g.Valores.GIBSUF), GIBSMun: toXMLComponente(g.Valores.GIBSMun),
			GCBS: toXMLComponente(g.Valores.GCBS),
			VTotIBS: g.Valores.VTotIBS, VTotCBS: g.Valores.VTotCBS,
		},
	}
	if g.GRefNFSe != nil {
		out.GRefNFSe = &xmlRefNFSe{ChNFSe: g.GRefNFSe.ChNFSe}
	}
	if g.Imovel != nil {
		out.Imovel = &xmlImovel{CIB: g.Imovel.CIB, InscImobFisc: g.Imovel.InscImobFisc, CMun: g.Imovel.CMun}
	}
	if d := g.Valores.GDif; d != nil {
		out.Valores.GDif = &xmlDiferimento{PDif: d.PDif, VDif: d.VDif}
	}
	if c := g.Valores.GCredPres; c != nil {
		out.Valores.GCredPres = &xmlCredPresumido{CCredPres: c.CCredPres,
			PCredPres: c.PCredPres, VCredPres: c.VCredPres}
	}
	return out
}

func toXMLComponente(c *nfse.IBSComponente) *xmlIBSComponente {
	if c == nil {
		return nil
	}
	return &xmlIBSComponente{PAliq: c.PAliq, PRedAliq: c.PRedAliq,
		VTribOp: c.VTribOp, VTrib: c.VTrib}
}
```

- [ ] **Step 5: Rodar os testes**

Run: `cd go-dfe && go test ./nfse/... -v`
Expected: PASS.

- [ ] **Step 6: Gravar o golden da DPS mínima**

Rode o build de uma DPS mínima e salve a saída formatada em `go-dfe/nfse/testdata/dps_minima.xml`, depois adicione um teste que compara byte-a-byte:

```go
func TestBuildDPS_MatchesGolden(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	got, _, err := BuildDPS(minimalDoc(), now)
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	want, err := os.ReadFile("../testdata/dps_minima.xml")
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	if string(got) != strings.TrimSpace(string(want)) {
		t.Errorf("DPS divergiu do golden.\ngot:  %s\nwant: %s", got, want)
	}
}
```

Para gerar o golden na primeira vez: rode o teste, copie a saída de `got` do output da falha para o arquivo, rode de novo e confirme PASS. O golden protege contra reordenação acidental de campos numa refatoração futura — é exatamente o risco que a spec §9 chama de "golden por leiaute".

- [ ] **Step 7: Commit**

```bash
git add go-dfe/nfse/nacional/dps.go go-dfe/nfse/nacional/dps_ibscbs.go \
        go-dfe/nfse/nacional/dps_test.go go-dfe/nfse/testdata
git commit -m "feat(nfse): serializacao da DPS 1.01 com IBS/CBS"
```

---

### Task 4: Pedido de registro de evento

**Files:**
- Create: `go-dfe/nfse/nacional/evento.go`
- Test: `go-dfe/nfse/nacional/evento_test.go`

**Interfaces:**
- Consumes: `nfse.EventRequest`, `nfse.ContribuinteEvents`, `nfse.Namespace` (Task 1).
- Produces: `nacional.BuildPedRegEvento(ev nfse.EventRequest) ([]byte, string, error)` — XML sem assinatura e o `Id` do `infPedReg`.

**Contexto:** estrutura confirmada em `tiposEventos_v1.01.xsd`:

```
TCPedRegEvt  = infPedReg (+ ds:Signature)
TCInfPedReg  = tpAmb, verAplic, dhEvento, CNPJAutor|CPFAutor, chNFSe,
               <um único elemento e{tipo}>                        (@Id)
```

Os grupos específicos que o contribuinte emite:

| Elemento | Tipo | Campos |
|---|---|---|
| `e101101` | TE101101 | `cMotivo`, `xMotivo` |
| `e105102` | TE105102 | `cMotivo`, `xMotivo?`, `chSubstituta` |
| `e101103` | TE101103 | `cMotivo`, `xMotivo` |
| `e202201` | TE202201 | vazio |
| `e203202` | TE203202 | vazio |
| `e204203` | TE204203 | vazio |
| `e202205` | TE202205 | `cMotivo`, `xMotivo?` |
| `e203206` | TE203206 | `cMotivo`, `xMotivo?` |
| `e204207` | TE204207 | `cMotivo`, `xMotivo?` |
| `e205208` | TE205208 | `CPFAgTrib`, `idEvManifRej`, `xMotivo` |

Os tipos `105104`, `105105`, `205204`, `305101`, `305102` e `305103` são privativos do fisco. Emitir um deles é erro — `ContribuinteEvents` é o conjunto fechado.

O `Id` do `infPedReg` segue `PRE` + chave da NFS-e (50) + tipo do evento (6) + `nSeqEvento` com 3 posições.

- [ ] **Step 1: Escrever o teste que falha**

```go
package nacional

import (
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

const chaveTeste = "NFS22110012211122233300018100000000000012026080000000001" // 50+ dígitos após "NFS"

func baseEvent(tipo string) nfse.EventRequest {
	return nfse.EventRequest{
		ChaveAcesso: strings.Repeat("1", 50), TipoEvento: tipo, NSeqEvento: 1,
		TpAmb: 2, VerAplic: "ctech-1.0", CNPJAutor: "11222333000181",
		DhEvento: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
	}
}

func TestBuildPedRegEvento_Cancelamento(t *testing.T) {
	ev := baseEvent(nfse.EventCancelamento)
	ev.Motivo = &nfse.EventMotivo{Codigo: "1", Descricao: "Erro na emissão"}
	out, id, err := BuildPedRegEvento(ev)
	if err != nil {
		t.Fatalf("BuildPedRegEvento: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<e101101>") {
		t.Error("elemento e101101 ausente")
	}
	if !strings.Contains(s, "<cMotivo>1</cMotivo>") {
		t.Error("cMotivo ausente")
	}
	if !strings.Contains(s, `Id="`+id+`"`) {
		t.Errorf("infPedReg sem Id=%q", id)
	}
	if !strings.HasPrefix(id, "PRE") || len(id) != 3+50+6+3 {
		t.Errorf("Id malformado: %q (len %d)", id, len(id))
	}
}

func TestBuildPedRegEvento_ConfirmacaoTomadorSemCorpo(t *testing.T) {
	out, _, err := BuildPedRegEvento(baseEvent(nfse.EventConfirmacaoTomador))
	if err != nil {
		t.Fatalf("BuildPedRegEvento: %v", err)
	}
	if !strings.Contains(string(out), "<e203202>") {
		t.Error("elemento e203202 ausente")
	}
}

func TestBuildPedRegEvento_Substituicao(t *testing.T) {
	ev := baseEvent(nfse.EventCancelamentoPorSubst)
	ev.Motivo = &nfse.EventMotivo{Codigo: "1"}
	ev.ChSubstituta = strings.Repeat("2", 50)
	out, _, err := BuildPedRegEvento(ev)
	if err != nil {
		t.Fatalf("BuildPedRegEvento: %v", err)
	}
	if !strings.Contains(string(out), "<chSubstituta>"+ev.ChSubstituta+"</chSubstituta>") {
		t.Error("chSubstituta ausente")
	}
}

func TestBuildPedRegEvento_RejectsFiscoOnlyEvent(t *testing.T) {
	// 305101 é privativo do município/fisco — só chega pela distribuição.
	if _, _, err := BuildPedRegEvento(baseEvent("305101")); err == nil {
		t.Fatal("esperado erro ao tentar emitir evento privativo do fisco")
	}
}

func TestBuildPedRegEvento_RequiresMotivoWhenTypeDemandsIt(t *testing.T) {
	if _, _, err := BuildPedRegEvento(baseEvent(nfse.EventCancelamento)); err == nil {
		t.Fatal("esperado erro: cancelamento exige motivo")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd go-dfe && go test ./nfse/nacional/ -run TestBuildPedRegEvento -v`
Expected: FAIL — `undefined: BuildPedRegEvento`.

- [ ] **Step 3: Escrever `evento.go`**

```go
package nacional

import (
	"encoding/xml"
	"fmt"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

const (
	idPedRegPrefix   = "PRE"
	widthTipoEvento  = 6
	widthNSeqEvento  = 3
)

type xmlPedRegEvento struct {
	XMLName   xml.Name      `xml:"pedRegEvento"`
	Xmlns     string        `xml:"xmlns,attr"`
	Versao    string        `xml:"versao,attr"`
	InfPedReg xmlInfPedReg  `xml:"infPedReg"`
}

// xmlInfPedReg espelha TCInfPedReg. Apenas UM dos ponteiros e* é preenchido.
type xmlInfPedReg struct {
	ID        string       `xml:"Id,attr"`
	TpAmb     int          `xml:"tpAmb"`
	VerAplic  string       `xml:"verAplic"`
	DhEvento  string       `xml:"dhEvento"`
	CNPJAutor string       `xml:"CNPJAutor,omitempty"`
	CPFAutor  string       `xml:"CPFAutor,omitempty"`
	ChNFSe    string       `xml:"chNFSe"`
	E101101   *xmlMotivo   `xml:"e101101,omitempty"`
	E105102   *xmlSubstEvt `xml:"e105102,omitempty"`
	E101103   *xmlMotivo   `xml:"e101103,omitempty"`
	E202201   *xmlVazio    `xml:"e202201,omitempty"`
	E203202   *xmlVazio    `xml:"e203202,omitempty"`
	E204203   *xmlVazio    `xml:"e204203,omitempty"`
	E202205   *xmlMotivo   `xml:"e202205,omitempty"`
	E203206   *xmlMotivo   `xml:"e203206,omitempty"`
	E204207   *xmlMotivo   `xml:"e204207,omitempty"`
	E205208   *xmlAnulacao `xml:"e205208,omitempty"`
}

type xmlVazio struct{}

type xmlMotivo struct {
	CMotivo string `xml:"cMotivo"`
	XMotivo string `xml:"xMotivo,omitempty"`
}

type xmlSubstEvt struct {
	CMotivo      string `xml:"cMotivo"`
	XMotivo      string `xml:"xMotivo,omitempty"`
	ChSubstituta string `xml:"chSubstituta"`
}

type xmlAnulacao struct {
	CPFAgTrib    string `xml:"CPFAgTrib"`
	IDEvManifRej string `xml:"idEvManifRej"`
	XMotivo      string `xml:"xMotivo"`
}

// eventsRequiringMotivo são os tipos cujo grupo específico tem cMotivo obrigatório.
var eventsRequiringMotivo = map[string]bool{
	nfse.EventCancelamento: true, nfse.EventCancelamentoPorSubst: true,
	nfse.EventSolicAnaliseFiscalCanc: true, nfse.EventRejeicaoPrestador: true,
	nfse.EventRejeicaoTomador: true, nfse.EventRejeicaoIntermediario: true,
}

// BuildPedRegEvento serializa o pedido de registro de evento, ainda SEM
// assinatura. Devolve o XML e o Id do infPedReg.
func BuildPedRegEvento(ev nfse.EventRequest) ([]byte, string, error) {
	if !nfse.ContribuinteEvents[ev.TipoEvento] {
		return nil, "", fmt.Errorf("nacional: evento %q não pode ser emitido pelo contribuinte", ev.TipoEvento)
	}
	if eventsRequiringMotivo[ev.TipoEvento] && (ev.Motivo == nil || ev.Motivo.Codigo == "") {
		return nil, "", fmt.Errorf("nacional: evento %q exige cMotivo", ev.TipoEvento)
	}
	if ev.CNPJAutor == "" && ev.CPFAutor == "" {
		return nil, "", fmt.Errorf("nacional: evento sem CNPJAutor nem CPFAutor")
	}

	seq := ev.NSeqEvento
	if seq == 0 {
		seq = 1
	}
	id := idPedRegPrefix + ev.ChaveAcesso +
		leftPad(ev.TipoEvento, widthTipoEvento) +
		leftPad(fmt.Sprintf("%d", seq), widthNSeqEvento)

	inf := xmlInfPedReg{
		ID: id, TpAmb: ev.TpAmb, VerAplic: ev.VerAplic,
		DhEvento:  ev.DhEvento.UTC().Format(time.RFC3339),
		CNPJAutor: ev.CNPJAutor, CPFAutor: ev.CPFAutor, ChNFSe: ev.ChaveAcesso,
	}

	motivo := &xmlMotivo{}
	if ev.Motivo != nil {
		motivo = &xmlMotivo{CMotivo: ev.Motivo.Codigo, XMotivo: ev.Motivo.Descricao}
	}

	switch ev.TipoEvento {
	case nfse.EventCancelamento:
		inf.E101101 = motivo
	case nfse.EventCancelamentoPorSubst:
		inf.E105102 = &xmlSubstEvt{CMotivo: motivo.CMotivo, XMotivo: motivo.XMotivo,
			ChSubstituta: ev.ChSubstituta}
	case nfse.EventSolicAnaliseFiscalCanc:
		inf.E101103 = motivo
	case nfse.EventConfirmacaoPrestador:
		inf.E202201 = &xmlVazio{}
	case nfse.EventConfirmacaoTomador:
		inf.E203202 = &xmlVazio{}
	case nfse.EventConfirmacaoIntermediario:
		inf.E204203 = &xmlVazio{}
	case nfse.EventRejeicaoPrestador:
		inf.E202205 = motivo
	case nfse.EventRejeicaoTomador:
		inf.E203206 = motivo
	case nfse.EventRejeicaoIntermediario:
		inf.E204207 = motivo
	case nfse.EventAnulacaoRejeicao:
		inf.E205208 = &xmlAnulacao{CPFAgTrib: ev.CPFAgTrib,
			IDEvManifRej: ev.IDEvManifRej, XMotivo: motivo.XMotivo}
	}

	out, err := xml.Marshal(xmlPedRegEvento{
		Xmlns: nfse.Namespace, Versao: nfse.LayoutVersion, InfPedReg: inf,
	})
	if err != nil {
		return nil, "", fmt.Errorf("nacional: serializar pedRegEvento: %w", err)
	}
	return out, id, nil
}
```

- [ ] **Step 4: Rodar os testes**

Run: `cd go-dfe && go test ./nfse/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go-dfe/nfse/nacional/evento.go go-dfe/nfse/nacional/evento_test.go
git commit -m "feat(nfse): pedido de registro de evento do padrao nacional"
```

---

### Task 5: Transporte — assinatura, gzip/base64 e HTTP mTLS

**Files:**
- Create: `go-dfe/nfse/nacional/transport.go`
- Test: `go-dfe/nfse/nacional/transport_test.go`

**Interfaces:**
- Consumes: `xmlops.Sign(xmlDoc []byte, idXPath string, cert *x509.Certificate, key *rsa.PrivateKey) ([]byte, error)` de `go-dfe/internal/xmlops`; `nfse.FiscalError`, `nfse.Message`.
- Produces:
  - `nacional.SignDPS(xmlBytes []byte, cert *x509.Certificate, key *rsa.PrivateKey) ([]byte, error)`
  - `nacional.SignPedRegEvento(...)` — mesma assinatura
  - `nacional.GzipB64(raw []byte) (string, error)` e `nacional.UngzipB64(s string) ([]byte, error)`
  - `nacional.httpDo(ctx, client *http.Client, method, url string, body any, out any) (int, error)`

**Contexto:**

- `xmlops.Sign` recebe o xpath no formato Clark (`.//{namespace}localName`) — o mesmo formato que `services.Client.Call` monta hoje (`client.go`, ramo `RequiresSignature`). Para NFS-e: `.//{http://www.sped.fazenda.gov.br/nfse}infDPS` e `.//{http://www.sped.fazenda.gov.br/nfse}infPedReg`.
- Envelopes JSON confirmados em `tmp/nfse-sefin.json`:
  - `POST /nfse` recebe `{"dpsXmlGZipB64": "..."}`; 201 devolve `{tipoAmbiente, versaoAplicativo, dataHoraProcessamento, idDps, chaveAcesso, nfseXmlGZipB64, alertas}`; 400 devolve `{tipoAmbiente, versaoAplicativo, dataHoraProcessamento, idDPS, erros}` — repare que a chave do id muda de `idDps` para `idDPS` no caminho de erro.
  - `POST /nfse/{chave}/eventos` recebe `{"pedidoRegistroEventoXmlGZipB64": "..."}`; sucesso devolve `{..., eventoXmlGZipB64}`.
  - `GET /nfse/{chave}` devolve `{..., chaveAcesso, nfseXmlGZipB64}`.
  - `GET /dps/{id}` devolve `{tipoAmbiente, versaoAplicativo, dataHoraProcessamento, idDps, chaveAcesso}`.
  - Erro genérico: `{tipoAmbiente, versaoAplicativo, dataHoraProcessamento, erro}` com `erro` sendo `MensagemProcessamento{codigo, descricao, complemento}`.
- Retry: só 5xx e erro de rede, nunca 4xx — a mesma política de `internal/services/client.go` (`retryableHTTPStatus`). Backoff base de 1s, igual.

- [ ] **Step 1: Escrever o teste que falha**

```go
package nacional

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func TestGzipB64_RoundTrip(t *testing.T) {
	raw := []byte("<DPS><infDPS/></DPS>")
	enc, err := GzipB64(raw)
	if err != nil {
		t.Fatalf("GzipB64: %v", err)
	}
	got, err := UngzipB64(enc)
	if err != nil {
		t.Fatalf("UngzipB64: %v", err)
	}
	if string(got) != string(raw) {
		t.Errorf("round-trip = %q, esperado %q", got, raw)
	}
}

func TestHTTPDo_FiscalErrorPreservesCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"erros": []map[string]string{
				{"codigo": "E0001", "descricao": "cTribNac inválido", "complemento": "linha 1"},
			},
		})
	}))
	defer srv.Close()

	var out map[string]any
	_, err := httpDo(context.Background(), srv.Client(), http.MethodGet, srv.URL, nil, &out, 0)
	var fe *nfse.FiscalError
	if !errors.As(err, &fe) {
		t.Fatalf("esperado *nfse.FiscalError, veio %v", err)
	}
	if len(fe.Messages) != 1 || fe.Messages[0].Codigo != "E0001" {
		t.Errorf("mensagem do fisco perdida: %+v", fe.Messages)
	}
	if fe.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, esperado 400", fe.Status)
	}
}

func TestHTTPDo_RetriesOn5xxNotOn4xx(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"chaveAcesso": "abc"})
	}))
	defer srv.Close()

	var out map[string]any
	if _, err := httpDo(context.Background(), srv.Client(), http.MethodGet, srv.URL, nil, &out, 3); err != nil {
		t.Fatalf("httpDo: %v", err)
	}
	if calls != 3 {
		t.Errorf("chamadas = %d, esperado 3 (dois 502 + sucesso)", calls)
	}
	if out["chaveAcesso"] != "abc" {
		t.Errorf("resposta não decodificada: %+v", out)
	}

	calls = 0
	srv4 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"erro": map[string]string{"codigo": "X"}})
	}))
	defer srv4.Close()
	_, _ = httpDo(context.Background(), srv4.Client(), http.MethodGet, srv4.URL, nil, &out, 3)
	if calls != 1 {
		t.Errorf("4xx foi repetido %d vezes; rejeição de negócio nunca se repete", calls)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd go-dfe && go test ./nfse/nacional/ -run 'TestGzipB64|TestHTTPDo' -v`
Expected: FAIL — `undefined: GzipB64`.

- [ ] **Step 3: Escrever `transport.go`**

```go
package nacional

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/internal/xmlops"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// XPaths de assinatura, no formato Clark que xmlops.Sign espera — o mesmo
// que internal/services/client.go monta para os demais doc types.
const (
	signXPathInfDPS    = ".//{" + nfse.Namespace + "}infDPS"
	signXPathInfPedReg = ".//{" + nfse.Namespace + "}infPedReg"
)

// Política de retry idêntica à de internal/services/client.go: só
// infraestrutura, nunca rejeição de negócio.
const backoffBase = 1 * time.Second

var retryableHTTPStatus = map[int]bool{500: true, 502: true, 503: true, 504: true}

// SignDPS assina infDPS com o XML-DSig já usado pelos demais documentos
// (enveloped, RSA-SHA1, digest SHA-1, C14N 1.0).
func SignDPS(xmlBytes []byte, cert *x509.Certificate, key *rsa.PrivateKey) ([]byte, error) {
	return xmlops.Sign(xmlBytes, signXPathInfDPS, cert, key)
}

// SignPedRegEvento assina infPedReg.
func SignPedRegEvento(xmlBytes []byte, cert *x509.Certificate, key *rsa.PrivateKey) ([]byte, error) {
	return xmlops.Sign(xmlBytes, signXPathInfPedReg, cert, key)
}

// GzipB64 comprime e codifica o XML no formato que a API nacional exige
// (dpsXmlGZipB64, pedidoRegistroEventoXmlGZipB64, nfseXmlGZipB64).
func GzipB64(raw []byte) (string, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return "", fmt.Errorf("nacional: gzip: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("nacional: gzip close: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// UngzipB64 é o inverso de GzipB64.
func UngzipB64(s string) ([]byte, error) {
	blob, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("nacional: base64: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		return nil, fmt.Errorf("nacional: gzip reader: %w", err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("nacional: gunzip: %w", err)
	}
	return out, nil
}

// errorEnvelope cobre as duas formas de erro do Sefin: "erro" singular
// (MensagemProcessamento) e "erros" plural (NFSePostResponseErro).
type errorEnvelope struct {
	Erro  *nfse.Message  `json:"erro"`
	Erros []nfse.Message `json:"erros"`
}

// httpDo executa a requisição com retry apenas em falha de infraestrutura e
// converte qualquer resposta não-2xx em *nfse.FiscalError com o código e a
// descrição do fisco preservados. out pode ser nil (resposta binária).
func httpDo(ctx context.Context, client *http.Client, method, url string, body, out any, maxRetries int) (int, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("nacional: encode request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(attempt) * backoffBase):
			}
		}

		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return 0, fmt.Errorf("nacional: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("nacional: %s %s: %w", method, url, err)
			continue
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("nacional: ler resposta: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(respBody) > 0 {
				if err := json.Unmarshal(respBody, out); err != nil {
					return resp.StatusCode, fmt.Errorf("nacional: decode resposta: %w", err)
				}
			}
			return resp.StatusCode, nil
		}

		if retryableHTTPStatus[resp.StatusCode] {
			lastErr = fmt.Errorf("nacional: HTTP %d", resp.StatusCode)
			continue
		}
		return resp.StatusCode, toFiscalError(resp.StatusCode, respBody)
	}
	return 0, lastErr
}

func toFiscalError(status int, body []byte) error {
	var env errorEnvelope
	_ = json.Unmarshal(body, &env)
	msgs := env.Erros
	if env.Erro != nil {
		msgs = append(msgs, *env.Erro)
	}
	if len(msgs) == 0 {
		msgs = []nfse.Message{{Descricao: string(body)}}
	}
	return &nfse.FiscalError{Status: status, Messages: msgs}
}
```

- [ ] **Step 4: Rodar os testes**

Run: `cd go-dfe && go test ./nfse/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go-dfe/nfse/nacional/transport.go go-dfe/nfse/nacional/transport_test.go
git commit -m "feat(nfse): transporte REST mTLS com gzip base64 e retry"
```

---

### Task 6: Provider nacional — emissão, evento e consultas

**Files:**
- Create: `go-dfe/nfse/nacional/provider.go`
- Test: `go-dfe/nfse/nacional/provider_test.go`

**Interfaces:**
- Consumes: tudo das Tasks 2 a 5.
- Produces:
  - `nacional.Config{Environment string; HTTPClient *http.Client; Cert *x509.Certificate; Key *rsa.PrivateKey; MaxRetries int; CNPJ string}`
  - `nacional.New(cfg Config) (*Nacional, error)`
  - `*Nacional` implementando `nfse.Provider`

**Contexto:** `Emit` faz, em ordem: `BuildDPS` → `SignDPS` → `GzipB64` → `POST /nfse` com `{"dpsXmlGZipB64": ...}` → descompacta `nfseXmlGZipB64` → devolve `Result` com `ChaveAcesso`, `IDDPS`, `NFSeXML` e o `DPSXML` assinado (o worker persiste os dois no S3, spec §6).

`QueryEvents` tem duas rotas: com `TipoEvento` e `NSeqEvento` preenchidos vai no Sefin (`GET /nfse/{chave}/eventos/{tipo}/{seq}`); sem eles vai no ADN (`GET /NFSe/{chave}/Eventos`), porque o Swagger do Sefin não expõe listagem completa.

- [ ] **Step 1: Escrever o teste que falha**

```go
package nacional

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// newTestProvider aponta o provider para um httptest.Server substituindo as
// bases resolvidas. baseOverride existe só para teste.
func newTestProvider(t *testing.T, srvURL string) *Nacional {
	t.Helper()
	p, err := New(Config{Environment: "hom", HTTPClient: http.DefaultClient, MaxRetries: 0})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.baseOverride = map[string]string{
		SystemSefin: srvURL, SystemADN: srvURL,
		SystemDANFSE: srvURL, SystemParametros: srvURL,
	}
	return p
}

func TestNacional_Emit(t *testing.T) {
	nfseXML := "<NFSe><infNFSe/></NFSe>"
	encoded, err := GzipB64([]byte(nfseXML))
	if err != nil {
		t.Fatalf("GzipB64: %v", err)
	}

	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != PathNFSe || r.Method != http.MethodPost {
			t.Errorf("rota inesperada: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente": 2, "versaoAplicativo": "1.0",
			"idDps": "DPS" + strings.Repeat("1", 42),
			"chaveAcesso":    strings.Repeat("9", 50),
			"nfseXmlGZipB64": encoded,
		})
	}))
	defer srv.Close()

	// Sem certificado o provider não assina; o teste cobre a montagem e o
	// transporte. A assinatura tem cobertura própria em xmlops/signer_test.go.
	p := newTestProvider(t, srv.URL)
	res, err := p.Emit(context.Background(), minimalDoc())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if received["dpsXmlGZipB64"] == "" {
		t.Error("dpsXmlGZipB64 não foi enviado")
	}
	if res.ChaveAcesso != strings.Repeat("9", 50) {
		t.Errorf("ChaveAcesso = %q", res.ChaveAcesso)
	}
	if res.NFSeXML != nfseXML {
		t.Errorf("NFSeXML = %q, esperado %q", res.NFSeXML, nfseXML)
	}
	if !strings.Contains(res.DPSXML, "<infDPS") {
		t.Error("DPSXML enviado não foi devolvido no Result")
	}
}

func TestNacional_QueryByDPSID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/dps/") {
			t.Errorf("rota inesperada: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idDps": "DPS123", "chaveAcesso": strings.Repeat("7", 50),
		})
	}))
	defer srv.Close()

	res, err := newTestProvider(t, srv.URL).QueryByDPSID(context.Background(), "DPS123")
	if err != nil {
		t.Fatalf("QueryByDPSID: %v", err)
	}
	if res.ChaveAcesso != strings.Repeat("7", 50) {
		t.Errorf("ChaveAcesso = %q", res.ChaveAcesso)
	}
}

func TestNacional_Event(t *testing.T) {
	eventoXML := "<evento><infEvento/></evento>"
	encoded, _ := GzipB64([]byte(eventoXML))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/eventos") || r.Method != http.MethodPost {
			t.Errorf("rota inesperada: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"eventoXmlGZipB64": encoded})
	}))
	defer srv.Close()

	ev := baseEvent(nfse.EventCancelamento)
	ev.Motivo = &nfse.EventMotivo{Codigo: "1", Descricao: "Erro na emissão"}
	res, err := newTestProvider(t, srv.URL).Event(context.Background(), ev)
	if err != nil {
		t.Fatalf("Event: %v", err)
	}
	if res.EventoXML != eventoXML {
		t.Errorf("EventoXML = %q", res.EventoXML)
	}
}

func TestNacional_EmitPropagatesFiscalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"erros": []map[string]string{{"codigo": "E1", "descricao": "rejeitado"}},
		})
	}))
	defer srv.Close()

	_, err := newTestProvider(t, srv.URL).Emit(context.Background(), minimalDoc())
	if err == nil || !strings.Contains(err.Error(), "rejeitado") {
		t.Fatalf("rejeição do fisco não propagada: %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd go-dfe && go test ./nfse/nacional/ -run TestNacional -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Escrever `provider.go`**

```go
package nacional

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// Nomes de campo dos envelopes JSON do Sefin Nacional (tmp/nfse-sefin.json).
const (
	fieldDpsXMLGZipB64      = "dpsXmlGZipB64"
	fieldPedRegEvtXMLGZipB64 = "pedidoRegistroEventoXmlGZipB64"
)

// Config configura o provider nacional. Cert/Key podem ser nil apenas em
// teste — sem eles a DPS segue sem assinatura e o fisco rejeita.
type Config struct {
	Environment string
	HTTPClient  *http.Client
	Cert        *x509.Certificate
	Key         *rsa.PrivateKey
	MaxRetries  int
	CNPJ        string
	Now         func() time.Time
}

// Nacional implementa nfse.Provider contra o Sistema Nacional NFS-e.
type Nacional struct {
	cfg Config
	// baseOverride substitui as bases resolvidas; usado só pelos testes,
	// que apontam todos os sistemas para um httptest.Server.
	baseOverride map[string]string
}

// New valida a configuração e devolve o provider.
func New(cfg Config) (*Nacional, error) {
	if cfg.HTTPClient == nil {
		return nil, fmt.Errorf("nacional: HTTPClient é obrigatório (mTLS)")
	}
	if _, err := ResolveBase(SystemSefin, cfg.Environment); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Nacional{cfg: cfg}, nil
}

func (n *Nacional) base(system string) (string, error) {
	if n.baseOverride != nil {
		if b, ok := n.baseOverride[system]; ok {
			return b, nil
		}
	}
	return ResolveBase(system, n.cfg.Environment)
}

// emitResponse cobre NFSePostResponseSucesso.
type emitResponse struct {
	TipoAmbiente          int            `json:"tipoAmbiente"`
	VersaoAplicativo      string         `json:"versaoAplicativo"`
	DataHoraProcessamento string         `json:"dataHoraProcessamento"`
	IDDps                 string         `json:"idDps"`
	ChaveAcesso           string         `json:"chaveAcesso"`
	NFSeXMLGZipB64        string         `json:"nfseXmlGZipB64"`
	Alertas               []nfse.Message `json:"alertas"`
}

// Emit monta a DPS, assina, comprime e envia. O POST é síncrono: a resposta
// 201 já traz a NFS-e gerada.
func (n *Nacional) Emit(ctx context.Context, doc nfse.Document) (nfse.Result, error) {
	raw, idDPS, err := BuildDPS(doc, n.cfg.Now())
	if err != nil {
		return nfse.Result{}, err
	}
	if n.cfg.Key != nil {
		raw, err = SignDPS(raw, n.cfg.Cert, n.cfg.Key)
		if err != nil {
			return nfse.Result{}, fmt.Errorf("nacional: assinar DPS: %w", err)
		}
	}
	packed, err := GzipB64(raw)
	if err != nil {
		return nfse.Result{}, err
	}

	base, err := n.base(SystemSefin)
	if err != nil {
		return nfse.Result{}, err
	}
	var resp emitResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodPost, base+PathNFSe,
		map[string]string{fieldDpsXMLGZipB64: packed}, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}

	res := nfse.Result{
		ChaveAcesso: resp.ChaveAcesso, IDDPS: resp.IDDps,
		DPSXML: string(raw), Ambiente: resp.TipoAmbiente,
		VersaoAplicativo: resp.VersaoAplicativo,
		DataHoraProcessamento: resp.DataHoraProcessamento,
		Alertas: resp.Alertas,
	}
	if res.IDDPS == "" {
		res.IDDPS = idDPS
	}
	if resp.NFSeXMLGZipB64 != "" {
		nfseXML, err := UngzipB64(resp.NFSeXMLGZipB64)
		if err != nil {
			return res, err
		}
		res.NFSeXML = string(nfseXML)
	}
	return res, nil
}

type eventResponse struct {
	TipoAmbiente          int    `json:"tipoAmbiente"`
	VersaoAplicativo      string `json:"versaoAplicativo"`
	DataHoraProcessamento string `json:"dataHoraProcessamento"`
	EventoXMLGZipB64      string `json:"eventoXmlGZipB64"`
}

// Event envia o pedido de registro de evento e devolve o evento gerado.
func (n *Nacional) Event(ctx context.Context, ev nfse.EventRequest) (nfse.Result, error) {
	raw, _, err := BuildPedRegEvento(ev)
	if err != nil {
		return nfse.Result{}, err
	}
	if n.cfg.Key != nil {
		raw, err = SignPedRegEvento(raw, n.cfg.Cert, n.cfg.Key)
		if err != nil {
			return nfse.Result{}, fmt.Errorf("nacional: assinar pedRegEvento: %w", err)
		}
	}
	packed, err := GzipB64(raw)
	if err != nil {
		return nfse.Result{}, err
	}
	base, err := n.base(SystemSefin)
	if err != nil {
		return nfse.Result{}, err
	}

	var resp eventResponse
	url := base + fmt.Sprintf(PathEventos, ev.ChaveAcesso)
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodPost, url,
		map[string]string{fieldPedRegEvtXMLGZipB64: packed}, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}

	res := nfse.Result{
		ChaveAcesso: ev.ChaveAcesso, PedRegEventoXML: string(raw),
		Ambiente: resp.TipoAmbiente, VersaoAplicativo: resp.VersaoAplicativo,
		DataHoraProcessamento: resp.DataHoraProcessamento,
	}
	if resp.EventoXMLGZipB64 != "" {
		evXML, err := UngzipB64(resp.EventoXMLGZipB64)
		if err != nil {
			return res, err
		}
		res.EventoXML = string(evXML)
	}
	return res, nil
}

type queryResponse struct {
	TipoAmbiente          int    `json:"tipoAmbiente"`
	VersaoAplicativo      string `json:"versaoAplicativo"`
	DataHoraProcessamento string `json:"dataHoraProcessamento"`
	ChaveAcesso           string `json:"chaveAcesso"`
	IDDps                 string `json:"idDps"`
	NFSeXMLGZipB64        string `json:"nfseXmlGZipB64"`
	EventoXMLGZipB64      string `json:"eventoXmlGZipB64"`
}

// QueryByKey consulta a NFS-e pela chave de acesso.
func (n *Nacional) QueryByKey(ctx context.Context, key string) (nfse.Result, error) {
	base, err := n.base(SystemSefin)
	if err != nil {
		return nfse.Result{}, err
	}
	var resp queryResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet,
		base+fmt.Sprintf(PathNFSeByKey, key), nil, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	return n.toResult(resp)
}

// QueryByDPSID recupera a chave de acesso a partir do identificador da DPS —
// é o caminho de recuperação em retry (spec §3.4).
func (n *Nacional) QueryByDPSID(ctx context.Context, idDPS string) (nfse.Result, error) {
	base, err := n.base(SystemSefin)
	if err != nil {
		return nfse.Result{}, err
	}
	var resp queryResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet,
		base+fmt.Sprintf(PathDPS, idDPS), nil, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	return n.toResult(resp)
}

// QueryEvents busca um evento específico no Sefin quando tipo e sequencial
// vêm preenchidos; caso contrário lista todos pelo ADN, porque o Sefin não
// expõe listagem completa (tmp/nfse-sefin.json vs tmp/nfse-adn-contribuintes.json).
func (n *Nacional) QueryEvents(ctx context.Context, f nfse.EventFilter) (nfse.Result, error) {
	if f.TipoEvento != "" && f.NSeqEvento > 0 {
		base, err := n.base(SystemSefin)
		if err != nil {
			return nfse.Result{}, err
		}
		var resp queryResponse
		url := base + fmt.Sprintf(PathEventoEspecifico, f.ChaveAcesso, f.TipoEvento, f.NSeqEvento)
		if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet, url, nil, &resp, n.cfg.MaxRetries); err != nil {
			return nfse.Result{}, err
		}
		return n.toResult(resp)
	}
	return n.listEventsADN(ctx, f.ChaveAcesso)
}

func (n *Nacional) toResult(resp queryResponse) (nfse.Result, error) {
	res := nfse.Result{
		ChaveAcesso: resp.ChaveAcesso, IDDPS: resp.IDDps,
		Ambiente: resp.TipoAmbiente, VersaoAplicativo: resp.VersaoAplicativo,
		DataHoraProcessamento: resp.DataHoraProcessamento,
	}
	if resp.NFSeXMLGZipB64 != "" {
		x, err := UngzipB64(resp.NFSeXMLGZipB64)
		if err != nil {
			return res, err
		}
		res.NFSeXML = string(x)
	}
	if resp.EventoXMLGZipB64 != "" {
		x, err := UngzipB64(resp.EventoXMLGZipB64)
		if err != nil {
			return res, err
		}
		res.EventoXML = string(x)
	}
	return res, nil
}
```

- [ ] **Step 4: Rodar os testes**

Run: `cd go-dfe && go test ./nfse/... -v`
Expected: FAIL em `TestNacional_QueryEvents`-adjacentes por `listEventsADN` indefinido — implementado na Task 7. Os demais testes desta task passam. Se preferir manter a suíte verde entre commits, mova o corpo de `QueryEvents` sem-filtro para `return nfse.Result{}, fmt.Errorf("nacional: listagem de eventos exige ADN")` e substitua na Task 7.

- [ ] **Step 5: Commit**

```bash
git add go-dfe/nfse/nacional/provider.go go-dfe/nfse/nacional/provider_test.go
git commit -m "feat(nfse): provider nacional com emissao, evento e consultas"
```

---

### Task 7: ADN — distribuição por NSU, DANFSE e parâmetros municipais

**Files:**
- Create: `go-dfe/nfse/nacional/adn.go`
- Test: `go-dfe/nfse/nacional/adn_test.go`

**Interfaces:**
- Consumes: `httpDo`, `ResolveBase`, paths da Task 2.
- Produces, em `*Nacional`:
  - `Distribute(ctx context.Context, nsu int64, cnpjConsulta string, lote bool) (nfse.Result, error)`
  - `DANFSE(ctx context.Context, chave string) ([]byte, error)`
  - `MunicipalParameters(ctx context.Context, kind string, args ...string) (nfse.Result, error)`
  - `listEventsADN(ctx context.Context, chave string) (nfse.Result, error)`
  - constantes `ParamAliquota`, `ParamConvenio`, `ParamBeneficio`, `ParamRegimesEspeciais`, `ParamRetencoes`

**Contexto:** `GET /DFe/{NSU}` devolve `LoteDistribuicaoNSUResponse` (`tmp/nfse-adn-contribuintes.json`), com **PascalCase** nos campos — diferente do Sefin, que usa camelCase:

```
{ StatusProcessamento, LoteDFe: [{NSU, ChaveAcesso, TipoDocumento, TipoEvento,
  ArquivoXml, DataHoraGeracao}], Alertas, Erros, TipoAmbiente,
  VersaoAplicativo, DataHoraProcessamento }
```

`StatusProcessamento` é enum: `REJEICAO`, `NENHUM_DOCUMENTO_LOCALIZADO`, `DOCUMENTOS_LOCALIZADOS`. `MensagemProcessamento` do ADN também é PascalCase (`Codigo`, `Descricao`, `Complemento`) — daí a struct própria `adnMessage`, sem reuso de `nfse.Message`, cujas tags são camelCase.

O DANFSE devolve PDF binário, não JSON.

- [ ] **Step 1: Escrever o teste que falha**

```go
package nacional

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNacional_Distribute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/DFe/10" {
			t.Errorf("path = %q, esperado /DFe/10", r.URL.Path)
		}
		if r.URL.Query().Get("cnpjConsulta") != "11222333000181" {
			t.Errorf("cnpjConsulta ausente: %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"StatusProcessamento": "DOCUMENTOS_LOCALIZADOS",
			"LoteDFe": []map[string]any{{
				"NSU": 11, "ChaveAcesso": "abc", "TipoDocumento": "NFSE",
				"ArquivoXml": "<NFSe/>", "DataHoraGeracao": "2026-08-04T12:00:00Z",
			}},
		})
	}))
	defer srv.Close()

	res, err := newTestProvider(t, srv.URL).Distribute(context.Background(), 10, "11222333000181", true)
	if err != nil {
		t.Fatalf("Distribute: %v", err)
	}
	if res.StatusDistribuicao != "DOCUMENTOS_LOCALIZADOS" {
		t.Errorf("StatusDistribuicao = %q", res.StatusDistribuicao)
	}
	if len(res.Distribuicao) != 1 || res.Distribuicao[0].NSU != 11 {
		t.Fatalf("lote não parseado: %+v", res.Distribuicao)
	}
	if res.Distribuicao[0].XML != "<NFSe/>" {
		t.Errorf("ArquivoXml perdido: %q", res.Distribuicao[0].XML)
	}
}

func TestNacional_DANFSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	}))
	defer srv.Close()

	pdf, err := newTestProvider(t, srv.URL).DANFSE(context.Background(), "chave")
	if err != nil {
		t.Fatalf("DANFSE: %v", err)
	}
	if string(pdf[:4]) != "%PDF" {
		t.Errorf("resposta não é PDF: %q", pdf)
	}
}

func TestNacional_MunicipalParameters_Convenio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2211001/convenio" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"aderenteAmbienteNacional": true})
	}))
	defer srv.Close()

	res, err := newTestProvider(t, srv.URL).MunicipalParameters(context.Background(), ParamConvenio, "2211001")
	if err != nil {
		t.Fatalf("MunicipalParameters: %v", err)
	}
	if res.Parametros["aderenteAmbienteNacional"] != true {
		t.Errorf("parâmetros não parseados: %+v", res.Parametros)
	}
}

func TestNacional_MunicipalParameters_WrongArity(t *testing.T) {
	if _, err := newTestProvider(t, "http://x").MunicipalParameters(context.Background(), ParamAliquota, "2211001"); err == nil {
		t.Fatal("esperado erro: aliquota exige município, serviço e competência")
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd go-dfe && go test ./nfse/nacional/ -run 'TestNacional_Distribute|TestNacional_DANFSE|TestNacional_Municipal' -v`
Expected: FAIL — `undefined: Distribute`.

- [ ] **Step 3: Escrever `adn.go`**

```go
package nacional

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// Tipos de consulta de parâmetros municipais.
const (
	ParamAliquota         = "aliquota"
	ParamConvenio         = "convenio"
	ParamBeneficio        = "beneficio"
	ParamRegimesEspeciais = "regimes_especiais"
	ParamRetencoes        = "retencoes"
)

// paramArity é quantos argumentos além do tipo cada consulta exige.
var paramArity = map[string]int{
	ParamAliquota: 3, ParamConvenio: 1, ParamBeneficio: 3,
	ParamRegimesEspeciais: 3, ParamRetencoes: 2,
}

// adnMessage é o MensagemProcessamento do ADN — PascalCase, diferente do
// Sefin (tmp/nfse-adn-contribuintes.json).
type adnMessage struct {
	Codigo     string `json:"Codigo"`
	Descricao  string `json:"Descricao"`
	Complemento string `json:"Complemento"`
}

type adnDistribuicaoItem struct {
	NSU             int64  `json:"NSU"`
	ChaveAcesso     string `json:"ChaveAcesso"`
	TipoDocumento   string `json:"TipoDocumento"`
	TipoEvento      string `json:"TipoEvento"`
	ArquivoXml      string `json:"ArquivoXml"`
	DataHoraGeracao string `json:"DataHoraGeracao"`
}

type adnLoteResponse struct {
	StatusProcessamento   string                `json:"StatusProcessamento"`
	LoteDFe               []adnDistribuicaoItem `json:"LoteDFe"`
	Alertas               []adnMessage          `json:"Alertas"`
	Erros                 []adnMessage          `json:"Erros"`
	TipoAmbiente          int                   `json:"TipoAmbiente"`
	VersaoAplicativo      string                `json:"VersaoAplicativo"`
	DataHoraProcessamento string                `json:"DataHoraProcessamento"`
}

func toNfseMessages(in []adnMessage) []nfse.Message {
	if len(in) == 0 {
		return nil
	}
	out := make([]nfse.Message, 0, len(in))
	for _, m := range in {
		out = append(out, nfse.Message{Codigo: m.Codigo, Descricao: m.Descricao, Complemento: m.Complemento})
	}
	return out
}

func (r adnLoteResponse) toResult() nfse.Result {
	res := nfse.Result{
		StatusDistribuicao: r.StatusProcessamento, Ambiente: r.TipoAmbiente,
		VersaoAplicativo: r.VersaoAplicativo, DataHoraProcessamento: r.DataHoraProcessamento,
		Alertas: toNfseMessages(r.Alertas), Erros: toNfseMessages(r.Erros),
	}
	for _, it := range r.LoteDFe {
		res.Distribuicao = append(res.Distribuicao, nfse.DistributionItem{
			NSU: it.NSU, ChaveAcesso: it.ChaveAcesso, TipoDocumento: it.TipoDocumento,
			TipoEvento: it.TipoEvento, XML: it.ArquivoXml, DataHoraGeracao: it.DataHoraGeracao,
		})
	}
	return res
}

// Distribute busca documentos fiscais de serviço a partir de um NSU.
// cnpjConsulta permite consultar outro CNPJ da mesma raiz do certificado.
func (n *Nacional) Distribute(ctx context.Context, nsu int64, cnpjConsulta string, lote bool) (nfse.Result, error) {
	base, err := n.base(SystemADN)
	if err != nil {
		return nfse.Result{}, err
	}
	url := base + fmt.Sprintf(PathDistribuicaoNSU, nsu) + fmt.Sprintf("?lote=%t", lote)
	if cnpjConsulta != "" {
		url += "&cnpjConsulta=" + cnpjConsulta
	}
	var resp adnLoteResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet, url, nil, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	return resp.toResult(), nil
}

// listEventsADN lista todos os eventos de uma chave. É o caminho usado por
// QueryEvents sem tipo/sequencial — o Sefin não expõe listagem completa.
func (n *Nacional) listEventsADN(ctx context.Context, chave string) (nfse.Result, error) {
	base, err := n.base(SystemADN)
	if err != nil {
		return nfse.Result{}, err
	}
	var resp adnLoteResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet,
		base+fmt.Sprintf(PathEventosADN, chave), nil, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	res := resp.toResult()
	res.ChaveAcesso = chave
	return res, nil
}

// DANFSE baixa o PDF da NFS-e. Resposta binária, não JSON.
func (n *Nacional) DANFSE(ctx context.Context, chave string) ([]byte, error) {
	base, err := n.base(SystemDANFSE)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+fmt.Sprintf(PathDANFSE, chave), nil)
	if err != nil {
		return nil, fmt.Errorf("nacional: build request DANFSE: %w", err)
	}
	req.Header.Set("Accept", "application/pdf")
	resp, err := n.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nacional: DANFSE: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nacional: ler DANFSE: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, toFiscalError(resp.StatusCode, body)
	}
	return body, nil
}

// MunicipalParameters consulta a parametrização do município. args segue a
// ordem do path de cada tipo:
//
//	aliquota          -> município, serviço, competência
//	convenio          -> município
//	beneficio         -> município, número do benefício, competência
//	regimes_especiais -> município, serviço, competência
//	retencoes         -> município, competência
func (n *Nacional) MunicipalParameters(ctx context.Context, kind string, args ...string) (nfse.Result, error) {
	want, ok := paramArity[kind]
	if !ok {
		return nfse.Result{}, fmt.Errorf("nacional: tipo de parâmetro municipal desconhecido %q", kind)
	}
	if len(args) != want {
		return nfse.Result{}, fmt.Errorf("nacional: %q exige %d argumentos, recebeu %d", kind, want, len(args))
	}

	base, err := n.base(SystemParametros)
	if err != nil {
		return nfse.Result{}, err
	}
	var path string
	switch kind {
	case ParamAliquota:
		path = fmt.Sprintf(PathParamAliquota, args[0], args[1], args[2])
	case ParamConvenio:
		path = fmt.Sprintf(PathParamConvenio, args[0])
	case ParamBeneficio:
		path = fmt.Sprintf(PathParamBeneficio, args[0], args[1], args[2])
	case ParamRegimesEspeciais:
		path = fmt.Sprintf(PathParamRegimesEspeciais, args[0], args[1], args[2])
	case ParamRetencoes:
		path = fmt.Sprintf(PathParamRetencoes, args[0], args[1])
	}

	var out map[string]any
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet, base+path, nil, &out, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	return nfse.Result{Parametros: out}, nil
}
```

- [ ] **Step 4: Restaurar `QueryEvents`**

Se na Task 6 você usou o `fmt.Errorf` provisório, troque-o de volta por `return n.listEventsADN(ctx, f.ChaveAcesso)`.

- [ ] **Step 5: Rodar toda a suíte**

Run: `cd go-dfe && go test ./nfse/... -v && CGO_ENABLED=0 GOARCH=arm64 go build ./...`
Expected: PASS, build limpo.

- [ ] **Step 6: Commit**

```bash
git add go-dfe/nfse/nacional/adn.go go-dfe/nfse/nacional/adn_test.go go-dfe/nfse/nacional/provider.go
git commit -m "feat(nfse): distribuicao ADN, DANFSE e parametros municipais"
```

---

### Task 8: Wiring em `dfe.Call` e `dfe.Implements`

**Files:**
- Modify: `go-dfe/dfe.go`
- Modify: `go-dfe/dfe_test.go`
- Create: `go-dfe/nfse/dispatch.go`

**Interfaces:**
- Consumes: `nacional.New`, `nacional.Config`, `nfse.DecodeDocument`, todos os métodos do provider.
- Produces: `nfse.Dispatch(ctx context.Context, p Provider, service string, body map[string]any) (Result, error)` e `nfse.NewProvider(providerName, environment string, httpClient *http.Client, cert *x509.Certificate, key *rsa.PrivateKey, maxRetries int, cnpj string) (Provider, error)`.

**Contexto:** `dfe.Call` hoje faz `certificate.Load` e depois `services.NewClient` (SOAP). NFS-e desvia **depois** do `certificate.Load` — o certificado é o mesmo — e **antes** do `services.NewClient`. O contrato externo não muda: `Response.Body` continua sendo JSON string.

Sobre o portão de promoção (`go-dfe/CLAUDE.md`, "dfe.Implements() — o portão de promoção"): a regra de shadow-mode/assinatura byte-idêntica existe para operações que **migram** do py-dfe. NFS-e não existe no py-dfe — não há autoridade anterior contra a qual comparar, então não há shadow-mode a rodar. Isso precisa ficar escrito no comentário do `implemented`, não presumido; a homologação real contra a produção restrita é a F6.

- [ ] **Step 1: Escrever o teste que falha**

Adicione a `go-dfe/dfe_test.go`:

```go
func TestImplements_NFSe(t *testing.T) {
	for _, svc := range []string{
		constants.ServiceNFSeRecepcao, constants.ServiceNFSeConsulta,
		constants.ServiceNFSeConsultaDPS, constants.ServiceNFSeEvento,
		constants.ServiceNFSeConsultaEvento, constants.ServiceNFSeDistribuicao,
		constants.ServiceNFSeDANFSE, constants.ServiceNFSeParametrosMunicipais,
	} {
		if !Implements(constants.DocTypeNFSE, svc) {
			t.Errorf("Implements(nfse, %q) = false, esperado true", svc)
		}
	}
	if Implements(constants.DocTypeNFSE, "ServicoInexistente") {
		t.Error("Implements aceitou serviço desconhecido")
	}
}

func TestCall_NFSeRequiresProvider(t *testing.T) {
	resp, err := Call(context.Background(), Request{
		DocType: constants.DocTypeNFSE, Service: constants.ServiceNFSeRecepcao,
		Environment: "hom", CertificateB64: "x", Body: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Call devolveu erro cru em vez de Problem: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("StatusCode = %d, esperado 400 para body sem provider", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd go-dfe && go test . -run 'TestImplements_NFSe|TestCall_NFSe' -v`
Expected: FAIL — `Implements(nfse, ...) = false`.

- [ ] **Step 3: Escrever `nfse/dispatch.go`**

```go
package nfse

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net/http"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"
)

// Chaves aceitas em dfe.Request.Body.
const (
	BodyKeyProvider    = "provider"
	BodyKeyDocument    = "document"
	BodyKeyEvent       = "event"
	BodyKeyAccessKey   = "chave_acesso"
	BodyKeyIDDPS       = "id_dps"
	BodyKeyNSU         = "nsu"
	BodyKeyCNPJConsulta = "cnpj_consulta"
	BodyKeyParamKind   = "param_kind"
	BodyKeyParamArgs   = "param_args"
)

// distributor e danfser são as capacidades que só o provider nacional tem.
// O ABRASF (F5) não as implementa e o dispatch falha explicitamente.
type distributor interface {
	Distribute(ctx context.Context, nsu int64, cnpjConsulta string, lote bool) (Result, error)
}

type danfser interface {
	DANFSE(ctx context.Context, chave string) ([]byte, error)
}

type parametrizer interface {
	MunicipalParameters(ctx context.Context, kind string, args ...string) (Result, error)
}

// NewProvider constrói o provider a partir do nome vindo do Body.
func NewProvider(name, environment string, httpClient *http.Client,
	cert *x509.Certificate, key *rsa.PrivateKey, maxRetries int, cnpj string) (Provider, error) {
	switch name {
	case ProviderNacional:
		return nacional.New(nacional.Config{
			Environment: environment, HTTPClient: httpClient, Cert: cert,
			Key: key, MaxRetries: maxRetries, CNPJ: cnpj,
		})
	case ProviderAbrasf204:
		return nil, fmt.Errorf("nfse: provider %q chega na fase F5", name)
	default:
		return nil, fmt.Errorf("nfse: provider desconhecido %q", name)
	}
}

// Dispatch roteia o serviço para o método correspondente do provider.
func Dispatch(ctx context.Context, p Provider, service string, body map[string]any) (Result, error) {
	switch service {
	case constants.ServiceNFSeRecepcao:
		sub, err := subMap(body, BodyKeyDocument)
		if err != nil {
			return Result{}, err
		}
		doc, err := DecodeDocument(sub)
		if err != nil {
			return Result{}, err
		}
		return p.Emit(ctx, doc)

	case constants.ServiceNFSeEvento:
		sub, err := subMap(body, BodyKeyEvent)
		if err != nil {
			return Result{}, err
		}
		ev, err := decodeEvent(sub)
		if err != nil {
			return Result{}, err
		}
		return p.Event(ctx, ev)

	case constants.ServiceNFSeConsulta:
		return p.QueryByKey(ctx, str(body, BodyKeyAccessKey))

	case constants.ServiceNFSeConsultaDPS:
		return p.QueryByDPSID(ctx, str(body, BodyKeyIDDPS))

	case constants.ServiceNFSeConsultaEvento:
		return p.QueryEvents(ctx, EventFilter{
			ChaveAcesso: str(body, BodyKeyAccessKey),
			TipoEvento:  str(body, "tipo_evento"),
			NSeqEvento:  intOf(body, "n_seq_evento"),
		})

	case constants.ServiceNFSeDistribuicao:
		d, ok := p.(distributor)
		if !ok {
			return Result{}, &FieldNotSupportedError{Provider: fmt.Sprintf("%T", p), Field: "distribuicao"}
		}
		return d.Distribute(ctx, int64(intOf(body, BodyKeyNSU)), str(body, BodyKeyCNPJConsulta), true)

	case constants.ServiceNFSeDANFSE:
		d, ok := p.(danfser)
		if !ok {
			return Result{}, &FieldNotSupportedError{Provider: fmt.Sprintf("%T", p), Field: "danfse"}
		}
		pdf, err := d.DANFSE(ctx, str(body, BodyKeyAccessKey))
		if err != nil {
			return Result{}, err
		}
		return Result{PDF: pdf}, nil

	case constants.ServiceNFSeParametrosMunicipais:
		pm, ok := p.(parametrizer)
		if !ok {
			return Result{}, &FieldNotSupportedError{Provider: fmt.Sprintf("%T", p), Field: "parametros_municipais"}
		}
		return pm.MunicipalParameters(ctx, str(body, BodyKeyParamKind), strSlice(body, BodyKeyParamArgs)...)

	default:
		return Result{}, fmt.Errorf("nfse: serviço desconhecido %q", service)
	}
}

func subMap(body map[string]any, key string) (map[string]any, error) {
	v, ok := body[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("nfse: body sem o objeto %q", key)
	}
	return v, nil
}

func str(body map[string]any, key string) string {
	s, _ := body[key].(string)
	return s
}

func intOf(body map[string]any, key string) int {
	switch v := body[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func strSlice(body map[string]any, key string) []string {
	raw, _ := body[key].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func decodeEvent(m map[string]any) (EventRequest, error) {
	raw, err := jsonMarshal(m)
	if err != nil {
		return EventRequest{}, err
	}
	var ev EventRequest
	if err := jsonUnmarshalStrict(raw, &ev); err != nil {
		return EventRequest{}, fmt.Errorf("nfse: decode event: %w", err)
	}
	return ev, nil
}
```

Adicione a `document.go` os dois helpers usados acima, ao lado de `DecodeDocument`, para manter a política de campo desconhecido idêntica nos dois caminhos:

```go
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func jsonUnmarshalStrict(raw []byte, out any) error {
	dec := json.NewDecoder(bytesReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}
```

- [ ] **Step 4: Alterar `dfe.go`**

Adicione a entrada no mapa `implemented`, com o comentário que registra por que não há shadow-mode:

```go
	// NFS-e não migra do py-dfe: py-dfe nunca implementou NFS-e, então não
	// existe autoridade anterior contra a qual rodar shadow-mode nem corpus
	// para o portão de assinatura byte-idêntica. O portão aplicável é a
	// homologação em produção restrita (fase F6 do plano de NFS-e), não a
	// comparação de paridade descrita acima.
	constants.DocTypeNFSE: {
		constants.ServiceNFSeRecepcao:             true,
		constants.ServiceNFSeConsulta:             true,
		constants.ServiceNFSeConsultaDPS:          true,
		constants.ServiceNFSeEvento:               true,
		constants.ServiceNFSeConsultaEvento:       true,
		constants.ServiceNFSeDistribuicao:         true,
		constants.ServiceNFSeDANFSE:               true,
		constants.ServiceNFSeParametrosMunicipais: true,
	},
```

E, em `Call`, logo após `certificate.Load` e **antes** de `services.NewClient`:

```go
	if req.DocType == constants.DocTypeNFSE {
		return callNFSe(ctx, req, httpClient, cert, key, maxRetries)
	}
```

Acrescente a função no fim de `dfe.go`:

```go
// callNFSe é o caminho NFS-e: REST + JSON, sem SOAP e sem endpoints.Resolve.
// Mantém o mesmo contrato de Response (Body como JSON string) que o caminho
// SOAP, para que worker/api não distingam os dois.
func callNFSe(ctx context.Context, req Request, httpClient *http.Client,
	cert *x509.Certificate, key *rsa.PrivateKey, maxRetries int) (Response, error) {
	providerName, _ := req.Body[nfse.BodyKeyProvider].(string)
	provider, err := nfse.NewProvider(providerName, req.Environment, httpClient, cert, key, maxRetries, req.CNPJ)
	if err != nil {
		return problemResponse(400, constants.ErrCodeValidation, err.Error())
	}

	result, err := nfse.Dispatch(ctx, provider, req.Service, req.Body)
	if err != nil {
		var fe *nfse.FiscalError
		if errors.As(err, &fe) {
			return problemResponse(fe.Status, constants.ErrCodeSOAPRequest, fe.Error())
		}
		return problemResponse(400, constants.ErrCodeValidation, err.Error())
	}

	bodyJSON, err := json.Marshal(result)
	if err != nil {
		return problemResponse(500, constants.ErrCodeUnexpected, "failed to encode response")
	}
	return Response{StatusCode: 200, Body: string(bodyJSON), Headers: map[string]string{}}, nil
}
```

Imports novos em `dfe.go`: `"crypto/rsa"`, `"crypto/x509"`, `"errors"`, `"net/http"`, `"gopkg.aoctech.app/dfe/go-dfe/nfse"`.

- [ ] **Step 5: Rodar tudo**

Run: `cd go-dfe && go test ./... -v && CGO_ENABLED=0 GOARCH=arm64 go build ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go-dfe/dfe.go go-dfe/dfe_test.go go-dfe/nfse/dispatch.go go-dfe/nfse/document.go
git commit -m "feat(nfse): wiring de NFS-e em dfe.Call e dfe.Implements"
```

---

### Task 9: Fechamento — suíte completa, documentação e correção da spec

**Files:**
- Modify: `DOCS.md`, `CONDUCT.md`, `docs/specs/2026-08-04-nfse-design.md`, `go-dfe/CLAUDE.md`

- [ ] **Step 1: Rodar tudo que a F2 toca**

```bash
cd go-dfe && CGO_ENABLED=0 GOARCH=arm64 go build ./... && go test ./...
cd ../api && go build ./... && go test ./...
cd ../worker && go build ./... && go test ./...
```

Expected: tudo verde. `api` e `worker` não mudam nesta fase, mas compartilham o `go.work` — a suíte deles é a rede contra uma quebra de compilação transitiva.

- [ ] **Step 2: Corrigir a tabela de ambientes da spec**

Em `docs/specs/2026-08-04-nfse-design.md`, §1 "Ambientes", troque a célula da produção restrita do Sefin Nacional:

```
| Sefin Nacional | `https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional` | `https://sefin.nfse.gov.br/SefinNacional` |
```

E acrescente logo abaixo da tabela:

> O segmento `/API` existe apenas na produção restrita do Sefin Nacional
> (`tmp/apis-prod-restrita-e-producao.txt`). A tabela de bases em
> `go-dfe/nfse/nacional/endpoints.go` é a fonte de verdade em código.

- [ ] **Step 3: Documentar em `DOCS.md`**

Na seção do go-dfe, acrescente uma subseção "Camada NFS-e" com: os oito serviços e o método de provider que cada um dispara; o formato do `Body` de `dfe.Request` para NFS-e (chaves `provider`, `document`, `event`, `chave_acesso`, `id_dps`, `nsu`, `param_kind`, `param_args`); a nota de que NFS-e não passa por `internal/soap` nem `internal/services`; e a tabela de bases por ambiente.

- [ ] **Step 4: Registrar as decisões duráveis em `CONDUCT.md`**

Três entradas:

1. **NFS-e não tem portão de shadow-mode.** py-dfe nunca implementou NFS-e; não há autoridade anterior para comparar. O portão aplicável é homologação em produção restrita (F6). Isso é uma exceção documentada à regra de promoção de `dfe.Implements`, não um descuido.
2. **A ordem dos campos das structs em `nfse/nacional/dps.go` é normativa.** Ela É a ordem do XSD. Reordenar campo de struct por estética quebra a validação no Sefin. O teste golden `TestBuildDPS_MatchesGolden` é o guarda.
3. **Campo não suportado falha explicitamente.** `FieldNotSupportedError` nomeia o campo. Nenhum adapter de NFS-e pode descartar dado em silêncio — vale para o ABRASF da F5 e para as capacidades opcionais do dispatch (distribuição, DANFSE, parâmetros).

- [ ] **Step 5: Atualizar `go-dfe/CLAUDE.md`**

Na "Directory Structure", acrescente o ramo `nfse/`. Na seção "dfe.Implements() — o portão de promoção", acrescente o parágrafo de exceção do item 1 acima.

- [ ] **Step 6: Marcar a F2 como concluída na spec**

Em §10, na linha da F2, troque o conteúdo da coluna "Entrega" para começar com `✅ (concluída)`.

- [ ] **Step 7: Commit**

```bash
git add DOCS.md CONDUCT.md go-dfe/CLAUDE.md docs/specs/2026-08-04-nfse-design.md
git commit -m "docs(nfse): documenta a camada NFS-e do go-dfe e corrige a base de producao restrita"
```

---

## Impacto entre projetos

| Projeto | Impacto nesta fase |
|---|---|
| `go-dfe` | Toda a mudança. Pacote `nfse/` novo, desvio em `dfe.Call`, `implemented` estendido |
| `api` | Nenhuma mudança de código. Passa a poder chamar `dfe.Call` com `doc_type=nfse` — consumido na F3 |
| `worker` | Idem |
| `py-dfe` | Nenhum. NFS-e nunca teve caminho py-dfe e não terá |
| `cdk` | Nenhum. Sem função Lambda nova; go-dfe é biblioteca linkada |
| `ui` | Nenhum |

## O que a F2 deliberadamente NÃO faz

- **Não emite nada de verdade.** Todo teste usa `httptest`. Homologação contra a produção restrita é a F6.
- **Não implementa ABRASF 2.04.** `NewProvider` devolve erro explícito para `abrasf204`. É a F5.
- **Não valida contra XSD.** `CGO_ENABLED=0` inviabiliza libxml2 — a mesma limitação já documentada dos demais doc types. As validações estruturais são as de `validateDoc`, e as regras fiscais ficam com o Sefin (spec §11).
- **Não persiste nada.** `go-dfe` é biblioteca: sem DynamoDB, sem S3, sem SQS. A persistência é da F3.
- **Não gera DANFSE própria.** `DANFSE` é download do PDF do ADN. Gerador próprio segue fora de escopo (spec §11).

