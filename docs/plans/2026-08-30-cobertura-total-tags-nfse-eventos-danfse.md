# Cobertura total da NFS-e Nacional — DPS, eventos orientados e DANFSe

**Data:** 2026-08-30  
**Escopo normativo:** Sistema Nacional NFS-e, leiaute DPS/NFS-e `1.01`, eventos `1.01` e DANFSe NT 008 v1.02.  
**Objetivo:** atingir cobertura verificável de 100% das tags emitíveis da DPS, transformar os eventos já suportados em
ações específicas e seguras, substituir o proxy descontinuado do ADN por geração própria do DANFSe e padronizar o
download de DANFSe e XMLs por URL pré-assinada do S3, sem streaming de arquivo pela API.

Este plano estende a régua usada em
[`2026-08-26-cobertura-total-tags-nfe-mdfe.md`](./2026-08-26-cobertura-total-tags-nfe-mdfe.md) e a execução detalhada de
[`2026-08-27-cobertura-total-tags-implementacao.md`](./2026-08-27-cobertura-total-tags-implementacao.md): derivar
primeiro, reusar cadastro depois, pedir dado por emissão apenas quando o valor realmente nasce naquela nota e nunca
oferecer texto livre para domínio fechado.

---

## 1. Fontes de verdade e limite da afirmação “100%”

### Fontes locais auditadas

- `tmp/nfse/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01/DPS_v1.01.xsd`;
- `tmp/nfse/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01/tiposComplexos_v1.01.xsd`;
- `tmp/nfse/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01/tiposEventos_v1.01.xsd`;
- `tmp/nfse/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01/tiposSimples_v1.01.xsd`;
- `tmp/nfse/anexo_i-sefin_adn-dps_nfse-snnfse-v1-01-20260209.xlsx`, em especial as abas **LEIAUTE DPS_NFS-e** e **RN
  DPS_NFS-e**;
- `tmp/nfse/Manual de Contribuintes - Guia de Utilização da API THE - V7.pdf`, para as operações efetivamente publicadas
  pelo autorizador de Teresina;
- `tmp/nfse/nt-008-se-cgnfse-danfse-20260714-v1-02.pdf`, fonte do DANFSe v2.0.

### O que entra na cobertura

1. Todas as linhas do bloco `DPS` da planilha oficial (linhas 99–416), incluindo atributos, escolhas (`xs:choice`),
   repetições e a assinatura XML gerada pelo sistema.
2. Todos os caminhos emitíveis pelo contribuinte no pedido de evento, respeitando papel do autor, estado do documento e
   regras municipais.
3. Leitura integral do XML `NFSe` autorizado para detalhe e DANFSe. As tags de `NFSe/infNFSe` calculadas pelo fisco não
   são “campos de emissão”, mas precisam ser preservadas e consumidas sem perda.
4. União de cenários: escolhas mutuamente exclusivas não cabem em um XML único. “100%” é a união dos goldens válidos,
   não um XML artificial que viole o XSD.

### O que não pode ser chamado de 100% neste trabalho

- **ABRASF 2.04:** é outro contrato e não há especificação ABRASF nos arquivos fornecidos. Continua configurável, mas
  bloqueado na emissão da UI. Só poderá receber uma afirmação própria de cobertura total após entrada e versionamento
  dos schemas/WSDLs municipais correspondentes.
- Eventos privativos do fisco (`105104`, `105105`, `205204`, `305101`–`305103`) são recebidos e exibidos; nunca devem
  ser emitidos pelo contribuinte.
- Novos fatos geradores citados pela NT 008, seção 3.a da NT 007, aguardam nota específica de DANFSe e não podem ser
  inventados.

---

## 2. Estado atual verificado

### Já existe e deve ser estendido, não refeito

- `go-dfe/nfse.Document` e `nacional.BuildDPS` já representam praticamente todo o DPS 1.01.
- Identidade NFS-e de organização/pessoa, endereço nacional/exterior e regime tributário já vivem em cadastros.
- `organization_services` já concentra código nacional/municipal, NBS, CNAE, ISS, tributos federais, IBS/CBS básico e
  transparência.
- Emissão, substituição, cancelamento, eventos, distribuição ADN, XML autorizado e XML da DPS já atravessam API →
  outbox/SNS → worker → go-dfe → S3/DynamoDB.
- A UI já possui atalhos **Cancelar** e **Substituir**, duplicação segura, timeline e o seletor genérico de evento.
- `api/internal/services/documents` já é o renderizador genérico usado por DANFE, DANFC-e e DAMDFE, com HTML/Gonja,
  Folio, QR Code, cache S3 e testes de PDF. O DANFSe deve entrar aqui.
- DANFE, DANFC-e e DAMDFE já devolvem o contrato JSON de URL pré-assinada; ele deve ser generalizado para DANFSe e
  XMLs, sem manter dois padrões de download na UI.

### Lacunas confirmadas

1. **DANFSe indisponível por desenho atual.** A NT 008 v1.02 suspendeu a API de geração do ADN em **03/08/2026**.
   `NfseService.GetDANFSE` ainda faz proxy síncrono dessa API.
2. **Cobertura do modelo não é cobertura do produto.** O modelo neutro tem os grupos, mas API/cadastros/UI só alimentam
   o caminho comum. A documentação atual declara como ausentes descontos/deduções e IBS/CBS detalhado.
3. **Única lacuna estrutural explícita no modelo neutro:** `IBSCBS/valores/gReeRepRes` e seus documentos.
4. `locPrest` é sempre forçado para `c_loc_emi`; não há prestação em outro município ou país.
5. `comExt`, `obra`, `atvEvento`, a maior parte de `infoCompl`, descontos, deduções, documentos de dedução e opções
   avançadas de IBS/CBS não são alimentados pela API.
6. O DTO do serviço possui `tp_imunidade` e `c_pais_resultado`, mas `buildValores` não os copia. Valores federais
   calculáveis (`vBCPisCofins`, `vPis`, `vCofins`) também não são derivados.
