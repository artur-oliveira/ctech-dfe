'use client'

import {useId} from 'react'
import {Input} from '@/components/ui/input'

export interface CertificateFieldsProps {
  file: File | null
  onFileChange: (file: File | null) => void
  password: string
  onPasswordChange: (password: string) => void
  fileError?: string | null
  passwordError?: string | null
  /** Optional intro text shown above the fields (e.g. the KYC explanation). */
  hint?: string
}

/**
 * A1 certificate picker (PFX/P12 file + password), shared by the standalone
 * certificate upload modal and the organization creation form so the markup
 * and accept filter live in one place. Uses native labels (no react-hook-form
 * context) so it works anywhere.
 */
export function CertificateFields({
  file,
  onFileChange,
  password,
  onPasswordChange,
  fileError,
  passwordError,
  hint,
}: CertificateFieldsProps) {
  const fileId = useId()
  const pwId = useId()

  return (
    <div className="space-y-4">
      {hint && <p className="text-sm text-gray-600 leading-relaxed">{hint}</p>}

      <div>
        <label htmlFor={fileId} className="mb-1 block text-sm font-medium text-gray-700">
          Arquivo do certificado (.pfx / .p12)
        </label>
        <input
          id={fileId}
          type="file"
          accept=".pfx,.p12"
          className="block w-full text-sm text-gray-600 file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border file:border-gray-300 file:text-sm file:font-medium file:bg-white file:text-gray-700 hover:file:bg-gray-50 cursor-pointer"
          onChange={(e) => onFileChange(e.target.files?.[0] ?? null)}
        />
        {file && <p className="mt-1 text-xs text-gray-500">{file.name}</p>}
        {fileError && <p className="mt-1 text-[0.8rem] font-medium text-destructive">{fileError}</p>}
      </div>

      <div>
        <label htmlFor={pwId} className="mb-1 block text-sm font-medium text-gray-700">
          Senha do certificado
        </label>
        <Input
          id={pwId}
          type="password"
          placeholder="Senha"
          autoComplete="off"
          value={password}
          onChange={(e) => onPasswordChange(e.target.value)}
        />
        {passwordError && <p className="mt-1 text-[0.8rem] font-medium text-destructive">{passwordError}</p>}
      </div>
    </div>
  )
}
