'use client'

import {useCallback, useEffect, useId, useRef, useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {queryKeys} from '@/lib/api/query-keys'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {Modal} from '@/components/ui/modal'
import {PersonForm} from '@/components/persons/PersonForm'
import {PERSON_ROLE_LABELS, type PersonRole} from '@/lib/schemas/entity'
import {formatCpfCnpj, personTaxId} from '@/lib/utils/document'
import type {PersonCreate, PersonItemOut} from '@/lib/types/api'

/** Mínimo de caracteres antes de consultar a API. Não é preferência de UX: a
 *  busca por papel usa um FilterExpression aplicado depois da condição de chave,
 *  então um termo curto faz o DynamoDB ler muito mais do que devolve. */
export const PERSON_PICKER_MIN_QUERY = 2

interface PersonPickerProps {
  value: PersonItemOut | null
  onChange: (p: PersonItemOut | null) => void
  placeholder?: string
  autoFocus?: boolean
  /** Restringe a busca a um papel de cadastro. Pessoa multi-papel aparece em todos os seus papéis. */
  role?: PersonRole
}

/**
 * Busca de pessoa por nome ou CPF/CNPJ, com criação inline — usada para
 * destinatário, transportadora, condutor, prestador, tomador e intermediário.
 * Espelha ReceiverSearch (NfeEmitForm.tsx) mas sem o estado extra da NF-e
 * (sem endereço de entrega/retirada).
 */
export function PersonPicker({
                               value,
                               onChange,
                               placeholder = 'Nome, CPF ou CNPJ',
                               autoFocus = false,
                               role,
                             }: PersonPickerProps) {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)
  const [open, setOpen] = useState(false)
  const [directError, setDirectError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [docSearchLoading, setDocSearchLoading] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const containerRef = useRef<HTMLDivElement>(null)
  const docSearchTimerRef = useRef<number | null>(null)
  const docSearchSequenceRef = useRef(0)
  const listboxId = useId()

  const digits = query.replace(/\D/g, '')
  const isCpf = digits.length === 11
  const isCnpj = digits.length === 14
  const isDoc = isCpf || isCnpj

  const canSearch = !!debouncedQuery && !isDoc && debouncedQuery.length >= PERSON_PICKER_MIN_QUERY

  const nameQuery = useQuery({
    queryKey: queryKeys.persons.search(`${role ?? ''}:${debouncedQuery}`),
    queryFn: () => apiClient.searchPersonsByName(debouncedQuery, role),
    enabled: open && canSearch,
  })

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => {
      document.removeEventListener('mousedown', handler)
      if (docSearchTimerRef.current !== null) window.clearTimeout(docSearchTimerRef.current)
    }
  }, [])

  const handleSearchByDoc = useCallback(async (docDigits: string, sequence: number) => {
    setDirectError(null)
    try {
      const person = await apiClient.getPersonByCpfCnpj(docDigits)
      if (sequence !== docSearchSequenceRef.current) return
      onChange(person)
      setQuery('')
      setOpen(false)
    } catch {
      if (sequence !== docSearchSequenceRef.current) return
      setDirectError('Pessoa não encontrada. Cadastre-a abaixo.')
    } finally {
      if (sequence === docSearchSequenceRef.current) setDocSearchLoading(false)
    }
  }, [onChange])

  const handleCreatePerson = async (data: PersonCreate) => {
    setCreateLoading(true)
    try {
      const created = await apiClient.createPerson(data)
      onChange(created)
      setShowCreate(false)
      setQuery('')
      setDirectError(null)
    } finally {
      setCreateLoading(false)
    }
  }

  if (value) {
    const cpfCnpj = personTaxId(value)
    return (
      <div className="flex items-center gap-3 rounded-lg border border-brand-200 bg-brand-50 px-4 py-3">
        <div className="flex-1 min-w-0">
          <p className="font-medium text-gray-900 text-sm">{value.name}</p>
          <p className="text-xs text-gray-500 font-mono mt-0.5">{formatCpfCnpj(cpfCnpj)}</p>
        </div>
        <Button type="button" variant="ghost" size="xs" onClick={() => onChange(null)}
                className="text-danger hover:text-red-700 shrink-0">
          Trocar
        </Button>
      </div>
    )
  }

  const suggestions = nameQuery.data?.items ?? []

  const selectSuggestion = (person: PersonItemOut) => {
    onChange(person)
    setQuery('')
    setOpen(false)
    setActiveIndex(-1)
  }

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Escape') {
      setOpen(false)
      setActiveIndex(-1)
      return
    }
    if (!open || suggestions.length === 0) return
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const direction = event.key === 'ArrowDown' ? 1 : -1
      setActiveIndex((current) => {
        const start = current < 0 ? (direction > 0 ? -1 : 0) : current
        return (start + direction + suggestions.length) % suggestions.length
      })
    }
    if (event.key === 'Enter' && activeIndex >= 0) {
      event.preventDefault()
      const person = suggestions[activeIndex]
      if (person) selectSuggestion(person)
    }
  }

  return (
    <div ref={containerRef} className="space-y-3">
      <div className="relative">
        <div className="relative">
          <Input
            autoFocus={autoFocus}
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={open && canSearch}
            aria-controls={listboxId}
            aria-activedescendant={activeIndex >= 0 ? `${listboxId}-${activeIndex}` : undefined}
            aria-busy={docSearchLoading}
            value={query}
            onChange={(e) => {
              const nextQuery = e.target.value
              const nextDigits = nextQuery.replace(/\D/g, '')
              const nextIsDoc = nextDigits.length === 11 || nextDigits.length === 14
              docSearchSequenceRef.current += 1
              const sequence = docSearchSequenceRef.current
              if (docSearchTimerRef.current !== null) window.clearTimeout(docSearchTimerRef.current)
              setQuery(nextQuery)
              setOpen(true)
              setActiveIndex(-1)
              setDirectError(null)
              setDocSearchLoading(nextIsDoc)
              if (nextIsDoc) {
                docSearchTimerRef.current = window.setTimeout(() => {
                  void handleSearchByDoc(nextDigits, sequence)
                }, 300)
              }
            }}
            onFocus={() => setOpen(true)}
            onKeyDown={handleKeyDown}
            placeholder={placeholder}
            className={docSearchLoading ? 'pr-24' : undefined}
          />
          {docSearchLoading && (
            <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-gray-500">
              Buscando…
            </span>
          )}
        </div>

        {open && !isDoc && !!query && !canSearch && (
          <p className="absolute z-10 mt-1 w-full rounded-lg border border-gray-200 bg-white px-4 py-3 text-sm text-gray-500 shadow-popover">
            Digite ao menos {PERSON_PICKER_MIN_QUERY} caracteres do nome, ou o CPF/CNPJ completo.
          </p>
        )}

        {open && canSearch && (
          <div id={listboxId} role="listbox"
               className="absolute z-10 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-popover max-h-64 overflow-auto">
            {nameQuery.isLoading ? (
              <p className="px-4 py-3 text-sm text-gray-400">Buscando…</p>
            ) : suggestions.length === 0 ? (
              <p className="px-4 py-3 text-sm text-gray-500">
                {role
                  ? `Nenhuma pessoa cadastrada como "${PERSON_ROLE_LABELS[role]}" encontrada.`
                  : 'Nenhuma pessoa encontrada.'}
              </p>
            ) : (
              suggestions.map((p, index) => (
                <button
                  key={p.sk}
                  id={`${listboxId}-${index}`}
                  role="option"
                  aria-selected={index === activeIndex}
                  type="button"
                  onMouseEnter={() => setActiveIndex(index)}
                  onClick={() => selectSuggestion(p)}
                  className="block min-h-11 w-full text-left px-4 py-2 text-sm hover:bg-gray-50 aria-selected:bg-brand-50"
                >
                  <span className="font-medium text-gray-900">{p.name}</span>
                  <span className="ml-2 text-xs text-gray-400 font-mono">{formatCpfCnpj(personTaxId(p))}</span>
                </button>
              ))
            )}
          </div>
        )}
      </div>

      {directError && <p role="alert" className="text-xs text-red-600">{directError}</p>}

      <button
        type="button"
        onClick={() => setShowCreate(true)}
        className="min-h-11 sm:min-h-0 text-xs font-medium text-brand-600 hover:text-brand-700"
      >
        + Cadastrar nova pessoa
      </button>

      <Modal isOpen={showCreate} title="Cadastrar pessoa" onClose={() => setShowCreate(false)} size="xl">
        <PersonForm onSubmit={handleCreatePerson} loading={createLoading}
                    initialRoles={role ? [role] : undefined}/>
      </Modal>
    </div>
  )
}
