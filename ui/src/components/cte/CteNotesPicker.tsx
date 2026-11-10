'use client';

import {useMemo} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button, Skeleton} from '@aoctech/ui';
import {Combobox} from '@/components/ui/combobox';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {CTE_MAX_NOTES, CTE_RECENT_NOTES_LIMIT} from '@/lib/constants/cte';
import {cteSummary, formatWeightKg, type CteOverrides} from '@/lib/utils/cte';
import {formatCurrency} from '@/lib/utils/helpers';
import type {CteDeriveOut, CteDeriveWarning, NfeListOut} from '@/lib/types/api';
import {CteSection, CteWarnings} from './CteBlock';

const AUTHORIZED = 'authorized';

const issuedParams = {sort: 'desc' as const, incoming: 0 as const, limit: CTE_RECENT_NOTES_LIMIT};
const receivedParams = {sort: 'desc' as const, incoming: 1 as const, limit: CTE_RECENT_NOTES_LIMIT};

interface Props {
  keys: string[]
  onChange: (keys: string[]) => void
  derive: CteDeriveOut | undefined
  deriving: boolean
  error: string | null
  onRetry: () => void
  overrides: CteOverrides
  warnings: CteDeriveWarning[]
}

/** Bloco 1: quais notas viajam neste CT-e. NF-e autorizadas recentes, emitidas e recebidas. */
export function CteNotesPicker({keys, onChange, derive, deriving, error, onRetry, overrides, warnings}: Props) {
  const {selectedOrg} = useAuth();
  const issued = useQuery({
    queryKey: queryKeys.nfes.list(selectedOrg?.pk, issuedParams),
    queryFn: () => apiClient.getNfes(issuedParams),
    enabled: !!selectedOrg,
  });
  const received = useQuery({
    queryKey: queryKeys.nfes.list(selectedOrg?.pk, receivedParams),
    queryFn: () => apiClient.getNfes(receivedParams),
    enabled: !!selectedOrg,
  });

  const byKey = useMemo(() => {
    const map = new Map<string, { nfe: NfeListOut; source: string }>();
    (issued.data?.items ?? []).filter((n) => n.status === AUTHORIZED)
      .forEach((nfe) => map.set(nfe.sk, {nfe, source: 'Emitida'}));
    (received.data?.items ?? []).filter((n) => n.status === AUTHORIZED)
      .forEach((nfe) => map.set(nfe.sk, {nfe, source: 'Recebida'}));
    return map;
  }, [issued.data, received.data]);

  const options = [...byKey.values()]
    .filter(({nfe}) => !keys.includes(nfe.sk))
    .map(({nfe, source}) => ({
      value: nfe.sk,
      label: `Nº ${nfe.number} · ${nfe.emit_name} para ${nfe.dest_name} · ${formatCurrency(nfe.total)} · ${source} · ${nfe.sk}`,
    }));

  const loadingList = issued.isLoading || received.isLoading;
  const atMax = keys.length >= CTE_MAX_NOTES;
  const derivedNote = (key: string) => derive?.notes.find((n) => n.access_key === key);

  return (
    <CteSection id="cte-notes" title="Quais notas vão no CT-e?"
                hint="Remetente, destinatário, carga e trajeto saem delas.">
      <Combobox
        id="cte-notes-picker"
        value=""
        fuzzySearch
        disabled={atMax || loadingList}
        options={options}
        placeholder={atMax ? `Limite de ${CTE_MAX_NOTES} notas alcançado`
          : loadingList ? 'Carregando notas…' : 'Adicionar nota autorizada'}
        searchPlaceholder="Número, nome ou chave..."
        onValueChange={(v) => {
          if (v && !keys.includes(v)) onChange([...keys, v]);
        }}
      />

      {keys.length > 0 && (
        <ul aria-label="Notas escolhidas" className="max-h-48 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200">
          {keys.map((key) => {
            const n = derivedNote(key);
            const listed = byKey.get(key)?.nfe;
            const number = n?.number ?? listed?.number;
            const from = n?.emitter.name ?? listed?.emit_name;
            const to = n?.recipient.name ?? listed?.dest_name;
            const value = n?.value ?? listed?.total;
            return (
              <li key={key} className="flex items-center gap-3 px-3 py-2">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-gray-900">
                    Nº {number ?? '…'} · {from ?? '…'} para {to ?? '…'}
                  </p>
                  <p className="text-xs text-gray-500">
                    {value ? formatCurrency(value) : '…'}
                    {' · '}
                    {deriving && !n ? <span aria-label="Lendo peso" className="inline-block h-3 w-12 animate-pulse rounded bg-gray-200 align-middle motion-reduce:animate-none"/>
                      : n?.weight_kg ? formatWeightKg(n.weight_kg) : 'sem peso'}
                  </p>
                </div>
                <Button type="button" variant="ghost" onClick={() => onChange(keys.filter((k) => k !== key))}
                        className="shrink-0 text-danger hover:text-red-700"
                        aria-label={`Remover nota ${number ?? key}`}>
                  Remover
                </Button>
              </li>
            );
          })}
        </ul>
      )}

      {keys.length > 0 && (
        error ? (
          <div role="alert" className="flex flex-col gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-danger sm:flex-row sm:items-center sm:justify-between">
            <span>{error}</span>
            <Button type="button" variant="outline" onClick={onRetry}>Tentar de novo</Button>
          </div>
        ) : derive ? (
          <p data-testid="cte-summary"
             className={`rounded-lg bg-gray-50 px-3 py-2 text-sm font-medium text-gray-800 transition-opacity ${deriving ? 'opacity-60' : ''}`}>
            {cteSummary(derive, overrides)}
          </p>
        ) : (
          <Skeleton className="h-9 rounded-lg"/>
        )
      )}

      <CteWarnings warnings={warnings}/>
    </CteSection>
  );
}
