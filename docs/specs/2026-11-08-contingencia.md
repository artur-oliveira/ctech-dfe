# Contingência: NF-e, NFC-e, MDF-e e CT-e

Fase F7 do plano `docs/plans/2026-11-04-moc-cobertura-total.md`. Fontes: MOC online
(`NFe_NFCe/Contingencia_NFe.html`, `NFe_NFCe/ContingenciaNFCe.html`, `MDFe/Contingencia.html`,
`CTe/Contingencia.html`, lidos em 2026-11-07) e os specs `2026-11-04-moc-gap-*.md`. A URL
`NFe_NFCe/Contingencia.html` dá 404; as páginas reais são as acima. O MOC **não** traz as URLs dos web
services da SVC (remete ao Portal Nacional); ver "Decisões pendentes".

## 1. Escopo (decisões do dono, 2026-11-07)

Opções de forma de emissão por documento (todas listadas na configuração; o usuário troca manualmente):

| Documento | Opções além de `normal` | Não entra |
|---|---|---|
| NF-e | `svc` (SVC-AN `tpEmis=6` / SVC-RS `7`, pela UF), `epec` (`4`) | FS-IA (2), FS-DA (5), SCAN (3), `tpEmis=9` com `tpImp=6` |
| NFC-e | `offline` (`9`, todas as UFs); `epec` (`4`, o MOC lista `tpEmis=4`; a UF decide) | SVC (o MOC proíbe: `B22-70`/783) |
| MDF-e | `offline` (`2`) | não há SVC nem EPEC |
| CT-e | `offline` (`2`); `svc` (SVC-RS `7` / SVC-SP `8`, mesmos autorizadores SVRS e SVSP) | EPEC (em remoção, não implementar), FS-DA (revogado) |

Duas modalidades, ambas entregues: (a) escolha manual da forma de emissão corrente; (b) bandeira `auto_contingency`
(desligada por padrão, por documento): com ela ligada e o autorizador indisponível, o sistema troca a forma de emissão
sozinho e emite, **regerando o documento**.

## 2. Máquina de estados

Por organização e documento (`emission_type`, `contingency_since`):

```
normal ──(usuário escolhe, ou auto_contingency detecta indisponibilidade)──> contingência (svc | epec | offline)
contingência ──(usuário volta, ou autorizador saudável de novo com auto_contingency)──> normal   [contingency_since := null]
contingência A ──(usuário escolhe B)──> contingência B   [contingency_since mantido]
```

Regeração (obrigatória na troca automática e em qualquer emissão cuja forma mudou): `tpEmis` está na chave de acesso
(posição 35), logo a troca muda a chave, e a chave muda a assinatura. A emissão em curso é **descartada e refeita** do
zero: nova chave com o novo `tpEmis` (mesmo número e série, `cNF` novo), novo XML, nova assinatura, `dhCont`/`xJust`.
Nunca se remenda o XML guardado. A tentativa em `normal` que falhou por indisponibilidade não deixou documento
autorizado (consulta por chave antes de regerar: se a SEFAZ a conhece, ela vale e nada é regerado).

Por documento emitido em EPEC/offline:

```
contingency_pending ──reenvio autorizado dentro do prazo──> authorized
contingency_pending ──reenvio autorizado fora do prazo──> authorized_late   (SUCESSO: decisão do dono; CT-e cStat 150, NF-e EPEC e MDF-e igual)
contingency_pending ──rejeição──> rejected_late (corrigir e regerar: mesmo número, série, chave e cNF)
contingency_pending ──prazo vencido sem reenvio──> overdue (alerta; EPEC pendente há mais de 168 h bloqueia a contingência EPEC do emitente)
```

SVC não tem estado pendente: a nota é autorizada na SVC e já vai ao Ambiente Nacional (MOC 1.1.3.4). Documentos em
contingência offline **precisam ser transmitidos** quando o ambiente volta: rotina automática (seção 6).

