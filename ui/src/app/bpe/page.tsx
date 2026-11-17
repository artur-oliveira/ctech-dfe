'use client';

import {Suspense, useState} from 'react';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {useRouter, useSearchParams} from 'next/navigation';
import Link from 'next/link';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {useAuth} from '@/lib/hooks/useAuth';
import {usePagination} from '@/lib/hooks/usePagination';
import {queryKeys} from '@/lib/api/query-keys';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {Button, EmptyState, PageHeader} from '@aoctech/ui';
import {NoOrgBanner} from '@/components/ui/no-org-banner';
import {Pagination} from '@/components/ui/pagination';
import {OptionsSelect} from '@/components/ui/options-select';
import {NumericInput} from '@/components/ui/numeric-input';
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton';
import {CancelDfeModal, CANCEL_JUSTIFICATION_MIN_LENGTH} from '@/components/dfe/CancelDfeModal';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {ConfigRequiredBanner} from '@/components/ui/config-required-banner';
import {LoadingSkeleton} from '@/components/ui/loading-skeleton';
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell';
import {DfeStatusCell} from '@/components/dfe/DfeStatusBadge';
import {TruckIcon} from '@/components/ui/icon';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import {dfeStatusOptions, NFSE_STATUSES} from '@/lib/data/dfe_status';
import {setDocStatusOptimistic} from '@/lib/utils/dfe-status';
import {formatCurrency} from '@/lib/utils/helpers';
import {formatDatetimeBR, triggerRemoteDownload} from '@/lib/utils/dfe';
import type {BpeListOut} from '@/lib/types/api';

const MONTHS = [
  {value: 1, label: 'Jan'}, {value: 2, label: 'Fev'}, {value: 3, label: 'Mar'},
  {value: 4, label: 'Abr'}, {value: 5, label: 'Mai'}, {value: 6, label: 'Jun'},
  {value: 7, label: 'Jul'}, {value: 8, label: 'Ago'}, {value: 9, label: 'Set'},
  {value: 10, label: 'Out'}, {value: 11, label: 'Nov'}, {value: 12, label: 'Dez'},
];
const CURRENT_YEAR = new Date().getFullYear();
const YEARS = Array.from({length: 5}, (_, i) => CURRENT_YEAR - i);
const STATUS_OPTIONS = [{value: '', label: 'Todos'}, ...dfeStatusOptions(NFSE_STATUSES)];
const PAGE_SIZE = 10;
const VARIANT_LABEL: Record<string, string> = {normal: 'Comum', tm: 'TM', ta: 'TA'};

/** Título do bilhete: trecho (origem para destino) no comum e no TA; competência e total de passageiros no TM. */
function ticketTitle(b: BpeListOut): string {
  if (b.variant === 'tm') return `Competência ${b.competence ?? '-'} · ${b.passengers ?? 0} passageiros`;
  return b.from_name && b.to_name ? `${b.from_name} para ${b.to_name}` : '-';
}

