'use client';

import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button} from '@aoctech/ui';
import {apiClient, ApiError} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {CIOT_FLEET_MAX_PLATES} from '@/lib/constants/ciot';
import {summarizeAntt} from '@/lib/utils/ciot';
import type {CiotCarrierResult, CiotEnvironment} from '@/lib/types/api';

/** Resposta da ANTT lida sem presumir o formato: o código decide o tom, os campos aparecem como vieram. */
export function AnttResultView({data, error}: { data?: CiotCarrierResult; error: unknown }) {
  if (error) {
    // Consulta é apoio: a ANTT fora do ar não impede declarar.
    const down = error instanceof ApiError && error.status === 503;
    return (
      <p role="alert" className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
        {down ? 'A ANTT está indisponível agora. Você pode seguir com a declaração e consultar de novo depois.'
          : error instanceof Error ? error.message : 'Não foi possível consultar a ANTT.'}
      </p>
    );
  }
  if (!data) return null;
  const s = summarizeAntt(data.result);
  return (
    <div role="status" className={`rounded-lg border px-3 py-2 text-xs space-y-1 ${s.ok ? 'border-green-200 bg-green-50 text-green-900' : 'border-red-200 bg-red-50 text-red-900'}`}>
      <p className="font-semibold">
        {s.ok ? 'Consulta realizada na ANTT' : 'A ANTT não confirmou'}
        {data.cached && <span className="ml-2 font-normal text-gray-500">(resposta guardada há menos de 1 hora)</span>}
      </p>
      {s.message && <p>{s.message}</p>}
      {s.fields.length > 0 && (
        <dl className="grid grid-cols-1 gap-x-4 gap-y-0.5 sm:grid-cols-2">
          {s.fields.map((f) => (
            <div key={f.label} className="flex gap-1.5">
              <dt className="text-gray-500">{f.label}:</dt>
              <dd className="font-medium wrap-break-word">{f.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  );
}

/** Situação do transportador na ANTT (RNTRC, tipo, elegibilidade). Consulta só quando a pessoa pede. */
export function CarrierStatusLookup({doc, environment, onRntrc}: {
  doc: string
  environment: CiotEnvironment
  /** Chamado com o RNTRC lido da resposta, para preencher o campo. */
  onRntrc: (rntrc: string) => void
}) {
  const [requested, setRequested] = useState(false);
  const query = useQuery({
    queryKey: queryKeys.ciots.carrierStatus(environment, doc),
    queryFn: () => apiClient.getCiotCarrierStatus(doc, environment),
    enabled: requested && !!doc,
    retry: false,
  });

  const lookup = async () => {
    setRequested(true);
    const res = await query.refetch();
    const rntrc = summarizeAntt(res.data?.result).rntrc;
    if (rntrc) onRntrc(rntrc);
  };

  return (
    <div className="space-y-2">
      <Button type="button" variant="outline" size="sm" onClick={lookup} disabled={!doc || query.isFetching}
              className="min-h-11 sm:min-h-0">
        {query.isFetching ? 'Consultando…' : 'Consultar situação na ANTT'}
      </Button>
      {requested && <AnttResultView data={query.data} error={query.error}/>}
    </div>
  );
}

/** Conferência das placas escolhidas contra a frota do transportador (ao vivo, sem cache, de 1 a 7 placas). */
export function FleetCheck({doc, plates, environment}: { doc: string; plates: string[]; environment: CiotEnvironment }) {
  const [requested, setRequested] = useState('');
  const key = plates.join(',');
  const tooMany = plates.length > CIOT_FLEET_MAX_PLATES;
  const query = useQuery({
    queryKey: queryKeys.ciots.carrierFleet(environment, doc, requested),
    queryFn: () => apiClient.getCiotCarrierFleet(doc, requested.split(','), environment),
    enabled: !!requested && !!doc,
    retry: false,
  });

  return (
    <div className="space-y-2">
      <Button type="button" variant="outline" size="sm" onClick={() => setRequested(key)}
              disabled={!doc || plates.length === 0 || tooMany || query.isFetching}
              className="min-h-11 sm:min-h-0">
        {query.isFetching ? 'Conferindo…' : 'Conferir frota na ANTT'}
      </Button>
      {!doc && <p className="text-xs text-gray-500">Escolha o transportador para conferir a frota.</p>}
      {requested && requested === key && <AnttResultView data={query.data} error={query.error}/>}
      {requested && requested !== key && (
        <p className="text-xs text-gray-500">As placas mudaram; consulte de novo.</p>
      )}
    </div>
  );
}
