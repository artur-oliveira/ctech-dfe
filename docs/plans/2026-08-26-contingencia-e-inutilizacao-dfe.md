# Contingência, inutilização e eventos faltantes

**Data:** 2026-08-26
**Escopo:** continuidade de emissão quando a SEFAZ falha (contingência em todos os documentos),
integridade da numeração (inutilização) e os eventos que faltam nos documentos já emitidos.
**Relacionado:** [`2026-08-26-cobertura-total-tags-nfe-mdfe.md`](./2026-08-26-cobertura-total-tags-nfe-mdfe.md)

---

## Estado da implementação (2026-08-26)

| Item | Estado |
|------|--------|
| `pISEspec` → `adRemIS` | ✅ feito (`builders_tax.go` + ordem em `xsdorder/table.go`, que também estava desatualizada) |
| §1.1 `CPF` em `infEvento` | ✅ feito — `services.IssuerDocTag`, usado pelos 4 builders NF-e/NFC-e; MDF-e migrado para o mesmo helper |
| §1.2 MDF-e 110116/110117/110118 | ✅ feito — `payment_events.go`, `buildInfPag` único; + `indEncPorTerceiro` no 110112 |
| §1.3 NF-e 111500–111503 | ✅ feito — `extra_events.go` |
| §1.3 NF-e 110001 | ✅ feito — mas só cancela eventos da reforma (`tpEventoAut` do XSD), que ainda não emitimos: endpoint pronto, sem alvo próprio até a Fase E |
| §2 Inutilização | ✅ feito ponta a ponta — API, persistência, detecção de lacunas, aba em NF-e e NFC-e, guia |
| §3 C0 | ✅ feito — `EmissionMode`, `dhCont`/`xJust`, `tpEmis`/`tpImp` parametrizáveis, `tpEmis` na chave |
| §3 C1 | ⛔ **bloqueado** — a tabela UF → SVC precisa ser transcrita do MOC (ver §3.2) |
| §3 C2–C7 | ⬜ não iniciado (C3/C4/C6 dependem de C1 ou C2) |

**Desvios do plano.** A inutilização **não** ganhou tabela própria: reusa `nfe_events`/`nfce_events`,
às quais o worker `nfe-inutilization` já tinha acesso IAM, com `pk` sintético `INUT#{env}#{org_pk}`.
Isso eliminou mudança de CDK, de IAM e de workflow — ver DOCS.md → *Inutilização de numeração*.

## Decisões registradas

| # | Decisão | Estado |
|---|---------|--------|
| Validador XSD | `py-dfe` não emite mais (só gera PDF) e `go-dfe` não valida por escolha de projeto — velocidade sem overhead. **Não repontar** `validator.py` para os XSDs novos | Não fazer |
| `pISEspec` → `adRemIS` | Correção de emissão, independe do validador: a tag foi removida no 010e e o builder ainda a emite | Fase A do plano de tags |
| `CPF` em `infEvento` | Emitente pessoa física não consegue emitir evento nenhum hoje | Fazer |
| Eventos MDF-e 110116/110117/110118 | Pagamento da operação, confirmação e alteração do pagamento do serviço | Fazer |
| Inutilização | Aba própria em NF-e e NFC-e | Fazer — seção 2 |
| EPEC | É forma de emissão, não evento avulso — entra na contingência | Seção 3 |
| Eventos da reforma (1121xx/2111xx/2121xx) | Casados com a Fase E de IBS/CBS na emissão | Depois da Fase E |

**Consequência de não validar:** o único ponto onde um XML malformado é detectado passa a ser
a rejeição da SEFAZ. Isso torna os testes de builder e os payloads de integração a rede de
proteção real — não são opcionais em nenhuma tag nova. Os XSDs em `py-dfe/schemas/xsds/`
permanecem como fonte de verdade para o `xsdorder/table.go` e para escrever os testes.

---

## 1. Eventos faltantes

### 1.1 `CPF` em `infEvento` (NF-e, NFC-e, MDF-e)

`buildCancelBody`, `buildCCeBody`, `buildManifestBody` (`nfes/service.go`) e
`buildSubstituteBody` (`nfce_service.go`) gravam sempre `infEvento.CNPJ`. O XSD é
`choice CNPJ | CPF`. Produtor rural e MEI pessoa física ficam sem cancelamento, sem CCe e sem
manifestação. `mdfes/events.go:270` já trata o caso — replicar.

Um helper compartilhado que recebe o `org_pk` (`CNPJ_…` ou `CPF_…`) e devolve a chave correta;
os quatro builders passam a usá-lo.

### 1.2 MDF-e — pagamento da operação de transporte

