# Redesign do modelo de tributação (perfil fiscal + produto)

Data: 2026-08-09
Status: proposto

## Motivação

Dois bugs reportados na UI de `/tax-profiles` levaram a uma revisão maior do modelo de
tributação de produto (NF-e/NFC-e mercadoria):

1. `cfop_config` do produto era `required,min=1` incondicional no DTO — impedia salvar um
   produto cuja tributação é 100% coberta por `tax_profiles`. **Já corrigido** fora desta spec
   (fix isolado, sem risco de design): `dto.go` passou a usar `required_without` cruzado entre
   `cfop_config`/`tax_profiles`, com testes de regressão e `DOCS.md` atualizado.
2. Perfis fiscais não têm como variar tratamento por UF de destino, e o texto da própria tela
   ("quando o tratamento realmente difere por CFOP, crie um segundo perfil") não cobre o caso de
   variar por UF.

Essas discussões expuseram um escopo maior: o modelo de tributação precisa cobrir mais casos de
uso da legislação brasileira (DIFAL, pauta fiscal, IBS/CBS opcional, PIS/COFINS-ST) e precisa de
testes rigorosos para não regressar silenciosamente em nenhuma combinação de CST/CSOSN.

## Escopo

Dentro:
- Tributação de mercadoria em NF-e/NFC-e: `cfop_config` (produto) e `tax_profile`.
- Eixo de UF de destino como dimensão de configuração.
- DIFAL (partilha ICMS interestadual para consumidor final não contribuinte): **já implementado**
  (`isDifalEligible`/`icmsCSTDifalEligible`/`buildICMSUFDest` em `builders_doc.go`/`builders_tax.go`)
  — só precisa continuar funcionando sob a nova resolução por UF, não é trabalho novo.
- Campos faltantes identificados por gap-analysis contra o XSD oficial da NF-e v4.00.
- Tornar IBS/CBS opcional (grupo tudo-ou-nada), dado que a vigência obrigatória para não-Simples
  (2026-08-03) já passou e a de Simples/MEI (2027-01-04) ainda não chegou — hoje o sistema não
  pode forçar o grupo para quem ainda não está sob a Reforma Tributária.
- Warning de alíquota customizada (ICMS/FCP) divergindo da tabela do sistema.
- Testes automatizados (matriz exaustiva + golden-file) front e back.

Fora:
- CT-e/MDF-e (tributação de serviço de transporte — mecanismo e dados completamente diferentes).
- Imposto de Importação (`II`) — dado operacional por DI/DUIMP, não configuração de cadastro.
- Suspensão/enquadramento fino de IPI (bebidas/cigarros) e detalhamento de retenções de ISSQN além
  do já existente — nicho, documentado como limitação conhecida, não implementado nesta spec.

## Modelo de dados

### 1. Eixo de UF — override parcial

`CfopConfigBody` e `TaxProfileBody` ganham um campo novo:

```go
UfOverrides []UfTaxOverride `json:"uf_overrides" validate:"omitempty,dive"`

type UfTaxOverride struct {
    Ufs       []string       `json:"ufs" validate:"required,min=1,dive,uf"`
    Overrides map[string]any `json:"overrides" validate:"omitempty"`
}
```

`Overrides` é parcial — mesmo mecanismo já usado em `ProductTaxProfileRef.Overrides` (só as
chaves presentes vencem a config base). Não duplica os ~60 campos de `TaxFieldsBody`; só grava o
que diverge para aquele conjunto de UFs. UF é sempre selecionada via picker multi-select (nunca
texto livre).

### 2. DIFAL — já implementado, sem trabalho novo

DIFAL (partilha do ICMS em venda interestadual para consumidor final não contribuinte) **já é
calculado automaticamente hoje**: `emit.go`/`builders_doc.go` detecta elegibilidade
(`isDifalEligible = idDest=="2" && destIE=="" && orgCRT==3`, `icmsCSTDifalEligible[cst]`) e
`builders_tax.go:buildICMSUFDest` monta o grupo a partir de `resolveICMSIntraAliq`/
`resolveICMSInterAliq`/`resolveFCPAliq`. Não é um gap — a única obrigação desta spec é não quebrar
esse cálculo ao introduzir a resolução por níveis/UF (§"Algoritmo de resolução"): os overrides de
`uf_overrides`/perfil devem alimentar as mesmas tabelas antes desse ponto, e um teste de regressão
cobre isso.

