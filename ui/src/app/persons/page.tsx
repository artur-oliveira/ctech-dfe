'use client'

import {useQueryClient} from '@tanstack/react-query'
import {useRouter} from 'next/navigation'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {useEntityDelete} from '@/lib/hooks/useEntityDelete'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import type {PersonItemOut} from '@/lib/types/api'
import {docLabel, formatCpfCnpj, unformatCpfCnpj} from '@/lib/utils/document'

function PersonsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<PersonItemOut>({
      queryKey: queryKeys.persons.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getPersons({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, isPending: isDeleting} = useEntityDelete<PersonItemOut>({
    mutationFn: (id) => apiClient.deletePerson(id),
    getId: (p) => unformatCpfCnpj(p.sk),
    getConfirmMessage: (p) => `Excluir "${p.name}"?`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.persons.list(selectedOrg?.pk)})
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Pessoas"
          description="Pessoas físicas e jurídicas cadastradas"
          action={selectedOrg ? {
            label: 'Nova pessoa',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/persons/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : items.length === 0 ? (
          <EmptyState
            title="Nenhuma pessoa cadastrada"
            description="Cadastre clientes e fornecedores para usar na emissão de documentos fiscais."
            action={{label: 'Nova pessoa', onClick: () => router.push('/persons/new')}}
          />
        ) : (
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden overflow-x-auto">
            <table className="w-full text-sm min-w-120">
              <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                {['Nome', 'Tipo', 'Documento', 'Cidade / UF', ''].map((h) => (
                  <th key={h}
                      className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    {h}
                  </th>
                ))}
              </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
              {items.map((p) => (
                <tr key={p.sk} className="hover:bg-gray-50 transition-colors">
                  <td className="px-5 py-3.5 font-medium text-gray-900">{p.name}</td>
                  <td className="px-5 py-3.5">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                      docLabel(p.sk) === 'CPF'
                        ? 'bg-blue-50 text-blue-700'
                        : 'bg-purple-50 text-purple-700'
                    }`}>
                      {docLabel(p.sk)}
                    </span>
                  </td>
                  <td className="px-5 py-3.5 font-mono text-xs text-gray-600">{formatCpfCnpj(p.sk)}</td>
                  <td className="px-5 py-3.5 text-gray-600">
                    {p.person.addresses[0]?.city} / {p.person.addresses[0]?.state_federation}
                  </td>
                  <td className="px-5 py-3.5 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => router.push(`/persons/edit?id=${unformatCpfCnpj(p.sk)}`)}
                        className="text-brand-600 hover:text-brand-700"
                      >
                        Editar
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => handleDelete(p)}
                        disabled={isDeleting}
                        className="text-red-500 hover:text-red-700"
                      >
                        Excluir
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              </tbody>
            </table>
          </div>
        )}
        <Pagination
          hasNext={hasNext}
          hasPrevious={hasPrevious}
          onNext={goNext}
          onPrevious={goPrevious}
          isLoading={isFetching}
        />
      </div>
    </RootLayout>
  )
}

export default function PersonsPage() {
  return (
    <ProtectedRoute>
      <PersonsContent/>
    </ProtectedRoute>
  )
}