Prazos (MOC): NF-e EPEC 168 h; NFC-e offline: começa em **3 dias** (constante configurável; varia por UF; o MOC fala
em "primeiro dia útil seguinte"); MDF-e e CT-e offline 168 h. Alertas em 24 h e 120 h para os de 168 h.

## 3. Endpoints SVC por UF (MOC, Ato COTEPE 39/2012)

SVC-AN (`tpEmis=6`): AC, AL, AP, DF, MG, PB, RJ, RS, RO, RR, SC, SE, SP, TO.
SVC-RS (`tpEmis=7`): AM, BA, CE, ES, GO, MA, MT, MS, PA, PE, PI, PR, RN.
Implementado em `go-dfe/internal/endpoints/table.go` (`svcByUF`, `svcRegistry`, `SVCFor`) com teste de
tabela (27 UFs, uma vez cada). Serviços da SVC: autorização, retorno, cancelamento (só de nota autorizada
pela própria SVC), consulta e status. **Sem inutilização e sem CC-e na SVC**: o pedido é represado para a
SEFAZ de origem. Status da SVC: `107` em operação, `113` em desativação (15 min de coexistência), `114` desabilitada.

## 4. Configuração e API

Por documento (tabelas `organization_{nfe,nfce,cte,mdfe}_configs`, sem tabela nova):

| Campo | Significado |
|---|---|
| `emission_type` | forma corrente: `normal` (padrão, ausente = normal), `svc`, `epec`, `offline`; só os valores válidos para o documento |
| `auto_contingency` | bool, padrão `false` |
| `contingency_since` | RFC3339 (`dhCont`); definido ao sair de `normal`, mantido enquanto continua em contingência, **removido (null) ao voltar a `normal`** |
| `contingency_reason` | `xJust`, 15 a 256 caracteres, obrigatório fora de `normal` |

Endpoints: `PUT /organizations/{org}/{nfe,nfce,cte,mdfe}-emission` (`emission_type`, `auto_contingency`,
`justification`); o GET da configuração devolve também `emission_options` (a lista válida do documento). Problemas:
422 `emission_type_invalid`, 422 `emission_reason_required`. O `PUT /{x}-config` nunca altera esses campos.

Compatibilidade: `PUT /nfe-contingency` (`enabled`, `justification`) continua, agora `deprecated`: `enabled` equivale a
`emission_type=svc`, desligar a `normal`; a bandeira automática é preservada. O atributo `contingency_mode` da primeira
rodada é lido como `emission_type` e removido na primeira escrita nova (sem migração em lote).

A SVC só funciona se a SEFAZ de origem a ativou (`107`); detecção automática (seção 4.1) usa `NfeStatusServico`.

### 4.1 Detecção automática (fatia E; desenho final de 2026-11-08)

Estado: a lógica de decisão e as guardas existem e estão testadas (`api/internal/services/autocontingency`); a troca em
si está **desligada** pela constante `autocontingency.SwitchEnabled = false`. Nada liga por padrão: `auto_contingency`
(padrão `false`) hoje só guarda a escolha do dono. A lista do que falta para ligar está no fim desta seção.

**Configuração.** `auto_contingency` (bool) mais `auto_contingency_type` (`svc`, `epec` ou `offline`): a contingência que
o dono escolhe para a troca automática, validada contra `emission_options` do documento (sem `normal`). Obrigatória com
`auto_contingency` ligada (422 `auto_contingency_type_required`; 422 `auto_contingency_type_invalid` se não é uma
contingência do documento, por exemplo `offline` na NF-e ou `svc` no MDF-e); removida da linha quando a bandeira desliga.
O sistema nunca troca para um tipo que o dono não escolheu: se a SVC escolhida também está fora (status da SVC diferente de
107), não há troca automática para EPEC; fica o alerta. Alvos por documento: NF-e `svc` ou `epec`; NFC-e `offline`;
CT-e `offline` ou `svc`; MDF-e `offline`.

