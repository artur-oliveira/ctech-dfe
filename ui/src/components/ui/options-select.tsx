'use client';

import {Select} from '@aoctech/ui';
import {useClearable, useFieldAria} from '@/components/ui/form';
import {ClearButton, CLEARABLE_TRIGGER_PADDING} from '@/components/ui/clear-button';
import {cn} from '@/lib/utils';

interface OptionsSelectProps {
  value?: string | null
  onValueChange?: (value: string) => void
  options: { value: string; label: string; group?: string }[]
  placeholder?: string
  disabled?: boolean
  className?: string
  id?: string
  ariaLabel?: string
  clearable?: boolean // força o X; por padrão aparece sozinho em campo opcional de formulário (ver useClearable)
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
                                clearable: clearableProp,
                              }: OptionsSelectProps) {
  const aria = useFieldAria(id);
  const hasEmpty = options.some((o) => o.value === '');
  const toInner = (v: string) => (v === '' && hasEmpty ? EMPTY : v);
  const clearable = useClearable(clearableProp);
  const showClear = clearable && !!value && !disabled;
  return (
    <div className="relative min-w-0">
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
      className={cn(className, showClear && CLEARABLE_TRIGGER_PADDING)}
    />
    {showClear && <ClearButton onClear={() => onValueChange?.('')}/>}
    </div>
  );
}
