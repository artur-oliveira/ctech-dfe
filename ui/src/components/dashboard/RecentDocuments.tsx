'use client';

import Link from 'next/link';
import {useQuery} from '@tanstack/react-query';
import {Skeleton} from '@aoctech/ui';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {DfeStatusBadge} from '@/components/dfe/DfeStatusBadge';
import {formatCurrency} from '@/lib/utils/helpers';
import {formatDatetimeBR} from '@/lib/utils/dfe';

const RECENT_LIMIT = 5;
// incoming 0 = emitidas pela empresa (mesma aba "Emitidas" da lista de NF-e).
const RECENT_PARAMS = {sort: 'desc' as const, limit: RECENT_LIMIT, incoming: 0 as const};

/** Últimas NF-e emitidas com o status; some quando não há nenhuma. */
export function RecentDocuments() {
  const {selectedOrg} = useAuth();
  const orgPk = selectedOrg?.pk;
  const {data, isLoading} = useQuery({
    queryKey: queryKeys.nfes.list(orgPk, RECENT_PARAMS),
    queryFn: () => apiClient.getNfes(RECENT_PARAMS),
    enabled: !!orgPk,
  });
  const items = data?.items ?? [];

  if (!orgPk || (!isLoading && items.length === 0)) return null;

  return (
    <section className="mb-10" aria-labelledby="recent-docs-title">
      <div className="mb-4 flex items-center justify-between">
        <h2 id="recent-docs-title" className="text-sm font-semibold text-gray-500 uppercase tracking-wider">
          Recentes
        </h2>
        <Link href="/nfe" className="flex min-h-11 items-center text-xs font-medium text-brand-600 hover:text-brand-700 sm:min-h-0">
          Ver todas
        </Link>
      </div>
      <ul className="divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white">
        {isLoading
          ? Array.from({length: 3}, (_, i) => (
            <li key={i} className="px-4 py-3"><Skeleton className="h-5 w-full"/></li>
          ))
          : items.map((nfe) => (
            <li key={nfe.sk}>
              <Link href={`/nfe/detail?key=${nfe.sk}`}
                    className="flex min-h-11 flex-wrap items-center justify-between gap-x-3 gap-y-1 px-4 py-3 hover:bg-gray-50">
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium text-gray-900">
                    NF-e {nfe.number} {nfe.dest_name ? `· ${nfe.dest_name}` : ''}
                  </span>
                  <span className="block text-xs text-gray-500">
                    {formatDatetimeBR(nfe.dh_emi ?? nfe.created_at)} · {formatCurrency(nfe.total)}
                  </span>
                </span>
                <DfeStatusBadge status={nfe.status}/>
              </Link>
            </li>
          ))}
      </ul>
    </section>
  );
}
