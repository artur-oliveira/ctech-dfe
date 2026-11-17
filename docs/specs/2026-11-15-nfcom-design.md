# Design: NFCom (modelo 62)

Status: implementada na branch `feat/nfcom` (api, worker, go-dfe, cdk, ui), sem homologação real; ver `CONDUCT.md`. Data: 2026-11-15. Escopo: novo documento no `go-dfe`, `worker`, `api`, `ui` e
`cdk`. Contexto e priorização: `2026-11-15-novos-documentos-svrs.md`. Precedente de módulo novo:
`2026-08-04-nfse-design.md`; de SOAP síncrono com mTLS: CT-e e MDF-e.

## Fontes

Portal `dfe-portal.svrs.rs.gov.br/NFCOM` e MOC `moc.sped.fazenda.pr.gov.br/NFCom/` (Visão Geral, Contingência,
Cancelamento, índice).

**Schemas e manuais (obtidos em 2026-11-15, endpoint estático público do portal):**
`GET https://dfe-portal.svrs.rs.gov.br/NFCOM/DownloadArquivoEstatico/?sistema=NFCOM&tipoArquivo=<1|2|3>&nomeArquivo=<arquivo>`
(1 manuais, 2 pacotes `PL_`, 3 notas técnicas). Pacote vigente: `PL_NFCOM_1.00_NT2026.005 RTC SN 1.00.zip` (NFCom 1.00
com NT 2026.005, Simples Nacional; 19 XSD: `nfcom`, `retNFCom`, `consSit`, `consStatServ`, `evento*`, `evCancNFCom`,
`evVincPgto`, `evCancVincPgto`, `evTributacaoSN`, `proc*`, `DFeTiposBasicos`, `xmldsig`; o leiaute fica em
`nfcomTiposBasico_v1.00.xsd`, 3182 linhas). Manuais: `MOC_NFCOM_Anexo I_Leiaute e Regras de Validacao_v1.00a.pdf` e
`Anexo II_DANFE-COM_v1.00.pdf`. Copiados só para o scratchpad; ao implementar, os XSD vão para `go-dfe/schemas/nfcom/`
com `SHA256SUMS`, como em `nfe`, `cte` e `mdfe`.

**Ainda não lidos:** o conteúdo dos PDFs (regras de validação, DANFE-COM), `QRCodeConsultaPublica.html`,
`TabelasECodigos.html` e a página dos eventos 110300/110301.

## O que é

Nota Fiscal Fatura de Serviço de Comunicação Eletrônica, substitui os modelos 21 e 22 (telecom, internet). Chave de 44
posições: cUF(2) AAMM(4) CNPJ(14) mod(2) série(3) nNF(9) tpEmis(1) nSiteAutoriz(1) cNFCom(7) DV(1), módulo 11. Os 12
primeiros caracteres do CNPJ podem ser alfanuméricos (conferir o `taxid` do `go-dfe`).

## Serviços (v1.00, SOAP síncrono, sem lote)

`NFComRecepcao`, `NFComConsulta`, `NFComStatusServico`, `NFComRecepcaoEvento`. Não há inutilização nem distribuição
listadas. Autorizadores: SVRS, MG, MS e MT (URLs no spec pai). O mapa UF do emitente para autorizador **não foi
encontrado**; levantar no MOC antes de implementar a tabela de `go-dfe/internal/endpoints`.

## Eventos que o emitente envia

| Evento | tpEvento | Regra |
|---|---|---|
| Cancelamento | 110111 | `nProt` + `xJust` (1 a 255); prazo: autorizada até 120 horas, 5 min de tolerância (cStat 220 fora do prazo); `nSeqEvento` 1; bloqueado se já cancelada, substituída, de substituição (`finNFCom=3`), referenciada por Ajuste ou Cofaturamento |
| Vinculação de pagamento | 110300 | detalhes na página do MOC, não lida |
| Cancelamento da vinculação | 110301 | idem |

