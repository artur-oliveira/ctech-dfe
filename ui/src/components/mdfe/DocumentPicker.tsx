'use client';

import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button} from '@aoctech/ui';
import {apiClient} from '@/lib/api/client';
import {useAuth} from '@/lib/hooks/useAuth';
import {useDebounce} from '@/lib/hooks/useDebounce';
import {queryKeys} from '@/lib/api/query-keys';
import {LoadingSkeleton} from '@/components/ui/loading-skeleton';
import {NumericInput} from '@/components/ui/numeric-input';
import {formatCpfCnpj} from '@/lib/utils/document';
import {formatCurrency} from '@/lib/utils/helpers';
import type {NfeListOut} from '@/lib/types/api';

/** NF-e autorizadas (emitidas ou recebidas) para o MDF-e: a emissão e a inclusão de DF-e escolhem daqui, nunca digitam a chave. */
export function DocumentPicker({selected, onToggle}: {
  selected: NfeListOut[]
  onToggle: (n: NfeListOut) => void
}) {
  const {selectedOrg} = useAuth();
  const [incoming, setIncoming] = useState<0 | 1>(0);
  const [numberSearch, setNumberSearch] = useState('');
  const debouncedNumber = useDebounce(numberSearch, 300);

  const params = {
    sort: 'desc' as const,
    incoming,
    limit: 50,
    ...(debouncedNumber ? {number: parseInt(debouncedNumber, 10)} : {}),
  };

  const {data, isLoading} = useQuery({
    queryKey: queryKeys.nfes.list(selectedOrg?.pk, params),
    queryFn: () => apiClient.getNfes(params),
    enabled: !!selectedOrg,
  });

  // Only authorized NF-es have an XML available for manifestation.
  const items = (data?.items ?? []).filter((n) => n.status === 'authorized');
  const selectedKeys = new Set(selected.map((s) => s.sk));

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Button type="button" size="sm" variant={incoming === 0 ? 'brand' : 'outline'}
                onClick={() => setIncoming(0)}>Emitidas</Button>
        <Button type="button" size="sm" variant={incoming === 1 ? 'brand' : 'outline'}
                onClick={() => setIncoming(1)}>Recebidas</Button>
        <div className="ml-auto w-32">
          <NumericInput value={numberSearch} onChange={setNumberSearch} integerPlaces={9}
                        placeholder="Nº da nota" className="w-full"/>
        </div>
      </div>

      <div className="max-h-72 overflow-y-auto rounded-lg border border-gray-200 divide-y divide-gray-100">
        {isLoading ? (
          <div className="p-3">
            <LoadingSkeleton count={4} height="h-10" rounded="rounded-md"/>
          </div>
        ) : items.length === 0 ? (
          <p className="p-4 text-sm text-gray-500 text-center">Nenhuma NF-e autorizada encontrada.</p>
        ) : (
          items.map((n) => {
            const checked = selectedKeys.has(n.sk);
            const counterparty = incoming === 0 ? n.dest_name : n.emit_name;
            const doc = incoming === 0 ? n.dest_cpf_cnpj : n.emit_cpf_cnpj;
            return (
              <button key={n.sk} type="button" onClick={() => onToggle(n)}
                      className={`flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors ${checked ? 'bg-brand-50' : 'hover:bg-gray-50'}`}>
                <span
                  className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border ${checked ? 'border-brand-600 bg-brand-600 text-white' : 'border-gray-300'}`}>
                  {checked && <span className="text-xs leading-none">✓</span>}
                </span>
                <span className="flex-1 min-w-0">
                  <span className="block text-sm font-medium text-gray-900 truncate">
                    Nº {n.number} · {counterparty || 'Sem destinatário'}
                  </span>
                  {doc && <span className="block text-xs text-gray-400 font-mono">{formatCpfCnpj(doc)}</span>}
                </span>
                <span className="text-xs text-gray-500 shrink-0">{formatCurrency(n.total)}</span>
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}
