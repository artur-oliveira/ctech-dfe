'use client'

import {useState} from 'react'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {CatalogStep} from '@/components/onboarding/CatalogStep'
import {ServiceForm} from '@/components/services/ServiceForm'
import {ONBOARDING_ROOT, STEP_DONE, STEP_SERVICES} from '@/lib/constants/onboarding'
import type {ServiceCreate} from '@/lib/types/api'

function ServicesStepContent() {
  const router = useRouter()
  const qc = useQueryClient()
  const {selectedOrg} = useAuth()
  const {skip} = useOnboarding()
  const [added, setAdded] = useState<string[]>([])

  const done = `${ONBOARDING_ROOT}/${STEP_DONE}`

  const create = useMutation({
    mutationFn: (d: ServiceCreate) => apiClient.createService(d),
    onSuccess: (_result, variables) => {
      void qc.invalidateQueries({queryKey: queryKeys.services.list(selectedOrg?.pk)})
      setAdded((a) => [...a, variables.description || variables.code])
    },
  })

  return (
    <CatalogStep
      step={STEP_SERVICES}
      title="Seus serviços"
      description="Cadastre pelo menos um serviço para emitir a primeira NFS-e. O código de tributação nacional é o que a prefeitura usa para calcular o ISS."
      added={added}
      noun={{singular: 'serviço', plural: 'serviços'}}
      onSkip={() => {
        skip(STEP_SERVICES)
        router.push(done)
      }}
      onDone={() => router.push(done)}
    >
      <ServiceForm
        key={added.length}
        onSubmit={async (d) => {
          await create.mutateAsync(d)
        }}
        loading={create.isPending}
      />
    </CatalogStep>
  )
}

export default function ServicesStepPage() {
  return (
    <ProtectedRoute>
      <ServicesStepContent/>
    </ProtectedRoute>
  )
}
