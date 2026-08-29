/**
 * Meios de pagamento (`pag/detPag/tPag`) e bandeiras de cartão (`card/tBand`).
 * Fonte: Portal Nacional da NF-e — "Tabela de meios de pagamento" (04/03/2026) e
 * "Tabela de bandeiras das Operadoras de cartão" (registros vigentes).
 *
 * O XSD não enumera nenhum dos dois: a validação é contra a tabela publicada, e
 * era por isso que o código vinha divergindo dela em silêncio.
 */

export interface PaymentTableEntry {
  code: string
  description: string
}

export const TPAG_TABLE: readonly PaymentTableEntry[] = [
  {code: "01", description: "Dinheiro"},
  {code: "02", description: "Cheque"},
  {code: "03", description: "Cartão de Crédito"},
  {code: "04", description: "Cartão de Débito"},
  {code: "05", description: "Cartão da Loja (Private Label), Crediário Digital, Outros Crediários"},
  {code: "10", description: "Vale Alimentação"},
  {code: "11", description: "Vale Refeição"},
  {code: "12", description: "Vale Presente"},
  {code: "13", description: "Vale Combustível"},
  {code: "14", description: "Duplicata Mercantil"},
  {code: "15", description: "Boleto Bancário"},
  {code: "16", description: "Depósito Bancário"},
  {code: "17", description: "Pagamento Instantâneo (PIX) - Dinâmico"},
  {code: "18", description: "TED (Transferência Eletrônica Disponível)"},
  {code: "19", description: "Programa de fidelidade, Cashback, Crédito Virtual"},
  {code: "20", description: "Pagamento Instantâneo (PIX) - Estático"},
  {code: "21", description: "Crédito em Loja"},
  {code: "22", description: "Pagamento Eletrônico não Informado - falha de hardware do sistema emissor"},
  {code: "23", description: "Pagamento Instantâneo (PIX) - Automático"},
  {code: "24", description: "TEF - \"Book Transfer\""},
  {code: "90", description: "Sem Pagamento"},
  {code: "91", description: "Pagamento Posterior"},
  {code: "99", description: "Outros"},
]

export const TBAND_TABLE: readonly PaymentTableEntry[] = [
  {code: "01", description: "Visa"},
  {code: "02", description: "Mastercard"},
  {code: "03", description: "American Express"},
  {code: "04", description: "Sorocred"},
  {code: "05", description: "Diners Club"},
  {code: "06", description: "Elo"},
  {code: "07", description: "Hipercard"},
  {code: "08", description: "Aura"},
  {code: "09", description: "Cabal"},
  {code: "10", description: "Alelo"},
  {code: "11", description: "Banes Card"},
  {code: "12", description: "CalCard"},
  {code: "13", description: "Credz"},
  {code: "14", description: "Discover"},
  {code: "15", description: "GoodCard"},
  {code: "16", description: "GreenCard"},
  {code: "17", description: "Hiper"},
  {code: "18", description: "JcB"},
  {code: "19", description: "Mais"},
  {code: "20", description: "MaxVan"},
  {code: "21", description: "Policard"},
  {code: "22", description: "RedeCompras"},
  {code: "23", description: "Sodexo"},
  {code: "24", description: "ValeCard"},
  {code: "25", description: "Verocheque"},
  {code: "26", description: "VR"},
  {code: "27", description: "Ticket"},
  {code: "99", description: "Outros"},
]

export const TPAG_LABELS: Record<string, string> =
  Object.fromEntries(TPAG_TABLE.map((e) => [e.code, e.description]))

export const TBAND_OPTIONS = TBAND_TABLE.map((e) => ({value: e.code, label: `${e.code} – ${e.description}`}))

/**
 * Meios de pagamento que são PIX. 17 é o PIX comum, 20 o estático e 23 o
 * automático — 12 e 13 são Vale Presente e Vale Combustível, e tratá-los como
 * PIX abria campos de transação no lugar errado.
 */
export const PIX_PAYMENT_TYPES: ReadonlySet<string> = new Set(['17', '20', '23'])

/** Meios que carregam dados de transação (`card`): cartões, vales e PIX. */
export const CARD_PAYMENT_TYPES: ReadonlySet<string> =
  new Set(['03', '04', '05', '10', '11', '12', '13', '17', '20', '21', '23', '24'])

export const isPixPaymentType = (tPag: string): boolean => PIX_PAYMENT_TYPES.has(tPag)
