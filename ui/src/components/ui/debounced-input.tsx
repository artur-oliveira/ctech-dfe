'use client'

import * as React from 'react'
import {Input} from '@/components/ui/input'

interface DebouncedInputProps
  extends Omit<React.ComponentProps<'input'>, 'onChange'> {
  value?: string
  onChange?: (value: string) => void
  debounceMs?: number
}

export function DebouncedInput({
                                 value = '',
                                 onChange,
                                 debounceMs = 300,
                                 ...props
                               }: DebouncedInputProps) {
  const [prevValue, setPrevValue] = React.useState(value)
  const [localValue, setLocalValue] = React.useState(value)
  if (prevValue !== value) {
    setPrevValue(value)
    setLocalValue(value)
  }

  const timerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null)
  React.useEffect(() => () => {
    if (timerRef.current) clearTimeout(timerRef.current)
  }, [])

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const next = e.target.value
    setLocalValue(next)
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => onChange?.(next), debounceMs)
  }

  return (
    <Input
      {...props}
      value={localValue}
      onChange={handleChange}
    />
  )
}
