import {type InputHTMLAttributes} from 'react'
import {Input} from './input'
import {cn} from '@/lib/utils'

interface LabeledInputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  help?: string
}

export function LabeledInput({label, error, help, className, ...props}: LabeledInputProps) {
  return (
    <div className={cn('mb-4', className)}>
      {label && (
        <label className="mb-1 block text-sm font-medium text-gray-700">{label}</label>
      )}
      <Input
        className={error ? 'border-red-500' : undefined}
        aria-invalid={!!error}
        {...props}
      />
      {error && <p className="mt-1 text-sm text-red-600">{error}</p>}
      {help && <p className="mt-1 text-sm text-gray-500">{help}</p>}
    </div>
  )
}
