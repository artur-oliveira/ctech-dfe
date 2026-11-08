'use client';

import Link from 'next/link';
import {TriangleAlert} from 'lucide-react';
import {useAuth} from '@/lib/hooks/useAuth';
import {useContingencyDocs} from '@/lib/hooks/useFiscalConfig';
import {DOC_LABELS, fiscalConfigHref} from '@/lib/contingency';

/**
 * Aviso permanente do app enquanto algum documento emite fora da forma normal. Âmbar e discreto: a empresa
 * continua emitindo, só precisa lembrar de voltar. Leva à aba do primeiro documento afetado.
 */
export function ContingencyIndicator() {
  const {selectedOrg} = useAuth();
  const docs = useContingencyDocs(selectedOrg?.pk);
  if (docs.length === 0) return null;

  const names = docs.map((d) => DOC_LABELS[d]).join(', ');
  return (
    <Link
      href={fiscalConfigHref(docs[0])}
      title={`Em contingência: ${names}`}
      className="mx-2 flex min-h-11 shrink-0 items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50 px-2.5 text-xs font-medium text-amber-800 transition-colors hover:bg-amber-100 sm:min-h-0 sm:py-1.5"
    >
      <TriangleAlert size={14} aria-hidden="true" className="shrink-0"/>
      <span className="sr-only sm:not-sr-only">Em contingência: {names}</span>
    </Link>
  );
}