**1. Sinais de "autorizador indisponível"** (`autocontingency.Classify`). Contam: falha de transporte ou timeout na chamada
ao egress, status HTTP 408 ou 5xx do autorizador, falha SOAP (fault), `cStat` 108 (paralisado momentaneamente) e 109
(paralisado sem previsão), tanto na resposta do serviço de status quanto na da autorização. **Não** contam: qualquer
resposta de negócio, inclusive rejeição do documento (o autorizador respondeu, logo está de pé; conta como saudável), e
429. Limiar: `FailureThreshold` = 3 falhas **consecutivas** dentro de `FailureWindow` = 2 min abrem o disjuntor; uma resposta
saudável zera a contagem; falhas fora da janela não somam.

**2. Onde a decisão roda.** Três papéis, por ordem de segurança:
- *Detecção* (worker): é o único que vê as respostas reais do egress; alimenta o disjuntor. Uma sonda agendada (a cada
  minuto, só para quem tem `auto_contingency` ligada) chama o serviço de status do autorizador normal (`NfeStatusServico`,
  `CTeStatusServico`, `MDFeStatusServico`; exige o certificado da empresa, por isso é por empresa e documento).
  O estado do disjuntor (`closed`/`open`, falhas, abertura, sequência de sondas) mora na linha de configuração do
  documento, escrito só pelo worker, nunca por `PUT /{x}-config`.
- *Pré-checagem na API* (**caminho principal e seguro**): `Emit` já lê a configuração. Com o disjuntor aberto, a bandeira
  ligada, o tipo escolhido e `emission_type=normal`, a API grava `emission_type` = o tipo escolhido,
  `contingency_since` e `contingency_auto=true` (condicional) e emite **a primeira e única vez** já na contingência
  (nova chave, novo `cNF`, `dhCont`/`xJust`, assinatura). Nada saiu antes com `tpEmis` 1 para aquele número, logo não há
  risco de duplicidade.
- *Reação a emissão em voo* (worker, documento cuja tentativa normal falhou): alimenta o disjuntor e **não regera**. O
  documento segue pelo reenvio idempotente de sempre (mesma chave; 204 reconcilia). Regerar um documento em voo é o caso
  de risco (próximo item) e fica fora desta rodada.

**3. Risco de duplicidade ao regerar um documento em voo.** A tentativa original (`tpEmis` 1) pode ter chegado à SEFAZ sem
que a resposta voltasse. Como `tpEmis` está na chave, o regerado tem **outra chave com a mesma série e número**: os dois
poderiam ser autorizados. Duas defesas, e nenhuma basta sozinha:
- *Prova por consulta* (`autocontingency.ProveOriginal`/`Reissue`): antes de regerar, consulta por chave do original
  (`NfeConsultaProtocolo` e equivalentes). Autorizado (100/150): o original **é** o documento; nada é regerado (adota-se
  o original). Cancelado ou denegado (101/151/155/110): o número foi consumido; não regera, alerta. "Não consta" (217)
  só prova depois de `InFlightQuarantine` = 10 min do último envio (um pedido preso em timeout pode chegar tarde à SEFAZ;
  antes disso o 217 não vale). Qualquer outra resposta, ou a consulta falhar: **sem prova**.
- *A regra 539 da SEFAZ* ("duplicidade de NF-e com diferença na chave", mesmo emitente, série e número com outra chave) é
  simétrica: ela recusa o segundo documento autorizado, seja o regerado (se o original chegou primeiro) ou o original
  tardio. Ela impede a autorização dupla, mas não impede o regerado ficar rejeitado com o original autorizado sem que o
  sistema saiba. Por isso o regerado carrega `reissued_from` (a chave original) e uma rejeição 539 nele dispara a consulta
  da **chave original** (hoje o worker só reconcilia o 204 pela chave do próprio documento): autorizada, o original vence e
  o regerado é descartado. Texto e escopo da 539 por modelo: **não relidos no MOC nesta rodada (não verificado)**.

