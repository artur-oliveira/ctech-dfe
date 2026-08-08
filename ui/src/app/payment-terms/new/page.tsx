'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {PaymentTermForm} from '@/components/payment-terms/PaymentTermForm'
import type {PaymentTermCreate} from '@/lib/types/api'

function NewPaymentTermContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: PaymentTermCreate) => apiClient.createPaymentTerm(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.paymentTerms.list(selectedOrg?.pk)})
      router.push('/payment-terms')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/payment-terms" className="hover:text-brand-600">Condições de pagamento</Link>
          <span>/</span>
          <span className="text-gray-600">Nova condição</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova condição de pagamento</h1>
        <PaymentTermForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewPaymentTermPage() {
  return (
    <ProtectedRoute>
      <NewPaymentTermContent/>
    </ProtectedRoute>
  )
}
