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
import {TollProviderForm} from '@/components/toll-providers/TollProviderForm'
import type {TollProviderCreate} from '@/lib/types/api'

function EditTollProviderContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: provider, isLoading} = useQuery({
    queryKey: queryKeys.tollProviders.detail(id),
    queryFn: () => apiClient.getTollProvider(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: TollProviderCreate) => apiClient.updateTollProvider(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.tollProviders.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.tollProviders.detail(id)})
      router.push('/toll-providers')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/toll-providers" className="hover:text-brand-600">Fornecedoras de vale-pedágio</Link>
          <span>/</span>
          <span className="text-gray-600">Editar fornecedora</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar fornecedora de vale-pedágio</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !provider ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Fornecedora de vale-pedágio não encontrada.
          </div>
        ) : (
          <TollProviderForm
            initialData={provider}
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

export default function EditTollProviderPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditTollProviderContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
