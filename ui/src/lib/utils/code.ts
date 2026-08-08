/**
 * Código interno de produto/serviço gerado pelo front. É informação técnica que
 * o usuário não precisa inventar: 16 caracteres de um alfabeto Crockford Base32
 * (sem I, L, O, U) dão ~2^80 combinações — colisão desprezível dentro de um
 * owner — e casam com o regex do cadastro (A–Z, 0–9).
 */
const ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ'
const CODE_LENGTH = 16

export function generateEntityCode(): string {
  const bytes = new Uint8Array(CODE_LENGTH)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => ALPHABET[b % ALPHABET.length]).join('')
}
