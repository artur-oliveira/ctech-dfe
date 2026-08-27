'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {PaymentTerminalForm} from '@/components/payment-terminals/PaymentTerminalForm'
import type {PaymentTerminalCreate} from '@/lib/types/api'

function NewPaymentTerminalContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: PaymentTerminalCreate) => apiClient.createPaymentTerminal(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.paymentTerminals.list(selectedOrg?.pk)})
      router.push('/payment-terminals')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/payment-terminals" className="hover:text-brand-600">Terminais de pagamento</Link>
          <span>/</span>
          <span className="text-gray-600">Novo terminal</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Novo terminal de pagamento</h1>
        <PaymentTerminalForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewPaymentTerminalPage() {
  return (
    <ProtectedRoute>
      <NewPaymentTerminalContent/>
    </ProtectedRoute>
  )
}
