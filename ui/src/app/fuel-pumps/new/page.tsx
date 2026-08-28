'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {FuelPumpForm} from '@/components/fuel-pumps/FuelPumpForm'
import type {FuelPumpCreate} from '@/lib/types/api'

function NewFuelPumpContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: FuelPumpCreate) => apiClient.createFuelPump(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.fuelPumps.list(selectedOrg?.pk)})
      router.push('/fuel-pumps')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/fuel-pumps" className="hover:text-brand-600">Bombas de combustível</Link>
          <span>/</span>
          <span className="text-gray-600">Nova bomba</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova bomba de combustível</h1>
        <FuelPumpForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewFuelPumpPage() {
  return (
    <ProtectedRoute>
      <NewFuelPumpContent/>
    </ProtectedRoute>
  )
}
