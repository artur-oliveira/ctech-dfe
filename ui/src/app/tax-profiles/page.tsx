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
import {PercentIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {TaxProfileItemOut} from '@/lib/types/api'

function TaxProfilesContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<TaxProfileItemOut>({
      queryKey: queryKeys.taxProfiles.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getTaxProfiles({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<TaxProfileItemOut>({
    mutationFn: (id) => apiClient.deleteTaxProfile(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.TAX_PROFILE),
    getDeletedMessage: (p) => `"${p.name}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.taxProfiles.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Perfis fiscais"
          description="Tributação configurada uma vez e reutilizada por vários produtos"
          action={selectedOrg ? {
            label: 'Novo perfil',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/tax-profiles/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhum perfil fiscal"
            description="Um perfil guarda a tributação de um conjunto de CFOPs e é reaproveitado por quantos produtos precisar — mudou a alíquota, muda em um lugar só."
            action={{label: 'Novo perfil', onClick: () => router.push('/tax-profiles/new')}}
            icon={<PercentIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Perfis fiscais"
            minWidth={480}
            headers={['Nome', 'CFOPs', 'Descrição', {label: '', align: 'right'}]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="CFOPs" className={TABLE_CELL}>
                  <span className="flex flex-wrap gap-1">
                    {(p.cfops ?? []).map((cfop) => (
                      <span key={cfop}
                            className="inline-flex items-center rounded bg-brand-50 px-2 py-0.5 font-mono text-xs font-medium text-brand-700">
                        {cfop}
                      </span>
                    ))}
                  </span>
                </td>
                <td data-label="Descrição" className={`${TABLE_CELL} text-gray-600`}>{p.description ?? '—'}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/tax-profiles/edit?id=${extractId(p.sk, SK_PREFIX.TAX_PROFILE)}`)}
                            className="text-brand-600 hover:text-brand-700">
                      Editar
                    </Button>
                    <Button variant="ghost" size="xs" onClick={() => handleDelete(p)} disabled={isDeleting}
                            className="text-danger hover:text-red-700">
                      Excluir
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </TableShell>
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

export default function TaxProfilesPage() {
  return (
    <ProtectedRoute>
      <TaxProfilesContent/>
    </ProtectedRoute>
  )
}
