'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {VehicleSetForm} from '@/components/vehicle-sets/VehicleSetForm'
import type {VehicleSetCreate} from '@/lib/types/api'

function NewVehicleSetContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: VehicleSetCreate) => apiClient.createVehicleSet(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.vehicleSets.list(selectedOrg?.pk)})
      router.push('/vehicle-sets')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/vehicle-sets" className="hover:text-brand-600">Composições veiculares</Link>
          <span>/</span>
          <span className="text-gray-600">Nova composição</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova composição veicular</h1>
        <VehicleSetForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewVehicleSetPage() {
  return (
    <ProtectedRoute>
      <NewVehicleSetContent/>
    </ProtectedRoute>
  )
}
