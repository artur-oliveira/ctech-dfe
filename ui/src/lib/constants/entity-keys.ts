export const SK_PREFIX = {
  VEHICLE: 'VEHICLE_',
  PRODUCT: 'PRODUCT_',
  CPF: 'CPF_',
  CNPJ: 'CNPJ_',
  /** Pessoa no exterior, sem CPF/CNPJ (dest/idEstrangeiro). */
  FOREIGN: 'IDEST_',
  CERTIFICATE: 'CERT_',
  SERVICE: 'SERVICE_',
  TAX_PROFILE: 'TAXPROFILE_',
  OPERATION: 'OPERATION_',
  PAYMENT_TERM: 'PAYMENTTERM_',
  VEHICLE_SET: 'VEHICLESET_',
} as const

export function extractId(sk: string, prefix: string): string {
  return sk.startsWith(prefix) ? sk.slice(prefix.length) : sk
}