### 3. Campos novos em `TaxFieldsBody`

- `icms_pauta_valor` (`*string`, `omitempty,money2`) — valor da pauta fiscal (R$), exigido quando
  `icms_mod_bc` ∈ {Pauta, PMPF}. A base de cálculo do ICMS/ICMS-ST nesses casos vem desse valor
  fixo, não do valor de venda.
- `ibs_cbs_p_dev_trib` (`*string`, `omitempty,percent`) — percentual de "cashback fiscal" (NT
  2025.002).
- Grupo opcional novo **PIS/COFINS-ST**: `pis_st_aliq`, `cofins_st_aliq`, `pis_st_v_bc`,
  `cofins_st_v_bc` (mesmo padrão liga/desliga dos grupos IPI/IS/ISSQN já existentes em
  `TaxGroups`).

### 4. IBS/CBS vira grupo opcional

Hoje `IbsCbsCst`/`IbsCbsClassTrib`/`IbsUfAliq`/`IbsMunAliq`/`CbsAliq` são `required` incondicional
em `TaxFieldsBody`. Passam a `omitempty`, com validação estrutural: se qualquer campo do grupo
IBS/CBS for preenchido, todos os campos obrigatórios do grupo passam a ser exigidos (tudo-ou-nada).
Se nenhum for preenchido, o grupo é omitido inteiro na emissão. `TaxFieldsEditor` ganha
`TaxGroups.ibsCbs` como os demais grupos opcionais (hoje sempre visível).

### 5. Tabela NCM+UF de alíquota ICMS migra para o backend

`ui/src/lib/data/icms_ncm_lookup.ts` (alíquota ICMS/FCP específica por NCM+UF, hoje só usada como
sugestão de autopreenchimento no frontend) migra para `tax_tables.go`. `resolveICMSAliq`/
`resolveFCPAliq` passam a receber o NCM e checar a tabela NCM+UF antes de cair na tabela genérica
por UF. Isso corrige uma divergência real: hoje, se o campo de override ficar vazio, o backend usa
a tabela genérica mesmo para NCMs com alíquota especial conhecida pelo frontend.

### 6. Override de alíquota com warning

`icms_aliq_override`/`fcp_aliq_override` já existem por `cfop_config`/perfil e já fazem fallback
para a tabela do sistema quando vazios — isso não muda. O que muda:

- Novo endpoint leve, `GET /v1.0/tax-tables/icms-aliq?emit_uf=&dest_uf=&ncm=`, expõe o valor que o
  backend resolveria (NCM+UF > UF genérico) sem duplicar a tabela em TypeScript.