**Sem prova possível (consulta também falha): opção segura.** Não regerar; o documento original permanece pendente e sai
um alerta (go-common `alerts`). Motivo: o dano é assimétrico. Esperar atrasa uma nota, mas o reenvio idempotente do
original continua valendo e nada fiscal é criado; regerar às cegas pode produzir duas notas válidas para o mesmo número, e
corrigir isso exige cancelamento com prazo e efeito fiscal para o cliente.

**4. Volta ao normal** (`ShouldReturn`). Só se a contingência foi aberta pela troca automática (`contingency_auto`); uma
contingência escolhida pelo dono nunca é desfeita pelo sistema. O disjuntor aberto só considera sondas (documentos
emitidos em contingência nada dizem sobre o autorizador normal). Histerese: depois de `CoolDown` = 5 min aberto, são
necessárias `RecoveryProbes` = 3 sondas saudáveis **consecutivas**; uma sonda ruim zera a sequência (evita o vai e vem).
Ao fechar: `emission_type` volta a `normal` e `contingency_since`/justificativa são removidos (mesmo caminho de
`SetEmission`), e os documentos offline/EPEC pendentes continuam sendo reenviados pela rotina de 15 min da seção 6.

**Para a UI:** campo `auto_contingency_type` no PUT e no GET da configuração; problemas 422
`auto_contingency_type_required` e `auto_contingency_type_invalid`; `AUTO_CONTINGENCY_AVAILABLE` deve continuar falso até
`SwitchEnabled` ligar. Previsto, ainda não gravado: `contingency_auto` (bool) e o estado do disjuntor na configuração.

**O que falta para ligar (`SwitchEnabled`):** (a) escrita do disjuntor pelo worker e a sonda agendada (infra no `cdk/`,
Scheduler a cada minuto); (b) a pré-checagem em `Emit` com a gravação condicional de `emission_type`/`contingency_auto`;
(c) a volta ao normal pela sonda; (d) para regerar documento em voo: guardar o pedido original (S3 sob o prefixo do
documento, só com a bandeira ligada), `reissued_from` e a reconciliação da 539 pela chave original no worker; (e) alerta
via go-common `alerts`. Sem (d) o caminho principal (pré-checagem) já é seguro e basta para ligar.

### 4.2 Decisões do dono sobre a contingência automática (2026-11-08)

1. Vale só para **emissões novas**: documento em voo fica pendente, não é regerado (a regeração de documento em voo,
   com `reissued_from` e reconciliação do 539, fica para depois e não bloqueia ligar o interruptor).
2. Parâmetros mantidos: quarentena de 10 minutos para o 217, espera de 5 minutos e 3 sondagens saudáveis seguidas para
   voltar ao normal, disjuntor de 3 falhas em 2 minutos.
3. O sistema **pode** cair de SVC para EPEC sozinho (NF-e), quando o SVC também estiver indisponível: leitura de "sim"
   do dono à pergunta "deve alguma vez cair sozinho de SVC para EPEC?". Ainda não implementado; o desenho precisa de uma
   ordem de preferência explícita (o tipo escolhido em `auto_contingency_type` primeiro, depois o seguinte da cadeia) e
   de um limite para não alternar entre os dois.

## 5. Modelo de dados dos documentos

Nos documentos (`nfes`, `nfces`, `mdfes`, `ctes`), para EPEC/offline: `contingency_state`
(`pending`/`authorized_late`/`rejected_late`/`overdue`), `contingency_deadline` (RFC3339), `contingency_alerted_at`,
`contingency_sweep` (chave de GSI esparso `contingency-index`, só em pendentes, para varrer sem Scan). O XML
assinado não transmitido fica no S3 sob o prefixo do documento.

## 6. Retransmissão (função agendada)

Implementado no `cdk/` no padrão do `distribution-dispatcher` do repositório: EventBridge Scheduler (a cada 15 min)
invoca o `contingency-dispatcher`, que faz `Query` no GSI esparso `contingency-index` (hash `contingency_sweep`,
range `contingency_deadline`, KEYS_ONLY) das quatro tabelas de documento e **invoca o worker
`contingency-retransmit-worker` de forma assíncrona uma vez por documento** (Lambda tenta 2 vezes e entrega ao
`dlq-processor`). Desvio consciente do pedido "SQS padrão com DLQ": o repositório já substituiu filas SQS por
invocação assíncrona (nada faz long-poll; teste `no SQS event source mapping and only the two parking queues exist`),
e a invocação assíncrona com destino de falha tem a mesma resiliência (redrive + DLQ) sem fila nova. Se o dono
preferir SQS, é trocar o `Invoke` por `SendMessage` e o worker por event source.

