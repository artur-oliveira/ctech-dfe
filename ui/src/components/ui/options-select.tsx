'use client'

import {cn} from '@/lib/utils'
import {Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue,} from './select'

interface OptionsSelectProps {
  value?: string | null
  onValueChange?: (value: string) => void
  options: { value: string; label: string; display?: string; group?: string }[]
  placeholder?: string
  disabled?: boolean
  className?: string
  id?: string
  ariaLabel?: string
}

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
  const selected = value ? options.find((o) => o.value === value) : undefined
  const selectedLabel = selected?.display ?? selected?.label

  // Group options by their `group` field, preserving first-seen group order.
  // Falls back to one ungrouped list when no option declares a group.
  const groups = new Map<string | undefined, typeof options>()
  for (const opt of options) {
    const key = opt.group
    const bucket = groups.get(key)
    if (bucket) bucket.push(opt)
    else groups.set(key, [opt])
  }
  const isGrouped = groups.size > 1 || (groups.size === 1 && [...groups.keys()][0] !== undefined)

  return (
    <Select
      value={value ?? null}
      onValueChange={(v) => {
        if (v !== null) onValueChange?.(v)
      }}
    >
      <SelectTrigger
        id={id}
        disabled={disabled}
        aria-label={ariaLabel}
        className={cn('w-full overflow-hidden', className)}
      >
        <SelectValue placeholder={placeholder} className="min-w-0 overflow-hidden">
          <span className="block w-full truncate">{selectedLabel}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {isGrouped
          ? [...groups.entries()].map(([group, opts]) => (
            <SelectGroup key={group ?? '__ungrouped'}>
              {group && <SelectLabel>{group}</SelectLabel>}
              {opts.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectGroup>
          ))
          : options.map((opt) => (
            <SelectItem key={opt.value} value={opt.value}>
              {opt.label}
            </SelectItem>
          ))}
      </SelectContent>
    </Select>
  )
}
