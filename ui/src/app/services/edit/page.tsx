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
import {ServiceForm} from '@/components/services/ServiceForm'
import type {ServiceCreate} from '@/lib/types/api'

function EditServiceContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: service, isLoading} = useQuery({
    queryKey: queryKeys.services.detail(id),
    queryFn: () => apiClient.getService(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: ServiceCreate) => apiClient.updateService(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.services.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.services.detail(id)})
      router.push('/services')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/services" className="hover:text-brand-600">Serviços</Link>
          <span>/</span>
          <span className="text-gray-600">Editar serviço</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar serviço</h1>

        {isLoading ? (
          <div className="space-y-4">
            {[...Array(4)].map((_, i) => (
              <div key={i} className="h-16 bg-gray-100 rounded-xl animate-pulse"/>
            ))}
          </div>
        ) : !service ? (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Serviço não encontrado.
          </div>
        ) : (
          <ServiceForm
            initialData={service}
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

export default function EditServicePage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditServiceContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
