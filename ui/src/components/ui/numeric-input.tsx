'use client'

import * as React from 'react'
import {cn} from '@/lib/utils'

interface NumericInputProps
  extends Omit<React.ComponentProps<'input'>, 'type' | 'onChange'> {
  decimal?: boolean
  decimalPlaces?: number
  integerPlaces?: number
  negative?: boolean
  prefix?: string
  suffix?: string
  onChange?: (value: string) => void
  debounceMs?: number
}

export const NumericInput = React.forwardRef<HTMLInputElement, NumericInputProps>(
  function NumericInput(
    {
      decimal = false,
      decimalPlaces,
      integerPlaces,
      negative = false,
      prefix,
      suffix,
      onChange,
      debounceMs,
      className,
      value,
      disabled,
      ...props
    },
    ref
  ) {
    const timerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null)
    React.useEffect(() => () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }, [])

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
      const passThrough = [
        'Backspace', 'Delete', 'Tab', 'Escape', 'Enter',
        'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End',
      ]
      if (passThrough.includes(e.key) || e.ctrlKey || e.metaKey) return
      if (e.key >= '0' && e.key <= '9') return
      const current = String(value ?? '')
      const pos = (e.target as HTMLInputElement).selectionStart ?? 0
      if (decimal && e.key === '.' && !current.includes('.')) return
      if (negative && e.key === '-' && !current.includes('-') && pos === 0) return
      e.preventDefault()
    }

    const fireChange = (raw: string) => {
      if (debounceMs !== undefined) {
        if (timerRef.current) clearTimeout(timerRef.current)
        timerRef.current = setTimeout(() => onChange?.(raw), debounceMs)
      } else {
        onChange?.(raw)
      }
    }

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      const raw = e.target.value
      if (raw === '' || (negative && raw === '-')) {
        fireChange(raw)
        return
      }
      const pattern = negative
        ? decimal ? /^-?\d*\.?\d*$/ : /^-?\d*$/
        : decimal ? /^\d*\.?\d*$/ : /^\d*$/
      if (!pattern.test(raw)) return
      const intPart = raw.replace(/^-/, '').split('.')[0] ?? ''
      if (integerPlaces !== undefined && intPart.length > integerPlaces) return
      if (decimal && decimalPlaces !== undefined && raw.includes('.')) {
        const after = raw.split('.')[1] ?? ''
        if (after.length > decimalPlaces) return
      }
      fireChange(raw)
    }

    const baseInputClass = cn(
      'h-8 min-w-0 flex-1 bg-transparent text-base md:text-sm outline-none',
      'placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50'
    )

    return (
      <div
        className={cn(
          'flex h-8 w-full min-w-0 items-center overflow-hidden rounded-lg border border-input bg-transparent transition-colors',
          'focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50',
          disabled && 'cursor-not-allowed opacity-50',
          className
        )}
      >
        {prefix && (
          <span
            className="flex h-full shrink-0 items-center border-r border-input bg-muted/30 px-2.5 text-sm text-muted-foreground select-none">
            {prefix}
          </span>
        )}
        <input
          ref={ref}
          type="text"
          inputMode={decimal ? 'decimal' : 'numeric'}
          value={value}
          disabled={disabled}
          onKeyDown={handleKeyDown}
          onChange={handleChange}
          className={cn(baseInputClass, 'px-2.5 py-1')}
          {...props}
          id={props.id ?? props.name}
        />
        {suffix && (
          <span
            className="flex h-full shrink-0 items-center border-l border-input bg-muted/30 px-2.5 text-sm text-muted-foreground select-none">
            {suffix}
          </span>
        )}
      </div>
    )
  }
)
