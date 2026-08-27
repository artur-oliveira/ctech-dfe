'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {CargoUnitForm} from '@/components/cargo-units/CargoUnitForm'
import type {CargoUnitCreate} from '@/lib/types/api'

function NewCargoUnitContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: CargoUnitCreate) => apiClient.createCargoUnit(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.cargoUnits.list(selectedOrg?.pk)})
      router.push('/cargo-units')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/cargo-units" className="hover:text-brand-600">Unidades de transporte e de carga</Link>
          <span>/</span>
          <span className="text-gray-600">Nova unidade</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova unidade</h1>
        <CargoUnitForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewCargoUnitPage() {
  return (
    <ProtectedRoute>
      <NewCargoUnitContent/>
    </ProtectedRoute>
  )
}
