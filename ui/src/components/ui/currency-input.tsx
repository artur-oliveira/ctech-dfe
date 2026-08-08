'use client'

import * as React from 'react'
import {cn} from '@/lib/utils'

interface CurrencyInputProps
  extends Omit<React.ComponentProps<'input'>, 'type' | 'onChange' | 'value'> {
  value: string
  onChange?: (value: string) => void
  decimalPlaces?: number
  maxDecimalPlaces?: number
  allowZero?: boolean
}

export function CurrencyInput({
                                value,
                                onChange,
                                decimalPlaces = 2,
                                maxDecimalPlaces,
                                allowZero = true,
                                className,
                                disabled,
                                placeholder,
                                ...props
                              }: CurrencyInputProps) {
  const maxDec = maxDecimalPlaces ?? decimalPlaces
  const [focused, setFocused] = React.useState(false)
  const [editValue, setEditValue] = React.useState('')

  const numericValue = parseFloat(value)
  const isValid = value !== '' && !isNaN(numericValue)

  const formattedDisplay = isValid
    ? numericValue.toLocaleString('pt-BR', {
      minimumFractionDigits: decimalPlaces,
      maximumFractionDigits: maxDec,
    })
    : ''

  const displayValue = focused ? editValue : formattedDisplay

  const handleFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    setFocused(true)
    if (isValid) {
      const editStr = numericValue.toLocaleString('pt-BR', {
        minimumFractionDigits: 0,
        maximumFractionDigits: maxDec,
        useGrouping: false,
      }).replace('.', ',')
      setEditValue(editStr)
    } else {
      setEditValue('')
    }
    props.onFocus?.(e)
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    let raw = e.target.value.replace(/[^\d,]/g, '')
    const commaIdx = raw.indexOf(',')
    if (commaIdx !== -1) {
      const afterComma = raw.slice(commaIdx + 1).replace(/,/g, '')
      if (afterComma.length > maxDec) return
      raw = raw.slice(0, commaIdx + 1) + afterComma
    }
    setEditValue(raw)
    const asDecimal = raw.replace(',', '.')
    onChange?.(asDecimal === '.' ? '' : asDecimal)
  }

  const handleBlur = (e: React.FocusEvent<HTMLInputElement>) => {
    setFocused(false)
    const rawStr = editValue.replace(',', '.')
    const num = parseFloat(rawStr)
    if (editValue === '' || isNaN(num)) {
      onChange?.('')
    } else if (!allowZero && num === 0) {
      onChange?.('')
    } else {
      const dotIdx = rawStr.indexOf('.')
      const typedDecimals = dotIdx === -1 ? 0 : rawStr.length - dotIdx - 1
      onChange?.(num.toFixed(Math.max(typedDecimals, decimalPlaces)))
    }
    props.onBlur?.(e)
  }

  return (
    <div
      className={cn(
        'flex min-h-11 sm:min-h-0 sm:h-8 w-full min-w-0 items-center overflow-hidden rounded-lg border border-input bg-transparent transition-colors',
        'focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50',
        disabled && 'cursor-not-allowed opacity-50',
        className
      )}
    >
      <span
        className="flex h-full shrink-0 items-center border-r border-input bg-muted/30 px-2 text-sm text-muted-foreground select-none">
        R$
      </span>
      <input
        type="text"
        inputMode="decimal"
        value={displayValue}
        disabled={disabled}
        placeholder={placeholder ?? `0,${'0'.repeat(decimalPlaces)}`}
        onChange={handleChange}
        onFocus={handleFocus}
        onBlur={handleBlur}
        className="h-full min-w-0 flex-1 bg-transparent text-base md:text-sm outline-none px-2 py-1 placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50"
        {...props}
        id={props.id ?? props.name}
      />
    </div>
  )
}
