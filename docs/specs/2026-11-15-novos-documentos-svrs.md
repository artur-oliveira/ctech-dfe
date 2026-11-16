# Novos documentos fiscais do portal SVRS

Status: ordem decidida (NFCom, depois BP-e); falta fechar leituras do MOC e escrever spec e plano por documento. Data: 2026-11-15.

## Intenção

O DF-e cobre NF-e, NFC-e, CT-e, MDF-e e NFS-e (esta pelo ADN nacional, fora da SVRS). Esta spec avalia quais
documentos do portal https://dfe-portal.svrs.rs.gov.br/ entram a seguir e em que ordem.

## O que o portal lista (home, 2026-11-15)

BPe, CTe, MDFe, NF3e, NFCe, NFCom, NFe, NFF, ONE, DCe, NFAg, NFABI, NFGas, CFF, PES, DIFAL.

Modelos (fonte de terceiros, conferir no MOC): NFCom 62, BP-e 63, NF3e 66, NFAg 75, NFGas 76, NF-e ABI 77, DC-e 99.
NFF, ONE, CFF, PES e DIFAL: a home não os descreve; não foram avaliados e podem não ser documentos emissíveis.

## O que as páginas confirmam

- **NFCom (modelo 62):** Nota Fiscal Fatura de Serviço de Comunicação Eletrônica; substitui os modelos 21 e 22.
  Mesmo conjunto de serviços do portal dos demais DF-e (consulta pública, validador XML, visualizador de schema,
  disponibilidade, download XML, CCC, tabelas de classificação tributária). MOC: `moc.sped.fazenda.pr.gov.br/NFCom`.
- **BP-e (modelo 63):** Bilhete de Passagem Eletrônico. Mesmo conjunto de serviços. MOC: `moc.sped.fazenda.pr.gov.br/BPe`.
  A página não trata de BP-e TM nem de outras variantes.
- Nenhuma das duas páginas lista eventos, URLs de webservice, obrigatoriedade ou contingência. Isso vem do MOC e de
  `/NFCOM/Servicos`, `/BPE/Servicos`, que **ainda não foram lidos**.

## Notas técnicas em curso (afetam os dois)

| NT | Situação |
|---|---|
| 2026.002 | Em produção na SVRS desde 31/08/2026 |
| 2026.004 v1.01 (ValorLiquidoProduto) | Homologação SVRS; produção adiada de 16/11 para 14/12/2026 |
| 2026.005 v1.00 (Simples Nacional) | Schemas e regras de NFCom e BP-e passam a valer em 14/12/2026 |

A homologação da SVRS aceita valores de 2026 e simulações de 2027 com IBS/CBS no total. Implementar já com o schema da
NT 2026.005, não com o anterior.

## Serviços e regras (portal `/NFCOM/Servicos`, `/BPE/Servicos` e MOC, lidos em 2026-11-15)

Decisão do usuário: implementar NFCom e BP-e, nessa ordem.

**NFCom (modelo 62), serviços v1.00, SOAP síncrono**
- Autorizadores: SVRS, MG, MS e MT. Serviços: `NFComRecepcao`, `NFComConsulta`, `NFComRecepcaoEvento`,
  `NFComStatusServico`. Não há inutilização listada.
- SVRS: `https://nfcom.svrs.rs.gov.br/WS/<Servico>/<Servico>.asmx` (homologação `nfcom-homologacao.svrs.rs.gov.br`).
  A tabela por UF de origem do emitente ainda precisa ser levantada.
- Eventos: cancelamento 110111; vinculação de pagamento 110300 e seu cancelamento 110301. Os de fisco (substituição,
  ajuste, cofaturamento, liberação de prazo 240170, Simples 412140/412141) são gerados pela autoridade, não pelo emitente.
- Chave de 44 posições: cUF, AAMM, CNPJ, mod, série, nNF, tpEmis, nSiteAutoriz, cNFCom (7), DV módulo 11.
  CNPJ alfanumérico previsto (os 12 primeiros caracteres).