7. O modal genérico sempre mostra descrição, mesmo em eventos vazios, e não contextualiza o papel do autor.
8. `205208` aparece no seletor, mas a UI não coleta/envia `cpf_ag_trib` e `id_ev_manif_rej`; a API também não valida o
   conjunto antes de enfileirar. O resultado possível é falha assíncrona evitável.
9. A substituição autorizada não deixa uma relação local explícita entre original/substituta suficiente para o watermark
   **SUBSTITUÍDA** e para navegação bidirecional.
10. Eventos recebidos pelo ADN não são unificados com a timeline operacional do documento, e a aba recebida continua sem
    manifestação orientada.
11. Todos os endpoints públicos de XML ainda leem o objeto e fazem streaming pela API. Isso inclui XML autorizado,
    evento, DPS, inutilização e distribuição; NFS-e/DANFSe também mantém o caminho legado de bytes.

---

## 3. Régua de alocação dos dados

| Nível                      | Fonte                                    | Regra                                                  | Exemplos NFS-e                                                                               |
|----------------------------|------------------------------------------|--------------------------------------------------------|----------------------------------------------------------------------------------------------|
| 0 — Derivado               | nenhum campo persistido                  | função de dados já conhecidos                          | `idDPS`, `dhEmi`, valores PIS/COFINS, totais de dedução, próximo `nSeqEvento`, QR Code       |
| 1 — Empresa/pessoa         | `organizations` / `organization_persons` | identidade estável de uma parte                        | prestador, tomador, intermediário, destinatário, endereço, IM, CAEPF, NIF, regime            |
| 2 — Serviço                | `organization_services`                  | classificação fiscal e defaults do serviço             | `cTribNac`, NBS, ISS, federal, IBS/CBS, flags obra/evento/exterior                           |
| 3 — Operação               | `organization_operations.nfse`           | cenário recorrente de contratação/faturamento          | local, comércio exterior, ente governamental, finalidade, defaults de desconto e referências |
| 4 — Local reutilizável     | novo `organization_service_locations`    | obra, imóvel ou local de evento com identidade própria | CIB, código de obra, inscrição imobiliária, endereço, identificador do evento                |
| 5 — Documento referenciado | novo `organization_reference_documents`  | documento externo que alimenta dedução/reembolso       | NFS-e municipal, NF/NFS em papel, documento fiscal ou não fiscal, fornecedor                 |
| 6 — Request                | `NfseEmitBody`                           | fato exclusivo daquela emissão                         | competência, valor, desconto efetivo, documento selecionado, data do evento, pedido          |

Regras obrigatórias:

- O request aponta para IDs de cadastro e só carrega overrides explícitos.
- Um valor monetário derivável nunca vira input. O usuário informa a base/fato; o sistema calcula e mostra a memória.
- Campos fechados saem de constantes geradas do XSD/planilha (`OptionsSelect` até cerca de 12 opções; `Combobox` acima).
- Identificadores abertos (`nDI`, processo, pedido, código municipal sem tabela publicada) continuam texto, com máscara,
  tamanho e regra condicional do XSD.
- `organization_operations` deve aceitar `nfse` em `doc_types` e ganhar um subobjeto `nfse`; não criar uma segunda
  tabela de “operações de NFS-e”.
- Endereço não será duplicado em cada grupo: `organization_service_locations` mantém um shape único e o builder adapta
  para `EnderecoSimples`/`Endereco` conforme o destino do XSD.

---

## 4. Mapa de cobertura por grupo da DPS

| Grupo                                               | Estado                    | Fonte final                     | Trabalho principal                                                                         |
|-----------------------------------------------------|---------------------------|---------------------------------|--------------------------------------------------------------------------------------------|
| `infDPS` básico, `subst`, `prest`, `toma`, `interm` | parcial/alto              | derivado + pessoas              | consolidar validações condicionais, motivo textual da substituição e papéis                |
| `serv/locPrest`                                     | parcial                   | operação → config               | suportar município diferente e país, sem campo solto                                       |
| `serv/cServ`                                        | alto                      | serviço                         | manter catálogo como fonte; validar compatibilidade nacional/municipal/NBS                 |
| `serv/comExt`                                       | modelo pronto, sem wiring | operação + emissão              | opções fixas, moeda tabelada, dados DI/RE e valores da nota                                |
| `serv/obra`                                         | modelo pronto, sem wiring | local reutilizável              | selecionar obra/CIB/endereço; escolha exclusiva validada                                   |
| `serv/atvEvento`                                    | modelo pronto, sem wiring | local reutilizável + emissão    | atividade, período e identificador/endereço                                                |
| `serv/infoCompl`                                    | só `xInfComp`             | operação + emissão              | doc técnico, referência, pedido e itens do pedido                                          |
| `valores/vServPrest`                                | só `vServ`                | serviço + emissão               | `vReceb` e regras de consistência                                                          |
| `vDescCondIncond`                                   | ausente                   | operação + emissão              | desconto condicionado/incondicionado e memória de cálculo                                  |
| `vDedRed` + documentos                              | ausente                   | referências + emissão           | percentual/valor, 6 escolhas documentais, fornecedor e rateio                              |
| `trib/tribMun`                                      | parcial                   | serviço + parâmetros municipais | imunidade, país de resultado, suspensão, benefício, retenção e alíquota                    |
| `trib/tribFed`                                      | parcial                   | serviço + derivado              | base/alíquotas/valores PIS-COFINS e retenções                                              |
| `trib/totTrib`                                      | parcial                   | serviço/config + derivado       | três escolhas válidas: valores, percentuais, indicador/Simples                             |
| `IBSCBS` cabeçalho                                  | parcial                   | serviço + operação + pessoas    | finalidade, consumidor final, operação, referências, ente, destinatário e imóvel           |
| `IBSCBS/valores/gReeRepRes`                         | inexistente               | referências + emissão           | implementar integralmente até 1000 documentos, com limites de produto menores e explícitos |
| `IBSCBS/valores/trib/gIBSCBS`                       | parcial                   | serviço + derivado              | crédito presumido, tributação regular e diferimento                                        |
| `Signature`                                         | pronto                    | assinatura                      | manter geração, acrescentar validação XSD/golden em todas as variantes                     |