Eventos de fisco (substituição, ajuste, cofaturamento, liberação de prazo 240170, Simples 412140/412141) o emitente
não envia; chegam como marcação e devem aparecer na consulta/histórico do documento.

## Contingência

Só offline, `tpEmis=2`: refaz a chave mantendo `cNFCom`, preenche `dhCont` e `xJust`, avança a numeração, transmite até o
fim do primeiro dia útil seguinte com a mesma chave. QR Code leva `sign` (assinatura da chave pelo certificado do
emitente); DANFE-COM traz "EMITIDA EM CONTINGÊNCIA". Reaproveitar `go-dfe/autocontingency` (decisão compartilhada) só
se as regras de gatilho servirem; a emissão offline é decisão do emitente, sem SVC.

## Assinatura

XMLDSig enveloped sobre `infNFCom` (atributo `Id`), RSA-SHA1, C14N, certificado ICP-Brasil A1/A3 com CNPJ. Mesmo
caminho do `go-dfe` (CT-e/MDF-e); mTLS na transmissão, sempre pela `go-dfe-egress`.

## Plano de arquitetura (a confirmar no plano)

1. **go-dfe:** pacote `nfcom` (tipos do leiaute, gerador de XML, validação por XSD embutido em `go-dfe/schemas`, chave e
   DV, QR Code, endpoints, parsers de retorno e eventos). Registrar os serviços em `dfe.go` (lista de operações
   permitidas, como `MDFe*`).
2. **worker:** nova entrada em `cdk/lib/worker-definitions.ts`, handler `emit`, `cancel`, `consult` e `event`,
   idempotente, resultado terminal por SNS (como CT-e/MDF-e).
3. **api:** `internal/services/nfcoms/`, rotas `/v1.0/nfcoms` (emitir, listar, detalhar, cancelar, consultar, XML,
   DANFE-COM), repositório e tabela(s) DynamoDB com `sweep` nas linhas de config; numeração com `TransactWrite`;
   `ResultsConsumer` e `WebhookService.Emit` com `document_type = "nfcom"`; erasure catalog; escopos OAuth.
4. **cdk:** tabelas e índices em `dynamodb-stack.ts`, worker, IAM, `DynamoDB-Tables.md`.
5. **ui:** emissão, listagem, detalhe, cancelamento, cadastro fiscal da empresa, `nav.tsx`, guia `/guide` com capturas,
   card na landing e na home (`DFE_DOCUMENTS`), valor novo no filtro de webhooks.
6. **docs:** DOCS.md, OVERVIEW.md, CONDUCT.md, `worker/CLAUDE.md`, `api/CLAUDE.md`.

## Riscos e pontos abertos

- Sem XSD e leiaute em mãos, estimar o tamanho do gerador é chute. Obter os arquivos da NT 2026.005 primeiro.
- RSA-SHA1 e chave de 1024 bits no MOC; conferir se a SVRS aceita o que o `go-dfe` já assina.
- Faturamento por assinatura mensal (muitas notas por empresa): considerar emissão em massa; fora da v1.
- Billing: decidir se NFCom entra no plano `prod_dfe` existente ou tem limite próprio (decisão do usuário).
- Cooperação com a reforma: IBS/CBS no leiaute (já vigente desde 01/10/2026 segundo terceiros); tabela de classificação
  tributária (IT 2025.002 v1.71) é dependência.
- Homologação com autorizador real exige certificado de teste; não commitar PFX.

## Não objetivos da v1

Eventos de fisco emitidos pelo emitente, cofaturamento, substituição e ajuste de NFCom (finNFCom 3 e 4, tpFat 2)
até o leiaute ser lido; avaliar no plano se entram.

## Decisões abertas

1. Autorizador por UF (levantar) e se a v1 cobre os quatro.
2. Entram na v1 substituição, ajuste e cofaturamento?
3. ~~Limite e cobrança no billing.~~ Decidido: entra no plano sob demanda apenas, sem limite próprio.
