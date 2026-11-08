'use client';

import {Input} from '@/components/ui/input';
import {NumericInput} from '@/components/ui/numeric-input';
import {
  DELIVERY_DOC_MAX, DELIVERY_HASH_LENGTH, DELIVERY_NAME_MAX, type ProofDraft, type ProofKind,
} from '@/lib/utils/delivery-events';

const LABEL = 'text-xs font-medium text-gray-600';

function Row({id, label, children, hint}: { id: string; label: string; children: React.ReactNode; hint?: string }) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className={LABEL}>{label}</label>
      {children}
      {hint && <p className="text-xs text-gray-500">{hint}</p>}
    </div>
  );
}

/**
 * Campos da prova de entrega, comuns ao comprovante da NF-e e do CT-e e ao insucesso do CT-e (que não leva
 * recebedor). Datas em `datetime-local`; o hash SHA-1 em Base64 vem do aplicativo de coleta.
 */
export function DeliveryProofFields({prefix, kind, value, onChange, keys}: {
  prefix: string; kind: ProofKind; value: ProofDraft; onChange: (patch: Partial<ProofDraft>) => void
  /** Notas do CT-e que a entrega cobre (opcional). */
  keys?: string[]
}) {
  const receiver = kind !== 'cte-failure';
  const hashRequired = kind !== 'nfe-receipt';
  return (
    <div className="space-y-3">
      <Row id={`${prefix}-when`} label={kind === 'nfe-receipt' ? 'Data e hora da entrega' : 'Data e hora da ocorrência'}>
        <Input id={`${prefix}-when`} type="datetime-local" className="h-11 w-full" value={value.when}
               onChange={(e) => onChange({when: e.target.value})}/>
      </Row>
      {receiver && (
        <>
          <Row id={`${prefix}-name`} label="Nome do recebedor">
            <Input id={`${prefix}-name`} className="h-11 w-full" maxLength={DELIVERY_NAME_MAX} value={value.name}
                   onChange={(e) => onChange({name: e.target.value})}/>
          </Row>
          <Row id={`${prefix}-doc`} label="Documento do recebedor">
            <Input id={`${prefix}-doc`} className="h-11 w-full" maxLength={DELIVERY_DOC_MAX} value={value.docNumber}
                   onChange={(e) => onChange({docNumber: e.target.value})}/>
          </Row>
        </>
      )}
      <Row id={`${prefix}-hash`} label="Hash do comprovante (SHA-1 em Base64)"
           hint={`${value.hash.length} de ${DELIVERY_HASH_LENGTH} caracteres`}>
        <Input id={`${prefix}-hash`} className="h-11 w-full font-mono" maxLength={DELIVERY_HASH_LENGTH} value={value.hash}
               autoComplete="off" spellCheck={false} onChange={(e) => onChange({hash: e.target.value.trim()})}/>
      </Row>
      <Row id={`${prefix}-hash-at`} label={hashRequired ? 'Data e hora do hash' : 'Data e hora do hash (opcional)'}>
        <Input id={`${prefix}-hash-at`} type="datetime-local" className="h-11 w-full" value={value.hashAt}
               onChange={(e) => onChange({hashAt: e.target.value})}/>
      </Row>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Row id={`${prefix}-lat`} label="Latitude (opcional)" hint="Ex.: -5.089200">
          <NumericInput id={`${prefix}-lat`} decimal negative integerPlaces={2} decimalPlaces={6}
                        value={value.latitude} onChange={(v) => onChange({latitude: v})}/>
        </Row>
        <Row id={`${prefix}-lon`} label="Longitude (opcional)" hint="Ex.: -42.801900">
          <NumericInput id={`${prefix}-lon`} decimal negative integerPlaces={3} decimalPlaces={6}
                        value={value.longitude} onChange={(v) => onChange({longitude: v})}/>
        </Row>
      </div>
      {keys && keys.length > 0 && (
        <fieldset className="space-y-1">
          <legend className={LABEL}>Notas cobertas pela entrega (opcional)</legend>
          {keys.map((k) => (
            <label key={k} className="flex min-h-11 items-center gap-2 text-xs text-gray-700">
              <input type="checkbox" className="size-5" checked={value.nfeKeys.includes(k)}
                     onChange={(e) => onChange({nfeKeys: e.target.checked ? [...value.nfeKeys, k] : value.nfeKeys.filter((x) => x !== k)})}/>
              <span className="min-w-0 break-all font-mono">{k}</span>
            </label>
          ))}
        </fieldset>
      )}
    </div>
  );
}
