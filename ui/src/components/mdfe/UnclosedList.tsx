'use client';

import Link from 'next/link';
import {useQuery} from '@tanstack/react-query';
import {Button, EmptyState} from '@aoctech/ui';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {MdfeIcon} from '@/components/ui/icon';
import {LoadingSkeleton} from '@/components/ui/loading-skeleton';
import {useMdfeActions} from '@/components/mdfe/MdfeActions';

/** cStat 112 da MDFeConsNaoEnc: nenhum MDF-e do emitente pendente de encerramento. */
const CSTAT_NONE_UNCLOSED = '112';

/** Uma consulta à SEFAZ por minuto basta: a lista só muda quando um manifesto é emitido ou encerrado. */
const UNCLOSED_STALE_MS = 60_000;

/** nMDF está nas posições 26 a 34 da chave de acesso. */
const numberFromKey = (key: string) => Number(key.slice(25, 34));

/**
 * MDF-e não encerrados do emitente na SEFAZ (`GET /mdfes/unclosed`, MDFeConsNaoEnc), cada um com o atalho para o
 * encerramento (110112) que a lista e o detalhe já usam.
 */
export function UnclosedList({orgPk}: { orgPk: string }) {
  const {openClose, modals} = useMdfeActions(orgPk);
  const {data, isLoading, isFetching, error, refetch} = useQuery({
    queryKey: queryKeys.mdfes.unclosed(orgPk),
    queryFn: () => apiClient.getMdfeUnclosed(),
    staleTime: UNCLOSED_STALE_MS,
    retry: false,
  });

  const refresh = (
    <Button type="button" variant="outline" onClick={() => void refetch()} disabled={isFetching}
            className="min-h-11 sm:min-h-0">
      {isFetching ? 'Consultando…' : 'Consultar de novo'}
    </Button>
  );

  if (isLoading) return <LoadingSkeleton count={3} height="h-16" rounded="rounded-xl"/>;

  if (error) {
    return (
      <div role="alert" className="space-y-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-danger">
        <p>{error instanceof Error ? error.message : 'A consulta na SEFAZ falhou.'}</p>
        {refresh}
      </div>
    );
  }

  if (!data || data.c_stat === CSTAT_NONE_UNCLOSED || data.items.length === 0) {
    return (
      <EmptyState title="Nenhum MDF-e aberto" icon={<MdfeIcon width={20} height={20}/>}
                  description="A SEFAZ não tem manifesto deste emitente esperando encerramento."
                  action={refresh}/>
    );
  }

  return (
    <div className={`space-y-3 ${isFetching ? 'opacity-60' : ''}`}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-gray-700">
          {data.items.length === 1 ? '1 manifesto aberto' : `${data.items.length} manifestos abertos`} na SEFAZ.
          Encerre cada um no município onde a viagem terminou.
        </p>
        {refresh}
      </div>
      <ul className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white" aria-label="MDF-e não encerrados">
        {data.items.map((item) => {
          const number = numberFromKey(item.access_key);
          return (
            <li key={item.access_key} className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0">
                <p className="text-sm font-medium text-gray-900">MDF-e nº {number}</p>
                <p className="font-mono text-xs text-gray-500 break-all">{item.access_key}</p>
                <p className="text-xs text-gray-500">Protocolo <span className="font-mono">{item.n_prot}</span></p>
              </div>
              <div className="flex shrink-0 gap-2">
                <Button variant="ghost" render={<Link href={`/mdfe/detail?key=${item.access_key}`}/>}
                        className="min-h-11 sm:min-h-0">
                  Detalhes
                </Button>
                <Button type="button" variant="brand" className="min-h-11 sm:min-h-0"
                        onClick={() => openClose({sk: item.access_key, number})}>
                  Encerrar
                </Button>
              </div>
            </li>
          );
        })}
      </ul>
      {modals}
    </div>
  );
}
