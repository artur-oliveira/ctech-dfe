'use client';

import {Suspense, useState} from 'react';
import Link from 'next/link';
import {apiClient} from '@/lib/api/client';
import {useAuth} from '@/lib/hooks/useAuth';
import {usePagination} from '@/lib/hooks/usePagination';
import {useCiotEnvironment} from '@/lib/hooks/useCiotEnvironment';
import {useCiotPermissions} from '@/lib/hooks/useCiotPermissions';
import {queryKeys} from '@/lib/api/query-keys';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {Button, EmptyState} from '@aoctech/ui';
import {MdfeIcon} from '@/components/ui/icon';
import {NoOrgBanner} from '@/components/ui/no-org-banner';
import {NoPermission} from '@/components/ui/no-permission';
import {OptionsSelect} from '@/components/ui/options-select';
import {Pagination} from '@/components/ui/pagination';
import {LoadingSkeleton} from '@/components/ui/loading-skeleton';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell';
import {CiotStatusBadge} from '@/components/ciot/CiotStatusBadge';
import {CIOT_ENVIRONMENTS, CIOT_STATUS_FILTERS} from '@/lib/constants/ciot';
import {formatCpfCnpj} from '@/lib/utils/document';
import {formatCurrency} from '@/lib/utils/helpers';
import type {CiotEnvironment, CiotOut, CiotStatus} from '@/lib/types/api';

function formatDay(day?: string): string {
  return day ? new Date(`${day.slice(0, 10)}T12:00:00`).toLocaleDateString('pt-BR') : '-';
}

function CiotList({orgPk, environment, status}: { orgPk: string; environment: CiotEnvironment; status: '' | CiotStatus }) {
  const params = {environment};
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<CiotOut>({
    queryKey: queryKeys.ciots.list(orgPk, params),
    queryFn: (cursor) => apiClient.listCiots({...params, cursor}),
  });

  if (isLoading) return <LoadingSkeleton/>;

  // A api ainda não filtra por status: o filtro age sobre a página carregada.
  const shown = status ? items.filter((c) => c.status === status) : items;
  if (shown.length === 0) {
    return (
      <EmptyState
        title={status ? 'Nenhum CIOT com esse status' : 'Nenhum CIOT declarado'}
        icon={<MdfeIcon width={20} height={20}/>}
        description={status
          ? 'Troque o filtro ou carregue mais páginas.'
          : 'Declare o primeiro CIOT da organização para a viagem do transportador.'}/>
    );
  }

  return (
    <>
      <TableShell
        ariaLabel="CIOTs declarados"
        minWidth={150}
        dimmed={isFetching}
        headers={['CIOT', 'Transportador', 'Frete', 'Viagem', 'Status', {label: '', align: 'right'}]}
      >
        {shown.map((c) => (
          <tr key={c.id} className={TABLE_ROW}>
            <td className={TABLE_CELL} data-label="CIOT">
              {c.ciot
                ? <span className="font-mono font-medium text-gray-900">{c.ciot}</span>
                : <span className="text-gray-400 text-xs">Sem número ainda</span>}
            </td>
            <td className={`${TABLE_CELL} font-mono text-xs text-gray-600`} data-label="Transportador">
              {c.carrier_doc ? formatCpfCnpj(c.carrier_doc) : '-'}
            </td>
            <td className={`${TABLE_CELL} text-gray-700 whitespace-nowrap`} data-label="Frete">
              {c.freight_value ? formatCurrency(c.freight_value) : '-'}
            </td>
            <td className={`${TABLE_CELL} text-gray-500 whitespace-nowrap text-xs`} data-label="Viagem">
              {formatDay(c.trip_start)} a {formatDay(c.trip_end)}
            </td>
            <td className={TABLE_CELL} data-label="Status"><CiotStatusBadge ciot={c}/></td>
            <td className={`${TABLE_CELL} text-right`}>
              <Link href={`/ciot/detail?id=${encodeURIComponent(c.id)}`}
                    className="text-xs font-medium text-brand-600 hover:text-brand-700">
                Detalhes
              </Link>
            </td>
          </tr>
        ))}
      </TableShell>
      <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious} isLoading={isFetching}/>
    </>
  );
}

function CiotContent() {
  const {selectedOrg} = useAuth();
  const {canList, canCreate} = useCiotPermissions();
  const [environment, setEnvironment] = useCiotEnvironment();
  const [status, setStatus] = useState<'' | CiotStatus>('');

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center justify-between mb-4 gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">CIOT</h1>
            <p className="text-gray-500 text-sm mt-0.5">Código Identificador da Operação de Transporte (ANTT)</p>
          </div>
          {selectedOrg && canCreate && (
            <Button variant="brand" render={<Link href="/ciot/emit"/>}>
              <span className="text-base leading-none">+</span>
              Declarar CIOT
            </Button>
          )}
        </div>

        <HomologationBanner environment={environment === 'hom' ? 2 : 1}/>

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : !canList ? (
          <NoPermission what="listar CIOTs"/>
        ) : (
          <div className="space-y-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
              <div role="group" aria-label="Filtrar por status" className="flex flex-wrap gap-2">
                {CIOT_STATUS_FILTERS.map((f) => (
                  <button
                    key={f.value || 'all'}
                    type="button"
                    aria-pressed={status === f.value}
                    onClick={() => setStatus(f.value)}
                    className={`min-h-11 rounded-full border px-3 text-xs font-medium sm:min-h-0 sm:py-1.5 ${
                      status === f.value
                        ? 'border-brand-600 bg-brand-50 text-brand-700'
                        : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50'}`}
                  >
                    {f.label}
                  </button>
                ))}
              </div>
              <div className="w-full sm:w-48">
                <OptionsSelect ariaLabel="Ambiente" value={environment} options={CIOT_ENVIRONMENTS}
                               onValueChange={(v) => setEnvironment(v as CiotEnvironment)}/>
              </div>
            </div>
            <CiotList key={environment} orgPk={selectedOrg.pk} environment={environment} status={status}/>
          </div>
        )}
      </div>
    </RootLayout>
  );
}

export default function CiotPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <CiotContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
