'use client';

import {StatusBadge, StatusCell} from '@/components/ui/status-badge';
import {ContingencyBadge} from '@/components/dfe/ContingencyNotice';
import {
  type DfeGender,
  dfeStatusClasses,
  dfeStatusLabel,
  dfeStatusMotiveTitle,
  dfeStatusTone,
  isTransitionalDfeStatus,
} from '@/lib/data/dfe_status';

/**
 * Badge de status de DF-e — único para documento (NF-e/NFC-e/CT-e/MDF-e/NFS-e)
 * e para evento SEFAZ. Rótulo, tom e pulso vêm de `lib/data/dfe_status`.
 *
 * `gender` concorda com o substantivo: nota é feminina (default), manifesto,
 * conhecimento e evento são masculinos.
 */
export function DfeStatusBadge({status, gender, size}: {
  status: string
  gender?: DfeGender
  size?: 'sm' | 'md'
}) {
  return (
    <StatusBadge
      label={dfeStatusLabel(status, gender)}
      className={dfeStatusClasses(status)}
      isTransitional={isTransitionalDfeStatus(status)}
      size={size}
    />
  );
}

/** Badge + modal de motivo, quando o status é explicado por um motivo do SEFAZ/worker. */
export function DfeStatusCell({status, sefazMotive, gender, contingencyState}: {
  status: string
  sefazMotive: string | null
  gender?: DfeGender
  /** `contingency_state` do documento: acrescenta o selo da contingência sob o status. */
  contingencyState?: string | null
}) {
  const cell = (
    <StatusCell
      badge={<DfeStatusBadge status={status} gender={gender}/>}
      sefazMotive={sefazMotive}
      motiveTitle={dfeStatusMotiveTitle(status)}
      // O ícone segue o tom do badge: retentativa avisa (âmbar), rejeição alarma (vermelho).
      iconClassName={dfeStatusTone(status) === 'warning' ? 'text-warning' : 'text-red-600'}
    />
  );
  if (!contingencyState) return cell;
  return (
    <div className="flex flex-col items-start gap-1">
      {cell}
      <ContingencyBadge state={contingencyState} male={gender === 'm'}/>
    </div>
  );
}
