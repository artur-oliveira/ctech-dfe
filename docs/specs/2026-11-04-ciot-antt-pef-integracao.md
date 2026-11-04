# Integração CIOT (ANTT, Pagamento Eletrônico de Frete): especificação

Fonte lida: "Documento de Contrato de Serviço, Pagamento Eletrônico de Frete, Serviços PEF, Versão Piso Mínimo de
Frete", v1.1 de 13/05/2026 (SUTEC/ANTT, 66 páginas), baseado na MP 1.343/2026 e na Resolução ANTT 5.892/2019.
URL: `https://www.gov.br/antt/pt-br/assuntos/cargas/ciot-para-todos-1/dcs_pef_v1-1.pdf`.
Tudo abaixo vem do PDF, exceto onde marcado **[lacuna]** (o PDF não define) ou **[proposta]** (decisão de desenho nossa).

## 1. O que o serviço é

- Webservice **REST com JSON, síncrono**, verbo POST (e GET em `ConsultarExcecao`). Sem SOAP, sem XML, sem assinatura de documento.
- **Autenticação só por certificado:** TLS 1.2 com autenticação mútua; certificado ICP-Brasil A1 ou A3, com CNPJ, `otherName`
  OID 2.16.76.1.3.3 e Extended Key Usage "Autenticação Cliente". O certificado precisa estar **previamente cadastrado** na ANTT.
- **Quem pode chamar (regra C1):** o certificado tem de pertencer a uma **Instituição de Pagamento (IP)** ou ao CPF/CNPJ de um
  **transportador registrado no RNTRC** (ETC, CTC ou TAC). Um embarcador ou contratante que não seja transportador não
  chama direto; usa uma IP (administradora de frete).
- Hosts: homologação `https://appservices-hml.antt.gov.br/pefServices`, produção `https://appservices.antt.gov.br/pefServices`.
  **[lacuna]** O PDF repete que host e contexto de cada serviço devem ser solicitados à ANTT; só se vê um caminho de exemplo
  (`api/ConsultarExcecao?CPFCNPJTransportador=...`). Homologação tem as mesmas regras de segurança e protocolos sem validade.
- Resposta de todos os serviços traz `Codigo`, `Mensagem` e `Protocolo` (16 caracteres). Sucesso: `110` (dados inseridos)
  ou `111` (consulta realizada). Rejeição: código da tabela da seção 5.

## 2. Os 8 serviços

| # | Serviço | Para que serve | Entrada (resumo) | Saída útil |
|---|---|---|---|---|
| 01 | `ConsultarSituacaoTransportador` | RNTRC ativo, tipo (TAC, ETC, CTC) e equiparado a TAC | `CpfCnpjInteressado`, `CpfCnpjTransportador`, `RNTRCTransportador` (8 dígitos completa com zero, 9 mantém) | `RNTRCAtivo`, `TipoTransportador`, `EquiparadoTAC`, `Protocolo` |
| 02 | `ConsultarFrotaTransportador` | Veículo pertence ao transportador? | idem mais `Placas[]` | `Frota[]` com `PlacaVeiculo` e `SituacaoVeiculoFrotaTransportador` |
| 03 | `DeclaracaoOperacaoTransporte` | Declara a operação e gera o CIOT | ver seção 3 | `CodigoIdentificacaoOperacao` (12), `CodigoVerificador` (4), `Protocolo`, `AvisoTransportador` |
| 04 | `CancelamentoOperacaoTransporte` | Cancela operação ainda não realizada | `CodigoIdentificacaoOperacao` (16, com DV), `MotivoCancelamento` (até 500) | `DataCancelamento`, `Protocolo` |
| 05 | `RetificacaoOperacaoTransporte` | Corrige dados declarados | CIOT mais campos retificáveis (seção 4) | `DataRetificacao`, `Protocolo` |
| 06 | `EncerramentoOperacaoTransporte` | Declara a operação cumprida | CIOT mais dados de viagem conforme o tipo | `DataEncerramento`, `Protocolo` |
| 07 | `ConsultarExcecao` (GET) | Transportador está na lista de exceções da Res. 5.862? | `CPFCNPJTransportador` | `Retorno.Flag` |
| 08 | `ConsultarCIOTGerado` | Devolve o CIOT com DV a partir do código de 12 | `CodigoIdentificacaoOperacao` (12), `AnoDeclaracao` opcional | CIOT de 16 |

Observação: o índice do PDF lista a consulta de situação do transportador como serviço 01 com leiaute próprio; a seção
"Serviços" fala em 8 serviços e o sumário numera de 02 a 08, então a numeração do PDF tem uma falha de edição.

