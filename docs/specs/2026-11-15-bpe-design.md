# Design: BP-e (modelo 63), com TM e TA

Status: proposta, aguardando aprovação. Data: 2026-11-15. Decisão do usuário: v1 cobre BP-e comum, **TM e TA também**.
Contexto: `2026-11-15-novos-documentos-svrs.md`. Fazer depois da NFCom; reaproveita o que a NFCom deixar pronto
(geração SOAP síncrona, QR Code, contingência offline, webhooks).

## Fontes

Portal `dfe-portal.svrs.rs.gov.br/BPE` e MOC `moc.sped.fazenda.pr.gov.br/BPe/` (Visão Geral, Contingência, Cancelamento,
Eventos Específicos, `BPeTMRecepcao`, `BPeTARecepcao`).

**Schemas e manuais (obtidos em 2026-11-15):** mesmo endpoint estático do portal, com `BPE` no lugar de `NFCOM`.
Pacote vigente: `PL_BPe_100b_NT2026.005 RTC SN_1.00_corr.zip`, que já traz `bpe`, `bpeTM`, `bpeTA`, `bpeTiposBasico`,
`DFeTiposBasicos`, retornos, consulta, status e os eventos (cancelamento, não embarque, alteração de poltrona, excesso de
bagagem, vinculação de pagamento e seu cancelamento, tributação SN). O pacote `PL_BPe_100b_NT022020.zip` é o do TM
original. Manuais: `MOC_BPe_Anexo I_Leiaute_v1.00b.pdf`, `Anexo II_DABPE_v1.00b.pdf`; notas técnicas
`BPe_Nota_Tecnica_2025_002_BPeTA_v2.00.pdf` (TA) e `BPe_NT 2020.002 BPeTM_v1.01.pdf` (TM). Ao implementar, XSD em
`go-dfe/schemas/bpe/` com `SHA256SUMS`.

**Ainda não lidos:** conteúdo dos PDFs (regras de validação, DABPE, leiaute do TA), QR Code, tabelas e a página de
recepção do BP-e comum. Os eventos 112140 (fornecimento não realizado) e 112130 não têm XSD neste pacote; o TA v2.00
pode defini-los, conferir na NT 2025.002.

## Variantes

| Variante | Identificação | Serviço | XSD | Credenciamento |
|---|---|---|---|---|
| BP-e comum | modelo 63, `tpBPe` comum | `BPeRecepcao` | `BPe` | Padrão |
| BP-e TM (metropolitano) | `tpBPe=4`, NT 2020.002 | `BPeRecepcaoTM` | `BPeTM_v9.99.xsd` | Específico, pode diferir do comum |
| BP-e TA (aéreo) | `modal=2`, NT 2025.002, raiz `BPeTA` | `BPeRecepcaoTA` (só SVRS) | `BPeTA_v9.99.xsd`; resposta `retBPe` | Específico |

TM e TA: GZip + Base64 na área de dados, limite de 1024 KB, um documento por envio, síncrono, sem lote. A mesma chave
não pode coexistir como TM e como Normal ou Substituição. O TA processado segue o leiaute do BP-e comum na resposta.

## Serviços e autorizadores

`BPeRecepcao`, `BPeRecepcaoEvento`, `BPeConsulta`, `BPeStatusServico` (v1.00), mais TM e TA. Autorizadores: SVRS, MG,
MS, MT, PR e SP (a PB só publica QR Code). URLs de homologação de MG e MS parecem trocadas na página; validar por
`?wsdl`. Como a SVRS é o único listado com TA, o TA só funciona via SVRS. Mapa UF do emitente para autorizador: **a levantar**.

## Eventos que o emitente envia

| Evento | tpEvento | Regra principal |
|---|---|---|
| Cancelamento | 110111 | `nProt` (15 dígitos) + `xJust` (1 a 255); `nSeqEvento` 1; antes do embarque (cStat 220; 5 min de tolerância, 24 h em contingência); no TM, no máximo 168 h após autorização (cStat 704), salvo liberação 240170; bloqueado com Não Embarque (504), cancelado (218), substituído (224) |
| Não embarque | 110115 | até 24 h após o embarque; exige BP-e não cancelado nem substituído |
| Alteração de poltrona | 110116 | até 24 h após o embarque; sem Não Embarque (681); poltrona diferente da original |
| Excesso de bagagem | 110117 | até 24 h após o embarque; sem Não Embarque (705) |
| Fornecimento não realizado | 112140 | só TA com pagamento antecipado |
| Ajuste de BP-e TA | 112130 | especificação pendente na própria página; **fora da v1** |
| Vinculação de pagamento e cancelamento | 110300, 110301 | NT 2026.001; mesmo desenho da NFCom |

Marcações de fisco (240140, 240160, 240170) e Simples (412140, 412141, cronograma da NT 2026.005) o emitente não envia.

## Contingência

Só offline, `tpEmis=2`: nova chave mantendo `cBP`, `dhCont` e `xJust`, avança numeração, transmite até o fim do primeiro
dia útil seguinte. QR Code com `sign`; DABPE com "EMITIDO EM CONTINGÊNCIA". Risco aceito pelo emitente: rejeição após a
viagem. Texto do MOC é ambíguo sobre trocar a chave; ler a regra de validação antes de codificar.

## Assinatura

XMLDSig enveloped, RSA-SHA1, certificado A1/A3 com CNPJ (`otherName`, OID 2.16.76.1.3.3). Mensagens assinadas com
certificado de estabelecimento do emissor; transmissão com certificado de cliente.

## Plano de arquitetura (a confirmar no plano)

Igual ao da NFCom, com três pacotes de leiaute no `go-dfe` (comum, TM, TA), um `document_type = "bpe"` e subtipo
`tm`/`ta` na API e na UI (campo no cadastro e seletor na emissão; sem texto livre). O número de eventos é maior
(não embarque, poltrona, bagagem, fornecimento não realizado); a UI de detalhe precisa de uma lista de eventos por
documento. DABPE e QR Code são entregáveis da v1.

## Riscos e pontos abertos

- TM e TA exigem credenciamento específico: não há como homologar sem ele. Ver com o usuário se há contribuinte de
  teste credenciado.
- TA tem especificação ainda incompleta (evento 112130 pendente); o leiaute pode mudar na vigência da NT 2025.002.
- BP-e comum tem regras por UF de origem (autorizador próprio de MG, MS, MT, PR e SP); conferir se a UF do emitente é
  atendida pela SVRS ou por autorizador próprio.
- Sem XSD em mãos, esforço desconhecido; obter os arquivos da NT 2026.005, NT 2020.002 e NT 2025.002 primeiro.
- Billing: limite e cobrança, decisão do usuário.

## Decisões abertas

1. Há contribuinte credenciado para homologar TM e TA?
2. Mapa UF para autorizador e se a v1 cobre os seis.
3. ~~Limite e cobrança no billing.~~ Decidido: entra no plano sob demanda apenas, sem limite próprio.

Decisão sobre credenciamento: não há contribuinte credenciado para TM e TA hoje; implementa-se com os XSD e testes
unitários, e a homologação real fica para quando houver credenciamento.
