# CT-e rodoviário: emissão a partir de NF-e (tomador, notas, valor do frete)

Escopo: CT-e modelo 57, tpCTe 0 (normal), tpServ 0 (normal), modal rodoviário (01), emitido a partir de NF-e já
existentes (emitidas pela organização ou recebidas por distribuição). Fora: CT-e OS, GTV-e, Simplificado,
complemento, anulação, substituição, outros modais, eventos (F9).

## Princípio de UX

Três decisões do usuário: **quais notas**, **quem paga o frete**, **quanto custa**. Todo o resto é derivado das notas
e editável. Derivar é um endpoint à parte (`derive`), então o formulário mostra o que foi derivado antes de emitir e
cada campo derivado aceita sobrescrita.

## API

Rotas novas em `/v1.0`, padrão das de MDF-e (`api/internal/api/v1/mdfes.go`), RFC 7807 nos erros, permissões
`create.ctes` / `list.ctes` / `get.ctes` no mesmo esquema das outras.

### `POST /v1.0/ctes/derive`

Corpo `{ "nfe_keys": ["<44 dígitos>", ...] }` (1 a 50 chaves, modelo 55, sem repetição).
Lê cada NF-e do próprio cadastro (tabela `nfes`, emitidas e recebidas) e o XML autorizado no S3 (`xml_s3_key`),
reaproveitando o leitor de XML de `api/internal/services/mdfes/xmlparse.go` (mover para pacote compartilhado se
preciso, sem duplicar). Resposta 200:

```json
{
  "notes": [{
    "access_key": "…", "number": 123, "serie": 1, "issued_at": "2026-11-01T10:00:00-03:00",
    "source": "issued|received", "value": "1500.00", "weight_kg": "320.000", "volumes": 12,
    "mod_frete": "0",
    "emitter":   {"doc": "…", "name": "…", "uf": "SP", "city_ibge": "3550308", "city": "São Paulo"},
    "recipient": {"doc": "…", "name": "…", "uf": "MG", "city_ibge": "3106200", "city": "Belo Horizonte"}
  }],
  "sender": Party|null, "recipient": Party|null,
  "taker_suggestion": {"role": "remetente|destinatario", "reason": "mod_frete_cif|mod_frete_fob"} | null,
  "cargo": {"total_value": "4500.00", "total_weight_kg": "960.000", "volumes": 36, "predominant_product": "…"},
  "origin": {"uf": "SP", "city_ibge": "…", "city": "…"},
  "destination": {"uf": "MG", "city_ibge": "…", "city": "…"},
  "warnings": [{"code": "…", "message": "pt-BR sem travessão", "access_key": "…|null"}]
}
```

`Party` = `{doc, name, uf, city_ibge, city, person_id|null}` (`person_id` quando o documento existe nos cadastros
de pessoas da organização). Regras de derivação:

- `sender` = emitente das notas e `recipient` = destinatário. Se as notas divergem, o campo vem `null` e há
  warning `mixed_sender` / `mixed_recipient` (um CT-e tem um remetente e um destinatário; o usuário separa as
  notas em mais de um CT-e ou escolhe).
- `taker_suggestion`: modFrete 0 (CIF) em todas as notas: remetente; modFrete 1 (FOB) em todas: destinatário;
  mistura, 2, 3, 4 ou 9: `null` + warning `taker_ambiguous`.
- `weight_kg`: soma de `transp/vol/pesoB`; sem peso nas notas, soma de `qCom` dos itens cuja unidade é KG
  (case insensitive); senão `null` + warning `weight_missing`.
- `origin` = município do remetente; `destination` = município do destinatário (do `enderEmit` / `enderDest`).
  `predominant_product` = `xProd` do item de maior `vProd` entre todas as notas.
- Warnings extras: nota não autorizada (`note_not_authorized`, e a nota entra mesmo assim para o usuário ver),
  nota de outro modelo, chave fora do cadastro (`note_not_found`, 422 se nenhuma for achada).

