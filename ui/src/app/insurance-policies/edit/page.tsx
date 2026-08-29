'use client'

import {Suspense} from 'react'
import Link from 'next/link'
import {useRouter, useSearchParams} from 'next/navigation'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {InsurancePolicyForm} from '@/components/insurance-policies/InsurancePolicyForm'
import type {InsurancePolicyCreate} from '@/lib/types/api'

function EditInsurancePolicyContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: policy, isLoading} = useQuery({
    queryKey: queryKeys.insurancePolicies.detail(id),
    queryFn: () => apiClient.getInsurancePolicy(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: InsurancePolicyCreate) => apiClient.updateInsurancePolicy(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.insurancePolicies.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.insurancePolicies.detail(id)})
      router.push('/insurance-policies')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/insurance-policies" className="hover:text-brand-600">Apólices de seguro</Link>
          <span>/</span>
          <span className="text-gray-600">Editar apólice</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar apólice de seguro</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !policy ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Apólice de seguro não encontrada.
          </div>
        ) : (
          <InsurancePolicyForm
            initialData={policy}
            onSubmit={async (d) => {
              await updateMutation.mutateAsync(d)
            }}
            loading={updateMutation.isPending}
          />
        )}
      </div>
    </RootLayout>
  )
}

export default function EditInsurancePolicyPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditInsurancePolicyContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