O worker reenvia a chave e o mesmo `cNF` (idempotente: duplicidade 204/539 leva a `NfeConsultaProtocolo` por chave
e reconcilia). `internal/contingency.Classify` decide o resultado: autorizada no prazo (`authorized`), autorizada
fora do prazo ou cStat 150 (`authorized_late`, sucesso), rejeição (`rejected_late`), sem resposta após o prazo
(`overdue`, sai do índice). Sem NAT, sem métricas ou alarmes customizados.

Regras de reconciliação (MOC): rejeição: regerar com mesmo número e série sem mudar valor do imposto, partes ou
datas; nota enviada sem resposta antes da contingência: consultar por chave e então cancelar (autorizada) ou
inutilizar (não autorizada); duplicidade de chave natural entre normal e SVC: cancelar a que não circulou.

## 7. Custo

Scheduler a cada 15 min: ~2.900 invocações por mês, dentro do free tier do Lambda e do Scheduler. GSI esparso
só tem itens pendentes (custo desprezível). Reenvio reaproveita a mesma chamada de autorização já cobrada; o uso
(billing) é reservado uma vez por documento, no momento da emissão.

## 8. QR Code offline da NFC-e (mudança de arquitetura)

Hoje `api/internal/services/nfes/nfce_qrcode.go` monta o QR antes de assinar porque o QR online não depende da
assinatura. No offline: v2 usa o `DigestValue` da assinatura e v3 o campo `assinatura`; ambos só existem depois
de assinar, e a assinatura acontece no `go-dfe-egress` (`xmlops.Sign`). Proposta: o egress recebe um pedido
"só assinar" (também necessário para persistir MDF-e/CT-e/NFC-e assinados e não transmitidos), devolve o XML assinado
com o `DigestValue`, e a API monta o `infNFeSupl` em cima do digest, **reassinando** não é necessário se o
`infNFeSupl` fica fora do `infNFe` assinado (fica; a assinatura cobre só `infNFe`). Fluxo: API monta o `infNFe`,
egress assina (modo `sign_only`), API lê o `DigestValue`, monta o `qrCode`, insere o `infNFeSupl` no XML assinado
e imprime o DANFE; a transmissão posterior envia esse XML pronto. Para MDF-e/CT-e offline o parâmetro `sign` do QR
é a assinatura RSA-SHA1 da chave de acesso com o certificado do emitente: também exige o egress (certificado só lá).

### 8.1 Slice 7: caminho aprovado (NFC-e offline, QR Code v3 com a assinatura)

O dono aprovou, em princípio, a mudança de arquitetura da seção 8. Fica fora de escopo: criar ou implantar
infraestrutura, tocar em `cdk/` e o gatilho agendado de retransmissão (só especificado na seção 6).

Caminho (o único a seguir): (1) `go-dfe` ganha o serviço "só assinar" para NFC-e (assina o `infNFe` e devolve o XML
assinado com o `DigestValue`, sem transmitir); entra em `dfe.Implements` só com o portão de assinatura do
`go-dfe/CLAUDE.md`; (2) a API monta o `infNFe` com `tpEmis=9`, `dhCont`, `xJust`, `idDest=1`, `finNFe=1`, `indFinal=1`,
`indPres=1`, avança a numeração e chama o egress em modo sign-only; (3) com a assinatura em mãos, a API monta o
`qrCode` v3 offline e o `infNFeSupl`, e grava o XML assinado com `contingency_state=pending`; (4) a transmissão
posterior reenvia o XML pronto, com a mesma chave e o mesmo `cNF`.

