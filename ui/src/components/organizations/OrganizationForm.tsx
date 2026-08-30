'use client'

import type {ReactNode} from 'react'
import {EntityForm} from '@/components/EntityForm'
import {CRT_NONE_VALUE, type EntityFormData, nfseInfoToApi} from '@/lib/schemas/entity'
import type {OrganizationFormData} from '@/lib/schemas/organizations'
import type {OrganizationCreate} from '@/lib/types/api'

// Re-export for any existing code that imports from here
export type {OrganizationFormData}

interface OrganizationFormProps {
  initialData?: OrganizationFormData
  /** pk of the existing org — determines PF/PJ lock in edit mode */
  orgPk?: string
  onSubmit: (data: OrganizationCreate) => Promise<void>
  loading?: boolean
  /** Extra fiscal detail of this company, shown inside "dados complementares". */
  advancedSection?: ReactNode
  /** Fill the blanks from the CNPJ — see EntityForm's `autoLookup`. */
  autoLookup?: boolean
}

/**
 * The company cadastro — editing only.
 *
 * It no longer creates. A company's identity belongs to ctech-account
 * (ctech-billing ADR 0022), so it is registered there and adopted here by
 * `/organizations/link`; this form is what fills in the fiscal side afterwards.
 * The certificate that used to be collected alongside a local create is now its
 * own layer of the onboarding flow, because a company that arrives through the
 * handoff never passes through a form that could ask for one.
 */
export function OrganizationForm({initialData, orgPk, onSubmit, loading, advancedSection, autoLookup}: OrganizationFormProps) {
  return (
    <EntityForm
      variant="organization"
      entityPk={orgPk}
      initialData={initialData as EntityFormData | undefined}
      onSubmit={async (data) => onSubmit(organizationFormToApi(data))}
      loading={loading}
      autoLookup={autoLookup}
      advancedSection={advancedSection}
    />
  )
}

export function organizationFormToApi(data: EntityFormData): OrganizationCreate {
  const addresses = data.person.addresses.map((a) => ({
    ...a,
    postal_code: a.postal_code.replace(/\D/g, ''),
    complement: a.complement || null,
  }))
  const crtRaw = data.person.crt
  const crt = crtRaw && crtRaw !== CRT_NONE_VALUE ? parseInt(crtRaw, 10) : null
  return {
    cpf_or_cnpj: data.cpf_or_cnpj,
    name: data.name,
    description: data.description || undefined,
    person: {
      fantasy_name: data.person.fantasy_name || null,
      crt,
      state_registrations: data.person.state_registrations,
      addresses,
      contacts: data.person.contacts,
      nfse: nfseInfoToApi(data.person.nfse),
      cnae: data.person.cnae || null,
      isuf_emit: data.person.isuf_emit || null,
    },
  }
}
