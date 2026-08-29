/**
 * A platform company id: a UUID in canonical 8-4-4-4-12 hex.
 *
 * Mirrors `repositories.IsCompanyKey` in the API, and the two must agree — a
 * browser that reshapes a key the server validates writes to a partition
 * nobody else addresses.
 *
 * Case-insensitive here, unlike the API, on purpose: the server decides what a
 * valid key IS and refuses uppercase, while this only decides what is NOT a
 * document. Treating an uppercased company id as a CNPJ would be the very bug
 * these helpers caused.
 */
export const isCompanyKey = (pk: string): boolean =>
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(pk)

export const formatCpfCnpj = (pk: string): string => {
  const raw = unformatCpfCnpj(pk);
  if (isCompanyKey(raw)) return raw
  if (raw.length === 11)
    return raw.replace(/(\d{3})(\d{3})(\d{3})(\d{2})/, '$1.$2.$3-$4')
  return raw.replace(/([A-Z0-9]{2})([A-Z0-9]{3})([A-Z0-9]{3})([A-Z0-9]{4})(\d{2})/, '$1.$2.$3/$4-$5')
}

export const unformatCpfCnpj = (pk: string): string => {
  // A company id is not a document. Stripping its hyphens and uppercasing its
  // hex produced a value the API refuses, which is how every screen in the
  // product would have started answering "organização inválida" at once.
  if (isCompanyKey(pk)) return pk
  return pk.replace(/^(CPF_|CNPJ_)/, '').replace(/[^A-Z0-9]/gi, '').toUpperCase();
}

export const docLabel = (pk: string): string => {
  // Empty rather than a guess: labelling a company id "CNPJ" prints a wrong
  // word next to a value that is not one. The caller decides what to render.
  if (isCompanyKey(pk)) return ''
  return pk.startsWith('CPF_') ? 'CPF' : 'CNPJ'
}

/** What an organization's document reader needs. */
type OrgLike = {pk: string; tax_id?: string; tax_id_kind?: 'cnpj' | 'cpf'}

/**
 * An organization's tax id, from the record and never from the key.
 *
 * Mirrors `services.IssuerDoc` in the API, including its fallback: a record
 * that has not been through the company re-key carries no `tax_id`, and its pk
 * is still a legacy CNPJ_/CPF_ key. Both eras answer, which is what lets this
 * ship before the migration and keeps the rollback honest.
 *
 * Returns '' for a company id with no tax id behind it — never the id itself.
 * Returning the key is exactly the bug this replaces.
 */
export const orgTaxId = (org: OrgLike): string => {
  if (org.tax_id) return org.tax_id
  if (isCompanyKey(org.pk)) return ''
  return unformatCpfCnpj(org.pk)
}

/** Whether the organization issues as a legal person. Same fallback as above. */
export const orgIsPJ = (org: OrgLike): boolean => {
  if (org.tax_id_kind) return org.tax_id_kind === 'cnpj'
  return org.pk.startsWith('CNPJ_')
}