Trade-offs: (a) uma invocação a mais do egress por nota offline (latência e custo desprezíveis, só na contingência);
(b) o certificado continua só no egress (sem segredo novo na API); (c) o `infNFeSupl` fica fora da área assinada,
logo montar o QR depois de assinar não invalida a assinatura; (d) a alternativa de pré-calcular o digest na API
duplicaria a canonicalização C14N do `signer.go` (arquivo de maior risco) e foi descartada; (e) depende da decisão
pendente 3 (a UF permite offline) e do formato exato do QR v3 offline no MOC (`DanfeQrCodeNFCe.html`), a ler antes de codar.


### 8.2 Correção da seção 8 depois de ler a NT 2025.001 e o Manual do DANFE NFC-e (2026-11-07)

Fontes lidas: `NT2025.001_v1.00 - NFCe_qrCode_3.pdf` (itens 02.1, 03.3 grupo ZX, 04) e o Manual de Padrões Técnicos do
DANFE NFC-e e QR Code (itens 3.1.8, 4.4.1 e 4.4.2). A seção 8 e a 8.1 partiam de que o QR v3 offline usa o `DigestValue`
da assinatura do XML; **não usa**. O que os documentos dizem:

- **QR v3 online** (`tpEmis` diferente de 9): `URL?p=<chave>|3|<tpAmb>`. Sem CSC, sem `idCSC`, sem hash, sem
  assinatura (informar `assinatura` fora da contingência é a rejeição 445).
- **QR v3 offline** (`tpEmis` 9): `URL?p=<chave>|3|<tpAmb>|<dia>|<vNF>|<tp_idDest>|<idDest>|<assinatura>`. `dia` com
  2 dígitos (dia de `dhEmi`), `vNF` com ponto e sem milhar, `tp_idDest` 1 CNPJ, 2 CPF, 3 idEstrangeiro (destinatário
  não identificado: só o separador `|`; estrangeiro ou não identificado: `idDest` vazio, só o separador), e
  `assinatura` = **RSA SHA-1 em Base64 da concatenação dos parâmetros 1 a 7, mantendo os `|`, com o mesmo certificado que
  assina a NFC-e**. Regras: ZX02-324 a 338 (rejeições 396, 397, 474, 583). Sem `CDATA` na v2 e na v3.
- **Consequência de arquitetura**: não há dependência do `DigestValue`; o que precisa do certificado é assinar uma
  string. O egress (único lugar com o certificado) ganha uma operação "assinar texto" (RSA-SHA1, Base64). O QR v3
  offline exige a chave de acesso, o dia e o valor, todos conhecidos antes de assinar o XML; então o fluxo é: a API
  monta o `infNFe`, pede ao egress a assinatura da string do QR, monta o `qrCode` e o `infNFeSupl` (fora da área
  assinada) e depois o egress assina o XML (modo "só assinar", para guardar e transmitir depois).
- **QR v2 offline** continua com `digVal` (SHA1 do `DigestValue` em hexadecimal) e CSC; só então a assinatura do XML
  precede o QR.
- **Escolha de versão**: a UF pode ainda não aceitar a v3 (rejeição 407, ZX02-220); a v3 é obrigatória para emitente
  produtor rural pessoa física, exceto PR (a v2 para pessoa física é a rejeição 444); para pessoa jurídica é opção.
  A configuração fiscal da NFC-e precisa de `qr_code_version` (2 ou 3), com a v2 como padrão até a UF aceitar a v3.
- **DANFC-e em contingência** (Manual, 3.1.8): em dois lugares (abaixo do cabeçalho e abaixo da identificação da NFC-e)
  o texto em duas linhas "EMITIDA EM CONTINGÊNCIA" e "Pendente de autorização"; a informação do protocolo é
  suprimida; uma **segunda via** com o texto "Via do Estabelecimento" ao lado da data e hora de emissão (a UF pode
  dispensar; alternativa: guarda eletrônica do XML mediante termo no livro modelo 6). Decisão do dono: duas páginas
  (via do consumidor e via do estabelecimento) com as tarjas.
