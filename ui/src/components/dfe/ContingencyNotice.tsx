'use client';

import {StatusBadge} from '@/components/ui/status-badge';
import {dfeToneClasses} from '@/lib/data/dfe_status';
import {type ContingencyDocFields, type ContingencyState, contingencyLabel, contingencyMeta, contingencyNotice} from '@/lib/contingency';
import {formatDatetimeBR} from '@/lib/utils/dfe';

/** Selo do estado de contingência de um documento (EPEC/offline); nada quando o documento não passou por ela. */
export function ContingencyBadge({state, male = false, size}: {
  state?: string | null
  male?: boolean
  size?: 'sm' | 'md'
}) {
  const meta = contingencyMeta(state);
  if (!meta) return null;
  return <StatusBadge label={contingencyLabel(state as ContingencyState, male)} className={dfeToneClasses(meta.tone)} size={size}/>;
}

const NOTICE_BOX: Record<'warning' | 'success' | 'danger', string> = {
  warning: 'border-amber-200 bg-amber-50 text-amber-800',
  success: 'border-green-200 bg-green-50 text-green-800',
  danger: 'border-red-200 bg-red-50 text-red-800',
};

/** Aviso do detalhe: o que aconteceu com o documento emitido em contingência e até quando ele pode ser transmitido. */
export function ContingencyNotice({doc, male = false}: { doc: ContingencyDocFields; male?: boolean }) {
  const meta = contingencyMeta(doc.contingency_state);
  if (!meta || !doc.contingency_state) return null;
  const deadline = doc.contingency_deadline ? formatDatetimeBR(doc.contingency_deadline) : undefined;
  return (
    <div role="status" className={`rounded-lg border px-4 py-3 text-sm text-pretty ${NOTICE_BOX[meta.tone]}`}>
      {contingencyNotice(doc.contingency_state, male, deadline)}
    </div>
  );
}