### Regras de cálculo que não podem ser delegadas à UI

- base do ISSQN depois de desconto/dedução/benefício;
- `vCalcDR`, `vCalcBM`, total dedutível e valor líquido estimado da DPS;
- `vBCPisCofins`, `vPis`, `vCofins` a partir de base e alíquotas;
- totais de documentos de dedução e de `gReeRepRes`;
- coerência entre `pDR` e `vDR`, escolhas de benefício e seus resultados;
- vínculos `tpOper` ↔ `gRefNFSe`, `indDest` ↔ `dest`, operação governamental ↔ `tpEnteGov`;
- exigências por CST/`cClassTrib` para crédito presumido, tributação regular e diferimento;
- limites de data, percentual, precisão e cardinalidade exatamente como XSD/RN.

---

## 5. Eventos: ações específicas, não um formulário fiscal genérico

### Catálogo operacional

| Código   | Ação na UI                                     | Disponibilidade                                                 | Entrada do usuário                                            |
|----------|------------------------------------------------|-----------------------------------------------------------------|---------------------------------------------------------------|
| `101101` | **Cancelar NFS-e**                             | autorizada, autor permitido, sem cancelamento concluído         | motivo fixo `1/2/9` + descrição                               |
| `101103` | **Solicitar análise fiscal para cancelamento** | autorizada e operação/município elegível                        | motivo fixo `1/2/9` + descrição                               |
| `105102` | **Substituir NFS-e**                           | autorizada, com snapshot de emissão completo                    | nova DPS + motivo fixo `01–05/99`; evento é gerado pelo fisco |
| `202201` | **Confirmar como prestador**                   | organização é prestadora da nota                                | confirmação simples                                           |
| `203202` | **Confirmar como tomador**                     | organização é tomadora                                          | confirmação simples                                           |
| `204203` | **Confirmar como intermediário**               | organização é intermediária                                     | confirmação simples                                           |
| `202205` | **Rejeitar como prestador**                    | organização é prestadora e não confirmou/rejeitou em definitivo | motivo fixo `1–5/9`; descrição só quando aplicável            |
| `203206` | **Rejeitar como tomador**                      | organização é tomadora                                          | motivo fixo `1–5/9`; descrição só quando aplicável            |
| `204207` | **Rejeitar como intermediário**                | organização é intermediária                                     | motivo fixo `1–5/9`; descrição só quando aplicável            |
| `205208` | **Anular rejeição**                            | **não expor ao tenant comum até fechar a autoridade normativa** | XSD exige CPF do agente tributário, ID da rejeição e motivo   |

`205208` está serializado no backend, mas o próprio XSD atribui o CPF a um agente da administração tributária. Antes de
mantê-lo em `ContribuinteEvents`, a implementação deve confirmar a autoria na documentação oficial. Até essa decisão:

- removê-lo do seletor comum para não oferecer uma ação inexequível;
- aceitar e exibir o evento quando recebido do ADN;
- nunca pedir ao usuário que digite CPF de agente tributário;
- se a autoridade confirmar emissão pelo contribuinte, derivar `idEvManifRej` de uma rejeição selecionada da timeline e
  obter os demais dados de fonte autenticada, não por texto livre.

### Regras de disponibilidade no servidor

Adicionar uma única função `AvailableNfseActions(document, events, actor)` usada pela API e espelhada pela UI apenas
para apresentação. Ela deve considerar:

- status e chave de acesso;
- documento fiscal da organização comparado a prestador/tomador/intermediário no payload/XML;
- eventos anteriores, inclusive os recebidos pelo ADN;
- operações publicadas pelo autorizador municipal (`ResolveOperation`);
- ação em voo (`pending`/`processing`) para impedir duplo clique e sequências concorrentes;
- prazo/regra municipal: quando não houver parâmetro confiável, mostrar a ação e deixar o fisco decidir, sem inventar
  contador.

A API deve devolver `available_actions` no detalhe e na distribuição. O frontend não monta permissões fiscais sozinho. O
endpoint genérico `POST /events` permanece para integrações, mas a UI usa funções específicas/typed payloads e não
mostra mais “Registrar evento” como ação principal.

### UX das ações

- **Cancelar**, **Substituir** e **Solicitar análise** ficam em “Mais ações”; cancelamento usa tom destrutivo.
- **Confirmar** e **Rejeitar** formam um par contextual no detalhe de uma NFS-e recebida; nunca mostrar as três
  variantes de papel ao mesmo usuário.
- Confirmação abre uma confirmação curta, sem textarea vazia.
- Rejeição e cancelamento usam selects de motivo e revelam descrição somente quando o leiaute/regra exigir.
- `nSeqEvento` é calculado no servidor; não aparece como input.
- Cada ação tem estado carregando, erro RFC 7807 ao lado do controle, foco devolvido ao gatilho e alvo de toque de 44 px
  em mobile.
- A timeline mostra autor/papel, origem (enviado/ADN), sequência, estado e XML. Eventos privativos do fisco aparecem
  como informação, nunca como ação repetível.

---

## 6. DANFSe v2.0 — geração própria obrigatória

### Decisão arquitetural

Implementar em `api/internal/services/documents`, ao lado de DANFE/DANFC-e/DAMDFE. Não colocar rendering em `go-dfe`
nem criar uma segunda stack PDF. O endpoint público `GET /v1.0/nfses/{id}/danfse` passa a devolver o contrato JSON
compartilhado de download; em cache miss, a API lê o XML autorizado do S3, renderiza e armazena o PDF e, em seguida,
devolve somente sua URL pré-assinada. Em cache hit, não há `GetObject`: apenas resolução da chave e assinatura.

### Extensões do renderer compartilhado

