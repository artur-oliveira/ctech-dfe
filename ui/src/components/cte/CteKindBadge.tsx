import {CTE_KIND_LABELS, type CteKind} from '@/lib/constants/cte';
import {cteKind} from '@/lib/utils/cte';
import type {CteListOut} from '@/lib/types/api';

/**
 * Espécie do documento (normal, complemento, substituto, CT-e OS). Fora da paleta de status: o tom da marca
 * distingue o que não é um CT-e normal sem competir com "Autorizado" ou "Rejeitado".
 */
export function CteKindBadge({kind, size = 'sm'}: { kind: CteKind; size?: 'sm' | 'md' }) {
  const sizeClass = size === 'md' ? 'px-2.5 py-1 text-sm' : 'px-2 py-0.5 text-xs';
  const tone = kind === 'normal' ? 'bg-gray-100 text-gray-600' : 'bg-brand-50 text-brand-700';
  return (
    <span data-testid="cte-kind" className={`inline-flex items-center rounded font-medium ${sizeClass} ${tone}`}>
      {CTE_KIND_LABELS[kind]}
    </span>
  );
}

export const CteDocKindBadge = ({doc, size}: { doc: Pick<CteListOut, 'cte_type' | 'model'>; size?: 'sm' | 'md' }) => (
  <CteKindBadge kind={cteKind(doc)} size={size}/>
);
