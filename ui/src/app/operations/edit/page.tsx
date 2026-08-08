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
import {OperationForm} from '@/components/operations/OperationForm'
import type {OperationCreate} from '@/lib/types/api'

function EditOperationContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: operation, isLoading} = useQuery({
    queryKey: queryKeys.operations.detail(id),
    queryFn: () => apiClient.getOperation(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: OperationCreate) => apiClient.updateOperation(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.operations.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.operations.detail(id)})
      router.push('/operations')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/operations" className="hover:text-brand-600">Naturezas de operação</Link>
          <span>/</span>
          <span className="text-gray-600">Editar operação</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar natureza de operação</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !operation ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Natureza de operação não encontrada.
          </div>
        ) : (
          <OperationForm
            initialData={operation}
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

export default function EditOperationPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditOperationContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
