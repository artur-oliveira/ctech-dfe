'use client';

import type {ReactNode} from 'react';
import {useQueryClient} from '@tanstack/react-query';
import {useRouter} from 'next/navigation';
import {useAuth} from '@/lib/hooks/useAuth';
import {usePagination} from '@/lib/hooks/usePagination';
import {useEntityDelete} from '@/lib/hooks/useEntityDelete';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {Button, EmptyState, PageHeader} from '@aoctech/ui';
import {NoOrgBanner} from '@/components/ui/no-org-banner';
import {Pagination} from '@/components/ui/pagination';
import {LoadingSkeleton} from '@/components/ui/loading-skeleton';
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell';
import {extractId} from '@/lib/constants/entity-keys';
import type {PaginatedResponse} from '@/lib/types/api';

export interface RegistryColumn<T> {
  label: string
  render: (item: T) => ReactNode
}

export interface RegistryListPageProps<T extends { sk: string; name: string }> {
  title: string
  description: string
  newLabel: string
  newHref: string
  editHref: (id: string) => string
  emptyTitle: string
  emptyDescription: string
  icon: ReactNode
  ariaLabel: string
  skPrefix: string
  queryKey: (orgPk: string | undefined) => readonly unknown[]
  fetchPage: (cursor: string | undefined) => Promise<PaginatedResponse<T>>
  deleteItem: (id: string) => Promise<void>
  columns: RegistryColumn<T>[]
}

function Content<T extends { sk: string; name: string }>(p: RegistryListPageProps<T>) {
  const {selectedOrg} = useAuth();
  const router = useRouter();
  const qc = useQueryClient();

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} = usePagination<T>({
    queryKey: p.queryKey(selectedOrg?.pk),
    queryFn: (cursor) => p.fetchPage(cursor),
    enabled: !!selectedOrg,
  });

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<T>({
    mutationFn: (id) => p.deleteItem(id),
    getId: (e) => extractId(e.sk, p.skPrefix),
    getDeletedMessage: (e) => `"${e.name}" excluído`,
    onSuccess: () => {
      reset();
      void qc.invalidateQueries({queryKey: p.queryKey(selectedOrg?.pk)});
    },
  });

  const visibleItems = filterVisible(items);

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title={p.title}
          description={p.description}
          action={selectedOrg ? <Button variant="brand" onClick={() => router.push(p.newHref)}><span aria-hidden className="text-base leading-none">+</span> {p.newLabel}</Button> : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title={p.emptyTitle}
            description={p.emptyDescription}
            action={<Button onClick={() => router.push(p.newHref)}>{p.newLabel}</Button>}
            icon={p.icon}
          />
        ) : (
          <TableShell ariaLabel={p.ariaLabel} minWidth={520} headers={[...p.columns.map((c) => c.label), {label: '', align: 'right'}]}>
            {visibleItems.map((e) => (
              <tr key={e.sk} className={TABLE_ROW}>
                {p.columns.map((c, i) => (
                  <td key={c.label} data-label={c.label} className={`${TABLE_CELL} ${i === 0 ? 'font-medium text-gray-900' : 'text-gray-600'}`}>
                    {c.render(e)}
                  </td>
                ))}
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="sm" onClick={() => router.push(p.editHref(extractId(e.sk, p.skPrefix)))}
                            className="text-brand-600 hover:text-brand-700">
                      Editar
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => handleDelete(e)} disabled={isDeleting}
                            className="text-danger hover:text-red-700">
                      Excluir
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </TableShell>
        )}
        <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious} isLoading={isFetching}/>
      </div>
    </RootLayout>
  );
}

/** Lista de um cadastro da organização (nome, colunas próprias, editar e excluir), no padrão das demais telas de cadastro. */
export function RegistryListPage<T extends { sk: string; name: string }>(props: RegistryListPageProps<T>) {
  return (
    <ProtectedRoute>
      <Content {...props}/>
    </ProtectedRoute>
  );
}
