/** CPF mask — numeric only, format: 000.000.000-00 */
export function maskCpf(value: string): string {
  const d = value.replace(/\D/g, '').slice(0, 11)
  if (d.length <= 3) return d
  if (d.length <= 6) return `${d.slice(0, 3)}.${d.slice(3)}`
  if (d.length <= 9) return `${d.slice(0, 3)}.${d.slice(3, 6)}.${d.slice(6)}`
  return `${d.slice(0, 3)}.${d.slice(3, 6)}.${d.slice(6, 9)}-${d.slice(9)}`
}

/**
 * CNPJ mask — supports the new alphanumeric format (IN RFB 2229/2024).
 * Format: AA.AAA.AAA/AAAA-DD (A = alphanumeric, D = digit for check digits)
 */
export function maskCnpj(value: string): string {
  const c = value.replace(/[^A-Z0-9]/gi, '').toUpperCase().slice(0, 14)
  if (c.length <= 2) return c
  if (c.length <= 5) return `${c.slice(0, 2)}.${c.slice(2)}`
  if (c.length <= 8) return `${c.slice(0, 2)}.${c.slice(2, 5)}.${c.slice(5)}`
  if (c.length <= 12) return `${c.slice(0, 2)}.${c.slice(2, 5)}.${c.slice(5, 8)}/${c.slice(8)}`
  return `${c.slice(0, 2)}.${c.slice(2, 5)}.${c.slice(5, 8)}/${c.slice(8, 12)}-${c.slice(12)}`
}

/** Combined mask — picks CPF or CNPJ format based on the clean content length. */
export function maskCpfCnpj(value: string): string {
  const clean = value.replace(/[^A-Z0-9]/gi, '').toUpperCase()
  // Up to 11 pure digits → CPF
  if (clean.length <= 11 && /^\d*$/.test(clean)) return maskCpf(clean)
  return maskCnpj(clean)
}

export function maskCep(value: string): string {
  const digits = value.replace(/\D/g, '').slice(0, 8)
  if (digits.length > 5) return `${digits.slice(0, 5)}-${digits.slice(5)}`
  return digits
}

/**
 * Access-key mask — 44 alphanumeric characters (digits + uppercase CNPJ
 * letters, IN RFB 2229/2024) grouped in blocks of 4, space-separated.
 */
export function maskAccessKey(value: string): string {
  const clean = value.replace(/[^A-Z0-9]/gi, '').toUpperCase().slice(0, 44)
  return clean.match(/.{1,4}/g)?.join(' ') ?? clean
}

export function maskPhone(value: string): string {
  const digits = value.replace(/\D/g, '').slice(0, 11)
  if (digits.length <= 10) {
    return digits
      .replace(/(\d{2})(\d)/, '($1) $2')
      .replace(/(\d{4})(\d{1,4})$/, '$1-$2')
  }
  return digits
    .replace(/(\d{2})(\d)/, '($1) $2')
    .replace(/(\d{5})(\d{1,4})$/, '$1-$2')
}
