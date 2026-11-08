'use client';

import {OptionsSelect} from '@/components/ui/options-select';

const SEQUENCE_MAX = 20;
const SEQUENCE_OPTIONS = Array.from({length: SEQUENCE_MAX}, (_, i) => ({value: String(i + 1), label: `Sequência ${i + 1}`}));

/** Número de sequência opcional do evento (1 a 20); vazio deixa a api escolher o próximo livre. */
export function EventSequenceField({id, value, onChange}: { id: string; value: string; onChange: (v: string) => void }) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-xs font-medium text-gray-600">Número de sequência (opcional)</label>
      <OptionsSelect id={id} value={value} onValueChange={onChange} clearable
                     options={SEQUENCE_OPTIONS} placeholder="Automático: o próximo livre"/>
    </div>
  );
}