- novo `DocTypeNFSe`, template `danfse_v2.html` e builder de contexto `nfse.go`;
- validação de chave por tipo (`44` dígitos nos DF-e atuais, `50` na NFS-e), sem condição mágica espalhada;
- estado do auxiliar deixa de ser apenas `canceled bool` e vira enum fechado `active|cancelled|substituted`;
- chave de cache inclui tipo, versão do template e estado para não servir PDF ativo depois de evento;
- QR Code reusa `qrDataURI` com URL fixa da NT 008:
  `https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave={chave}`;
- logomarca oficial é asset local versionado, com origem e hash documentados; nenhum fetch em runtime;
- canhoto é uma opção fixa na configuração NFS-e (`incluir_canhoto_danfse`, default `false`), não layout livre.

### Contrato único de download direto

Generalizar o contrato atual de documentos auxiliares para um `SignedFileDownload` compartilhado por PDF e XML:

```json
{
  "url": "https://...",
  "expires_at": "2026-08-30T18:00:00Z",
  "filename": "nfse-<chave>.xml",
  "content_type": "application/xml",
  "cached": true
}
```

- `url` e `expires_at` são obrigatórios; `filename` e `content_type` tornam o cliente independente do tipo de arquivo.
- `cached` permanece para artefatos gerados, como DANFE/DANFC-e/DAMDFE/DANFSe, e é omitido para XML de origem.
- O TTL reutiliza a constante vigente de documentos auxiliares. Nome e `Content-Disposition` são definidos na
  pré-assinatura, com sanitização contra quebra de cabeçalho; a UI não inventa extensão.
- O cliente nunca envia bucket nem chave S3. Cada serviço resolve a chave persistida depois de validar organização,
  documento e evento; somente então chama o assinador compartilhado.
- A URL assinada não deve aparecer em logs, traces, analytics nem Problem JSON, pois sua query contém credencial
  temporária.
- Leituras internas necessárias à emissão, manifestação, distribuição, composição do MDF-e ou geração de PDF continuam
  usando `GetObject` pelo SDK. A mudança vale para entrega pública ao navegador, evitando uma segunda requisição HTTP
  do servidor à própria URL assinada.

O mesmo contrato passa a atender, sem exceções silenciosas:

1. XML autorizado de NF-e, NFC-e, MDF-e e NFS-e;
2. XML da DPS assinada;
3. XML de eventos de NF-e, NFC-e, MDF-e e NFS-e;
4. XML de inutilizações de NF-e e NFC-e;
5. XMLs da distribuição de NF-e, CT-e, MDF-e e NFS-e;
6. DANFE, DANFC-e, DAMDFE e o novo DANFSe.

Os paths existentes permanecem, mas a resposta deixa de ser o arquivo e passa a ser JSON. Como isso altera o contrato
de `/xml`, API e UI devem ser publicados coordenadamente; OpenAPI, SDKs e changelog precisam marcar a alteração. Não
será mantido fallback de streaming, porque ele perpetuaria o custo e os dois comportamentos que esta refatoração elimina.

### Conteúdo obrigatório

O contexto lê exclusivamente o XML `NFSe` autorizado e deve cobrir os blocos da NT 008 v1.02:

1. identificação, chave de 50 dígitos, número, competência, emissão, DPS, emitente, situação e finalidade;
2. prestador/fornecedor;
3. tomador/adquirente;
4. destinatário;
5. intermediário;
6. serviço, classificação e local;
7. tributação municipal;
8. tributação federal;
9. IBS/CBS;
10. totais e valor líquido;
11. informações complementares, obra/imóvel/evento e totais aproximados;
12. canhoto opcional.

Regras visuais obrigatórias:

- uma única página A4, retrato, margens de 0,15–0,20 cm, borda de 1 pt e divisórias de 0,5 pt;
- tamanhos mínimos e hierarquia da NT; sem reduzir fonte abaixo do mínimo para “fazer caber”;
- supressões permitidas para partes ausentes e expansão controlada de descrição/informações;
- campos ausentes exibem `-` quando a NT assim determina;
- homologação mostra **NFS-e SEM VALIDADE JURÍDICA** no cabeçalho;
- watermark diagonal **CANCELADA** ou **SUBSTITUÍDA**, mínimo de 50 pt;
- texto longo usa truncamento com reticências somente nos campos permitidos;
- nenhuma informação que não exista no XML pode ser impressa.

As fontes Arial/Microsoft Sans Serif exigidas pela NT precisam de decisão de licença antes do merge. O pacote de deploy
deve carregar fontes legalmente distribuíveis que satisfaçam a exigência; não depender das fontes instaladas na máquina
do desenvolvedor. A checagem de fonte incorporada entra no teste do PDF.

### Relação de substituição

Ao autorizar uma DPS com `subst`, o worker/API deve persistir, de forma idempotente:

- na substituta: `substitutes_access_key` e `substitutes_id_dps`;
- na original: `status=cancelled`, `cancel_kind=substitution`, `substituted_by_access_key` e
  `substituted_by_id_dps`;
- na timeline da original: evento `105102` recebido/derivado, sem fingir que o contribuinte o enviou.

Isso define o watermark correto e cria links “Substituiu” / “Substituída por” no detalhe.

---

## 7. Plano de implementação

### Bloco 0 — gates verificáveis antes de ampliar contrato

#### Tarefa 1 — inventário de cobertura gerado do XSD

**Arquivos:** `go-dfe/nfse/tables/gen/`, novo manifesto versionado e testes em `go-dfe/nfse/nacional/`.

- Gerar a lista canônica de caminhos do `TCDPS`, com ocorrência, tipo, choice e origem normativa.
- Manter exclusões explícitas somente para conteúdo gerado pelo assinador.
- Criar goldens por famílias de escolha e computar a união dos caminhos efetivamente emitidos.
- Fazer o teste falhar com lista exata de caminhos ausentes e extras.
- Rodar validação XSD real em todos os goldens por uma ferramenta de CI/teste sobre os schemas locais (por exemplo,
  `scripts/verify_nfse_xsd.py` com `lxml`). Isso **não** habilita XSD em runtime no `go-dfe`: a biblioteca continua
  `CGO_ENABLED=0`, conforme sua restrição arquitetural.

