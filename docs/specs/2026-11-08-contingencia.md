# Contingência: NF-e, NFC-e, MDF-e e CT-e

Fase F7 do plano `docs/plans/2026-11-04-moc-cobertura-total.md`. Fontes: MOC online
(`NFe_NFCe/Contingencia_NFe.html`, `NFe_NFCe/ContingenciaNFCe.html`, `MDFe/Contingencia.html`,
`CTe/Contingencia.html`, lidos em 2026-11-07) e os specs `2026-11-04-moc-gap-*.md`. A URL
`NFe_NFCe/Contingencia.html` dá 404; as páginas reais são as acima. O MOC **não** traz as URLs dos web
services da SVC (remete ao Portal Nacional); ver "Decisões pendentes".

## 1. Escopo (decisão do dono)

| Documento | Entra | Não entra |
|---|---|---|
| NF-e | SVC-AN (`tpEmis=6`), SVC-RS (`7`), EPEC (`4`) | FS-IA (2), FS-DA (5), SCAN (3), `tpEmis=9` com `tpImp=6` (fase posterior) |
| NFC-e | offline (`tpEmis=9`) | SVC (o MOC proíbe: `B22-70`/783), EPEC (a critério da UF, fora) |
| MDF-e | offline (`tpEmis=2`) | não há SVC nem EPEC no MDF-e |
| CT-e | offline (`tpEmis=2`) | EPEC revogado (não implementar), FS-DA revogado, SVC-RS/SVC-SP (7/8) fora do escopo desta fase |

## 2. Máquina de estados por documento

```
normal ──(janela aberta + emissão)──> contingência emitida
contingência emitida (SVC)  ──autorizada pela SVC──> authorized      (sem retransmissão)
contingência emitida (EPEC/offline) ──> contingency_pending
contingency_pending ──reenvio autorizado dentro do prazo──> authorized
contingency_pending ──reenvio autorizado fora do prazo──> authorized_late   (CT-e cStat 150; demais: ver pendências)
contingency_pending ──rejeição──> rejected_late (corrigir e regerar: mesmo número e série, mesma chave e cNF)
contingency_pending ──prazo vencido sem reenvio──> overdue (alerta; EPEC pendente há mais de 168 h bloqueia a contingência EPEC do emitente)
```

SVC não tem estado pendente: a nota é autorizada na SVC e já vai ao Ambiente Nacional (MOC 1.1.3.4).

Prazos (MOC): NF-e EPEC 168 h; NFC-e offline até o fim do primeiro dia útil seguinte; MDF-e e CT-e offline
168 h. Alertas em 24 h e 120 h para os de 168 h; para NFC-e, alerta no dia da emissão e às 12 h do dia útil seguinte.

## 3. Endpoints SVC por UF (MOC, Ato COTEPE 39/2012)

SVC-AN (`tpEmis=6`): AC, AL, AP, DF, MG, PB, RJ, RS, RO, RR, SC, SE, SP, TO.
SVC-RS (`tpEmis=7`): AM, BA, CE, ES, GO, MA, MT, MS, PA, PE, PI, PR, RN.
Implementado em `go-dfe/internal/endpoints/table.go` (`svcByUF`, `svcRegistry`, `SVCFor`) com teste de
tabela (27 UFs, uma vez cada). Serviços da SVC: autorização, retorno, cancelamento (só de nota autorizada
pela própria SVC), consulta e status. **Sem inutilização e sem CC-e na SVC**: o pedido é represado para a
SEFAZ de origem. Status da SVC: `107` em operação, `113` em desativação (15 min de coexistência), `114` desabilitada.

## 4. O que o usuário escolhe

Recomendação (default): **chave manual por organização**, endpoint `PUT /organizations/{org}/nfe-contingency`
(`enabled`, `justification`), sem detecção automática na primeira entrega.

Justificativa: a SVC só funciona se a SEFAZ de origem a ativou (`107`); o MOC diz que a empresa só deve consultar
a SVC quando o ambiente normal estiver indisponível; a decisão de entrar em EPEC/offline é exclusiva do
contribuinte e deve ser exceção; e um detector errado produz nota com chave `tpEmis` diferente e numeração a conciliar
(cancelar a autorizada substituída, inutilizar a não autorizada). Automático fica como fase seguinte, por organização,
com N falhas consecutivas de `NfeStatusServico` do autorizador normal mais `107` na SVC, saída automática e alerta
via go-common `alerts`. A janela é por organização (UF do emitente define a SVC); por modelo fica para a decisão do dono.

