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
import {PaymentTerminalForm} from '@/components/payment-terminals/PaymentTerminalForm'
import type {PaymentTerminalCreate} from '@/lib/types/api'

function EditPaymentTerminalContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: terminal, isLoading} = useQuery({
    queryKey: queryKeys.paymentTerminals.detail(id),
    queryFn: () => apiClient.getPaymentTerminal(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: PaymentTerminalCreate) => apiClient.updatePaymentTerminal(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.paymentTerminals.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.paymentTerminals.detail(id)})
      router.push('/payment-terminals')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/payment-terminals" className="hover:text-brand-600">Terminais de pagamento</Link>
          <span>/</span>
          <span className="text-gray-600">Editar terminal</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar terminal de pagamento</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !terminal ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Terminal de pagamento não encontrado.
          </div>
        ) : (
          <PaymentTerminalForm
            initialData={terminal}
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

export default function EditPaymentTerminalPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditPaymentTerminalContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
