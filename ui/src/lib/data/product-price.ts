import type {ProductOut} from '@/lib/types/api'

/** CNPJ has 14 digits; CPF has 11. */
const CNPJ_DIGITS = 14

/**
 * Resolve the unit price to prefill for an emission item.
 *
 * - CNPJ recipient: use the resale price (`value_resale`), falling back to the
 *   consumer-final price (`value`) when no resale price is set.
 * - CPF recipient (or unknown): use the consumer-final price (`value`).
 *
 * @param product       product carrying both prices.
 * @param recipientDoc  recipient CPF/CNPJ, formatted or not (or '' when unknown).
 */
export function resolveUnitPrice(
  product: Pick<ProductOut, 'value' | 'value_resale'>,
  recipientDoc: string,
): string {
  const digits = (recipientDoc ?? '').replace(/\D/g, '')
  const isCnpj = digits.length === CNPJ_DIGITS
  if (isCnpj && product.value_resale) return product.value_resale
  return product.value
}
