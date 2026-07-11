'use client'

import {cn} from '@/lib/utils'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from './select'

interface OptionsSelectProps {
  value?: string | null
  onValueChange?: (value: string) => void
  options: { value: string; label: string; display?: string }[]
  placeholder?: string
  disabled?: boolean
  className?: string
  id?: string
}

export function OptionsSelect({
  value,
  onValueChange,
  options,
  placeholder = 'Selecione',
  disabled,
  className,
  id,
}: OptionsSelectProps) {
  const selected = value ? options.find((o) => o.value === value) : undefined
  const selectedLabel = selected?.display ?? selected?.label

  return (
    <Select
      value={value ?? null}
      onValueChange={(v) => { if (v !== null) onValueChange?.(v) }}
    >
      <SelectTrigger
        id={id}
        disabled={disabled}
        className={cn('w-full overflow-hidden', className)}
      >
        <SelectValue placeholder={placeholder} className="min-w-0 overflow-hidden">
          <span className="block w-full truncate">{selectedLabel}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {options.map((opt) => (
          <SelectItem key={opt.value} value={opt.value}>
            {opt.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