**Pronto quando:** não existir porcentagem manual; a CI provar `missing = 0`, `unexpected = 0`.

#### Tarefa 2 — catálogo gerado de domínios fechados e regras

- Estender o gerador atual para produzir constantes Go e opções TypeScript a partir do XSD/anexo.
- Cobrir comércio exterior, ISSQN, dedução/redução, benefícios, retenções, operação governamental, tipos de documentos
  referenciados e eventos/motivos.
- Adicionar teste de paridade Go ↔ TypeScript e teste contra a fonte versionada.

**Pronto quando:** nenhum enum fiscal da NFS-e for redigitado em componente/serviço.

### Bloco 1 — restaurar DANFSe antes da expansão funcional

#### Tarefa 3 — adaptar o serviço genérico de documentos auxiliares

- Introduzir estado tipado do documento, validação de chave por doc type e retorno de `SignedFileDownload` após o cache.
- Preservar os chamadores de NF-e/NFC-e/MDF-e e migrar o tipo `Download` existente para o contrato genérico sem quebra
  dos campos atuais.
- Injetar o serviço de documentos no `NfseService` pelo app; remover a chamada síncrona ao `ServiceDANFSE`.

#### Tarefa 4 — parser/contexto NFS-e

- Criar `documents/nfse.go` com helpers reutilizados de `xml.go`/`format.go`.
- Ler XML 1.00/1.01 tolerando namespace, mas renderizar conforme NT 008 v1.02.
- Derivar QR, labels, totais, partes, situação e linhas condicionais sem inventar conteúdo.

#### Tarefa 5 — template e assets DANFSe v2.0

- Criar template A4 e macro (s) apenas onde houver repetição real.
- Versionar logomarca/fontes aprovadas; impor limites de asset/HTML do renderer existente.
- Implementar canhoto opcional, homologação e watermarks.

#### Tarefa 6 — testes do DANFSe

- Unitários por bloco e regra condicional.
- Golden visual com XML mínimo, completo, exterior, IBS/CBS, sem partes, cancelado e substituído.
- Parser de PDF confirma uma página, texto obrigatório, URL/QR, fontes incorporadas e watermark.
- Integração S3 confirma cache miss/hit e mudança de chave por estado.
- Regressão prova que NF-e/NFC-e/MDF-e continuam byte a byte ou visualmente estáveis conforme o gate atual.

### Bloco 1B — downloads públicos diretos do S3

#### Tarefa 7 — unificar DANFSe e todos os downloads de XML por URL pré-assinada

**API e armazenamento**

- Extrair do serviço de documentos um assinador reutilizável que receba apenas uma referência interna já autorizada
  (`bucket`, `key`, nome e MIME) e devolva `SignedFileDownload`.
- Migrar os getters/handlers de XML autorizado, DPS, evento, inutilização e distribuição para resolver metadados e
  pré-assinar o `GetObject`, sem baixar bytes para a API.
- Cobrir explicitamente as rotas NF-e, NFC-e, MDF-e, NFS-e e distribuição de CT-e. Não criar uma rota de CT-e emitido,
  pois esse módulo não existe no escopo atual.
- Configurar `response-content-type` e `response-content-disposition` na assinatura; usar nomes determinísticos e
  sanitizados para documento, evento, DPS, inutilização e NSU distribuído.
- Validar tenant e posse do recurso antes de assinar. Serviços de evento que hoje localizam somente por chave/SK devem
  receber `orgPK` e provar vínculo; uma chave conhecida de outro tenant nunca pode produzir URL.
- Manter a auditoria no momento da emissão da URL, sem registrar a URL completa, e reutilizar o TTL constante vigente.
- Revisar IAM do papel assinador e políticas do bucket por prefixo, mantendo Block Public Access; validar CORS apenas se
  algum cliente precisar usar `fetch`, pois navegação direta pelo link não deve ampliar a exposição do bucket.
- Atualizar o schema OpenAPI comum e todas as respostas `/xml`/`/danfse`; manter RFC 7807 para objeto ausente, acesso
  negado, estado inválido e falha de assinatura.

**Frontend**

- Substituir os onze clientes `Promise<Blob>` de documento/evento/DPS/inutilização/distribuição por
  `Promise<SignedFileDownload>` e abrir `download.url` pelo helper remoto compartilhado.
- Generalizar `DownloadPdfButton` para arquivo remoto, remover o ramo legado de `Blob`, `URL.createObjectURL` e os
  downloads manuais duplicados. Upload/importação local de XML não faz parte desta remoção.
- Preservar loading, retry, erros acessíveis e nome do arquivo; expiração é transparente porque cada clique solicita uma
  URL nova.

**Verificação**

- Teste de contrato por família confirma JSON, TTL, MIME, filename e ausência de corpo XML/PDF na resposta da API.
- Integração S3 confirma bucket/key corretos e prova que o handler público não executa `GetObject`.
- Matriz de autorização cobre outro tenant, evento, DPS, inutilização e todas as distribuições; falha de autorização não
  chama o presigner.
- Testes UI confirmam abertura da URL e ausência de `responseType: 'blob'` nos downloads fiscais.
- E2E usa uma URL assinada real de homologação/localstack até expirar e valida headers de download.

**Pronto quando:** nenhum endpoint público fiscal streamar XML/PDF pela API e DANFSe usar exatamente o mesmo fluxo de
URL pré-assinada dos demais auxiliares.

### Bloco 2 — contratos e cadastros reutilizáveis

#### Tarefa 8 — estender `organization_services`

Adicionar subgrupos versionados, sem achatar dezenas de campos:

