'use client'

import {useCallback, useEffect, useRef, useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {queryKeys} from '@/lib/api/query-keys'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {Modal} from '@/components/ui/modal'
import {PersonForm} from '@/components/persons/PersonForm'
import {formatCpfCnpj, unformatCpfCnpj} from '@/lib/utils/document'
import type {PersonCreate, PersonItemOut} from '@/lib/types/api'

interface NfsePersonSearchProps {
  value: PersonItemOut | null
  onChange: (p: PersonItemOut | null) => void
  placeholder?: string
}

/**
 * Busca de pessoa por nome ou CPF/CNPJ, com criação inline — reusado para
 * prestador (tp_emit 2/3), tomador e intermediário no wizard de emissão de
 * NFS-e. Espelha ReceiverSearch (NfeEmitForm.tsx) mas sem o estado extra da
 * NF-e (sem endereço de entrega/retirada).
 */
export function NfsePersonSearch({value, onChange, placeholder = 'Nome, CPF ou CNPJ'}: NfsePersonSearchProps) {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)
  const [open, setOpen] = useState(false)
  const [directError, setDirectError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [docSearchLoading, setDocSearchLoading] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const digits = query.replace(/\D/g, '')
  const isCpf = digits.length === 11
  const isCnpj = digits.length === 14
  const isDoc = isCpf || isCnpj

  const nameQuery = useQuery({
    queryKey: queryKeys.persons.search(debouncedQuery),
    queryFn: () => apiClient.searchPersonsByName(debouncedQuery),
    enabled: open && !!debouncedQuery && !isDoc && debouncedQuery.length >= 2,
  })

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const handleSearchByDoc = useCallback(async () => {
    if (!isDoc) return
    setDirectError(null)
    setDocSearchLoading(true)
    try {
      const person = await apiClient.getPersonByCpfCnpj(digits)
      onChange(person)
      setQuery('')
      setOpen(false)
    } catch {
      setDirectError('Pessoa não encontrada. Cadastre-a abaixo.')
      setShowCreate(true)
    } finally {
      setDocSearchLoading(false)
    }
  }, [digits, isDoc, onChange])

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
    const cpfCnpj = unformatCpfCnpj(value.sk)
    return (
      <div className="flex items-center gap-3 rounded-lg border border-green-200 bg-green-50 px-4 py-3">
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

  return (
    <div ref={containerRef} className="space-y-3">
      <div className="relative">
        <div className="flex gap-2">
          <Input
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setOpen(true)
              setDirectError(null)
            }}
            onFocus={() => setOpen(true)}
            placeholder={placeholder}
            className="flex-1"
          />
          {isDoc && (
            <Button type="button" size="sm" onClick={handleSearchByDoc} disabled={docSearchLoading}>
              {docSearchLoading ? 'Buscando…' : 'Buscar'}
            </Button>
          )}
        </div>

        {open && !isDoc && debouncedQuery.length >= 2 && (
          <div className="absolute z-10 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-lg max-h-64 overflow-auto">
            {nameQuery.isLoading ? (
              <p className="px-4 py-3 text-sm text-gray-400">Buscando…</p>
            ) : suggestions.length === 0 ? (
              <p className="px-4 py-3 text-sm text-gray-400">Nenhuma pessoa encontrada.</p>
            ) : (
              suggestions.map((p) => (
                <button
                  key={p.sk}
                  type="button"
                  onClick={() => {
                    onChange(p)
                    setQuery('')
                    setOpen(false)
                  }}
                  className="block w-full text-left px-4 py-2 text-sm hover:bg-gray-50"
                >
                  <span className="font-medium text-gray-900">{p.name}</span>
                  <span className="ml-2 text-xs text-gray-400 font-mono">{formatCpfCnpj(unformatCpfCnpj(p.sk))}</span>
                </button>
              ))
            )}
          </div>
        )}
      </div>

      {directError && <p className="text-xs text-red-600">{directError}</p>}

      <button
        type="button"
        onClick={() => setShowCreate(true)}
        className="text-xs font-medium text-brand-600 hover:text-brand-700"
      >
        + Cadastrar nova pessoa
      </button>

      <Modal isOpen={showCreate} title="Cadastrar pessoa" onClose={() => setShowCreate(false)} size="xl">
        <PersonForm onSubmit={handleCreatePerson} loading={createLoading}/>
      </Modal>
    </div>
  )
}
