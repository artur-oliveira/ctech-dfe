'use client'

import {useState} from 'react'
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
import {UsersIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TableShell, TABLE_ROW, TABLE_CELL, RowCheckbox} from '@/components/ui/table-shell'
import {BulkActionBar} from '@/components/ui/bulk-action-bar'
import {useRowSelection} from '@/lib/hooks/useRowSelection'
import {OptionsSelect} from '@/components/ui/options-select'
import {PERSON_ROLE_LABELS, PERSON_ROLE_OPTIONS, type PersonRole} from '@/lib/schemas/entity'
import type {PersonItemOut} from '@/lib/types/api'
import {docLabel, formatCpfCnpj, unformatCpfCnpj} from '@/lib/utils/document'

// Valor sentinela do select "todos os tipos" — o Radix Select não aceita "".
const ROLE_FILTER_ALL = '__all__'
const ROLE_FILTER_OPTIONS = [{value: ROLE_FILTER_ALL, label: 'Todos os tipos'}, ...PERSON_ROLE_OPTIONS]

function PersonsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()
  const [roleFilter, setRoleFilter] = useState<string>(ROLE_FILTER_ALL)
  const role = roleFilter === ROLE_FILTER_ALL ? undefined : (roleFilter as PersonRole)

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<PersonItemOut>({
      // O papel entra na chave: trocar o filtro é uma paginação nova, não a
      // continuação da anterior.
      queryKey: [...queryKeys.persons.list(selectedOrg?.pk), role ?? ROLE_FILTER_ALL],
      queryFn: (cursor) => apiClient.getPersons({cursor, role}),
      enabled: !!selectedOrg,
    })
  
  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<PersonItemOut>({
    mutationFn: (id) => apiClient.deletePerson(id),
    getId: (p) => unformatCpfCnpj(p.sk),
    getDeletedMessage: (p) => `"${p.name}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.persons.list(selectedOrg?.pk)})
    },
  })
  
  // Rows inside the undo window are hidden until the delete commits (or is undone).
  const visibleItems = filterVisible(items)

  const rowId = (p: PersonItemOut) => unformatCpfCnpj(p.sk)
  const selection = useRowSelection(visibleItems.map(rowId))
  const bulkDelete = () => {
    const byId = new Map(visibleItems.map((p) => [rowId(p), p]))
    selection.selectedIds.forEach((id) => {
      const p = byId.get(id)
      if (p) handleDelete(p)
    })
    selection.clear()
  }

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
        
        {selectedOrg && (
          <div className="mb-4 flex flex-col sm:flex-row sm:items-center gap-2">
            <span className="text-xs font-medium text-gray-600">Filtrar por tipo de cadastro</span>
            <div className="w-full sm:w-64">
              <OptionsSelect value={roleFilter} onValueChange={setRoleFilter} options={ROLE_FILTER_OPTIONS}/>
            </div>
          </div>
        )}

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title={role ? `Nenhuma pessoa cadastrada como "${PERSON_ROLE_LABELS[role]}"` : 'Nenhuma pessoa cadastrada'}
            description={role
              ? 'Marque esse tipo no cadastro das pessoas que devem aparecer aqui.'
              : 'Cadastre clientes e fornecedores para usar na emissão de documentos fiscais.'}
            action={{label: 'Nova pessoa', onClick: () => router.push('/persons/new')}}
            icon={<UsersIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Pessoas cadastradas"
            minWidth={560}
            headers={[
              {label: '__select', className: 'w-10', node: (
                <RowCheckbox
                  checked={selection.allSelected}
                  indeterminate={selection.someSelected}
                  onChange={selection.toggleAll}
                  ariaLabel="Selecionar todos"
                />
              )},
              'Nome', 'PF/PJ', 'Tipo de cadastro', 'Documento', 'Cidade / UF', {label: '', align: 'right'},
            ]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td className={TABLE_CELL}>
                  <RowCheckbox
                    checked={selection.isSelected(rowId(p))}
                    onChange={() => selection.toggle(rowId(p))}
                    ariaLabel={`Selecionar ${p.name}`}
                  />
                </td>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="PF/PJ" className={TABLE_CELL}>
                  <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                    docLabel(p.sk) === 'CPF'
                      ? 'bg-blue-50 text-blue-700'
                      : 'bg-purple-50 text-purple-700'
                  }`}>
                    {docLabel(p.sk)}
                  </span>
                </td>
                <td data-label="Tipo de cadastro" className={TABLE_CELL}>
                  {/* Uma pessoa multi-papel tem que ser visivelmente multi-papel,
                      senão o filtro parece estar errado. */}
                  {p.roles?.length ? (
                    <span className="flex flex-wrap gap-1">
                      {p.roles.map((r) => (
                        <span key={r}
                              className="inline-flex items-center rounded bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700">
                          {PERSON_ROLE_LABELS[r] ?? r}
                        </span>
                      ))}
                    </span>
                  ) : (
                    <span className="text-xs text-gray-500">—</span>
                  )}
                </td>
                <td data-label="Documento" className={`${TABLE_CELL} font-mono text-xs text-gray-600`}>{formatCpfCnpj(p.sk)}</td>
                <td data-label="Cidade / UF" className={`${TABLE_CELL} text-gray-600`}>
                  {p.person.addresses[0]?.city} / {p.person.addresses[0]?.state_federation}
                </td>
                <td className={`${TABLE_CELL} text-right`}>
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
                      className="text-danger hover:text-red-700"
                    >
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
        <BulkActionBar count={selection.count} onClear={selection.clear}>
          <Button
            variant="ghost"
            size="sm"
            onClick={bulkDelete}
            disabled={isDeleting}
            className="text-red-600 hover:text-red-700"
          >
            Excluir selecionados
          </Button>
        </BulkActionBar>
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