- `location_defaults`;
- `foreign_trade_defaults`;
- `iss` completo (`tp_imunidade`, `c_pais_resultado`, suspensão e benefício);
- `federal` completo;
- `ibs_cbs` completo (crédito, regular, diferimento e defaults governamentais);
- flags `requires_work`, `requires_event`, `allows_deductions`, `allows_reimbursements`.

Campos calculados não são persistidos. Registros legados continuam legíveis e ganham diagnóstico de completude por
cenário, sem migração destrutiva.

#### Tarefa 9 — NFS-e dentro de `organization_operations`

- Aceitar `nfse` em `doc_types`.
- Adicionar `nfse` com defaults de local, exterior, pedido/documento técnico, descontos, finalidade IBS/CBS, ente
  governamental e mensagens complementares.
- Atualizar resolução de operação para retornar merge `operação → serviço → request`, com ordem documentada.
- O `is_default` global existente só é considerado quando a operação inclui `nfse`; este plano não introduz um segundo
  default implícito nem muda silenciosamente a operação padrão dos demais documentos.

#### Tarefa 10 — cadastro de locais de serviço

Criar `organization_service_locations` pelo recipe de `OrgEntityService`, com tipos combináveis
`work|property|event_venue`, endereço único, `c_obra`, `cib`, `insc_imob_fisc` e `id_atv_evt`.

- Reusar a entidade para `serv/obra`, `serv/atvEvento` e `IBSCBS/imovel`.
- GSI por nome, RBAC, OAuth, OpenAPI, CDK e telas CRUD.
- Picker debounced na emissão; criação rápida abre cadastro em fluxo separado, não modal gigante.
- DynamoDB on-demand, volume baixo por organização, sem TTL; PITR e `RETAIN` seguem a política de ambiente existente.

#### Tarefa 11 — cadastro de documentos referenciados

Criar `organization_reference_documents`, com união tipada:

- DF-e nacional (`tipo_chave_dfe`, `chave_dfe`);
- NFS-e municipal anterior;
- NF/NFS não eletrônica;
- outro documento fiscal;
- documento não fiscal;
- fornecedor por `person_id`, datas e descrição.

O mesmo cadastro alimenta `vDedRed/documentos` e `gReeRepRes/documentos`. Para documentos já existentes no sistema, o
picker aponta para NF-e/NFS-e local e não duplica o XML/dado.

Usar DynamoDB on-demand, GSI por nome/documento, sem TTL porque o vínculo integra a escrituração. O custo esperado é de
poucas dezenas/centenas de itens por organização; lifecycle de XML continua no bucket existente, sem bucket novo.

### Bloco 3 — cobertura dos grupos de serviço e valores

#### Tarefa 12 — local de prestação e partes

- Resolver `cLocPrestacao|cPaisPrestacao` pela operação/config e validar escolha exclusiva.
- Reutilizar pessoa para destinatário IBS/CBS e exigir apenas quando `indDest=1`.
- Fechar regras de NIF/`cNaoNIF`, endereço exterior, contatos e nome por papel.

#### Tarefa 13 — comércio exterior

- Ligar `comExt` completo.
- Moeda e mecanismos são opções fixas; DI/RE permanecem identificadores validados.
- Valor em moeda, movimento temporário e MDIC são por emissão quando variáveis; defaults ficam na operação.

#### Tarefa 14 — obra, imóvel e atividade de evento

- Seletores do cadastro de locais, com overrides somente de período/nome da atividade.
- Validar `cObra|cCIB|end` e `idAtvEvt|end` como escolhas, não campos independentes.
- Mostrar resumo do local na prévia da DPS.

#### Tarefa 15 — informações complementares estruturadas

- Ligar `idDocTec`, `docRef`, `xPed`, até 99 `gItemPed/xItemPed` e `xInfComp`.
- Defaults/interpolação na operação; referências do pedido por emissão.
- Limpar campos vazios antes do snapshot e preservar IDs para duplicação segura.

#### Tarefa 16 — valores, descontos e dedução/redução

- Ligar `vReceb`, descontos condicionado/incondicionado e `pDR|vDR`.
- Selecionar documentos cadastrados, valor aplicado e fornecedor.
- Calcular totais, impedir rateio superior ao documento/serviço e mostrar memória de cálculo.
- Suportar as seis escolhas documentais do XSD em goldens separados.

#### Tarefa 17 — ISSQN, federal e transparência completos

- Corrigir o wiring de `tpImunidade` e `cPaisResult`.
- Implementar suspensão/benefício municipal e consultar parâmetros municipais quando disponíveis.
- Derivar base/valores de PIS/COFINS; manter retenções no serviço/request conforme natureza.
- Implementar as escolhas de `totTrib` sem permitir combinações inválidas.

### Bloco 4 — IBS/CBS completo

#### Tarefa 18 — cabeçalho IBS/CBS

- Ligar `indFinal`, `tpOper`, `gRefNFSe`, `tpEnteGov`, `indDest`, `dest` e `imovel`.
- `finNFSe=0` continua literal derivado enquanto o XSD só admitir esse valor.
- Seletores de NFS-e referenciada usam documentos locais/distribuídos antes de chave manual.

#### Tarefa 19 — `gReeRepRes`

- Adicionar o grupo ao modelo neutro, XML e validações.
- Suportar as três famílias documentais, fornecedor, datas, tipo `tpReeRepRes`, descrição condicional e valor.
- Usar limite de UX/payload seguro (por exemplo 100 itens por request) abaixo do teto XSD de 1000; paginação/lote é fase
  posterior se uso real exigir, sem afirmar suporte operacional a 1000 itens numa tela.

#### Tarefa 20 — tributação IBS/CBS avançada

- Ligar `cCredPres`, `gTribRegular` e `gDif` a perfis do serviço/operação.
- Validar CST ↔ `cClassTrib` ↔ crédito/diferimento contra tabelas oficiais vigentes.
- Não calcular os totalizadores de `NFSe/infNFSe/IBSCBS`: são resposta do fisco; apenas parsear e exibir.

#### Tarefa 21 — golden matrix e gate final da DPS

Cobrir no mínimo:

