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
import {PaymentTermForm} from '@/components/payment-terms/PaymentTermForm'
import type {PaymentTermCreate} from '@/lib/types/api'

function EditPaymentTermContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: term, isLoading} = useQuery({
    queryKey: queryKeys.paymentTerms.detail(id),
    queryFn: () => apiClient.getPaymentTerm(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: PaymentTermCreate) => apiClient.updatePaymentTerm(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.paymentTerms.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.paymentTerms.detail(id)})
      router.push('/payment-terms')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/payment-terms" className="hover:text-brand-600">Condições de pagamento</Link>
          <span>/</span>
          <span className="text-gray-600">Editar condição</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar condição de pagamento</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !term ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Condição de pagamento não encontrada.
          </div>
        ) : (
          <PaymentTermForm
            initialData={term}
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

export default function EditPaymentTermPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditPaymentTermContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