| Evento | `detEvento` | Caso de uso |
|--------|-------------|-------------|
| `110117` evConfirmaServMDFe | `descEvento`, `nProt` | Contratante confirma a prestação do serviço. Trivial |
| `110116` evPagtoOperMDFe | `descEvento`, `nProt`, `infViagens` (qtdViagens, nroViagem), `infPag` | Declaração do pagamento ao transportador |
| `110118` evAlteracaoPagtoServMDFe | `descEvento`, `nProt`, `infPag` | Alteração do pagamento declarado |

`infPag` é a **mesma estrutura** de `infANTT/infPag` que falta na emissão (Fase A do MDF-e no
plano de tags): `xNome`, CPF/CNPJ/idEstrangeiro, `Comp` (tpComp/vComp/xComp), `vContrato`,
`indPag`, `vAdiant`, `indAntecipaAdiant`, `infPrazo` (nParcela/dVenc/vParcela), `tpAntecip`,
`infBanc` (codBanco/codAgencia/CNPJIPEF/PIX).

**Implementar um único builder de `infPag` e reusá-lo nos quatro pontos** (emissão + três
eventos). A ordem dos três eventos já existe em `xsdorder/table.go:588-599`.

Remover ou usar a constante morta `TpEventoPagamentoOper` (`mdfes.go:78`).

Complementos menores: `indEncPorTerceiro` no encerramento (110112).

### 1.3 Eventos NF-e ainda em aberto

`111500`/`111501` (pedido de prorrogação de ICMS suspenso) e `111502`/`111503` (cancelamento do
pedido) — caso de uso: remessa para industrialização com prazo a prorrogar. `110001`
(cancelamento de evento, novo no 010e). Sem dependência da reforma; entram após 1.1 e 1.2.

Os eventos `412120`/`412130` são manifestação **do fisco** — só precisam ser reconhecidos no
parser de distribuição, nunca emitidos.

---

## 2. Inutilização de numeração — NF-e e NFC-e

### Por que importa

Numeração de documento fiscal não pode ter buraco. Toda falha que consome um número sem gerar
documento autorizado (rejeição definitiva, crash entre a reserva do número e a transmissão,
troca de série) deixa uma lacuna que o fisco cobra. Sem inutilização o cliente fica sem saída
dentro do produto.

### Situação atual

- `go-dfe` **já tem** o transporte: serviço `NfeInutilizacao` no `endpoints/table.go` para
  todas as UFs, ordem em `xsdorder/table.go`.
- API: nada. UI: nada. O recurso está pronto na camada mais baixa e inacessível.

### Trabalho

1. **API** — `POST /nfes/inutilizations` e `POST /nfces/inutilizations`, com
   `serie`, `numero_inicial`, `numero_final`, `justificativa` (mín. 15 caracteres), `ano`.
   Resposta em Problem JSON como o resto. Validar contra a numeração corrente em
   `organization_nfe_configs`/`organization_nfce_configs`: recusar faixa que contenha número
   já autorizado.
2. **Persistência** — nova tabela `nfe_inutilizations` / `nfce_inutilizations`
   (`pk = {env}#{CNPJ}`, `sk = {ano}#{serie}#{n_ini}-{n_fin}`), guardando protocolo, XML de
   retorno e status. Sem isso não há como provar a inutilização depois.
3. **Detecção de lacuna** — varredura da numeração por série que aponte os buracos existentes
   e ofereça inutilizá-los. É o que transforma a aba em ferramenta e não em formulário.
4. **UI** — aba "Inutilizações" em NF-e e NFC-e: lista das faixas inutilizadas + lacunas
   detectadas + ação de inutilizar. Faixa por seletor de intervalo, justificativa por
   textarea com contagem mínima. Nada de digitar número solto.
5. **XML de retorno** guardado no S3 como qualquer outro documento.

---

## 3. Contingência

### 3.1 O problema

Hoje toda emissão é `tpEmis=1`. Se o autorizador da UF cai, o cliente para. Contingência é o
mecanismo previsto para continuar emitindo — e é o único item desta lista que afeta
**todos os quatro documentos**.

### 3.2 Matriz por documento

| Documento | Modo | `tpEmis` | Para onde vai | O que muda |
|-----------|------|----------|---------------|------------|
| NF-e 55 | SVC-AN | `6` | Sefaz Virtual de Contingência — Ambiente Nacional | Autorização normal, autorizador diferente |
| NF-e 55 | SVC-RS | `7` | Sefaz Virtual de Contingência — RS | Idem |
| NF-e 55 | EPEC | `4` | Ambiente Nacional (`RecepcaoEvento`, evento `110140`) | Evento prévio autoriza a circulação; a NF-e é transmitida depois |
| NFC-e 65 | Offline | `9` | Nenhum na hora — transmissão posterior | DANFE impresso, QR Code com assinatura offline, prazo de transmissão |
| CT-e | SVC-SP / SVC-RS | `8` / `7` | Sefaz virtual correspondente | Autorizador diferente |
| MDF-e | Contingência | `2` | Nenhum na hora — transmissão posterior | Emissão local, transmissão quando voltar |

