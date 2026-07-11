'use client'

import {useState} from 'react'
import {Input} from '@/components/ui/input'

const NAT_OP_MAX_LENGTH = 60

/**
 * Discreet inline-editable "natureza da operação" line. Shows the value as plain
 * text with a subtle "editar" affordance; clicking turns it into a compact input
 * (≤60 chars). Shared by the NF-e and NFC-e issuance forms.
 */
export function NatOpInlineEdit({value, onChange, onReset, canReset = false, suffix}: {
  value: string
  onChange: (next: string) => void
  onReset?: () => void
  canReset?: boolean
  suffix?: React.ReactNode
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')

  const commit = () => {
    const trimmed = draft.trim()
    if (trimmed) onChange(trimmed.slice(0, NAT_OP_MAX_LENGTH))
    setEditing(false)
  }

  if (editing) {
    return (
      <div className="flex items-center gap-2">
        <span className="text-xs text-gray-500 shrink-0">Natureza da operação:</span>
        <Input
          autoFocus
          value={draft}
          maxLength={NAT_OP_MAX_LENGTH}
          onChange={(e) => setDraft(e.target.value.slice(0, NAT_OP_MAX_LENGTH))}
          onBlur={commit}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              commit()
            }
            if (e.key === 'Escape') setEditing(false)
          }}
          className="h-7 max-w-xs text-xs"
        />
      </div>
    )
  }

  return (
    <p className="text-xs text-gray-500">
      Natureza da operação: <span className="font-medium text-gray-700">{value}</span>
      {suffix}
      <button type="button" onClick={() => {
        setDraft(value)
        setEditing(true)
      }} className="ml-1.5 text-brand-600 hover:text-brand-700">editar</button>
      {canReset && onReset && (
        <button type="button" onClick={onReset} className="ml-1.5 text-gray-400 hover:text-gray-600">automático</button>
      )}
    </p>
  )
}
