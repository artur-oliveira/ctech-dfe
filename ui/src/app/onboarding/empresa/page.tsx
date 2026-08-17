'use client'

import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {OnboardingShell} from '@/components/onboarding/OnboardingShell'
import {type CertificateInput, OrganizationForm} from '@/components/organizations/OrganizationForm'
import {ONBOARDING_ROOT, STEP_COMPANY, STEP_DOCUMENTS} from '@/lib/constants/onboarding'
import type {OrganizationCreate} from '@/lib/types/api'

function CompanyStepContent() {
  const router = useRouter()
  const qc = useQueryClient()
  const {refreshUser, setSelectedOrg} = useAuth()

  const create = useMutation({
    mutationFn: ({data, cert}: { data: OrganizationCreate; cert?: CertificateInput }) =>
      apiClient.createOrganization(data, cert),
    onSuccess: async (created) => {
      void qc.invalidateQueries({queryKey: queryKeys.organizations.all()})
      // The membership only exists after /auth/me is refetched; the new company
      // then becomes the active one, because every later layer is scoped to it.
      const me = await refreshUser()
      const newOrg = me?.organizations.find((o) => o.pk === created.pk)
      if (newOrg) setSelectedOrg(newOrg)
      router.push(`${ONBOARDING_ROOT}/${STEP_DOCUMENTS}`)
    },
  })

  return (
    <OnboardingShell
      current={STEP_COMPANY}
      title="Cadastre sua empresa"
      description="Os dados vêm da Receita a partir do CNPJ. O certificado digital A1 é o que assina cada documento — sem ele a SEFAZ não aceita a emissão."
    >
      <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
        <OrganizationForm
          onSubmit={async (data, cert) => {
            await create.mutateAsync({data, cert})
          }}
          loading={create.isPending}
        />
      </div>
    </OnboardingShell>
  )
}

export default function CompanyStepPage() {
  return (
    <ProtectedRoute>
      <CompanyStepContent/>
    </ProtectedRoute>
  )
}
