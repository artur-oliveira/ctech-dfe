'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {OperationForm} from '@/components/operations/OperationForm'
import type {OperationCreate} from '@/lib/types/api'

function NewOperationContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: OperationCreate) => apiClient.createOperation(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.operations.list(selectedOrg?.pk)})
      router.push('/operations')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/operations" className="hover:text-brand-600">Naturezas de operação</Link>
          <span>/</span>
          <span className="text-gray-600">Nova operação</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova natureza de operação</h1>
        <OperationForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewOperationPage() {
  return (
    <ProtectedRoute>
      <NewOperationContent/>
    </ProtectedRoute>
  )
}