### `POST /v1.0/ctes`

Corpo (`CteEmitBody`). O que o usuário não informa vem do `derive` recalculado no servidor (o cliente não é
fonte de verdade das notas):

```json
{
  "nfe_keys": ["…"],
  "taker": {"role": "remetente|destinatario|expedidor|recebedor|outro", "person_id": "só para outro, expedidor e recebedor"},
  "freight": {"total": "1850.00", "components": [{"name": "Pedágio", "value": "120.00"}]},
  "icms": {"cst": "00|20|40|41|51|60|90|SN", "p_icms": "12.00", "p_red_bc": "0.00"},
  "cfop": "6353",
  "overrides": {
    "sender_person_id": "…", "recipient_person_id": "…",
    "cargo": {"total_value": "…", "total_weight_kg": "…", "volumes": 0, "predominant_product": "…"},
    "origin": {"city_ibge": "…"}, "destination": {"city_ibge": "…"}
  },
  "additional_info": "…"
}
```

- `freight.total` é o vTPrest; sem `components` o builder emite um Comp "FRETE VALOR" com o total; com `components`,
  o primeiro Comp é "FRETE VALOR" com `total - soma dos componentes` e os demais seguem (vTPrest = soma dos Comp,
  `vRec = vTPrest`).
- `cfop`: CFOP da prestação (4 dígitos), obrigatório; nunca presumido (422 `cfop_required`). A UI oferece só os
  CFOP de transporte (`cte: true` em `ui/src/lib/data/cfop.ts`) no escopo da prestação: 5 com origem e destino na
  mesma UF, 6 entre UFs.
- `icms` (opcional): CRT 1, 2 ou 4 da organização sem ICMS destacado: omitido, o padrão é `SN` (ICMSSN, indSN 1). Demais: `cst` e
  `p_icms` são obrigatórios (nenhuma alíquota é presumida: regra fiscal nunca é assumida). CST 40, 41 e 51
  dispensam alíquota. Cálculo vBC = vTPrest com redução opcional, vICMS = vBC x pICMS, 2 casas, arredondamento
  bancário (mesmo helper `q2` / `RoundBank` das NF-e).
- Tomador: `remetente` e `destinatario` vêm das notas; `expedidor`, `recebedor` e `outro` exigem `person_id` de pessoa
  cadastrada. `outro` vira `toma4` (CNPJ/CPF, IE, xNome, enderToma); os demais viram `toma3` (0 remetente, 1
  expedidor, 2 recebedor, 3 destinatário).
- Rodoviário: `rodo/RNTRC` do transportador vem de `organization_mdfe_configs.rntrc` (campo opcional `rntrc`, 8
  dígitos, de `MdfeConfigBody` em `PUT …/mdfe-config`); sem ele, 422 `rntrc_required` dizendo onde cadastrar. A UI
  edita o campo na aba MDF-e de Configuração Fiscal e, no 422, mostra a ação "Cadastrar RNTRC"
  (`/fiscal-config?tab=mdfe`).
- Resposta como a do MDF-e: 202 + registro `pending`, outbox transacional, worker existente de CT-e
  (`DocType "cte"`, serviço de autorização do CT-e 4.00), numeração/série pelo mesmo mecanismo das outras.

### `GET /v1.0/ctes` e `GET /v1.0/ctes/{access_key}`

Lista paginada (`incoming` 0 emitidos, 1 recebidos; `limit`, `cursor`, `sort`) e detalhe, mesmo padrão das NF-e.
Cada item, e o corpo do 202 da emissão, é o `CteDocument` do OpenAPI (`FiscalDocument` mais):

