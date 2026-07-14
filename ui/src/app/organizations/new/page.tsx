'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {type CertificateInput, OrganizationForm} from '@/components/organizations/OrganizationForm'
import type {OrganizationCreate} from '@/lib/types/api'

function NewOrganizationContent() {
  const router = useRouter()
  const qc = useQueryClient()
  const {refreshUser, setSelectedOrg} = useAuth()

  const createMutation = useMutation({
    mutationFn: ({data, cert}: { data: OrganizationCreate; cert?: CertificateInput }) =>
      apiClient.createOrganization(data, cert),
    onSuccess: async (created) => {
      void qc.invalidateQueries({queryKey: queryKeys.organizations.all()})
      // Refetch /auth/me so the new org appears in the membership list, then make
      // it the active org. refreshUser prioritizes the previously-saved org, so we
      // override with the freshly-created one here.
      const me = await refreshUser()
      const newOrg = me?.organizations.find((o) => o.pk === created.pk)
      if (newOrg) setSelectedOrg(newOrg)
      router.push('/organizations')
    },
  })
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/organizations" className="hover:text-brand-600">Organizações</Link>
          <span>/</span>
          <span className="text-gray-600">Nova organização</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova organização</h1>
        <OrganizationForm
          onSubmit={async (data, cert) => {
            await createMutation.mutateAsync({data, cert})
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewOrganizationPage() {
  return (
    <ProtectedRoute>
      <NewOrganizationContent/>
    </ProtectedRoute>
  )
}