- O campo no frontend mostra o valor do sistema como referência (ex.: texto auxiliar "sistema:
  18%"), consultado com debounce quando UF/NCM mudam.
- Se o valor digitado divergir do valor do sistema, um banner de aviso aparece (não bloqueia
  salvar — é aviso, não validação).
- O autopreenchimento automático atual (`ProductForm.tsx:532-549`, que grava a sugestão de NCM
  direto no campo override ao trocar o NCM) é removido. O campo fica vazio por padrão e usa o
  fallback do backend; só grava um valor quando o usuário digita algo explicitamente.

## Algoritmo de resolução (6 níveis)

Ordem de precedência na emissão, da maior para a menor (supersede a lista atual em
`resolveCfopTax`, `nfes/tax_profiles.go`):

1. `cfop_config[cfop]` do produto + `uf_overrides` da UF de destino
2. `cfop_config[cfop]` do produto (sem UF)
3. `ProductTaxProfileRef.Overrides` (vínculo produto→perfil) + `uf_overrides` da UF de destino
4. `ProductTaxProfileRef.Overrides` (vínculo produto→perfil, sem UF)
5. `tax_profile.cfops[cfop]` + `uf_overrides` da UF de destino
6. `tax_profile.cfops[cfop]` (sem UF)
7. Erro: nenhuma camada cobre o CFOP

Dentro de cada nível "+UF", a config base do nível é mesclada com o `uf_overrides` cuja lista
`ufs` contém a UF de destino da operação, usando o mesmo merge raso já usado por
`mergeTaxFields` (chave ausente/nula/`""` não sobrescreve). Continua valendo a regra: a primeira
camada que cobrir o CFOP resolve; não há mistura entre camadas de níveis diferentes.

## Frontend

- `TaxFieldsEditor`: novo grupo opcional PIS/COFINS-ST; `icms_pauta_valor` condicional a
  `icms_mod_bc` ∈ {Pauta, PMPF}; `ibsCbs` migra de sempre-visível para grupo opcional; warning de
  alíquota (ICMS/FCP) descrito acima.
- Nova UI de `uf_overrides`: lista de cards, cada um com picker multi-select de UF e os mesmos
  campos de `TaxFieldsEditor`, todos opcionais (só preenche o que diverge).
- `TaxProfileForm`: texto de ajuda atualizado — hoje diz "crie um segundo perfil" quando o
  tratamento difere; passa a mencionar também a opção de `uf_overrides` quando a diferença é só
  por UF de destino.

## Backend

- `dto.go`: `UfTaxOverride`, campos novos em `TaxFieldsBody`, IBS/CBS `omitempty` + validação
  estrutural tudo-ou-nada via `validator.RegisterStructValidation` (não usado ainda no projeto,
  mas mecanismo nativo do go-playground/validator já importado em `internal/validation`).
- `tax_tables.go`: tabela NCM+UF, `resolveICMSAliq`/`resolveFCPAliq` com parâmetro NCM.
- `nfes/tax_profiles.go`: `resolveCfopTax` estendido para os 6 níveis + merge por UF;
  `mergeTaxFields` reusado sem alteração de assinatura pública.
- Nova rota `GET /v1.0/tax-tables/icms-aliq` (service fino, sem estado, só chama
  `resolveICMSAliq`/`resolveFCPAliq` e retorna o valor).
- DIFAL (`buildICMSUFDest`) não muda — teste de regressão garante que continua correto com a nova
  resolução em níveis.

## Testes

- **Go, matriz exaustiva** (table-driven): CST/CSOSN × grupo opcional (IPI/IS/ISSQN/IBS-CBS/
  PIS-COFINS-ST on/off) × nível de resolução (1 a 7) — valida que campos exigidos aparecem,
  campos irrelevantes ficam ausentes/zerados, e o nível certo vence.
- **Go, golden-file**: conjunto de payloads reais anonimizados por cenário comum (Simples+ST,
  Normal+ICMS-ST, isento, IBS/CBS, DIFAL interestadual) comparando XML/payload gerado esperado.
- **Frontend, Vitest**: exibição condicional (grupo ligado/desligado, CST trocando esconde/mostra
  campo, warning de alíquota aparecendo/desaparecendo), resolução de prioridade nos componentes
  que exibem qual config está ativa.
- Regressão dos dois bugs originais já cobertos pelos testes existentes em `dto_test.go`.

## Documentação

- `DOCS.md`: modelo de dados (`uf_overrides`, campos novos, IBS/CBS opcional), algoritmo de
  resolução em 6 níveis, novo endpoint de tabela de alíquota.
- `CONDUCT.md`: atualizar a regra única de resolução (`resolveCfopTax`) para refletir os 6 níveis;
  documentar a migração da tabela NCM+UF pro backend como fonte única de verdade.

## Limitações conhecidas (fora de escopo)

- `II` (Imposto de Importação): não modelado — é dado de operação (DI/DUIMP), não de cadastro.
- Suspensão/enquadramento fino de IPI (`clEnq`, `cSelo`, `qSelo`) e detalhamento de retenções de
  ISSQN (`vDeducao`, `vDescIncond`, etc. completos): cobertura parcial mantida, nicho.
- CT-e/MDF-e: tributação de serviço de transporte não entra nesta spec.
