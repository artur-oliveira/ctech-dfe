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
import {VehicleSetForm} from '@/components/vehicle-sets/VehicleSetForm'
import type {VehicleSetCreate} from '@/lib/types/api'

function EditVehicleSetContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: set, isLoading} = useQuery({
    queryKey: queryKeys.vehicleSets.detail(id),
    queryFn: () => apiClient.getVehicleSet(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: VehicleSetCreate) => apiClient.updateVehicleSet(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.vehicleSets.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.vehicleSets.detail(id)})
      router.push('/vehicle-sets')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/vehicle-sets" className="hover:text-brand-600">Composições veiculares</Link>
          <span>/</span>
          <span className="text-gray-600">Editar composição</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar composição veicular</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !set ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Composição veicular não encontrada.
          </div>
        ) : (
          <VehicleSetForm
            initialData={set}
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

export default function EditVehicleSetPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditVehicleSetContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
