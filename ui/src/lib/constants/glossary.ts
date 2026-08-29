/**
 * Plain-language definitions for fiscal acronyms surfaced in the emit wizards.
 * Copy lives here once (DRY) — rendered by <GlossaryTerm> (components/ui/glossary-term.tsx).
 * On brand: translate the jargon into what the user actually needs to know.
 */
export const GLOSSARY = {
  cfop: {
    label: 'CFOP',
    definition:
      'Código Fiscal de Operações e Prestações. Classifica o tipo de operação (venda, devolução, transferência) e determina quais impostos se aplicam.',
  },
  ind_pag: {
    label: 'Forma de pagamento',
    definition: 'Indica se a operação é à vista ou a prazo, e por qual meio o cliente paga (dinheiro, cartão, PIX…).',
  },
  mod_frete: {
    label: 'Modalidade do frete',
    definition:
      'Define quem contrata e paga o transporte: o emitente, o destinatário, um terceiro — ou se não há frete.',
  },
  nat_op: {
    label: 'Natureza da operação',
    definition:
      'Descrição do que a nota representa (ex.: «Venda de mercadoria»). Gerada automaticamente a partir dos CFOPs dos itens.',
  },
  csosn: {
    label: 'CSOSN',
    definition:
      'Código de Situação da Operação no Simples Nacional. Substitui o CST de ICMS para quem é do Simples, e diz se a operação gera crédito, tem substituição tributária ou é isenta.',
  },
  cst_icms: {
    label: 'CST do ICMS',
    definition:
      'Código de Situação Tributária. Diz como o ICMS incide nesta operação: tributado integralmente, com redução de base, isento, diferido, substituído, e assim por diante.',
  },
  mod_bc: {
    label: 'Modalidade da base de cálculo',
    definition:
      'De onde sai o valor sobre o qual o ICMS é calculado: o valor da operação (o normal), uma pauta fixada pela UF, o preço máximo ao consumidor, ou uma margem de valor agregado.',
  },
  mva: {
    label: 'MVA',
    definition:
      'Margem de Valor Agregado. Percentual que a UF presume de lucro na revenda, usado para calcular o ICMS-ST que você recolhe hoje pela operação que o seu cliente fará depois.',
  },
  fcp: {
    label: 'FCP',
    definition:
      'Fundo de Combate à Pobreza. Adicional de ICMS (geralmente 1% a 2%) que algumas UFs cobram sobre produtos específicos, recolhido junto com o imposto.',
  },
  c_benef: {
    label: 'Código de benefício fiscal',
    definition:
      'Identifica, na tabela da sua UF, o incentivo fiscal aplicado ao item. Quando o produto não tem benefício, a UF pode exigir o literal «SEM CBENEF».',
  },
  ext_ipi: {
    label: 'EX TIPI',
    definition:
      'Exceção da tabela do IPI. Alguns NCMs têm alíquotas diferentes conforme a finalidade do produto, e o número da exceção é o que identifica qual delas se aplica.',
  },
  nve: {
    label: 'NVE',
    definition:
      'Nomenclatura de Valor Aduaneiro e Estatística. Detalha o NCM em características específicas do produto; exigida em algumas operações de importação.',
  },
  n_fci: {
    label: 'FCI',
    definition:
      'Ficha de Conteúdo de Importação. Declara quanto de um produto industrializado aqui veio de insumo importado — define se a operação interestadual usa a alíquota de 4%.',
  },
  c_enq: {
    label: 'Enquadramento legal do IPI',
    definition:
      'Código da norma que fundamenta a tributação do IPI no item. Quando não há enquadramento específico, o leiaute usa 999.',
  },
  ad_rem: {
    label: 'Alíquota específica (ad rem)',
    definition:
      'Imposto cobrado por unidade vendida (R$ por litro, por quilo), não por percentual sobre o valor. É como o ICMS monofásico de combustíveis e a monofasia do IBS/CBS funcionam.',
  },
  ibs_cbs: {
    label: 'IBS e CBS',
    definition:
      'Os dois tributos da reforma tributária que substituem ICMS, ISS, PIS e COFINS: o IBS é estadual e municipal, a CBS é federal. Convivem com os antigos durante a transição.',
  },
  c_class_trib: {
    label: 'Classificação tributária',
    definition:
      'Código de 6 dígitos que, junto do CST, identifica exatamente qual regra de IBS/CBS se aplica ao item — é o que determina alíquota, redução e crédito presumido.',
  },
  issqn_exigibilidade: {
    label: 'Exigibilidade do ISS',
    definition:
      'Diz se o ISS é devido nesta operação ou por que não é: não incidência, isenção, exportação, imunidade ou suspensão por decisão judicial ou administrativa.',
  },
  n_recopi: {
    label: 'RECOPI',
    definition:
      'Registro de Controle de Papel Imune. Número obtido no sistema da Receita para circular papel destinado a livros, jornais e periódicos, que são imunes a impostos.',
  },
  nsu: {
    label: 'NSU',
    definition:
      'Número Sequencial Único. Identificador que a SEFAZ dá a cada documento na distribuição — usado para localizar notas emitidas contra o seu CNPJ.',
  },
} as const

export type GlossaryKey = keyof typeof GLOSSARY
