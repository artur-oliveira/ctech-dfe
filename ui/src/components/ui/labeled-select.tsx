import {type SelectHTMLAttributes} from 'react'
import {cn} from '@/lib/utils'

interface LabeledSelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  error?: string
  options: { value: string; label: string }[]
}

export function LabeledSelect({
  label,
  error,
  options,
  className,
  ...props
}: LabeledSelectProps) {
  return (
    <div className={cn('mb-4', className)}>
      {label && (
        <label className="mb-1 block text-sm font-medium text-gray-700">{label}</label>
      )}
      <select
        className={cn(
          'w-full rounded-lg border px-3 py-2 text-sm font-medium text-black focus:outline-none focus:ring-2 focus:ring-primary-500 bg-white',
          error ? 'border-red-500' : 'border-gray-300'
        )}
        {...props}
      >
        <option value="">Selecione uma opção</option>
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
      {error && <p className="mt-1 text-sm text-red-600">{error}</p>}
    </div>
  )
}