**Qual UF usa qual SVC não pode ser chutado.** A designação (SVC-AN vs SVC-RS por UF, SVC-SP
vs SVC-RS para CT-e) vem da tabela oficial do MOC e precisa ser transcrita e testada como
tabela, do mesmo jeito que `nfeUFAuth` já é hoje.

### 3.3 O que falta na base de código

1. **Endpoints** — `go-dfe/internal/endpoints/table.go` não tem **nenhuma** entrada SVC.
   Existe `"SVRS"` (autorizador regional em operação normal) e `"AN"` (distribuição + eventos),
   que não são a mesma coisa. Adicionar registries `SVC-AN`, `SVC-RS` e, para CT-e, `SVC-SP`,
   mais a tabela UF → SVC designado por tipo de documento.
   O `"AN"` existente já serve ao EPEC — o transporte do evento `110140` está pronto.
2. **`ide/dhCont` + `xJust`** — o XSD é explícito: o grupo é exigido "apenas para tpEmis
   diferente de 1". Nenhum modo de contingência funciona sem isso, e nenhum dos dois é
   emitido hoje. É pré-requisito de tudo nesta seção.
3. **`ide/tpEmis` parametrizável** — hoje `tpEmisNormal = "1"` está fixo em
   `builders_doc.go`. A forma de emissão passa a ser resolvida no momento da emissão.
4. **`ide/tpImp`** — DANFE em contingência muda de formato (Simplificado / Simplificado
   Tipo 2). Hoje só `1` (retrato) e `4` (NFC-e) são usados.
5. **Máquina de estados de contingência** — a decisão não pode ser um checkbox manual:
   - detectar indisponibilidade (timeout / `cStat` de serviço paralisado / falhas consecutivas);
   - entrar em contingência registrando `dhCont` e `xJust` **uma vez** para toda a janela;
   - emitir no modo designado para a UF e o documento;
   - detectar o retorno do autorizador e sair da contingência;
   - **retransmitir** o que ficou pendente (NFC-e offline e MDF-e contingência têm prazo legal).
   Estado por organização + documento, não global.
6. **Fila de pendentes** — NFC-e `tpEmis=9` e MDF-e `tpEmis=2` são autorizados *depois*.
   Reusar `worker_outbox` em vez de criar mecanismo próprio.
7. **QR Code offline da NFC-e** — a assinatura do QR Code muda no modo offline (usa o CSC de
   forma distinta). `nfce_qrcode.go` só implementa o caminho online.
8. **EPEC (`110140`)** — `detEvento` com `cOrgaoAutor`, `tpAutor`, `verAplic`, `dhEmi`,
   `tpNF`, `IE`, `dest` (UF, CNPJ/CPF/idEstrangeiro, IE, `vNF`, `vICMS`, `vST`). A ordem já
   está em `xsdorder/table.go:342,351`. Todos os valores saem da NF-e que está sendo emitida —
   nenhum input novo do usuário.

### 3.4 Fases sugeridas

| Fase | Entrega | Depende de |
|------|---------|------------|
| C0 | `dhCont`/`xJust` na emissão + `tpEmis` e `tpImp` parametrizáveis | — |
| C1 | Tabela UF → SVC + endpoints SVC-AN/SVC-RS/SVC-SP | C0 |
| C2 | Detecção de indisponibilidade + estado de contingência por org/documento + saída automática | C1 |
| C3 | NF-e SVC-AN/SVC-RS e CT-e SVC-SP/SVC-RS (autorização direta no virtual) | C2 |
| C4 | EPEC `110140` + transmissão posterior da NF-e | C2 |
| C5 | NFC-e offline (`tpEmis=9`): QR Code offline, DANFE simplificado, fila de retransmissão com prazo | C2 |
| C6 | MDF-e contingência (`tpEmis=2`) + retransmissão | C2 |
| C7 | Painel de contingência: janelas abertas, pendentes, prazo restante | C5, C6 |

### 3.5 Critério de pronto

- Simulação de indisponibilidade em homologação entra e sai de contingência sozinha.
- Nenhum documento pendente perde o prazo sem alarme.
- A escolha do SVC por UF tem teste de tabela, como `nfeUFAuth` já tem hoje.
- `DOCS.md` e `CONDUCT.md` documentam a máquina de estados — é a parte do sistema onde um
  comportamento não documentado vira multa para o cliente.