- Assinatura: XMLDSig enveloped sobre `infNFCom`, RSA-SHA1, certificado ICP-Brasil A1/A3 com CNPJ.
- Documento auxiliar: DANFE-COM (Anexo II do MOC). Contingência: página própria do MOC, ainda não lida; a emissão
  off-line usa QR Code assinado.

**BP-e (modelo 63), serviços v1.00, SOAP síncrono**
- Autorizadores: SVRS, MG, MS, MT, PR e SP. Serviços: `BPeRecepcao`, `BPeRecepcaoEvento`, `BPeConsulta`,
  `BPeStatusServico`, mais `BPeRecepcaoTM` (transporte metropolitano) e `BPeRecepcaoTA` (transporte aéreo) na SVRS.
- SVRS: `https://bpe.svrs.rs.gov.br/ws/bpe<Servico>/bpe<Servico>.asmx` (homologação `bpe-homologacao.svrs.rs.gov.br`).
  A página tem URLs de homologação suspeitas em MG e MS; conferir o WSDL antes de usar.
- Eventos (cancelamento, não embarque, alteração de poltrona): detalhes nas páginas de eventos do MOC, **não lidas**.
- Variantes: BP-e comum, TM (NT 2020.002, leiaute próprio, modelo 63) e TA (NT 2025.002, raiz `BPeTA`). Decisão do
  usuário: a v1 cobre as três.

Specs por documento: `2026-11-15-nfcom-design.md` e `2026-11-15-bpe-design.md`.
- Assinatura igual à do CT-e (XMLDSig enveloped, RSA-SHA1). Documento auxiliar: DABPE (Anexo II do MOC).

**Pendências de leitura antes do plano:** páginas de contingência e de eventos do MOC dos dois documentos; mapa
UF do emitente para autorizador; leiaute (XSD) da NT 2026.005 e prazos de cancelamento.

## Avaliação

| Documento | Público provável | Reuso | Prioridade |
|---|---|---|---|
| NFCom | Provedores de internet e telecom (muitos pequenos) | Alto: fatura mensal recorrente, parecido com NFS-e/NF-e em volume | 1 |
| BP-e | Transporte de passageiros | Alto: mesma família do CT-e e do MDF-e | 2 |
| NF3e | Distribuidoras de energia | Médio; público pequeno | 3, sob demanda |
| DC-e | E-commerce e marketplace | Médio | 3, sob demanda |
| NFAg, NFGas, NF-e ABI | Água, gás | Baixo; nichos | 4, sob demanda |

Prazos de IBS/CBS (terceiros, conferir no Ato Conjunto RFB/CGIBS nº 4): NF3e e DC-e desde 03/08/2026; NFCom desde
01/10/2026; NFGas, NFAg e NF-e ABI a partir de 01/12/2026.

## Custo por documento

Cada documento repete o caminho de NF-e/CT-e/MDF-e: pacote `go-dfe` (XML, assinatura, SOAP mTLS pela
`go-dfe-egress`), worker, serviço e rotas da API, tabelas no CDK, UI (emissão, listagem, detalhe), guia com capturas,
`DFE_DOCUMENTS` na landing e na home, valor novo de `document_type` nos webhooks, catálogo de erasure e documentação.
É trabalho de uma feature grande por documento; não cabe num PR só.

## Recomendação

1. NFCom primeiro; 2. BP-e depois. Os demais só com cliente pedindo.
2. Antes de planejar, ler `/NFCOM/Servicos`, `/BPE/Servicos` e os MOCs para fechar eventos, contingência e URLs.
3. Cada documento tem sua própria spec e plano em `docs/specs/` e `docs/plans/`.

## Decisões abertas (do usuário)

- Confirmar a ordem NFCom, BP-e.
- Se há cliente real para algum documento de nicho antes de planejar.
- Se NFF, ONE, CFF, PES e DIFAL merecem avaliação separada.
