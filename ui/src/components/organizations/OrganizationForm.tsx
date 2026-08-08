'use client'

import {useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {EntityForm} from '@/components/EntityForm'
import {CertificateFields} from '@/components/organizations/CertificateFields'
import {apiClient} from '@/lib/api/client'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {CRT_NONE_VALUE, type EntityFormData, nfseInfoToApi} from '@/lib/schemas/entity'
import type {OrganizationFormData} from '@/lib/schemas/organizations'
import type {OrganizationCreate} from '@/lib/types/api'

// Re-export for any existing code that imports from here
export type {OrganizationFormData}

export interface CertificateInput {
  file: File
  password: string
}

interface OrganizationFormProps {
  initialData?: OrganizationFormData
  /** pk of the existing org — determines PF/PJ lock in edit mode */
  orgPk?: string
  onSubmit: (data: OrganizationCreate, cert?: CertificateInput) => Promise<void>
  loading?: boolean
}

export function OrganizationForm({initialData, orgPk, onSubmit, loading}: OrganizationFormProps) {
  // Create mode only (no orgPk): KYC requires an A1 certificate unless the user
  // can inherit a matriz certificate for the same CNPJ root (filial).
  const isCreate = !orgPk
  const [file, setFile] = useState<File | null>(null)
  const [password, setPassword] = useState('')
  const [certError, setCertError] = useState<string | null>(null)

  const toApi = (data: EntityFormData): OrganizationCreate => {
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
      },
    }
  }

  // Transform the form state into the clean API contract, attaching the
  // certificate when one is needed. If a certificate is required but missing,
  // throw — EntityForm surfaces the thrown message inline.
  const handleSubmit = async (data: EntityFormData) => {
    const payload = toApi(data)
    if (!isCreate) {
      await onSubmit(payload)
      return
    }
    if (file) {
      await onSubmit(payload, {file, password})
      return
    }
    await onSubmit(payload)
  }

  return (
    <EntityForm
      variant="organization"
      entityPk={orgPk}
      initialData={initialData as EntityFormData | undefined}
      onSubmit={handleSubmit}
      loading={loading}
      extraSection={isCreate ? ({cpfCnpj}) => (
        <CertificateSection
          cpfCnpj={cpfCnpj}
          file={file}
          onFileChange={(f) => { setFile(f); setCertError(null) }}
          password={password}
          onPasswordChange={setPassword}
          certError={certError}
        />
      ) : undefined}
    />
  )
}

function CertificateSection({
  cpfCnpj,
  file,
  onFileChange,
  password,
  onPasswordChange,
  certError,
}: {
  cpfCnpj: string
  file: File | null
  onFileChange: (f: File | null) => void
  password: string
  onPasswordChange: (p: string) => void
  certError: string | null
}) {
  const clean = cpfCnpj.replace(/\D/g, '')
  const debounced = useDebounce(clean, 300)
  const complete = debounced.length === 11 || debounced.length === 14

  // Whether the certificate is required (false when a matriz cert can be
  // inherited for a filial). Defaults to required until we know otherwise.
  const {data, isFetching} = useQuery({
    queryKey: ['organizations', 'certificate-requirement', debounced],
    queryFn: () => apiClient.certificateRequirement(debounced),
    enabled: complete,
    staleTime: 60_000,
  })
  const required = data?.required ?? true

  return (
    <section className="rounded-xl border border-gray-200 p-4 md:p-6 space-y-3">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-gray-900">Certificado Digital A1</h3>
        {complete && isFetching && (
          <span className="text-xs text-gray-400">verificando…</span>
        )}
      </div>
      {complete && !required ? (
        <p className="text-sm text-gray-600 leading-relaxed">
          Esta empresa faz parte de um grupo que você já administra (mesma raiz de CNPJ). O
          certificado da matriz será usado automaticamente — não é necessário enviar um novo. Você
          ainda pode enviar um certificado próprio, se preferir.
        </p>
      ) : (
        <CertificateFields
          file={file}
          onFileChange={onFileChange}
          password={password}
          onPasswordChange={onPasswordChange}
          fileError={certError}
          hint="Para comprovar que a empresa é sua, envie o certificado digital A1 (arquivo .pfx/.p12) e a senha. O CNPJ do certificado precisa corresponder ao informado."
        />
      )}
    </section>
  )
}