## 5. Modelo de dados (sem tabela nova)

Em `organization_nfe_configs` (ver `DynamoDB-Tables.md`): `contingency_mode` (`svc`), `contingency_since`
(RFC3339, vira `dhCont`, gravado uma vez por janela), `contingency_reason` (`xJust`). Escritos só pelo endpoint
próprio; o `PUT /nfe-config` os preserva.

Nos documentos (`nfes`, `nfces`, `mdfes`, `ctes`), para EPEC/offline: `contingency_state`
(`pending`/`authorized_late`/`rejected_late`/`overdue`), `contingency_deadline` (RFC3339), `contingency_alerted_at`,
`contingency_sweep` (chave de GSI esparso `contingency-index`, só em pendentes, para varrer sem Scan). O XML
assinado não transmitido fica no S3 sob o prefixo do documento. Os atributos de documento ainda **não** foram criados.

## 6. Retransmissão (função agendada)

Mesmo padrão do `distribution-dispatcher`: EventBridge Scheduler (a cada 15 min) invoca um dispatcher que faz
`Query` no GSI esparso `contingency-index`, e enfileira um job por documento numa fila SQS **padrão** com DLQ; o
worker reenvia a chave e o mesmo `cNF` (idempotente: se a SEFAZ responder duplicidade, `NfeConsultaProtocolo` por
chave e reconcilia). Sketch Terraform (não criar nem implantar nesta fase):

```hcl
resource "aws_sqs_queue" "contingency_retransmit" {
  name                       = "ctech-dfe-${var.env}-contingency-retransmit"
  visibility_timeout_seconds = 180
  redrive_policy = jsonencode({ deadLetterTargetArn = aws_sqs_queue.contingency_retransmit_dlq.arn, maxReceiveCount = 5 })
}
resource "aws_sqs_queue" "contingency_retransmit_dlq" { name = "ctech-dfe-${var.env}-contingency-retransmit-dlq" }
resource "aws_scheduler_schedule" "contingency_dispatcher" {
  schedule_expression = "rate(15 minutes)"
  flexible_time_window { mode = "OFF" }
  target { arn = aws_lambda_function.contingency_dispatcher.arn  role_arn = aws_iam_role.scheduler.arn }
}
```

Sem NAT, sem métricas ou alarmes customizados; falhas que exigem humano usam go-common `alerts`.

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

## 9. Estado de implementação

| Fatia | Estado |
|---|---|
| Design (este documento) | feito |
| go-dfe: SVC-AN/SVC-RS, UF para SVC, `Request.Contingency`, `dfe.SVCFor` | feito |
| NF-e SVC: janela por organização, `tpEmis` 6/7, `dhCont`/`xJust`, roteamento, cancelamento na SVC de origem | feito |
| EPEC (evento 110140, `tpEmis=4`) | não feito |
| MDF-e/CT-e `tpEmis=2` | não feito (depende de "só assinar" no egress e do `sign` do QR) |
| Retransmissão no worker | não feito |
| NFC-e offline (slice 7) | caminho registrado na seção 8.1; código não iniciado |

## Decisões pendentes (dono)

1. **URLs da SVC-AN e SVC-RS**: não estão no MOC. As do código vêm do conhecimento público e precisam ser confirmadas no Portal Nacional (`www.nfe.fazenda.gov.br`, ambientes de produção e homologação) antes de qualquer uso em produção. Modelo de resposta (nó do resultado) da SVC-AN também a confirmar em homologação.
2. Entrada automática por detecção de indisponibilidade (seção 4) ou só manual; e janela por organização ou por organização e modelo.
3. NFC-e offline: a UF do emitente permite (`B22-20`/712)? Onde guardar essa autorização por UF; segunda via "Via do Estabelecimento" ou guarda eletrônica com termo no livro modelo 6.
4. Autorização fora de prazo de NF-e EPEC/MDF-e: tratar como sucesso (como CT-e `cStat 150`) ou como erro a tratar.
5. Prazo da NFC-e: calendário de dias úteis (feriados por UF) a usar.
6. Aceitar a mudança "só assinar" no egress (seção 8) e o novo GSI esparso `contingency-index` nas tabelas de documentos (infra em Terraform, cdk congelado).
7. CT-e: SVC (`tpEmis` 7/8) fica fora desta fase; confirmar.
