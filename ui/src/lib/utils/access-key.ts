import { validateCPF, validateCNPJ } from '@/lib/utils/validators';

/** Modelos de documento fiscal na chave (posições 21 e 22); mirror de validation.AccessKeyMod*. */
export const ACCESS_KEY_MOD_NFE = '55';
export const ACCESS_KEY_MOD_CTE = '57';
export const ACCESS_KEY_MOD_MDFE = '58';

/**
 * tpEmis (posição 35) aceito por modelo. NF-e: todos menos 9 (contingência offline da NFC-e).
 * CT-e: 1 normal, 4 EPEC, 5 FS-DA, 7 SVC-RS, 8 SVC-SP. MDF-e: 1 normal, 2 contingência, 3 NFF.
 * Mirror de accessKeyTpEmis em api/internal/validation/access_key.go.
 */
const ACCESS_KEY_TP_EMIS: Record<string, string> = {
  [ACCESS_KEY_MOD_NFE]: '1234567',
  [ACCESS_KEY_MOD_CTE]: '14578',
  [ACCESS_KEY_MOD_MDFE]: '123',
};

// IBGE cUF codes — mirrors api/internal/services/shared.go UFCode values.
// Keep both lists in lock-step; there is no shared source to import across
// the Go/TypeScript boundary.
const IBGE_UF_CODES = new Set([
  '12', '27', '13', '16', '29', '23', '53', '32', '52', '21', '31', '50',
  '51', '15', '25', '26', '22', '41', '33', '24', '11', '14', '43', '42',
  '28', '35', '17',
]);

export type AccessKeyField = 'length' | 'cUF' | 'AAMM' | 'doc' | 'mod' | 'tpEmis' | 'cDV'

export interface AccessKeyValidation {
  valid: boolean
  error?: AccessKeyField
}

/**
 * NT 2023.002 access-key check digit (cDV): weights 2-9 cycling
 * right-to-left, mod-11. Character value = ASCII code − 48, the same value the
 * alphanumeric CNPJ's own check digits use (validateCNPJ). Mirrors
 * api/internal/validation/access_key.go calcAccessKeyDV — keep in lock-step.
 */
function calcAccessKeyDV(key43: string): number {
  const weights = [2, 3, 4, 5, 6, 7, 8, 9];
  let sum = 0;
  for (let i = key43.length - 1, wi = 0; i >= 0; i--, wi++) {
    sum += (key43.charCodeAt(i) - 48) * weights[wi % 8];
  }
  const rem = sum % 11;
  return rem < 2 ? 0 : 11 - rem;
}

/**
 * Validates an access key of one of `models` beyond its 44-character length: cUF, AAMM,
 * CNPJ (numeric or alphanumeric, uppercase) or "000"+CPF with check digit, model, tpEmis of that
 * model and the final cDV. A CNPJ may itself start with "000", so the CNPJ is tried first.
 * Default model is NF-e (55). Mirrors api/internal/validation/access_key.go AccessKeyReason —
 * keep both in lock-step (docs/specs/2026-08-12-manifestacao-importacao-nfe.md §E).
 */
export function validateAccessKey(key: string, models: string[] = [ACCESS_KEY_MOD_NFE]): AccessKeyValidation {
  // 44 caracteres; só o CNPJ (posições 7 a 20) aceita letras maiúsculas, o resto é dígito.
  if (!/^\d{6}[0-9A-Z]{14}\d{24}$/.test(key)) {
    return { valid: false, error: 'length' };
  }

  const cUF = key.slice(0, 2);
  const mm = parseInt(key.slice(4, 6), 10);
  const doc = key.slice(6, 20);
  const mod = key.slice(20, 22);
  const tpEmis = key[34];
  const cDV = key[43];

  if (!IBGE_UF_CODES.has(cUF)) return { valid: false, error: 'cUF' };
  if (mm < 1 || mm > 12) return { valid: false, error: 'AAMM' };

  const validDoc = validateCNPJ(doc) || (doc.startsWith('000') && validateCPF(doc.slice(3)));
  if (!validDoc) return { valid: false, error: 'doc' };

  if (!models.includes(mod)) return { valid: false, error: 'mod' };
  if (!(ACCESS_KEY_TP_EMIS[mod] ?? '').includes(tpEmis)) return { valid: false, error: 'tpEmis' };
  if (parseInt(cDV, 10) !== calcAccessKeyDV(key.slice(0, 43))) return { valid: false, error: 'cDV' };

  return { valid: true };
}
