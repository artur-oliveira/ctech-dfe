'use client'

import {useState} from 'react'
import {useRouter} from 'next/navigation'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {CatalogStep} from '@/components/onboarding/CatalogStep'
import {ProductForm} from '@/components/products/ProductForm'
import {
  ONBOARDING_ROOT,
  SERVICE_DOC_VARIANTS,
  STEP_DONE,
  STEP_PRODUCTS,
  STEP_SERVICES,
} from '@/lib/constants/onboarding'
import type {ProductCreate} from '@/lib/types/api'

function ProductsStepContent() {
  const router = useRouter()
  const qc = useQueryClient()
  const {selectedOrg} = useAuth()
  const {configured, skip} = useOnboarding()
  const [added, setAdded] = useState<string[]>([])

  const {data: org} = useQuery({
    queryKey: queryKeys.organizations.detail(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getOrganization(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })

  // A company that also issues NFS-e still has the service catalogue ahead.
  const nextStep = SERVICE_DOC_VARIANTS.some((v) => configured[v])
    ? `${ONBOARDING_ROOT}/${STEP_SERVICES}`
    : `${ONBOARDING_ROOT}/${STEP_DONE}`

  const create = useMutation({
    mutationFn: (d: ProductCreate) => apiClient.createProduct(d),
    onSuccess: (_result, variables) => {
      void qc.invalidateQueries({queryKey: queryKeys.products.list(selectedOrg?.pk)})
      setAdded((a) => [...a, variables.description ?? variables.code])
    },
  })

  return (
    <CatalogStep
      step={STEP_PRODUCTS}
      title="Seus produtos"
      description="Cadastre pelo menos um produto para emitir a primeira nota. Dá para cadastrar o resto depois, e também importar em lote."
      added={added}
      noun={{singular: 'produto', plural: 'produtos'}}
      onSkip={() => {
        skip(STEP_PRODUCTS)
        router.push(nextStep)
      }}
      onDone={() => router.push(nextStep)}
    >
      <ProductForm
        key={added.length}
        crt={org?.person?.crt}
        uf={org?.person?.state_registrations?.[0]?.uf}
        onSubmit={async (d) => {
          await create.mutateAsync(d)
        }}
        loading={create.isPending}
      />
    </CatalogStep>
  )
}

export default function ProductsStepPage() {
  return (
    <ProtectedRoute>
      <ProductsStepContent/>
    </ProtectedRoute>
  )
}