```json
{
  "pk": "…", "sk": "<chave>", "status": "pending", "sefaz_status": null, "sefaz_motive": null,
  "number": 31, "serie": 1, "dh_emi": "…", "created_at": "…",
  "freight_total": "1850.00", "cfop": "6353", "icms_cst": "SN",
  "sender": {"doc": "…", "name": "…"}, "recipient": {"doc": "…", "name": "…"},
  "taker": {"role": "remetente", "doc": "…", "name": "…"},
  "nfe_keys": ["…"], "origin": CtePlace, "destination": CtePlace,
  "cargo": {"total_value": "…", "total_weight_kg": "…", "volumes": 0, "predominant_product": "…"},
  "xml_s3_key": null
}
```

Tipos da UI: `CteListOut` / `CteDetailOut` em `ui/src/lib/types/api.ts`. No `derive`, `origin` e `destination` vêm
`null` quando remetente ou destinatário divergem, e `notes[].weight_kg` pode ser `null`.

## Builder (`api/internal/services/ctes`)

- `BuildCTe` devolve o dicionário `enviCTe` como as NF-e, com `services.PruneEmpty` e o validador em shadow
  (`dfecheck`: acrescentar `SpecCTe` para CT-e com `ShadowCTe`, doc `cte`).
- A tabela `xsdorder` de CT-e (`go-dfe/internal/xmlops/xsdorder/table.go`) precisa cobrir todo grupo que o builder
  emite; o teste `go-dfe/validate/xsdorder_parity_test.go` hoje só registra (log) as lacunas do CT-e: preencher
  (ordem dos XSDs vendorizados em `go-dfe/schemas/cte`) e torná-lo estrito para o CT-e também.
- Teste de paridade builder x XSD como o de NF-e e MDF-e.
- Regras do MOC do CT-e já existem em `go-dfe/validate/rules_cte.go`; o builder deve produzir documento que não
  dispare nenhuma regra `block`.

## UI (`ui/`)

Rota `/cte/emit`, estendendo a superfície de CT-e existente (acento violeta do DF-e theme, mesmo shell do
`/mdfe/emit`: `RequireFiscalConfig variant="cte"` se existir, senão criar a variante). Mobile-first 375px.
Estrutura (uma página, sem wizard):

1. **Notas**: seletor múltiplo (Combobox com busca por número, nome ou chave, notas autorizadas recentes emitidas e
   recebidas) e a lista escolhida abaixo: por nota, número, remetente para destinatário, valor e peso, botão remover.
   Faixa de resumo derivada: `3 notas · 960 kg · R$ 4.500,00 · SP para MG`.
2. **Quem paga o frete**: escolha segmentada (Remetente, Destinatário, Expedidor, Recebedor, Outro) com o nome de
   quem seria em cada opção; a sugerida vem marcada com o motivo ("Frete CIF na nota"); Outro/Expedidor/Recebedor
   abrem seletor de pessoa (Combobox dos cadastros, nunca digitação de CNPJ).
3. **Valor do frete**: campo monetário (`CurrencyInput`), CFOP da prestação (picker filtrado pelo escopo) e
   "Adicionar componente" (nome escolhido de uma lista: pedágio, GRIS, outros) em linhas nome e valor; abaixo,
   "Imposto" recolhido: CST (picker; `SN` já marcado para CRT 1, 2 e 4), alíquota e o ICMS calculado ao vivo.
4. **Dados do transporte** (recolhido, "Derivado das notas"): remetente, destinatário, origem, destino, peso, valor da
   carga, produto predominante. Cada campo mostra o valor derivado e aceita edição; editado ganha o marcador
   "editado" e um botão "voltar ao derivado".
5. Avisos do `derive` aparecem como callouts no bloco a que pertencem, com a ação para resolver (por exemplo
   "Notas com remetentes diferentes: emita um CT-e por remetente").
6. Barra de ação fixa (`sticky bottom-(--bottomnav-height)`) com o total do frete e o botão "Emitir CT-e";
   desabilitado enquanto faltar nota, tomador ou valor, dizendo o que falta.

