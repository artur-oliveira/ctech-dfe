'use client';

import {Select} from '@aoctech/ui';
import {useFieldAria} from '@/components/ui/form';

interface OptionsSelectProps {
  value?: string | null
  onValueChange?: (value: string) => void
  options: { value: string; label: string; group?: string }[]
  placeholder?: string
  disabled?: boolean
  className?: string
  id?: string
  ariaLabel?: string
}

// The shared Select treats "" as "nothing chosen", but some lists have a real
// "" option ("Todos"). Swap it for a sentinel on the way in and out.
const EMPTY = '\u0000empty';

export function OptionsSelect({
                                value,
                                onValueChange,
                                options,
                                placeholder = 'Selecione',
                                disabled,
                                className,
                                id,
                                ariaLabel,
                              }: OptionsSelectProps) {
  const aria = useFieldAria(id);
  const hasEmpty = options.some((o) => o.value === '');
  const toInner = (v: string) => (v === '' && hasEmpty ? EMPTY : v);
  return (
    <Select
      id={id}
      value={toInner(value ?? '')}
      onValueChange={(v) => onValueChange?.(v === EMPTY ? '' : v)}
      options={options.map((o) => ({...o, value: toInner(o.value)}))}
      placeholder={placeholder}
      disabled={disabled}
      aria-label={ariaLabel}
      aria-invalid={aria['aria-invalid']}
      aria-describedby={aria['aria-describedby']}
      className={className}
    />
  );
}
