'use client';

import {useState} from 'react';
import {Modal} from '@aoctech/ui';
import {Input} from '@/components/ui/input';
import {JustificationField} from '@/components/ui/justification-field';
import {BPE_JUSTIFICATION_MAX, BPE_JUSTIFICATION_MIN} from '@/lib/constants/bpe';

export type BpeEventKind = 'no-show' | 'seat-change' | 'excess-baggage' | 'supply-not-performed'

export type BpeEventValues = { justification: string; seat: string; quantity: string; value: string; ibs: string; cbs: string }

const TITLES: Record<BpeEventKind, string> = {
  'no-show': 'Registrar não embarque',
  'seat-change': 'Registrar alteração de poltrona',
  'excess-baggage': 'Registrar excesso de bagagem',
  'supply-not-performed': 'Registrar fornecimento não realizado',
};

const EMPTY: BpeEventValues = {justification: '', seat: '', quantity: '', value: '', ibs: '', cbs: ''};
const DECIMAL = /^\d+(\.\d+)?$/;

const VALID: Record<BpeEventKind, (v: BpeEventValues) => boolean> = {
  'no-show': (v) => v.justification.trim().length >= BPE_JUSTIFICATION_MIN,
  'seat-change': (v) => /^\d{1,3}$/.test(v.seat),
  'excess-baggage': (v) => v.quantity.trim().length > 0 && DECIMAL.test(v.value),
  'supply-not-performed': (v) => DECIMAL.test(v.ibs) && DECIMAL.test(v.cbs),
};

interface Props {
  kind: BpeEventKind | null
  docNumber: string | number
  loading: boolean
  error?: unknown
  onClose: () => void
  onConfirm: (kind: BpeEventKind, values: BpeEventValues) => void
}

/** Modal único dos eventos do BP-e além do cancelamento: cada tipo pede só os campos do seu leiaute. */
export function BpeEventModal({kind, docNumber, loading, error, onClose, onConfirm}: Props) {
  const [values, setValues] = useState<BpeEventValues>(EMPTY);
  const set = (k: keyof BpeEventValues) => (e: React.ChangeEvent<HTMLInputElement>) => setValues((v) => ({...v, [k]: e.target.value}));
  const close = () => { setValues(EMPTY); onClose(); };
  if (!kind) return null;
  const field = (id: string, label: string, k: keyof BpeEventValues, props: {inputMode?: 'numeric' | 'decimal'; maxLength?: number} = {}) => (
    <div>
      <label htmlFor={id} className="mb-1 block text-sm font-medium text-gray-700">{label}</label>
      <Input id={id} value={values[k]} onChange={set(k)} inputMode={props.inputMode} maxLength={props.maxLength}/>
    </div>
  );
  return (
    <Modal open title={`${TITLES[kind]} (BP-e nº ${docNumber})`} onClose={close} onSubmit={() => onConfirm(kind, values)}
           submitLabel="Registrar" cancelLabel="Voltar" loading={loading} submitDisabled={!VALID[kind](values)}>
      <div className="space-y-4">
        {kind === 'no-show' && (
          <JustificationField id="bpe-event-justification" value={values.justification}
                              onChange={(v) => setValues((s) => ({...s, justification: v}))}
                              minLength={BPE_JUSTIFICATION_MIN} maxLength={BPE_JUSTIFICATION_MAX}
                              placeholder="Descreva o motivo (mínimo 15 caracteres)…"/>
        )}
        {kind === 'seat-change' && field('bpe-event-seat', 'Nova poltrona', 'seat', {inputMode: 'numeric', maxLength: 3})}
        {kind === 'excess-baggage' && (
          <>
            {field('bpe-event-quantity', 'Quantidade (texto do leiaute, até 20 caracteres)', 'quantity', {maxLength: 20})}
            {field('bpe-event-value', 'Valor total cobrado (R$)', 'value', {inputMode: 'decimal'})}
          </>
        )}
        {kind === 'supply-not-performed' && (
          <>
            {field('bpe-event-ibs', 'IBS do serviço não fornecido (R$)', 'ibs', {inputMode: 'decimal'})}
            {field('bpe-event-cbs', 'CBS do serviço não fornecido (R$)', 'cbs', {inputMode: 'decimal'})}
          </>
        )}
        {error != null && <p className="text-xs text-red-600">{(error as Error)?.message ?? 'Erro ao registrar o evento.'}</p>}
      </div>
    </Modal>
  );
}
