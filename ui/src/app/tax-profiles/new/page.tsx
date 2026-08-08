'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {TaxProfileForm} from '@/components/tax-profiles/TaxProfileForm'
import type {TaxProfileCreate} from '@/lib/types/api'

function NewTaxProfileContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  // O CRT vive na organização, não no token — é ele que decide entre CSOSN e
  // CST de ICMS no editor de tributação.
  const {data: org} = useQuery({
    queryKey: queryKeys.organizations.detail(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getOrganization(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })

  const createMutation = useMutation({
    mutationFn: (d: TaxProfileCreate) => apiClient.createTaxProfile(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.taxProfiles.list(selectedOrg?.pk)})
      router.push('/tax-profiles')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/tax-profiles" className="hover:text-brand-600">Perfis fiscais</Link>
          <span>/</span>
          <span className="text-gray-600">Novo perfil</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Novo perfil fiscal</h1>
        <TaxProfileForm
          crt={org?.person.crt}
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewTaxProfilePage() {
  return (
    <ProtectedRoute>
      <NewTaxProfileContent/>
    </ProtectedRoute>
  )
}