function BpesContent() {
  const {selectedOrg} = useAuth();
  const router = useRouter();
  const params = useSearchParams();
  const qc = useQueryClient();
  const [cancelTarget, setCancelTarget] = useState<BpeListOut | null>(null);
  const [justification, setJustification] = useState('');
  const [xmlLoading, setXmlLoading] = useState<string | null>(null);

  const orgPk = selectedOrg?.pk ?? '';
  const {config, isMissing} = useFiscalConfig('bpe', orgPk);

  const filterYear = params.get('year') ?? '';
  const filterMonth = params.get('month') ?? '';
  const numberSearch = params.get('number') ?? '';
  const filterStatus = params.get('status') ?? '';

  const setFilter = (key: string, value: string) => {
    const sp = new URLSearchParams(params.toString());
    if (value) sp.set(key, value); else sp.delete(key);
    if (key === 'year' && !value) sp.delete('month');
    router.replace(`/bpe?${sp.toString()}`, {scroll: false});
  };

  const queryParams = {
    sort: 'desc' as const,
    limit: PAGE_SIZE,
    ...(numberSearch ? {number: parseInt(numberSearch, 10)} : {}),
    ...(filterYear ? {year: parseInt(filterYear, 10)} : {}),
    ...(filterMonth ? {month: parseInt(filterMonth, 10)} : {}),
  };

  const {items: all, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<BpeListOut>({
    queryKey: queryKeys.bpes.list(orgPk, queryParams),
    queryFn: (cursor) => apiClient.getBpes({...queryParams, cursor}),
    enabled: !!orgPk,
  });
  // O filtro de status é aplicado na página carregada: a API lista por data, não por situação.
  const items = filterStatus ? all.filter((n) => n.status === filterStatus) : all;
  const hasFilters = numberSearch || filterYear || filterMonth || filterStatus;

  const cancelMutation = useMutation({
    mutationFn: ({key, reason}: { key: string; reason: string }) => apiClient.cancelBpe(key, reason),
    onSuccess: (_d, {key}) => {
      setCancelTarget(null);
      setJustification('');
      setDocStatusOptimistic(qc, queryKeys.bpes.lists(orgPk), key, 'cancel_pending');
      void qc.invalidateQueries({queryKey: queryKeys.bpes.detail(key)});
    },
  });

  const handleDownloadXml = async (item: BpeListOut) => {
    setXmlLoading(item.sk);
    try {
      triggerRemoteDownload((await apiClient.downloadBpeXml(item.sk)).url);
    } catch {
      toast.error('Erro ao baixar XML.');
    } finally {
      setXmlLoading(null);
    }
  };

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="BP-e"
          description="Bilhete de Passagem Eletrônico (comum, TM e TA)"
          action={selectedOrg ? <Button variant="brand" onClick={() => router.push('/bpe/emit')}><span aria-hidden className="text-base leading-none">+</span> Emitir BP-e</Button> : undefined}
        />
        <HomologationBanner environment={config?.environment}/>
        <ConfigRequiredBanner show={isMissing} variant="bpe" docLabel="BP-e"/>

        {!selectedOrg ? <NoOrgBanner/> : (
          <>
            <form onSubmit={(e) => e.preventDefault()} className="flex items-start gap-3 mb-5 flex-wrap">
              <div className="flex flex-col gap-1">
                <label htmlFor="bpe-filter-year" className="text-xs font-medium text-gray-600">Ano</label>
                <OptionsSelect id="bpe-filter-year" value={filterYear} onValueChange={(v) => setFilter('year', v)}
                               options={[{value: '', label: 'Todos'}, ...YEARS.map((y) => ({value: String(y), label: String(y)}))]}
                               placeholder="Todos" className="h-8 w-24 text-sm"/>
              </div>
              {filterYear && (
                <div className="flex flex-col gap-1">
                  <label htmlFor="bpe-filter-month" className="text-xs font-medium text-gray-600">Mês</label>
                  <OptionsSelect id="bpe-filter-month" value={filterMonth} onValueChange={(v) => setFilter('month', v)}
                                 options={[{value: '', label: 'Todos'}, ...MONTHS.map((m) => ({value: String(m.value), label: m.label}))]}
                                 placeholder="Todos" className="h-8 w-28 text-sm"/>
                </div>
              )}
              <div className="flex flex-col gap-1">
                <label htmlFor="bpe-filter-number" className="text-xs font-medium text-gray-600">Número</label>
                <NumericInput id="bpe-filter-number" value={numberSearch} onChange={(v) => setFilter('number', v)}
                              placeholder="Ex: 42" integerPlaces={9} debounceMs={300} className="w-28"/>
              </div>
              <div className="flex flex-col gap-1">
                <label htmlFor="bpe-filter-status" className="text-xs font-medium text-gray-600">Status</label>
                <OptionsSelect id="bpe-filter-status" value={filterStatus} onValueChange={(v) => setFilter('status', v)}
                               options={STATUS_OPTIONS} placeholder="Todos" className="h-8 w-36 text-sm"/>
              </div>
              {hasFilters && (
                <Button type="button" variant="outline" size="sm" onClick={() => router.replace('/bpe', {scroll: false})}
                        className="self-end text-gray-500 hover:text-gray-700">Limpar</Button>
              )}
            </form>

            {isLoading ? <LoadingSkeleton/> : items.length === 0 ? (
              <EmptyState title="Nenhum BP-e emitido" description="Emita o primeiro bilhete de passagem da organização."
                          action={<Button onClick={() => router.push('/bpe/emit')}>Emitir BP-e</Button>}
                          icon={<TruckIcon width={20} height={20}/>}/>
            ) : (
              <TableShell ariaLabel="BP-e emitidos" minWidth={150} dimmed={isFetching}
                          headers={['BP-e', 'Bilhete', 'Valor', 'Status', 'Emitido em', {label: '', align: 'right'}]}>
                {items.map((nf) => (
                  <tr key={nf.sk} className={TABLE_ROW}>
                    <td className={TABLE_CELL} data-label="BP-e">
                      <span className="font-mono font-medium text-gray-900">{nf.number}</span>
                      <span className="text-gray-400 text-xs ml-1">/ {nf.serie}</span>
                    </td>
                    <td className={TABLE_CELL} data-label="Bilhete">
                      <p className="font-medium text-gray-900 truncate max-w-50">{ticketTitle(nf)}</p>
                      <p className="text-xs text-gray-400">{VARIANT_LABEL[nf.variant] ?? nf.variant}{nf.passenger_name ? ` · ${nf.passenger_name}` : ''}</p>
                    </td>
                    <td className={`${TABLE_CELL} text-gray-700 whitespace-nowrap`} data-label="Valor">{formatCurrency(nf.total)}</td>
                    <td className={TABLE_CELL} data-label="Status"><DfeStatusCell status={nf.status} sefazMotive={nf.sefaz_motive}/></td>
                    <td className={`${TABLE_CELL} text-gray-500 whitespace-nowrap text-xs`} data-label="Emitido em">{formatDatetimeBR(nf.created_at)}</td>
                    <td className={`${TABLE_CELL} text-right`} data-label="Ações">
                      <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1">
                        <Link href={`/bpe/detail?key=${encodeURIComponent(nf.sk)}`}
                              className="flex items-center min-h-11 sm:min-h-0 text-xs font-medium text-brand-600 hover:text-brand-700">Detalhes</Link>
                        {nf.status === 'authorized' && (
                          <>
                            <Button variant="ghost" size="sm" onClick={() => handleDownloadXml(nf)} disabled={xmlLoading === nf.sk}
                                    className="text-brand-600 hover:text-brand-700">{xmlLoading === nf.sk ? 'Baixando…' : 'XML'}</Button>
                            {nf.variant !== 'tm' && <DownloadPdfButton fetchPdf={() => apiClient.downloadBpeDabpe(nf.sk)} label="DABPE"/>}
                            <Button variant="ghost" size="default" onClick={() => setCancelTarget(nf)}
                                    className="text-xs text-danger hover:text-red-700">Cancelar</Button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </TableShell>
            )}
            <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious} isLoading={isFetching}/>
          </>
        )}
      </div>

      <CancelDfeModal
        isOpen={cancelTarget !== null}
        docLabel="BP-e"
        docNumber={cancelTarget?.number ?? ''}
        justification={justification}
        onJustificationChange={setJustification}
        loading={cancelMutation.isPending}
        error={cancelMutation.error}
        onClose={() => { setCancelTarget(null); setJustification(''); }}
        onConfirm={() => {
          if (!cancelTarget || justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH) return;
          cancelMutation.mutate({key: cancelTarget.sk, reason: justification.trim()});
        }}
      />
    </RootLayout>
  );
}

export default function BpesPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <BpesContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
