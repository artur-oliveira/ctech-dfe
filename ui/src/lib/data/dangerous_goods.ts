/**
 * Classificação de produto perigoso do transporte rodoviário. Fonte: Resolução
 * ANTT nº 5.998/2022 (altera a 5.947/2021), Regulamento para o Transporte
 * Rodoviário de Produtos Perigosos, Parte 2 — Classificação.
 *
 * Na NF-e `xClaRisco` é texto livre: a SEFAZ não publica tabela de códigos, e
 * quem define a classificação é a ANTT. O picker existe porque digitar "3" ou
 * "inflamável" onde o MDF-e espera a subclasse é o erro que só aparece no
 * manifesto, dias depois de o produto ter sido cadastrado.
 */

export interface RiskClassEntry {
  code: string
  description: string
  /** Classe-pai que só existe para agrupar; o expedidor informa a subclasse. */
  parentOnly?: boolean
}

export const RISK_CLASSES: readonly RiskClassEntry[] = [
  {code: '1', description: 'Explosivos', parentOnly: true},
  {code: '1.1', description: 'Substâncias e artigos com risco de explosão em massa'},
  {code: '1.2', description: 'Substâncias e artigos com risco de projeção, mas sem risco de explosão em massa'},
  {code: '1.3', description: 'Substâncias e artigos com risco de fogo e pequeno risco de explosão ou projeção, sem risco de explosão em massa'},
  {code: '1.4', description: 'Substâncias e artigos que não apresentam risco significativo'},
  {code: '1.5', description: 'Substâncias muito insensíveis, com risco de explosão em massa'},
  {code: '1.6', description: 'Artigos extremamente insensíveis, sem risco de explosão em massa'},
  {code: '2', description: 'Gases', parentOnly: true},
  {code: '2.1', description: 'Gases inflamáveis'},
  {code: '2.2', description: 'Gases não inflamáveis, não tóxicos (asfixiantes ou oxidantes)'},
  {code: '2.3', description: 'Gases tóxicos'},
  {code: '3', description: 'Líquidos inflamáveis'},
  {code: '4', description: 'Sólidos inflamáveis e substâncias correlatas', parentOnly: true},
  {code: '4.1', description: 'Sólidos inflamáveis, substâncias autorreagentes, explosivos sólidos insensibilizados e substâncias polimerizantes'},
  {code: '4.2', description: 'Substâncias sujeitas a combustão espontânea'},
  {code: '4.3', description: 'Substâncias que, em contato com a água, emitem gases inflamáveis'},
  {code: '5', description: 'Substâncias oxidantes e peróxidos orgânicos', parentOnly: true},
  {code: '5.1', description: 'Substâncias oxidantes'},
  {code: '5.2', description: 'Peróxidos orgânicos'},
  {code: '6', description: 'Substâncias tóxicas e infectantes', parentOnly: true},
  {code: '6.1', description: 'Substâncias tóxicas'},
  {code: '6.2', description: 'Substâncias infectantes'},
  {code: '7', description: 'Material radioativo'},
  {code: '8', description: 'Substâncias corrosivas'},
  {code: '9', description: 'Substâncias e artigos perigosos diversos, incluindo os que apresentam risco ao meio ambiente'},
]

/** Só o que o expedidor pode informar: a classe-pai com subclasse fica de fora. */
export const RISK_CLASS_OPTIONS = RISK_CLASSES
  .filter((c) => !c.parentOnly)
  .map((c) => ({value: c.code, label: `${c.code} - ${c.description}`}))

/**
 * Classes que não recebem grupo de embalagem (Res. ANTT 5.998/2022, Parte 2).
 * Com uma delas, o campo fica vazio — e não é o operador que tem que saber disso.
 */
export const RISK_CLASSES_WITHOUT_PACKING_GROUP: ReadonlySet<string> =
  new Set(['1', '1.1', '1.2', '1.3', '1.4', '1.5', '1.6', '2', '2.1', '2.2', '2.3', '5.2', '6.2', '7'])

/** Grupo de embalagem (Parte 2, item 2.8.2.1): o nível de risco no transporte. */
export const PACKING_GROUP_OPTIONS = [
  {value: 'I', label: 'I - Alto risco'},
  {value: 'II', label: 'II - Risco médio'},
  {value: 'III', label: 'III - Baixo risco'},
]

export function packingGroupApplies(riskClass?: string | null): boolean {
  return !!riskClass && !RISK_CLASSES_WITHOUT_PACKING_GROUP.has(riskClass)
}