- **Fora do QR, na mesma NT 2025.001** (verificar se já está implementado): B09-20 (atraso de 7 dias com cStat 150,
  e só contingência depois de 30 dias a critério da UF), E16a-30/35, Y09-40 e Y09-50, YA03-10 a YA06-10 e a mudança no
  tratamento de `indIEDest` (5E17-12, rejeição 300).

## 9. Estado de implementação

| Fatia | Estado |
|---|---|
| Design (este documento) | atualizado com as decisões de 2026-11-07 |
| NFC-e QR Code v3 online (`qr_code_version`, sem CSC; rejeição 444 para CPF com v2) | feito (fatia 1) |
| NFC-e QR Code v3 offline: string e assinatura (`go-dfe` `nfce/SignText`, `nfes/nfce_offline.go`) | feito (fatia 2); o plumbing de emissão offline fica na fatia 3 |
| NFC-e offline (tpEmis 9): `SignOnly` no egress, `@signed_xml`, QR v2/v3, registro pendente no índice | feito (fatia 3); `status` segue `pending` |
| Retransmissão: `contingency-retransmit` reenvia `contingency_msg` por `DfeService.Process` e classifica com `Classify` (150 ou 100 fora do prazo = sucesso, duplicidade reconciliada pelo Process, vencido sai do índice) | feito (fatia 4); infra pronta (seção 9.2) |
| go-dfe: SVC-AN/SVC-RS, UF para SVC, `Request.Contingency`, `dfe.SVCFor` | feito |
| NF-e SVC: `tpEmis` 6/7, `dhCont`/`xJust`, roteamento, cancelamento na SVC de origem | feito |
| B: modelo de configuração por documento, endpoints `*-emission`, validação, `contingency_since` | feito |
| C: EPEC da NF-e | feito (2026-11-08): `nfe/SignOnly` no egress, emissão `tpEmis` 4 (`nfes/epec_emit.go`), evento 110140 pelo outbox, `awaiting_epec`, worker arma só com o evento registrado (135/136/573) e rejeita sem armar, pré-validação 121/228; opção `epec` ativa só na NF-e. Não verificado: textos de 121/142/228 no MOC |
| D1: MDF-e offline (`tpEmis` 2): sem `dhCont`/`xJust` (leiaute), QR Code com `sign` (RSA SHA-1 da chave, `mdfe/SignText`), XML assinado guardado (`mdfe/SignOnly`), DAMDFE com EMISSÃO EM CONTINGÊNCIA no lugar do protocolo, prazo 168 h do MOC | feito |
| D2: CT-e offline (`tpEmis` 2): `dhCont`/`xJust`, QR Code com `sign`, `cte/SignOnly`, 168 h, 150 fora do prazo | feito |
| D3: CT-e SVC-RS/SVC-SP (`tpEmis` 7/8): endpoints, `dfe.SVCForCTe`, chave e `ide` sem `dhCont`/`xJust`, QR Code sem `sign`, cancelamento na mesma SVC | feito; **UF para SVC e URLs não verificadas** |
| D4: classificação tardia: 150 do CT-e confirmado (G103); MDF-e sem cStat tardio no MOC (100 fora do prazo é sucesso; 150 do MDF-e não verificado) | feito |
| DACTE | não há renderizador de DACTE no repositório: contingência (texto e QR com `sign`) só especificada aqui |
| E: contingência automática | desenho final na seção 4.1; `auto_contingency_type` (campo, validação e problemas) e a lógica de decisão (`autocontingency`: sinais, disjuntor com histerese, prova de duplicidade, `Reissue`) feitos e testados; **troca desligada** (`SwitchEnabled = false`); falta o que a seção 4.1 lista |
| F: retransmissão (worker + cdk) | parcial: `contingency-index`, dispatcher a cada 15 min, worker com `Classify` e marcação de vencido; falta o reenvio pelo egress (depende de D) |
| G: DANFC-e de duas vias | feito (fatia 5, seção 9.3) |