- CNPJ, CPF e estrangeiro;
- prestação nacional e exterior;
- serviço simples, exterior, obra, evento e informação complementar;
- cada escolha de documento de dedução e reembolso;
- ISS tributado, imune, exportação, não incidência, suspenso e com benefício;
- retenções/federal/transparência em cada choice;
- IBS/CBS comum, destinatário distinto, imóvel, governo, referência, crédito, regular e diferimento;
- substituição e emissão por prestador/tomador/intermediário.

Cada caso passa por unitário de builder, validação XSD, golden e integração API → worker mockado.

### Bloco 5 — API e persistência da emissão expandida

#### Tarefa 22 — contrato tipado e snapshot completo

- Expandir `NfseEmitBody` com subobjetos, nunca dezenas de campos flat.
- IDs de cadastro + overrides; `DisallowUnknownFields` permanece.
- Persistir `emit_input` completo e normalizado para duplicação/substituição.
- OpenAPI/INTEGRATION mostram condicionais e exemplos por cenário.
- Erros permanecem RFC 7807 com `field` em caminho pontilhado/indexado.

#### Tarefa 23 — transação, tamanho e idempotência

- Medir tamanho máximo de item/worker message para documentos repetíveis antes de escrever.
- Se o payload puder ultrapassar limites DynamoDB/SNS, armazenar payload grande no S3 e enviar referência versionada;
  não descobrir o limite em produção.
- Manter número + linha + outbox atômicos e idempotência do worker.

### Bloco 6 — formulário NFS-e simples/avançado

#### Tarefa 24 — reorganizar sem virar “formulário de XSD”

Fluxo rápido visível:

1. tomador;
2. serviço;
3. valor;
4. competência;
5. prévia e emitir.

Seções avançadas condicionais:

- Operação e local;
- Comércio exterior;
- Obra, imóvel ou evento;
- Descontos, deduções e reembolsos;
- Tributação e reforma;
- Referências e informações complementares.

A seção só aparece quando ativada pelo serviço/operação ou pelo checklist “Esta prestação também tem…”. Se houver erro
dentro de seção fechada, abrir, mostrar contador e focar o primeiro campo.

#### Tarefa 25 — componentes e opções reutilizáveis

- Reusar `PersonPicker`, `Combobox`, `OptionsSelect`, `CurrencyInput`, `SectionCard`, `CollapsibleSection`, draft,
  confirmação e preview.
- Criar apenas pickers de local/documento e pequenos editores repetíveis.
- Nenhum código fiscal fechado em `Input`.
- Preview mostra memória de cálculo, partes, local, deduções e total líquido; não tenta simular tags calculadas pelo
  fisco.

#### Tarefa 26 — responsividade, acessibilidade e falhas

- 375 px sem overflow; ações primárias com 44 px; listas repetíveis viram cards no mobile.
- Labels, descrição/erro ligados por ARIA; foco e teclado completos nos pickers.
- Loading skeleton, empty state instrutivo, retry preservando rascunho e `prefers-reduced-motion`.
- `npx eslint src --ext .ts,.tsx` com zero erros e zero warnings.

### Bloco 7 — eventos e lifecycle

#### Tarefa 27 — fechar validação dos eventos no backend

- Tabela única de campos obrigatórios e códigos válidos por evento.
- Validar `205208` antes da fila e corrigir sua classificação normativa.
- Derivar sequência sob controle de concorrência e impedir evento incompatível/duplicado.
- Testar todos os códigos emitíveis e todos os privativos recusados.

#### Tarefa 28 — disponibilidade por papel e atalhos tipados

- Implementar `available_actions`.
- Criar métodos/client payloads específicos para cancelar, análise fiscal, confirmar e rejeitar.
- Remover códigos desses fluxos da UI; componentes recebem uma ação semântica.
- Manter `POST /events` genérico documentado para integradores.

#### Tarefa 29 — manifestações de documentos recebidos

- Resolver contexto a partir do XML distribuído pelo ADN, inclusive `idDPS`, chave e papel da organização.
- Persistir evento na partição correta de `nfse_events` e unir sua timeline ao item recebido.
- Expor Confirmar/Rejeitar na aba **Recebidas via ADN** conforme `available_actions`.
- Eventos privativos recebidos atualizam estado e timeline de forma idempotente.

#### Tarefa 30 — cancelamento/substituição consistentes

- Atualização otimista usa estado transitório, não pula direto para `cancelled` antes da resposta.
- Substituição atualiza as duas notas e invalida caches de lista/detalhe/PDF.
- DANFSe muda de `active` para `cancelled|substituted` sem servir cache antigo.
- WebSocket informa documento e evento com mensagens distintas.

### Bloco 8 — documentação, guia e rollout

#### Tarefa 31 — documentação obrigatória

Atualizar no mesmo conjunto:

- `DOCS.md` (modelo, builders, eventos, lifecycle, documentos auxiliares);
- `INTEGRATION.md` (contratos, migração de Blob para URL assinada e `available_actions`);
- `DynamoDB-Tables.md` (novas entidades e vínculos de substituição);
- `CONDUCT.md` (gate de cobertura XSD, proibição de proxy DANFSe e de streaming público de arquivos fiscais);
- OpenAPI;
- guia NFS-e da UI, com capturas em `ui/public/guide/`.

#### Tarefa 32 — rollout sem quebrar cadastros existentes

1. Deploy aditivo de tabelas/IAM e leitura tolerante de campos ausentes.
2. Backend/modelo completo atrás de feature flag por organização/ambiente.
3. DANFSe próprio liberado primeiro em homologação e comparado visualmente com amostras oficiais.
4. Publicar API e UI coordenadamente para migrar todos os downloads ao contrato assinado; invalidar SDKs antigos e
   monitorar emissão de URLs, `403` por expiração e redução de bytes transferidos pela API.
5. Cadastros e formulário avançado liberados por grupos.
6. Eventos recebidos e atalhos por papel liberados após E2E no autorizador.
7. Remover código do proxy ADN e dos handlers de streaming somente depois de métricas estáveis e confirmação de que não
   há consumidor conhecido do contrato antigo.

