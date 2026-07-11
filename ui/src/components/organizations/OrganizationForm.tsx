'use client'

import {EntityForm} from '@/components/EntityForm'
import {CRT_NONE_VALUE, type EntityFormData} from '@/lib/schemas/entity'
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
}

export function OrganizationForm({initialData, orgPk, onSubmit, loading}: OrganizationFormProps) {
  // Transform the form state into the clean API contract (OrganizationCreate):
  // the UI-only `tipo` discriminator is dropped, CRT becomes a number (or null),
  // and postal codes/complements are normalized — mirroring PersonForm so the
  // strictly-validated backend accepts the payload.
  const handleSubmit = async (data: EntityFormData) => {
    const addresses = data.person.addresses.map((a) => ({
      ...a,
      postal_code: a.postal_code.replace(/\D/g, ''),
      complement: a.complement || null,
    }))
    const crtRaw = data.person.crt
    const crt = crtRaw && crtRaw !== CRT_NONE_VALUE ? parseInt(crtRaw, 10) : null
    await onSubmit({
      cpf_or_cnpj: data.cpf_or_cnpj,
      name: data.name,
      description: data.description || undefined,
      person: {
        fantasy_name: data.person.fantasy_name || null,
        crt,
        state_registrations: data.person.state_registrations,
        addresses,
        contacts: data.person.contacts,
      },
    })
  }

  return (
    <EntityForm
      variant="organization"
      entityPk={orgPk}
      initialData={initialData as EntityFormData | undefined}
      onSubmit={handleSubmit}
      loading={loading}
    />
  )
}
