import type {OrganizationOut} from '@/lib/types/api'
import {type EntityFormData, nfseInfoFromApi} from '@/lib/schemas/entity'
import {unformatCpfCnpj} from "@/lib/utils/document";

export type {EntityFormData as OrganizationFormData}

export function organizationOutToFormData(org: OrganizationOut): EntityFormData {
  const {crt, addresses, state_registrations, contacts, nfse, ...rest} = org.person
  const isPJ = org.pk.startsWith('CNPJ_')
  return {
    tipo: isPJ ? 'pj' : 'pf',
    cpf_or_cnpj: unformatCpfCnpj(org.pk),
    name: org.name,
    description: org.description ?? '',
    person: {
      ...rest,
      crt: String(crt) as EntityFormData['person']['crt'],
      addresses: addresses.map((a) => ({
        ...a,
        complement: a.complement ?? '',
        state_federation: a.state_federation as EntityFormData['person']['addresses'][number]['state_federation'],
      })),
      state_registrations: state_registrations.map((r) => ({
        uf: r.uf as EntityFormData['person']['state_registrations'][number]['uf'],
        state_registration: r.state_registration,
      })),
      contacts: contacts ?? {emails: [], phones: []},
      nfse: nfseInfoFromApi(nfse),
    },
  }
}
