'use client'

import {EntityForm} from '@/components/EntityForm'
import {
  CRT_NONE_VALUE,
  type EntityFormData,
  nfseInfoFromApi,
  nfseInfoToApi,
  PERSON_ROLES,
  type PersonRole,
} from '@/lib/schemas/entity'
import type {PersonCreate, PersonItemOut} from '@/lib/types/api'
import {SK_PREFIX} from '@/lib/constants/entity-keys'
import {unformatCpfCnpj} from "@/lib/utils/document";

// Re-export for any existing code that imports UF_OPTIONS from here
export {UF_OPTIONS} from '@/lib/schemas/entity'

interface PersonFormProps {
  initialData?: PersonItemOut
  onSubmit: (data: PersonCreate) => Promise<void>
  loading?: boolean
  /** Force PF/PJ and hide the toggle (e.g. NFC-e consumer = always PF). */
  lockTipo?: 'pf' | 'pj'
  /** Prefill the document field (used with lockTipo). */
  initialCpfCnpj?: string
  /** Papéis pré-marcados num cadastro novo — o picker que abriu o formulário
   *  já sabe o papel que está procurando. */
  initialRoles?: PersonRole[]
}

function fromPersonOut(p: PersonItemOut): EntityFormData {
  const isPJ = p.sk.startsWith('CNPJ_')
  const crt = p.person.crt != null ? String(p.person.crt) : ''
  const validCrt = ['1', '2', '3', '4'].includes(crt)
  return {
    tipo: isPJ ? 'pj' : 'pf',
    cpf_or_cnpj: p.sk.startsWith(SK_PREFIX.FOREIGN) ? '' : unformatCpfCnpj(p.sk),
    id_estrangeiro: p.sk.startsWith(SK_PREFIX.FOREIGN) ? p.sk.slice(SK_PREFIX.FOREIGN.length) : '',
    name: p.name,
    description: '',
    // Pessoa cadastrada antes dos papéis existirem volta sem `roles` — lista
    // vazia é o estado correto, não um default de 'customer' aplicado por engano.
    roles: (p.roles ?? []).filter((r): r is PersonRole => (PERSON_ROLES as readonly string[]).includes(r)),
    person: {
      fantasy_name: p.person.fantasy_name ?? '',
      // PF with no stored CRT shows "Não especificar"; PJ falls back to Simples Nacional.
      crt: (validCrt ? crt : (isPJ ? '1' : CRT_NONE_VALUE)) as EntityFormData['person']['crt'],
      state_registrations: p.person.state_registrations.map((r) => ({
        uf: r.uf as EntityFormData['person']['state_registrations'][number]['uf'],
        state_registration: r.state_registration,
      })),
      addresses: p.person.addresses.map((a) => ({
        ...a,
        complement: a.complement ?? '',
        state_federation: a.state_federation as EntityFormData['person']['addresses'][number]['state_federation'],
      })),
      contacts: p.person.contacts ?? {emails: [], phones: []},
      nfse: nfseInfoFromApi(p.person.nfse),
    },
  }
}

export function PersonForm({initialData, onSubmit, loading, lockTipo, initialCpfCnpj, initialRoles}: PersonFormProps) {
  const handleSubmit = async (data: EntityFormData) => {
    // Payload shape follows the selected type, not just initialData — this lets a
    // brand-new PF be created correctly (e.g. NFC-e consumer).
    const isPJ = data.tipo === 'pj'
    const addresses = data.person.addresses.map((a) => ({
      ...a,
      postal_code: a.postal_code.replace(/\D/g, ''),
      complement: a.complement || null,
    }))
    const nfse = nfseInfoToApi(data.person.nfse)
    const personPayload = isPJ
      ? {
        fantasy_name: data.person.fantasy_name ?? '',
        crt: parseInt(data.person.crt ?? '3', 10),
        state_registrations: data.person.state_registrations,
        addresses,
        contacts: data.person.contacts,
        nfse,
      }
      : {
        fantasy_name: null,
        // PF CRT is optional: null tells the backend to omit it (defaults on emission).
        crt: data.person.crt && data.person.crt !== CRT_NONE_VALUE ? parseInt(data.person.crt, 10) : null,
        state_registrations: [],
        addresses,
        contacts: data.person.contacts,
        nfse,
      }

    await onSubmit({
      cpf_or_cnpj: data.cpf_or_cnpj,
      id_estrangeiro: data.id_estrangeiro || null,
      // Persist names uppercase so person search stays assertive (see searchPersonsByName).
      name: data.name.toUpperCase(),
      roles: data.roles,
      person: personPayload,
    })
  }

  return (
    <EntityForm
      variant="person"
      entityPk={initialData?.sk}
      lockTipo={lockTipo}
      initialCpfCnpj={initialCpfCnpj}
      initialRoles={initialRoles}
      initialData={initialData ? fromPersonOut(initialData) : undefined}
      onSubmit={handleSubmit}
      loading={loading}
    />
  )
}