Não há backfill obrigatório dos serviços antigos. O sistema calcula “completude para o cenário” e direciona o usuário ao
cadastro quando uma nova emissão exigir dado que o registro legado não possui.

---

## 8. Testes e comandos de verificação

### go-dfe

- unitários de cada struct/choice/validação;
- goldens de DPS e pedido de evento;
- validação XSD 1.01;
- integração em produção restrita para emissão, cancelamento, análise e manifestações permitidas.

Comandos: `cd go-dfe && CGO_ENABLED=0 GOARCH=arm64 go build ./... && go test ./...`, verificador XSD de teste e suíte de
integração fiscal separada com credenciais de homologação.

### API

- unitários de merge cadastro → request → `nfse.Document`, cálculos e regras cruzadas;
- contrato OpenAPI;
- integração DynamoDB/S3/SNS/outbox para novas entidades, emissão grande, eventos e substituição;
- renderer DANFSe unitário + integração cache/PDF;
- contrato assinado e autorização multi-tenant para documento, DPS, evento, inutilização e distribuição;
- prova de que endpoints públicos não chamam `GetObject` nem devolvem bytes do arquivo.

Comando: `cd api && go test ./...`.

### Worker

- emissão completa, upload dos dois XMLs e preservação do payload;
- evento, manifestação ADN, cancelamento e substituição com redelivery;
- regressão de idempotência e publicação WebSocket;
- integração fiscal de emissão e eventos.

Comando: `cd worker && go test ./...`.

### UI

- schemas Zod por grupo/condicional;
- pickers e editores repetíveis;
- emissão simples e todos os cenários avançados;
- atalhos por papel/estado e ausência de ações proibidas;
- DANFSe ativo/cancelado/substituído e todos os XMLs abertos por `SignedFileDownload`;
- regressão que impede `Blob`, object URL e `responseType: 'blob'` no fluxo de download fiscal;
- visual desktop + 375 px, teclado, foco e contraste WCAG AA.

Comandos: `cd ui && npm test` e `npx eslint src --ext .ts,.tsx` (zero erros e zero warnings).

### CDK

- snapshots das novas tabelas/policies;
- revisão de IAM mínima para API/worker e assinatura `GetObject` limitada aos prefixos fiscais necessários;
- confirmação de que bucket continua privado e de que nenhuma policy/CORS libera leitura sem assinatura;
- `cd cdk && npm test` e `cdk synth`.

---

## 9. Critério de pronto do trabalho inteiro

- [ ] Manifesto gerado da DPS 1.01 reporta zero caminhos ausentes e zero inesperados.
- [ ] Cada `xs:choice` tem ao menos um golden válido e a união cobre todas as alternativas emitíveis.
- [ ] Todo campo tem uma fonte declarada: derivado, cadastro, operação ou request.
- [ ] Nenhum domínio fechado depende de texto livre.
- [ ] Emissão comum continua curta; grupos raros aparecem apenas quando relevantes.
- [ ] Eventos possíveis aparecem como ações semânticas, filtradas pelo servidor por papel/estado.
- [ ] Eventos privativos do fisco nunca são emitidos pela UI/API de contribuinte.
- [ ] Não existe evento aceito sincronicamente para falhar depois por campo estrutural ausente.
- [ ] Documentos recebidos pelo ADN podem ser confirmados/rejeitados quando a organização tiver papel aplicável.
- [ ] DANFSe v2.0 é gerado localmente, em uma página, com QR, homologação e watermarks corretos.
- [ ] O endpoint de DANFSe não depende da API suspensa do ADN.
- [ ] DANFSe, XML autorizado, DPS, eventos, inutilizações e distribuições retornam o mesmo contrato de URL pré-assinada.
- [ ] Nenhum endpoint público fiscal faz streaming de XML/PDF ou executa `GetObject` apenas para entregar o arquivo.
- [ ] Toda assinatura valida tenant antes do presigner e nunca registra a URL completa.
- [ ] Duplicação/substituição preservam referências de cadastro e limpam semântica que não pode ser copiada.
- [ ] Testes unitários, contratos e integrações fiscais exigidos passam.
- [ ] ESLint passa com zero erros/warnings; UI validada a 375 px e WCAG AA.
- [ ] DOCS, INTEGRATION, DynamoDB-Tables, CONDUCT, OpenAPI e guia estão atualizados.
- [ ] Impacto cruzado revisado em `go-dfe ↔ api ↔ worker ↔ ui ↔ cdk ↔ py-dfe`.
- [ ] `py-dfe` permanece sem mudança: NFS-e nunca passou por essa Lambda e DANFSe pertence ao renderer local da API.

---

## 10. Ordem recomendada de entrega

1. **P0:** tarefas 1–7 — gate XSD, DANFSe próprio e download direto, pois o proxy oficial já foi suspenso e a API ainda
   transporta todos os XMLs.
2. **P1:** tarefas 8–17 — cadastros e todos os grupos pré-reforma.
3. **P1:** tarefas 27–30 — eventos orientados, manifestações ADN e lifecycle correto.
4. **P2:** tarefas 18–21 — IBS/CBS completo e `gReeRepRes`.
5. **Transversal:** tarefas 22–26 e 31–32 acompanham cada bloco; não ficam para uma “fase de acabamento”.

Cada tarefa deve resultar em um commit Conventional Commit pequeno. Sugestões de famílias:

- `test(nfse): add xsd-backed dps coverage manifest`
- `feat(danfse): generate danfse v2 from authorized nfse xml`
- `refactor(download): return signed urls for fiscal xml and pdf files`
- `feat(nfse): add reusable service locations and references`
- `feat(nfse): complete dps service and value groups`
- `feat(nfse): complete ibs cbs and reimbursement groups`
- `feat(nfse): expose role-aware fiscal actions`
- `docs(nfse): document full dps events and danfse coverage`