## 3. Declaração (serviço 03): campos

`TipoOperacao`: **1** carga lotação, **2** carga fracionada, **3** TAC-Agregado. Datas: `yyyy-MM-dd`; data e hora
`yyyy-MM-ddTHH:mm:ss`. Valores com ponto decimal.

| Campo | Tipo e tamanho | Obrigatório | Regra principal |
|---|---|---|---|
| `IdOperacaoTransporte` | C12 | sim | gerado pelo cliente com a DDL/executável da ANTT; único por ano (B17); formato com ID da administradora e DV (B16) |
| `TipoOperacao` | N1 | sim | 1, 2 ou 3 |
| `CpfCnpjContratado`, `RNTRCContratado` | C11 ou 14, C9 | sim | pertencem ao mesmo transportador com RNTRC ativo (B3, B14); TAC-Agregado exige contratado TAC (B27) |
| `CpfCnpjContratante`, `RNTRCContratante` | C11 ou 14, C9 | contratante sim; RNTRC não | RNTRC, se informado, pertence ao contratante (B71) |
| `CpfCnpjDestinatario` | C11 ou 14 | sim, exceto tipo 3 | proibido no tipo 3 (B62) |
| `ValorFrete` | F10,2 | sim | maior que zero (B120); em lotação, não menor que o **piso mínimo** (B80) |
| `DataDeclaracao` | D19 | sim | tolerância de 15 min antes a 5 min depois da hora de Brasília, fora de contingência (B11); mínimo 168 h atrás (B96) |
| `IndContingencia`, `JustificativaContingencia` | B, C255 | indicador sim | `true` exige justificativa, `false` a proíbe (B103, B104); data da declaração até 30 dias atrás em contingência (B19) |
| `DataInicioViagem`, `DataFimViagem` | D10 | início sim, fim sim (tipo 3 define as duas) | início não menor que hoje (B6) nem maior que hoje mais 30 dias (B22); fim não menor que início (B13); duração máxima 90 dias (B21), 30 no tipo 3 (B30); tipo 3 não informa início (B56) |
| `Veiculos[]` | A | sim | no máximo 5 (B49); sem placa repetida (B5); exatamente 1 automotor (B83, B84); vínculo com o transportador (B15, B20) |
| `Veiculos[].Placa`, `.RNTRCVeiculo`, `.NumeroEixos` | C7, C9, C2 | placa e eixos sim | placa `AAA9999` ou Mercosul `AAA9A99`; eixos: automotor 2 a 4, implemento 1 a 4 (B101) |
| `OrigemDestino[]` | A | tipos 1 e 2 | origem e destino por IBGE, CEP ou lat/long; usar sempre o mais específico (lat/long, depois CEP, depois cidade) |
| `OrigemDestino[].DistanciaPercorrida` | N8 (km) | tipos 1 e 2 | maior que zero; compatível com origem e destino (B82) |
| `DadosCarga` | O | tipos 1 e 2 | natureza N4 (existe na tabela, B9), `PesoCarga` F10,2 maior que zero (B18), tipo de carga N2 (1 a 12, ver PDF) |
| `DadosCarga.ContratantesCargaFrac[]` | CPF ou CNPJ | só tipo 2 | |
| `InfPagamento` | O | sim | `TipoPagamento`: 1 IP, 2 conta corrente, 3 poupança, 4 conta pagamento, 5 outros, 6 Pix; `IndPagamento` 0 à vista, 1 a prazo |
| `InfPagamento` bancário | C | tipos 1 a 4 | instituição N3, agência C10 (tipos 2 a 4), conta C20; proibidos nos demais (B92, B99) |
| `InfPagamento` Pix | C | tipo 6 | `ChavePix` C77 e `IdentificadorPix` C32; proibidos nos demais |
| `InfPagamento` parcelas | N3, D10, F12,2 | se `IndPagamento=1` | `NumeroParcela`, `DataVencimento`, `ValorParcela`; proibidas à vista (B105, B106) |
| `InfPagamento.CpfCnpjCreditado` | C11 ou 14 | sim | recebedor |
| `InfIndicadoresOperacionais` | O | tipo 1 | `IndAltoDesempenho`, `IndRetornoVazio`, `ComposicaoVeicular` (booleanos) |

Regras gerais: B1 (campo obrigatório, 229) e B4 (formato, tamanho e tipo, 205), com CPF e CNPJ validados por dígito
verificador. TAC ou equiparado não pode usar pagamento tipo 5 "Outros" (B100). B42: a administradora pode estar bloqueada
para declarar com certo contratante. Ao retornar `AvisoTransportador` (até 300 caracteres), **mostrar ao transportador e
imprimir no documento correspondente é obrigatório**.

