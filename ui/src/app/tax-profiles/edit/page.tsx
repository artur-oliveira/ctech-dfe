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
import {TaxProfileForm} from '@/components/tax-profiles/TaxProfileForm'
import type {TaxProfileCreate} from '@/lib/types/api'

function EditTaxProfileContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
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

  const {data: profile, isLoading} = useQuery({
    queryKey: queryKeys.taxProfiles.detail(id),
    queryFn: () => apiClient.getTaxProfile(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: TaxProfileCreate) => apiClient.updateTaxProfile(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.taxProfiles.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.taxProfiles.detail(id)})
      router.push('/tax-profiles')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/tax-profiles" className="hover:text-brand-600">Perfis fiscais</Link>
          <span>/</span>
          <span className="text-gray-600">Editar perfil</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar perfil fiscal</h1>

        {/* Editar um perfil muda a tributação de toda emissão futura dos produtos
            que o referenciam — o aviso fica visível antes de qualquer campo. */}
        <div className="mb-6 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          Alterar este perfil afeta todas as emissões futuras dos produtos que o utilizam.
          Documentos já emitidos não mudam.
        </div>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !profile ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Perfil fiscal não encontrado.
          </div>
        ) : (
          <TaxProfileForm
            initialData={profile}
            crt={org?.person?.crt}
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

export default function EditTaxProfilePage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditTaxProfileContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
