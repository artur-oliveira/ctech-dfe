/**
 * Which fields the "dados complementares" section is answerable for.
 *
 * The section used to announce a count taken from every error under `person`,
 * and a good part of `person` is rendered in the card ABOVE it: the CRT, the
 * main address, and an organization's inscrições estaduais. So it would say
 * "1 erro" with nothing wrong inside it — worse than saying nothing, because it
 * sends somebody looking where the error is not.
 *
 * A module of its own so the mapping is testable without driving the form, and
 * so adding a field to the section means adding it here rather than discovering
 * the badge was silently wrong.
 */
const ADVANCED_PERSON_LABELS: Record<string, string> = {
  fantasy_name: 'Nome fantasia',
  state_registrations: 'Inscrições estaduais',
  cnae: 'CNAE',
  isuf_emit: 'Suframa',
  intermediary_id: 'Intermediador',
  technical_manager_cpf: 'Responsável técnico',
  bank: 'Dados bancários',
  freight_retention: 'Retenção do frete',
  nfse: 'NFS-e',
  contacts: 'Contatos',
}

/** The names of the fields with errors inside the section, in reading order. */
export function advancedErrorLabels(
  personErrors: Record<string, unknown> | undefined,
  isOrg: boolean,
): string[] {
  const labels = Object.entries(ADVANCED_PERSON_LABELS)
    .filter(([key]) => {
      // An organization's IE is asked for up front, in the main card.
      if (key === 'state_registrations' && isOrg) return false
      return !!personErrors?.[key]
    })
    .map(([, label]) => label)

  // The main address lives in the card above; only the extras are in here.
  const addresses = personErrors?.addresses
  if (Array.isArray(addresses) && addresses.some((e, i) => i > 0 && e)) {
    labels.push('Endereços adicionais')
  }
  return labels
}

/** "A", "A e B", "A, B e C" — a short list as it is written, not as it is joined. */
export function listPtBR(items: string[]): string {
  if (items.length <= 1) return items[0] ?? ''
  return `${items.slice(0, -1).join(', ')} e ${items[items.length - 1]}`
}