## 4. Cancelamento, retificação e encerramento: prazos

| Operação | Regras |
|---|---|
| Cancelar | até 24 h depois do início da viagem (B35); proibido se a ANTT já fiscalizou a operação (B36); não pode estar cancelada (B37) nem encerrada (B38) |
| Retificar | **proibida em carga lotação** (B121, 320); fracionada até o encerramento; TAC-Agregado até 30 dias depois do fim da viagem (B39, só com status "Declarado"); proibida se já fiscalizada (B40); campos do tipo 2 e 3 por B57, B58, B62, B87 |
| Encerrar | TAC-Agregado informa os dados das viagens (`OrigemDestino`, `DistanciaPercorrida`, `QtdViagens`, B43); tipo padrão não informa (B44); data não anterior ao início (B46); lotação informa `PesoCarga` (B65) |

Regras de convivência do TAC-Agregado: no máximo 2 CIOTs abertos ou vigentes entre o mesmo par contratante e contratado com
os mesmos veículos (B24, B31, B32); novo CIOT bloqueado se houver um pendente de encerramento (B25) ou pendente há mais de
60 dias (B33); o veículo fica com exclusividade por 30 dias ou até o encerramento (B73, 285).

## 5. Rejeições mais prováveis na integração

`203` serviço inativo; `205` campo inválido; `207` transportador não encontrado; `208` início da viagem no passado; `210`
eixos; `211` natureza da carga; `214` declaração com mais de 168 h; `216` RNTRC inativo ou de outro transportador; `217`
placa de outro transportador; `218` administradora sem permissão para o contratante; `219` **CIOT já cadastrado**; `220`
operação não encontrada; `221` prazo de cancelamento excedido; `222` já cancelada; `229` campo obrigatório; `233`
campos do pagamento; `235` e `237` já fiscalizada; `236` prazo de retificação; `251` já encerrada; `268` placa repetida;
`269` hora fora da tolerância; `271` início além de 30 dias; `276` mais de 5 veículos; `285` veículo exclusivo de outro
contratante; `291` **abaixo do piso mínimo de frete**; `292` **distância incompatível**; `293` e `294` automotor; `319`
frete zero; `320` retificação de lotação; `321` consulta de outro transportador. Lista completa B1 a B122 e C1 no PDF.

## 6. O que o PDF não define (resolver antes de implementar)

1. **Host e caminho (contexto) de cada serviço** e se há versionamento. Pedir à ANTT (pef@antt.gov.br, contato do PDF).
2. **Algoritmo do `IdOperacaoTransporte`** e do `CodigoVerificador`: o PDF manda usar uma "DDL/Executável" da ANTT. Como o
   projeto compila com `CGO_ENABLED=0`, é preciso a especificação do cálculo para portar para Go, ou obter o código-fonte.
3. **Tabelas de domínio**: natureza da carga (N4), tabela de piso mínimo de frete (usada pela ANTT nas rejeições 291 e 292) e
   lista de instituições financeiras. Sem elas não dá para pré-validar B9, B80 e B82 localmente.
4. **Cadastro do certificado** na ANTT e perfil de acesso (IP ou transportador), inclusive para homologação.
5. Se o certificado A1 que a organização já guarda para DF-e serve como cliente mTLS (serve se a organização for transportador
   do RNTRC e o certificado tiver o OID 2.16.76.1.3.3).
6. Formato exato do JSON de erro (o PDF mostra só `Codigo` e `Mensagem`).

## 7. Desenho proposto no ctech-dfe

**[proposta]** Segue o padrão do projeto: chamada fiscal sempre pelo `go-dfe-egress`, emissão assíncrona pelo outbox, worker
idempotente, resultado por SNS e WebSocket.

**Camadas**
- `go-dfe/ciot`: cliente REST/JSON com mTLS (reaproveita o gerenciador de certificado e o transporte HTTP do provedor NFS-e
  nacional), tipos de requisição e resposta, normalização (RNTRC de 8 para 9 dígitos, placa) e **validador local** das regras
  que não dependem de base da ANTT: B1, B4 (CPF e CNPJ com DV, placa, datas), B5, B6, B12, B13, B18, B21, B22, B30, B49,
  B56 a B58, B62 a B67, B92, B99, B101, B103 a B106, B120. Entra no `go-dfe/validate` do plano F2.
