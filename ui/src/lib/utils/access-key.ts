import { validateCPF, validateCNPJ } from '@/lib/utils/validators'

const ACCESS_KEY_LEN = 44
const ACCESS_KEY_MOD_NFE = '55' // this feature is NF-e-only (mod 65 = NFC-e, out of scope)
const ACCESS_KEY_VALID_TP_EMIS = new Set(['1', '2', '3', '4', '5', '6', '7'])

// IBGE cUF codes — mirrors api/internal/services/shared.go UFCode values.
// Keep both lists in lock-step; there is no shared source to import across
// the Go/TypeScript boundary.
const IBGE_UF_CODES = new Set([
  '12', '27', '13', '16', '29', '23', '53', '32', '52', '21', '31', '50',
  '51', '15', '25', '26', '22', '41', '33', '24', '11', '14', '43', '42',
  '28', '35', '17',
])

export type AccessKeyField = 'length' | 'cUF' | 'AAMM' | 'doc' | 'mod' | 'tpEmis' | 'cDV'

export interface AccessKeyValidation {
  valid: boolean
  error?: AccessKeyField
}

/**
 * NT 2023.002 access-key check digit (cDV): weights 2-9 cycling
 * right-to-left, mod-11. Character value = ASCII code − 48 — a DIFFERENT
 * algorithm from the CNPJ field's own internal check digits (validateCNPJ,
 * ui/src/lib/utils/validators.ts, which uses A=10..Z=35). Mirrors
 * api/internal/validation/access_key.go calcAccessKeyDV — keep in lock-step.
 */
function calcAccessKeyDV(key43: string): number {
  const weights = [2, 3, 4, 5, 6, 7, 8, 9]
  let sum = 0
  for (let i = key43.length - 1, wi = 0; i >= 0; i--, wi++) {
    sum += (key43.charCodeAt(i) - 48) * weights[wi % 8]
  }
  const rem = sum % 11
  return rem < 2 ? 0 : 11 - rem
}

/**
 * Validates an NF-e access key beyond its 44-character length: cUF, AAMM,
 * CNPJ-xor-CPF (with check digit), mod=55, tpEmis, and the final cDV check
 * digit. Mirrors api/internal/validation/access_key.go ValidAccessKey — keep
 * both in lock-step (docs/specs/2026-08-12-manifestacao-importacao-nfe.md §E).
 */
export function validateAccessKey(key: string): AccessKeyValidation {
  const digitsOutsideDoc = key.length === ACCESS_KEY_LEN && /^\d{6}$/.test(key.slice(0, 6)) && /^\d{24}$/.test(key.slice(20))
  if (!digitsOutsideDoc) {
    return { valid: false, error: 'length' }
  }

  const cUF = key.slice(0, 2)
  const mm = parseInt(key.slice(4, 6), 10)
  const doc = key.slice(6, 20)
  const mod = key.slice(20, 22)
  const tpEmis = key[34]
  const cDV = key[43]

  if (!IBGE_UF_CODES.has(cUF)) return { valid: false, error: 'cUF' }
  if (mm < 1 || mm > 12) return { valid: false, error: 'AAMM' }

  const validDoc = doc.startsWith('000') ? validateCPF(doc.slice(3)) : validateCNPJ(doc)
  if (!validDoc) return { valid: false, error: 'doc' }

  if (mod !== ACCESS_KEY_MOD_NFE) return { valid: false, error: 'mod' }
  if (!ACCESS_KEY_VALID_TP_EMIS.has(tpEmis)) return { valid: false, error: 'tpEmis' }
  if (parseInt(cDV, 10) !== calcAccessKeyDV(key.slice(0, 43))) return { valid: false, error: 'cDV' }

  return { valid: true }
}
