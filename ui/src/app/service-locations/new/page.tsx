'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {ServiceLocationForm} from '@/components/service-locations/ServiceLocationForm'
import type {ServiceLocationCreate} from '@/lib/types/api'

function NewServiceLocationsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: ServiceLocationCreate) => apiClient.createServiceLocation(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.serviceLocations.list(selectedOrg?.pk)})
      router.push('/service-locations')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/service-locations" className="hover:text-brand-600">Locais de prestação</Link>
          <span>/</span>
          <span className="text-gray-600">Novo local</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Novo local</h1>
        <ServiceLocationForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewServiceLocationsPage() {
  return (
    <ProtectedRoute>
      <NewServiceLocationsContent/>
    </ProtectedRoute>
  )
}
