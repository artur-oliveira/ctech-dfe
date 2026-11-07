'use client';

import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button, Skeleton} from '@aoctech/ui';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {DfeStatusBadge} from '@/components/dfe/DfeStatusBadge';
import {dfeStatusLabel} from '@/lib/data/dfe_status';
import {EVENT_TYPE_LABELS} from '@/lib/data/dfe_event';
import {formatDatetimeBR} from '@/lib/utils/dfe';
import type {MdfeSefazSituation} from '@/lib/types/api';

/** Situação na SEFAZ que equivale a cada status local; `not_found` não tem equivalente. */
const LOCAL_FOR_SEFAZ: Record<MdfeSefazSituation, string | null> = {
  authorized: 'authorized',
  cancelled: 'cancelled',
  closed: 'closed',
  not_found: null,
};

/** Status locais que dizem que a SEFAZ tem o MDF-e; com eles, `not_found` também é divergência. */
const LOCAL_KNOWN_TO_SEFAZ = new Set(['authorized', 'cancelled', 'closed', 'close_pending', 'cancel_pending']);

/** A SEFAZ e o registro local discordam? Só compara; nenhuma reconciliação automática existe. */
export function sefazDiverges(sefaz: MdfeSefazSituation, local: string): boolean {
  const expected = LOCAL_FOR_SEFAZ[sefaz];
  return expected === null ? LOCAL_KNOWN_TO_SEFAZ.has(local) : expected !== local;
}

const sefazLabel = (s: MdfeSefazSituation) => s === 'not_found' ? 'Não encontrado na SEFAZ' : dfeStatusLabel(s, 'm');

/**
 * "Consultar na SEFAZ" do detalhe do MDF-e (`GET /mdfes/{key}/sefaz-status`, MDFeConsulta síncrona): situação,
 * protocolo e eventos registrados lá. Só lê; quando a SEFAZ e o registro local discordam, avisa, sem corrigir nada.
 */
export function MdfeSefazStatus({accessKey, localStatus}: { accessKey: string; localStatus: string }) {
  const [requested, setRequested] = useState(false);
  const {data, isFetching, error, refetch} = useQuery({
    queryKey: queryKeys.mdfes.sefazStatus(accessKey),
    queryFn: () => apiClient.getMdfeSefazStatus(accessKey),
    enabled: requested,
    retry: false,
    staleTime: 0,
  });

  const consult = () => (requested ? void refetch() : setRequested(true));

  return (
    <section aria-labelledby="mdfe-sefaz-title" className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 id="mdfe-sefaz-title" className="text-sm font-semibold text-gray-900">Situação na SEFAZ</h2>
          <p className="text-xs text-gray-500">Consulta direta no autorizador; não altera o registro aqui.</p>
        </div>
        <Button type="button" variant="outline" onClick={consult} disabled={isFetching}
                className="min-h-11 sm:min-h-0">
          {isFetching ? 'Consultando…' : data ? 'Consultar de novo' : 'Consultar na SEFAZ'}
        </Button>
      </div>

      {isFetching && !data && (
        <div className="space-y-2" aria-busy="true">
          <Skeleton className="h-5 w-40 rounded"/>
          <Skeleton className="h-4 w-full rounded"/>
        </div>
      )}

      {error && !isFetching && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-danger">
          {error instanceof Error ? error.message : 'A consulta na SEFAZ falhou.'}
        </p>
      )}

      {data && (
        <div className={`space-y-3 ${isFetching ? 'opacity-60' : ''}`}>
          <div className="flex flex-wrap items-center gap-2">
            {data.status === 'not_found'
              ? <span className="text-sm font-medium text-gray-900">{sefazLabel(data.status)}</span>
              : <DfeStatusBadge status={data.status} gender="m" size="md"/>}
            <span className="text-xs text-gray-500">cStat {data.c_stat} · {data.x_motivo}</span>
          </div>

          {sefazDiverges(data.status, localStatus) && (
            <div role="status" className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">
              A SEFAZ diz <b>{sefazLabel(data.status)}</b>, e aqui o MDF-e está{' '}
              <b>{dfeStatusLabel(localStatus, 'm')}</b>. Esta consulta não corrige o registro: confira os eventos
              abaixo antes de agir.
            </div>
          )}

          {data.protocol && (
            <p className="text-xs text-gray-500">
              Protocolo <span className="font-mono text-gray-700">{data.protocol.n_prot}</span>
              {data.protocol.dh_recbto && <> · {formatDatetimeBR(data.protocol.dh_recbto)}</>}
            </p>
          )}

          {data.events.length > 0 ? (
            <ul className="divide-y divide-gray-100 rounded-lg border border-gray-100" aria-label="Eventos na SEFAZ">
              {data.events.map((e) => (
                <li key={`${e.tp_evento}-${e.n_seq_evento}`} className="px-3 py-2 text-sm">
                  <p className="font-medium text-gray-900">
                    {EVENT_TYPE_LABELS[e.tp_evento] ?? e.tp_evento}
                    <span className="ml-2 font-mono text-xs text-gray-400">seq. {e.n_seq_evento}</span>
                  </p>
                  <p className="text-xs text-gray-500">
                    {e.x_motivo}
                    {e.n_prot && <> · protocolo <span className="font-mono">{e.n_prot}</span></>}
                    {e.dh_reg_evento && <> · {formatDatetimeBR(e.dh_reg_evento)}</>}
                  </p>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-xs text-gray-500">Nenhum evento registrado na SEFAZ.</p>
          )}
        </div>
      )}
    </section>
  );
}
