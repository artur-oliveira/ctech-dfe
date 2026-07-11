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
import {VehicleForm} from '@/components/vehicles/VehicleForm'
import type {VehicleCreate} from '@/lib/types/api'

function EditVehicleContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: vehicle, isLoading} = useQuery({
    queryKey: queryKeys.vehicles.detail(id),
    queryFn: () => apiClient.getVehicle(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: VehicleCreate) => apiClient.updateVehicle(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.vehicles.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.vehicles.detail(id)})
      router.push('/vehicles')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/vehicles" className="hover:text-brand-600">Veículos</Link>
          <span>/</span>
          <span className="text-gray-600">Editar veículo</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar veículo</h1>

        {isLoading ? (
          <div className="space-y-4 max-w-2xl">
            {[...Array(4)].map((_, i) => (
              <div key={i} className="h-14 bg-gray-100 rounded-xl animate-pulse"/>
            ))}
          </div>
        ) : !vehicle ? (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Veículo não encontrado.
          </div>
        ) : (
          <VehicleForm
            initialData={vehicle}
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

export default function EditVehiclePage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditVehicleContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