### 9.1 DANFC-e offline (decisão 2)

O DANFC-e de uma NFC-e offline tem duas páginas: via do consumidor e "Via do Estabelecimento" (MOC DANFE NFC-e: texto
"Via do Estabelecimento" ao lado da data e hora de emissão; a UF pode dispensar a via, e o contribuinte pode optar pela
guarda eletrônica do XML), ambas com tarja de emissão em contingência e pendente de autorização.
O QR Code offline do MOC lido em 2026-11-07 (`DanfeQrCodeNFCe.html`, item 3.2.2) é a **versão 2**:
`chave|2|tpAmb|dia|vNF|DigestValue(hex, 56)|idCSC|hash`. O formato "v3 com a assinatura" **não consta** nessa página
e fica **não verificado**.

### 9.2 Pendência de infra da fatia 4

`cmd/contingency-retransmit` agora usa `config.Load()` e `service.New`, como `cmd/worker`: a Lambda precisa de
`DOCUMENTS_BUCKET`, `CERTIFICATES_BUCKET`, `DFE_LAMBDA_NAME`, `DFE_EGRESS_REGION`, `RESULTS_TOPIC_ARN`, leitura/escrita
S3, `lambda:InvokeFunction` no egress, `sns:Publish` e escrita nas tabelas. Feito em `cdk/`: o worker está em `WORKERS` (`worker-definitions.ts`) e recebe do laço de `worker-stack.ts` as mesmas variáveis, S3, `lambda:InvokeFunction` no egress, `sns:Publish` e DynamoDB (Get/Put/Update/Query, inclusive índices) das tabelas de documento e de eventos (`nfce_events` acrescentada); o dispatcher tem o `lambda:InvokeFunction` assíncrono e `Query` só no `contingency-index`. Sem SQS, sem NAT, sem alarme; sem concorrência reservada (custo e piso da conta). Coberto em `cdk/test/worker-stack.test.ts`.

### 9.3 DANFC-e offline (fatia 5)

O renderizador (`api/internal/services/documents`, `nfce.go` e `templates/danfce.html`) já tinha as duas tarjas, a
supressão do protocolo e as duas vias para `tpEmis=9`. Ajustes: o texto "Via do Estabelecimento" passa para o lado da data
e hora de emissão (Manual 3.1.8) e só a segunda via é identificada; `GET` do DANFC-e de uma NFC-e offline ainda sem
`xml_s3_key` imprime a partir do XML assinado guardado em `contingency_msg`, com o estado de cache próprio
`contingency_pending` (o PDF é regerado, com protocolo, depois da autorização).

## Decisões do dono (2026-11-07), substituem as pendentes

1. Duas modalidades: forma de emissão corrente manual por documento (todas as opções válidas) e `auto_contingency` (desligada por padrão), regerando o documento; `contingency_since` nulo ao voltar a normal; retransmissão automática dos offline.
2. NFC-e offline em todas as UFs; DANFC-e de duas páginas (consumidor e Via do Estabelecimento) com tarjas.
3. NF-e EPEC e MDF-e autorizados fora do prazo são sucesso.
4. Prazo de retransmissão da NFC-e: 3 dias, constante configurável, varia por UF.
5. CT-e: offline (2) mais SVC-RS/SVC-SP (7/8); EPEC do CT-e não será implementado.
6. cdk não está congelado neste repositório: infra (dispatcher, fila padrão com DLQ, GSI esparso) vai no `cdk/`.

## Ainda em aberto

1. URLs da SVC-AN e SVC-RS: não estão no MOC; confirmar no Portal Nacional (produção e homologação), bem como o nó de resposta da SVC-AN.
2. URLs do SVC-RS/SVC-SP do CT-e (mesmo ponto).
3. Formato exato do QR Code v3 offline da NFC-e (a página lida só traz a v2).
4. Calendário de dias úteis por UF para o prazo da NFC-e (hoje 3 dias corridos).
5. Segunda via impressa ou guarda eletrônica com termo no livro modelo 6, por UF.
