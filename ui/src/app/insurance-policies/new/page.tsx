'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {InsurancePolicyForm} from '@/components/insurance-policies/InsurancePolicyForm'
import type {InsurancePolicyCreate} from '@/lib/types/api'

function NewInsurancePolicyContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: InsurancePolicyCreate) => apiClient.createInsurancePolicy(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.insurancePolicies.list(selectedOrg?.pk)})
      router.push('/insurance-policies')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/insurance-policies" className="hover:text-brand-600">Apólices de seguro</Link>
          <span>/</span>
          <span className="text-gray-600">Nova apólice</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova apólice de seguro</h1>
        <InsurancePolicyForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewInsurancePolicyPage() {
  return (
    <ProtectedRoute>
      <NewInsurancePolicyContent/>
    </ProtectedRoute>
  )
}
