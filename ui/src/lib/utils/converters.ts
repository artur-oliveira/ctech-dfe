import type {OrganizationOut} from '@/lib/types/api'
import {type EntityFormData, nfseInfoFromApi} from '@/lib/schemas/entity'
import {orgIsPJ, orgTaxId} from "@/lib/utils/document";

export type {EntityFormData as OrganizationFormData}

export function organizationOutToFormData(org: OrganizationOut): EntityFormData {
  const {crt, addresses, state_registrations, contacts, nfse, ...rest} = org.person
  const isPJ = orgIsPJ(org)
  return {
    tipo: isPJ ? 'pj' : 'pf',
    cpf_or_cnpj: orgTaxId(org),
    name: org.name,
    description: org.description ?? '',
    // Organização não tem papéis; o campo existe só na variante 'person'.
    roles: [],
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
      cnae: rest.cnae ?? '',
      isuf_emit: rest.isuf_emit ?? '',
      intermediary_id: rest.intermediary_id ?? '',
      technical_manager_cpf: rest.technical_manager_cpf ?? '',
      // Organização não recebe frete; o grupo existe só pra satisfazer o schema.
      bank: {pix_key: '', bank_code: '', branch_code: '', cnpj_ipef: ''},
      // Organização não é transportadora de si mesma; o grupo existe só para
      // satisfazer o schema compartilhado.
      freight_retention: {v_serv: '', v_bc_ret: '', p_icms_ret: '', cfop: '', c_mun_fg: ''},
    },
  }
}
