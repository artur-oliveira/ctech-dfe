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
import {ServiceLocationForm} from '@/components/service-locations/ServiceLocationForm'
import type {ServiceLocationCreate} from '@/lib/types/api'

function EditServiceLocationsContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: entity, isLoading} = useQuery({
    queryKey: queryKeys.serviceLocations.detail(id),
    queryFn: () => apiClient.getServiceLocation(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: ServiceLocationCreate) => apiClient.updateServiceLocation(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.serviceLocations.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.serviceLocations.detail(id)})
      router.push('/service-locations')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/service-locations" className="hover:text-brand-600">Locais de prestação</Link>
          <span>/</span>
          <span className="text-gray-600">Editar local</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar local</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !entity ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Local de prestação não encontrado.
          </div>
        ) : (
          <ServiceLocationForm
            initialData={entity}
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

export default function EditServiceLocationsPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditServiceLocationsContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