- `go-dfe-egress`: novo `doc_type` `ciot` com os 8 serviços; o `go-dfe` ganha a entrada correspondente em `Implements`.
- `api`: entidade `ciot` com estados `declared`, `rejected`, `cancelled`, `closed` e `pending`; endpoints `POST /v1.0/ciots`
  (declarar), `POST /v1.0/ciots/{id}/cancel`, `/rectify`, `/close` e `GET` de lista e detalhe com cursor; escrita pelo outbox.
- `worker`: consumidor `ciot` com a mesma estrutura de `handleSefazResponse`.
- Tabela DynamoDB `ciots` (PK da organização, SK do CIOT; GSI por código de 12 dígitos e por chave do MDF-e). Nota de custo:
  on-demand, volume igual ao de MDF-e emitidos por TAC, sem TTL.
- Configuração por organização: ambiente (homologação ou produção) e papel (transportador próprio ou IP); o certificado vem
  do mesmo cadastro de certificados.

**Resiliência (decisão por chamada, regra da família)**

| Chamada | Timeout e retry | Idempotência | Se a ANTT cair |
|---|---|---|---|
| Declaração | 30 s; retry só em falha de transporte | `IdOperacaoTransporte` único por ano: o reenvio devolve `219`, e o worker reconcilia com `ConsultarCIOTGerado` e adota o CIOT (mesmo padrão do 204 da autorização) | **fecha**: sem CIOT, o MDF-e de TAC não emite |
| Cancelar, retificar, encerrar | 30 s; retry em falha de transporte | `222` e `251` confirmam o estado pedido | fila e nova tentativa; prazo de 24 h do cancelamento vira alerta |
| Consultas 01, 02, 07, 08 | 15 s; até 2 retries | naturalmente idempotentes | abre: aviso na tela, validação passa a depender da resposta da declaração |

Cache das consultas 01 e 02 com TTL curto (minutos) para o pré-voo na tela de declaração; sem chamada por tecla digitada.
Sem circuit breaker, pelo mesmo motivo do restante do worker (o egress cria o cliente mTLS por chamada).

**Vínculo com o MDF-e**
- O `infCIOT` do MDF-e leva o `CodigoIdentificacaoOperacao` (12 dígitos) e o CPF ou CNPJ do responsável; hoje o builder usa o CPF
  do primeiro condutor e aceita um só CIOT (ver `docs/specs/2026-11-04-moc-gap-mdfe.md`). Passa a aceitar vários CIOTs, cada
  um com o seu responsável, escolhidos entre os CIOTs `declared` da organização (picker, não texto livre).
- Cancelar o MDF-e oferece cancelar o CIOT (prazo B35); encerrar o MDF-e oferece encerrar o CIOT.
- Regra F113 do MOC (NT 2026.001, produção em 23/11/2026) passa a exigir CIOT no MDF-e de TAC e de `tpEmit` 1 e 3.

**UI e documentação**: tela de CIOT na navegação do MDF-e (`nav.tsx`), formulário com seletores para tipo de operação, tipo de
pagamento e tipo de carga, pickers para contratado, veículos e município, validação de CPF, CNPJ e placa no formulário, capturas
de tela e seção no guia (regra do projeto), fixture no mock.

## 8. Fases e estimativa

Uma pessoa, semanas úteis, sem contar a espera da ANTT.

| Fase | Entrega | Estimativa |
|---|---|---|
| C0 | Respostas da ANTT (seção 6), acesso de homologação, certificado cadastrado | externo |
| C1 | `go-dfe/ciot`: tipos, cliente, normalização, validador local, testes com `httptest` | 1,5 |
| C2 | `go-dfe-egress` `ciot` e `Implements`; teste de contrato | 0,5 |
| C3 | `api`: entidade, repositório, endpoints, outbox, tabela no CDK (com nota de custo) | 1,5 |
| C4 | `worker`: consumidor, reconciliação do `219`, alertas de prazo | 1 |
| C5 | MDF-e: `infCIOT` múltiplo, responsável, regra F113, cancelar e encerrar em cascata | 1 |
| C6 | UI, mock, capturas e guia | 1,5 |
| C7 | Piso mínimo de frete e distância (tabelas da ANTT) como pré-validação | 1, depende do item 3 da seção 6 |

Total de 7 a 8 semanas. Para o prazo de 23/11/2026 do MDF-e, o caminho mínimo é C0, C1, C2 e a parte de `infCIOT` do C5, que
permite informar um CIOT emitido por fora (IP ou sistema do transportador) sem ainda declará-lo por aqui.
