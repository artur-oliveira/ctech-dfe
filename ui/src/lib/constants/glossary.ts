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
  nsu: {
    label: 'NSU',
    definition:
      'Número Sequencial Único. Identificador que a SEFAZ dá a cada documento na distribuição — usado para localizar notas emitidas contra o seu CNPJ.',
  },
} as const

export type GlossaryKey = keyof typeof GLOSSARY
