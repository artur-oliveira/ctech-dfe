'use client'

import {useRouter} from 'next/navigation'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {OrganizationsTable} from '@/components/organizations/OrganizationsTable'
import type {Organization} from '@/components/organizations/OrganizationsTable'
import {Button} from '@/components/ui/button'

function OrganizationsContent() {
  const router = useRouter()
  const qc = useQueryClient()

  const {data, isPending, error} = useQuery({
    queryKey: queryKeys.organizations.all(),
    queryFn: () => apiClient.getOrganizations(),
  })

  const deleteMutation = useMutation({
    mutationFn: (pk: string) => apiClient.deleteOrganization(pk),
    onSuccess: () => qc.invalidateQueries({queryKey: queryKeys.organizations.all()}),
  })

  const handleEdit = (org: Organization) => {
    router.push(`/organizations/edit?pk=${encodeURIComponent(org.pk)}`)
  }

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        {error && (
          <div className="mb-6 bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg text-sm">
            {error.message}
          </div>
        )}

        <div className="flex items-center justify-between mb-8 gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">Organizações</h1>
            <p className="text-gray-500 mt-1 text-sm">Gerencie suas organizações e configurações fiscais</p>
          </div>
          <Button
            variant="brand"
            onClick={() => router.push('/organizations/new')}
            className="shrink-0 gap-1.5"
          >
            <span className="text-lg leading-none">+</span>
            Nova Organização
          </Button>
        </div>

        <OrganizationsTable
          organizations={data ?? []}
          onEdit={handleEdit}
          loading={isPending || deleteMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function OrganizationsPage() {
  return (
    <ProtectedRoute>
      <OrganizationsContent/>
    </ProtectedRoute>
  )
}
