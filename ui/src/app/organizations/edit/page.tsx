'use client'

import {Suspense} from 'react'
import Link from 'next/link'
import {useRouter, useSearchParams} from 'next/navigation'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {OrganizationForm} from '@/components/organizations/OrganizationForm'
import {AuthorizedViewersSection} from '@/components/organizations/AuthorizedViewersSection'
import type {OrganizationUpdate} from '@/lib/types/api'
import {organizationOutToFormData} from '@/lib/utils/converters'

function EditOrganizationContent() {
  const params = useSearchParams()
  const pk = params.get('pk') ?? ''
  const router = useRouter()
  const qc = useQueryClient()
  
  const {data: org, isLoading} = useQuery({
    queryKey: queryKeys.organizations.detail(pk),
    queryFn: () => apiClient.getOrganization(pk),
    enabled: !!pk,
  })
  
  const updateMutation = useMutation({
    mutationFn: (data: OrganizationUpdate) => apiClient.updateOrganization(pk, data),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.organizations.all()})
      router.push('/organizations')
    },
  })
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/organizations" className="hover:text-brand-600">Organizações</Link>
          <span>/</span>
          <span className="text-gray-600">Editar organização</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar organização</h1>
        
        {isLoading ? (
          <div className="space-y-4 max-w-2xl">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-14 bg-gray-100 rounded-xl animate-pulse"/>
            ))}
          </div>
        ) : !org ? (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Organização não encontrada.
          </div>
        ) : (
          <div className="space-y-8">
            <OrganizationForm
              initialData={organizationOutToFormData(org)}
              orgPk={org.pk}
              onSubmit={async (d) => {
                // PUT is partial and keyed by pk — never send cpf_or_cnpj in the body.
                await updateMutation.mutateAsync({name: d.name, description: d.description, person: d.person})
              }}
              loading={updateMutation.isPending}
            />
            <AuthorizedViewersSection orgPk={org.pk} viewers={org.authorized_xml_viewers ?? []}/>
          </div>
        )}
      </div>
    </RootLayout>
  )
}

export default function EditOrganizationPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditOrganizationContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
