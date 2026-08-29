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
import {CargoUnitForm} from '@/components/cargo-units/CargoUnitForm'
import type {CargoUnitCreate} from '@/lib/types/api'

function EditCargoUnitContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: unit, isLoading} = useQuery({
    queryKey: queryKeys.cargoUnits.detail(id),
    queryFn: () => apiClient.getCargoUnit(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: CargoUnitCreate) => apiClient.updateCargoUnit(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.cargoUnits.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.cargoUnits.detail(id)})
      router.push('/cargo-units')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/cargo-units" className="hover:text-brand-600">Unidades de transporte e de carga</Link>
          <span>/</span>
          <span className="text-gray-600">Editar unidade</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar unidade</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !unit ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Unidade não encontrada.
          </div>
        ) : (
          <CargoUnitForm
            initialData={unit}
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

export default function EditCargoUnitPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditCargoUnitContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
