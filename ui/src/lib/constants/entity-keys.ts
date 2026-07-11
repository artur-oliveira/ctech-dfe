export const SK_PREFIX = {
  VEHICLE: 'VEHICLE_',
  PRODUCT: 'PRODUCT_',
  CPF: 'CPF_',
  CNPJ: 'CNPJ_',
  CERTIFICATE: 'CERT_',
} as const

export function extractId(sk: string, prefix: string): string {
  return sk.startsWith(prefix) ? sk.slice(prefix.length) : sk
}
