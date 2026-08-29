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
import {FuelPumpForm} from '@/components/fuel-pumps/FuelPumpForm'
import type {FuelPumpCreate} from '@/lib/types/api'

function EditFuelPumpContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: pump, isLoading} = useQuery({
    queryKey: queryKeys.fuelPumps.detail(id),
    queryFn: () => apiClient.getFuelPump(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: FuelPumpCreate) => apiClient.updateFuelPump(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.fuelPumps.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.fuelPumps.detail(id)})
      router.push('/fuel-pumps')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/fuel-pumps" className="hover:text-brand-600">Bombas de combustível</Link>
          <span>/</span>
          <span className="text-gray-600">Editar bomba</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar bomba de combustível</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !pump ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Bomba não encontrada.
          </div>
        ) : (
          <FuelPumpForm
            initialData={pump}
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

export default function EditFuelPumpPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditFuelPumpContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
