import {Textarea} from './textarea'

const DEFAULT_MIN_LENGTH = 15
const DEFAULT_MAX_LENGTH = 255

interface JustificationFieldProps {
  id?: string
  label?: string
  value: string
  onChange: (value: string) => void
  minLength?: number
  maxLength?: number
  rows?: number
  placeholder: string
}

/** Shared label + textarea + min-length hint + char counter for justification/motive fields. */
export function JustificationField({
  id,
  label = 'Justificativa',
  value,
  onChange,
  minLength = DEFAULT_MIN_LENGTH,
  maxLength = DEFAULT_MAX_LENGTH,
  rows = 4,
  placeholder,
}: JustificationFieldProps) {
  const trimmedLength = value.trim().length
  const tooShort = trimmedLength > 0 && trimmedLength < minLength

  return (
    <div>
      <label htmlFor={id} className="block text-sm font-medium text-gray-700 mb-1.5">{label}</label>
      <Textarea
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        rows={rows}
        maxLength={maxLength}
        placeholder={placeholder}
        aria-invalid={tooShort}
        className="resize-none"
      />
      <div className="flex justify-between mt-1">
        {tooShort && (
          <p className="text-xs text-red-600">
            Mínimo {minLength} caracteres ({minLength - trimmedLength} restantes)
          </p>
        )}
        <p className="text-xs text-gray-400 ml-auto">{value.length}/{maxLength}</p>
      </div>
    </div>
  )
}
