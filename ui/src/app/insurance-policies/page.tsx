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
import {InsuranceIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {RESP_SEG_OPTIONS} from '@/lib/schemas/insurance-policies'
import type {InsurancePolicyItemOut} from '@/lib/types/api'

/** Rótulo do responsável pelo seguro. */
function respSegLabel(code: unknown): string {
  if (typeof code !== 'string' || !code) return '—'
  return RESP_SEG_OPTIONS.find((o) => o.value === code)?.label ?? code
}

function str(v: unknown): string {
  return typeof v === 'string' && v ? v : '—'
}

function InsurancePoliciesContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<InsurancePolicyItemOut>({
      queryKey: queryKeys.insurancePolicies.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getInsurancePolicies({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<InsurancePolicyItemOut>({
    mutationFn: (id) => apiClient.deleteInsurancePolicy(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.INSURANCE_POLICY),
    getDeletedMessage: (p) => `"${p.name}" excluída`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.insurancePolicies.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Apólices de seguro"
          description="Responsável, seguradora e número da apólice cadastrados uma vez; por viagem entram só as averbações"
          action={selectedOrg ? {
            label: 'Nova apólice',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/insurance-policies/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhuma apólice de seguro"
            description="A apólice guarda o responsável, a seguradora e o número. Na emissão do MDF-e entram só as averbações da viagem."
            action={{label: 'Nova apólice', onClick: () => router.push('/insurance-policies/new')}}
            icon={<InsuranceIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Apólices de seguro"
            minWidth={480}
            headers={['Nome', 'Responsável', 'Seguradora', 'Apólice', {label: '', align: 'right'}]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="Responsável" className={`${TABLE_CELL} text-gray-600`}>{respSegLabel(p.resp_seg)}</td>
                <td data-label="Seguradora" className={`${TABLE_CELL} text-gray-600`}>{str(p.x_seg)}</td>
                <td data-label="Apólice" className={`${TABLE_CELL} text-gray-600`}>{str(p.n_apol)}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/insurance-policies/edit?id=${extractId(p.sk, SK_PREFIX.INSURANCE_POLICY)}`)}
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

export default function InsurancePoliciesPage() {
  return (
    <ProtectedRoute>
      <InsurancePoliciesContent/>
    </ProtectedRoute>
  )
}
