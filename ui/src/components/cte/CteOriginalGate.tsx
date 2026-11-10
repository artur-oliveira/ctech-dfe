'use client';

import type {ReactNode} from 'react';
import Link from 'next/link';
import {useSearchParams} from 'next/navigation';
import {useQuery} from '@tanstack/react-query';
import {Skeleton} from '@aoctech/ui';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {DfeStatusBadge} from '@/components/dfe/DfeStatusBadge';
import {CopyableCode} from '@/components/ui/copyable-code';
import {CTE_TAKER_LABELS} from '@/lib/constants/cte';
import {ACCESS_KEY_MOD_CTE, validateAccessKey} from '@/lib/utils/access-key';
import {cteChangeBlocker, sameDoc} from '@/lib/utils/cte';
import {orgTaxId} from '@/lib/utils/document';
import {formatCurrency} from '@/lib/utils/helpers';
import {formatDatetimeBR} from '@/lib/utils/dfe';
import type {CteDetailOut} from '@/lib/types/api';
import {CteDocKindBadge} from './CteKindBadge';
import {CTE_SURFACE} from './CteBlock';

const ERROR_BOX = 'rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700';

/** O CT-e original, em poucas linhas: o 1º passo do complemento e do substituto, e a recusa quando não dá para seguir. */
export function OriginalSummary({doc}: { doc: CteDetailOut }) {
  const place = (p: { city: string; uf: string } | null) => (p ? `${p.city} / ${p.uf}` : null);
  const route = [place(doc.origin), place(doc.destination)].filter(Boolean).join(' → ');
  return (
    <section aria-label="CT-e original" className={`${CTE_SURFACE} space-y-2`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-base font-semibold text-gray-900">
          CT-e original: {doc.number}<span className="ml-2 text-sm font-normal text-gray-400">série {doc.serie}</span>
        </h3>
        <div className="flex items-center gap-2">
          <CteDocKindBadge doc={doc}/>
          <DfeStatusBadge status={doc.status} gender="m"/>
        </div>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1">
        <CopyableCode value={doc.sk}/>
        <Link href={`/cte/detail?key=${doc.sk}`} className="text-sm font-medium text-brand-600 hover:text-brand-700">
          Ver o CT-e original
        </Link>
      </div>
      <dl className="grid grid-cols-1 gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
        <div><dt className="text-gray-500">Frete</dt><dd className="font-medium text-gray-900">{formatCurrency(doc.freight_total)}</dd></div>
        <div><dt className="text-gray-500">Emissão</dt><dd className="font-medium text-gray-900">{formatDatetimeBR(doc.dh_emi ?? doc.created_at)}</dd></div>
        {doc.taker && (
          <div>
            <dt className="text-gray-500">Tomador ({CTE_TAKER_LABELS[doc.taker.role] ?? doc.taker.role})</dt>
            <dd className="font-medium text-gray-900">{doc.taker.name || '-'}</dd>
          </div>
        )}
        {route && <div><dt className="text-gray-500">Trajeto</dt><dd className="font-medium text-gray-900">{route}</dd></div>}
      </dl>
    </section>
  );
}

/**
 * Carrega o CT-e original pela chave da URL (`?key=`), nunca digitada, e só entrega o formulário quando ele aceita
 * complemento ou substituto (o resumo do original aparece no 1º passo do formulário, `OriginalSummary`). Quem decide o resto (12 meses, prazo exato, limite de 10 complementos) é o servidor.
 */
export function CteOriginalGate({children}: { children: (original: CteDetailOut) => ReactNode }) {
  const key = useSearchParams().get('key') ?? '';
  const {selectedOrg} = useAuth();
  const valid = validateAccessKey(key, [ACCESS_KEY_MOD_CTE]).valid;
  const {data: doc, isLoading, error} = useQuery({
    queryKey: queryKeys.ctes.detail(key),
    queryFn: () => apiClient.getCte(key),
    enabled: valid && !!selectedOrg,
  });

  if (!key) return <p className="text-sm text-gray-500">Abra o CT-e na lista e escolha a ação no detalhe dele.</p>;
  if (!valid) return <p role="alert" className={ERROR_BOX}>A chave de acesso não é de um CT-e válido; abra o CT-e pela lista.</p>;
  if (isLoading) return <div className="space-y-4">{[...Array(2)].map((_, i) => <Skeleton key={i} className="h-24 rounded-xl"/>)}</div>;
  if (error || !doc) return <p role="alert" className={ERROR_BOX}>CT-e não encontrado.</p>;

  const isIssuer = !doc.emit_cpf_cnpj || sameDoc(doc.emit_cpf_cnpj, selectedOrg ? orgTaxId(selectedOrg) : '');
  const blocker = cteChangeBlocker(doc, isIssuer);
  return (
    <div className="max-w-3xl space-y-4">
      {blocker ? (
        <>
          <OriginalSummary doc={doc}/>
          <p role="alert" className={ERROR_BOX}>{blocker}</p>
        </>
      ) : children(doc)}
    </div>
  );
}