Regras do `ui/CLAUDE.md` valem: loading e skeleton, debounce de busca, guia (`/guide`) com seção e captura,
`nav.tsx` com a rota e `keywords` (cte, conhecimento de transporte, frete, tomador), fixtures no mock,
`npm run lint` zerado, testes (Vitest), sem travessão em texto visível, sem eyebrow acima de título, sem cards
iguais de ícone com texto, `ClearButton` nos selects opcionais.

## Complemento, substituto e CT-e OS (F6 parte 3)

Rotas novas, mesmo padrão de `POST /v1.0/ctes` (202 + `pending`, outbox, permissão `create.ctes`, 422 com `code`). Códigos
e regras do MOC cobertas: `CONDUCT.md`, seções "CT-e complementar e substituto" e "CT-e OS". OpenAPI: `openapi/ctes.yaml`.

- `POST /v1.0/ctes/complement`: `original_keys` (1 a 10 chaves de CT-e autorizados da organização), `freight`, `icms`,
  `ibs_cbs`, `authorized_xml`, `cfop`, `additional_info`. Participantes, percurso, tipo de serviço e tomador vêm do XML do
  original (`xml_s3_key`), lido por `nfexml.ParseCTe`; nunca do cadastro de pessoas.
- `POST /v1.0/ctes/substitute`: `original_key`, `change_taker` (indAlteraToma), `taker` (só com `change_taker`:
  papel que existe no original ou `outro` com `person_id`), mais os campos de valor acima. Carga e NF-e repetem o original.
- `POST /v1.0/ctes/os`: `service_type` (`people`, `values`, `baggage`), `taker.person_id`, `service`, `origin`,
  `destination`, `route_ufs`, `road`, `doc_refs`, `gtve`, `federal_taxes`, mais os campos de valor.
- O registro do CT-e ganha `cte_type` (`complement`, `substitute`), `complemented_keys`, `substituted_key`, e no CT-e OS
  `model` = `67` e `os_service`. A UI precisa tratar a ausência de `nfe_keys`, `sender`, `recipient` e `cargo` nesses registros.

### Status F6 parte 4 (2026-11-08)

Feito: detalhe do CT-e com `expedidor`, `recebedor`, `complemented_by` e `substituted_by` (linhas de referência `ref#` na tabela
`ctes`, sem Scan nem GSI; detalhes em `DOCS.md`, "CT-e: detalhe com referência reversa..."); troca de tomador do substituto com
todos os participantes; DACTE OS (MOC Anexo II, seções 4 e 6); eventos do CT-e OS só 110111 e 110110 (os do 57 respondem 422
`event_not_for_cte_os`) e a UI esconde o resto. Perguntas em aberto: (1) CFOP do CT-e OS: o pedido citava 5206/6206 como transporte
de pessoas, mas a tabela de CFOP do repositório (`ui/src/lib/data/cfop.ts`) descreve 5206/6206/7206 como "Anulação de valor relativo
a aquisição de serviço de transporte"; nenhum documento do repositório ou fornecido lista CFOP específico do CT-e OS, então a lista
continua a do CT-e e nada foi inventado (a fixture do mock deixou de usar 6206). Precisa do Anexo I do MOC (regras H, CFOP do 67).
(2) 110300/110301 (vinculação de pagamento) são do 57 e do 67 no catálogo, mas as regras (autor, autorizador, uso) não estão em
nenhum documento disponível e também não existem para o 57: não implementado. (3) Referências reversas de complementares e
substitutos emitidos antes desta mudança.

### Status da UI (F6 parte 3)

Construída em `ui/`: `/cte/complement`, `/cte/substitute` e `/cte/os`, espécie na lista e no detalhe, links para o original,
mock e guia (detalhes em `DOCS.md`, "CT-e complementar, substituto e CT-e OS na UI"). Os itens que dependiam do backend (link do original,
expedidor e recebedor, DACTE e eventos do CT-e OS) foram feitos na parte 4; a lista de CFOP específica do CT-e OS segue em aberto.
