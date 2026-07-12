'use client'

import {useCallback, useEffect, useMemo, useRef, useState} from 'react'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {useRouter} from 'next/navigation'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {Textarea} from '@/components/ui/textarea'
import {NumericInput} from '@/components/ui/numeric-input'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {Modal} from '@/components/ui/modal'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import type {
  NfeArmaIn,
  NfeCardIn,
  NfeDuplicataIn,
  NfeEmit,
  NfeFatIn,
  NfeListOut,
  NfeLocalIn,
  NfeTransportIn,
  PersonCreate,
  PersonItemOut,
  ProductOut,
  VehicleOut
} from '@/lib/types/api'
import {NF_PAYMENT_TYPES} from '@/lib/types/api'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {PersonForm} from '@/components/persons/PersonForm'
import {formatCpfCnpj, unformatCpfCnpj} from "@/lib/utils/document"
import {
  buildNatOpFromCfops,
  cfopDirection,
  cfopGroupCodes,
  cfopSuffix,
  cfopTpNf,
  groupCfopConfigBySuffix,
  NO_PAYMENT_CFOPS,
  resolveCfopForUf
} from "@/lib/data/cfop"
import {resolveUnitPrice} from "@/lib/data/product-price"
import {PaymentCardFields} from "@/components/nfe/PaymentCardFields"
import {NatOpInlineEdit} from "@/components/nfe/NatOpInlineEdit"
import {LocationPicker} from "@/components/nfe/LocationPicker"

// ─── Local state types ────────────────────────────────────────────────────────

interface EmitProduct {
  product: ProductOut
  cfop: string
  cfopSuffix: string
  qty: string
  unitValue: string
  discount: string
  // veicProd — por unidade
  veic_chassi?: string
  veic_n_serie?: string
  veic_n_motor?: string
  veic_c_cor?: string
  veic_x_cor?: string
  // arma — por unidade (list)
  armas?: NfeArmaIn[]
}

interface EmitPayment {
  payment_type: string
  value: string
  ind_pag: '0' | '1'
  card: NfeCardIn | null
}

interface EmitTransport {
  mod_frete: string
  transporta_cnpj: string
  transporta_nome: string
  transporta_uf: string
  veiculo_placa: string
  veiculo_uf: string
  veiculo_rntrc: string
}

interface EmitDuplicata {
  n_dup: string
  d_venc: string
  v_dup: string
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function computeTotal(p: EmitProduct): number {
  const qty = parseFloat(p.qty) || 0
  const unit = parseFloat(p.unitValue) || 0
  const disc = parseFloat(p.discount) || 0
  return Math.max(0, qty * unit - disc)
}

function fmt(n: number): string {
  return n.toLocaleString('pt-BR', {style: 'currency', currency: 'BRL'})
}

const PAYMENT_OPTIONS = Object.entries(NF_PAYMENT_TYPES).map(([value, label]) => ({
  value,
  label: `${value} – ${label}`,
  display: label,
})).sort((a, b) => parseInt(a.value) - parseInt(b.value))

const MOD_FRETE_OPTIONS = [
  {value: '9', label: '9 – Sem frete'},
  {value: '0', label: '0 – Contratação (remetente)'},
  {value: '1', label: '1 – Contratação (destinatário)'},
  {value: '2', label: '2 – Contratação por conta de terceiros'},
  {value: '3', label: '3 – Transporte próprio do remetente'},
  {value: '4', label: '4 – Transporte próprio do destinatário'},
]

// Types that typically carry card data
const CARD_PAYMENT_TYPES = new Set(['03', '04', '10', '11', '12', '13'])

// ─── Receiver search ──────────────────────────────────────────────────────────

interface ReceiverSearchProps {
  value: PersonItemOut | null
  onChange: (p: PersonItemOut | null) => void
}

function ReceiverSearch({value, onChange}: ReceiverSearchProps) {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)
  const [open, setOpen] = useState(false)
  const [directError, setDirectError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const [docSearchLoading, setDocSearchLoading] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  // Detect CPF (11) or CNPJ (14) regardless of formatting
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
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
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
                className="text-red-500 hover:text-red-700 shrink-0">
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
            type="text"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value.toUpperCase())
              setDirectError(null)
              setShowCreate(false)
              setOpen(true)
            }}
            onFocus={() => setOpen(true)}
            placeholder="Nome, CPF ou CNPJ (com ou sem formatação)"
            className="flex-1"
          />
          {isDoc && (
            <Button
              type="button"
              variant="brand"
              size="sm"
              onClick={handleSearchByDoc}
              disabled={docSearchLoading}
              className="shrink-0"
            >
              {docSearchLoading ? (
                <span className="flex items-center gap-1.5">
                  <span
                    className="inline-block w-3 h-3 border-2 border-white/40 border-t-white rounded-full animate-spin"/>
                  Buscando…
                </span>
              ) : 'Buscar'}
            </Button>
          )}
        </div>

        {directError && (
          <p className="text-xs text-amber-600 mt-1">{directError}</p>
        )}

        {open && !isDoc && suggestions.length > 0 && (
          <div
            className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-lg overflow-hidden">
            {suggestions.map((p) => {
              const cpfCnpj = unformatCpfCnpj(p.pk)
              return (
                <button
                  key={p.sk}
                  type="button"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => {
                    onChange(p)
                    setQuery('')
                    setOpen(false)
                  }}
                  className="w-full text-left px-4 py-2.5 hover:bg-gray-50 transition-colors"
                >
                  <p className="text-sm font-medium text-gray-900">{p.name}</p>
                  <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(cpfCnpj)}</p>
                </button>
              )
            })}
          </div>
        )}

        {open && !isDoc && nameQuery.isLoading && (
          <div
            className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-lg px-4 py-3 space-y-2">
            <div className="h-4 w-3/4 rounded bg-gray-100 animate-pulse"/>
            <div className="h-4 w-1/2 rounded bg-gray-100 animate-pulse"/>
          </div>
        )}

        {open && !isDoc && debouncedQuery.length >= 2 && !nameQuery.isLoading && suggestions.length === 0 && !showCreate && (
          <div className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-lg px-4 py-3">
            <p className="text-sm text-gray-500 mb-2">Nenhuma pessoa encontrada.</p>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => {
                setShowCreate(true);
                setOpen(false)
              }}
              className="text-brand-600 hover:text-brand-700 px-0"
            >
              + Cadastrar nova pessoa
            </Button>
          </div>
        )}
      </div>

      {/* Modal de cadastro de nova pessoa */}
      <Modal
        isOpen={showCreate}
        title="Cadastrar nova pessoa"
        onClose={() => setShowCreate(false)}
        size="xl"
      >
        <PersonForm
          onSubmit={handleCreatePerson}
          loading={createLoading}
        />
      </Modal>
    </div>
  )
}

// ─── Carrier search (transportadora) ─────────────────────────────────────────

interface CarrierSearchProps {
  onSelect: (p: PersonItemOut) => void
}

function CarrierSearch({onSelect}: CarrierSearchProps) {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)
  const [open, setOpen] = useState(false)
  const [docSearchLoading, setDocSearchLoading] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const digits = query.replace(/\D/g, '')
  const isDoc = digits.length === 11 || digits.length === 14

  const {data: nameResults} = useQuery({
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
    setDocSearchLoading(true)
    try {
      const person = await apiClient.getPersonByCpfCnpj(digits)
      onSelect(person)
    } catch {
      // not found — user should create new
    } finally {
      setDocSearchLoading(false)
    }
  }, [digits, isDoc, onSelect])

  const suggestions = nameResults?.items ?? []

  return (
    <div ref={containerRef} className="relative">
      <div className="flex gap-2">
        <Input
          type="text"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value.toUpperCase());
            setOpen(true)
          }}
          onFocus={() => setOpen(true)}
          placeholder="Buscar por nome ou CNPJ/CPF da transportadora"
          className="flex-1"
        />
        {isDoc && (
          <Button type="button" variant="brand" size="sm" onClick={handleSearchByDoc} disabled={docSearchLoading}>
            {docSearchLoading ? (
              <span className="flex items-center gap-1.5">
                <span
                  className="inline-block w-3 h-3 border-2 border-white/40 border-t-white rounded-full animate-spin"/>
                Buscando…
              </span>
            ) : 'Buscar'}
          </Button>
        )}
      </div>
      {open && !isDoc && suggestions.length > 0 && (
        <div className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-lg overflow-hidden">
          {suggestions.map((p) => (
            <button key={p.sk} type="button"
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => {
                      onSelect(p);
                      setQuery('');
                      setOpen(false)
                    }}
                    className="w-full text-left px-4 py-2.5 hover:bg-gray-50 transition-colors">
              <p className="text-sm font-medium text-gray-900">{p.name}</p>
              <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(unformatCpfCnpj(p.pk))}</p>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Vehicle select ───────────────────────────────────────────────────────────

interface VehicleSelectProps {
  vehicles: VehicleOut[]
  onSelect: (v: VehicleOut) => void
  query: string
  onQueryChange: (q: string) => void
}

function VehicleSelect({vehicles, onSelect, query, onQueryChange}: VehicleSelectProps) {
  const filtered = query
    ? vehicles.filter((v) =>
      v.plate.toLowerCase().includes(query.toLowerCase()) ||
      v.owner?.name?.toLowerCase().includes(query.toLowerCase())
    )
    : vehicles

  if (vehicles.length === 0) {
    return (
      <p className="text-xs text-gray-400 py-1">
        Nenhum veículo cadastrado. Use as opções manuais abaixo.
      </p>
    )
  }

  return (
    <div className="space-y-2">
      <Input
        type="text"
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        placeholder="Buscar veículo por placa ou proprietário"
      />
      {filtered.length > 0 && (
        <div className="max-h-36 overflow-y-auto space-y-0.5 border border-gray-100 rounded-lg">
          {filtered.map((v) => (
            <button key={v.sk} type="button"
                    onClick={() => onSelect(v)}
                    className="w-full text-left px-3 py-2 hover:bg-gray-50 transition-colors flex items-center justify-between">
              <span className="font-mono text-sm font-medium text-gray-900">{v.plate}</span>
              <span className="text-xs text-gray-400">{v.plate_uf} · {v.owner?.name}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Inline product picker ────────────────────────────────────────────────────

interface ProductPickerProps {
  onSelect: (product: ProductOut) => void
  onClose: () => void
}

function ProductPicker({onSelect, onClose}: ProductPickerProps) {
  const {selectedOrg} = useAuth()
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)

  const {data, isLoading} = useQuery({
    queryKey: queryKeys.products.list(selectedOrg?.pk),
    queryFn: () => apiClient.getProducts({limit: 50}),
    enabled: !!selectedOrg,
  })

  const allProducts = data?.items ?? []
  const filtered = debouncedQuery
    ? allProducts.filter(
      (p) =>
        p.description.toLowerCase().includes(debouncedQuery.toLowerCase()) ||
        p.code.toLowerCase().includes(debouncedQuery.toLowerCase()),
    )
    : allProducts

  return (
    <div className="rounded-lg border border-brand-200 bg-brand-50/30 p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Buscar produto</p>
        <Button type="button" variant="ghost" size="xs" onClick={onClose} className="text-gray-400 hover:text-gray-600">
          Fechar
        </Button>
      </div>
      <Input
        type="text"
        autoFocus
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Código ou descrição..."
      />
      <div className="max-h-48 overflow-y-auto space-y-0.5">
        {isLoading ? (
          <div className="space-y-2 py-1">
            <div className="h-8 rounded-md bg-gray-100 animate-pulse"/>
            <div className="h-8 rounded-md bg-gray-100 animate-pulse"/>
            <div className="h-8 rounded-md bg-gray-100 animate-pulse"/>
          </div>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-gray-500 py-2">Nenhum produto encontrado.</p>
        ) : (
          filtered.map((p) => (
            <button
              key={p.sk}
              type="button"
              onClick={() => onSelect(p)}
              className="w-full text-left px-3 py-2 rounded-md hover:bg-white transition-colors flex items-center justify-between gap-2"
            >
              <span className="text-sm text-gray-900 min-w-0 truncate">
                {p.description}
                {p.brand && <span className="ml-1.5 text-xs text-gray-400">{p.brand}</span>}
              </span>
              <span className="text-xs text-gray-400 shrink-0">
                {parseFloat(p.value).toLocaleString('pt-BR', {minimumFractionDigits: 2, maximumFractionDigits: 2})}
              </span>
            </button>
          ))
        )}
      </div>
    </div>
  )
}

// ─── Product row ──────────────────────────────────────────────────────────────

interface ProductRowProps {
  item: EmitProduct
  index: number
  sameUf: boolean | null
  onChange: (index: number, updated: Partial<EmitProduct>) => void
  onRemove: (index: number) => void
}

export function ProductRow({item, index, sameUf, onChange, onRemove}: ProductRowProps) {
  const cfopGroups = groupCfopConfigBySuffix(item.product.cfop_config)
  const cfopOptions = cfopGroups.map((g) => {
    const codes = cfopGroupCodes(g)
    const label = g.label ? `${codes} – ${g.label}` : codes
    return {value: g.suffix, label, display: label}
  })
  const selectedGroup = cfopGroups.find(g => g.suffix === item.cfopSuffix) ?? null
  // UF unknown (no recipient UF / issuer UF) — cannot resolve scope yet.
  const cfopUfUnknown = selectedGroup !== null && sameUf === null
  // Required-scope variant (5xxx/6xxx) not configured for this destination.
  const cfopMissingVariant = selectedGroup !== null && sameUf !== null
    && resolveCfopForUf(selectedGroup, sameUf) === null
  const total = computeTotal(item)
  const isVeiculo = item.product.prod_type === 'veiculo'
  const isArma = item.product.prod_type === 'arma'

  const [newArma, setNewArma] = useState<NfeArmaIn>({n_serie: '', n_cano: '', descr: ''})

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-3 md:p-4 space-y-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="font-medium text-gray-900 text-sm">
            {item.product.description}
            {item.product.brand && (
              <span className="ml-1.5 text-xs text-gray-400 font-normal">{item.product.brand}</span>
            )}
          </p>
          <div className="flex flex-wrap items-center gap-1 mt-0.5">
            {isVeiculo && <span className="px-1.5 py-0.5 bg-indigo-100 text-indigo-700 rounded text-xs">Veículo</span>}
            {isArma && <span className="px-1.5 py-0.5 bg-red-100 text-red-700 rounded text-xs">Armamento</span>}
          </div>
        </div>
        <Button type="button" variant="ghost" size="xs" onClick={() => onRemove(index)}
                className="shrink-0 text-red-500 hover:text-red-700">
          Remover
        </Button>
      </div>
      <div className="grid grid-cols-3 md:grid-cols-12 gap-2 items-end">
        <div className="col-span-3 md:col-span-6 flex flex-col gap-1">
          <Label className="text-xs font-medium text-gray-600">CFOP</Label>
          {cfopOptions.length > 0 ? (
            <OptionsSelect
              value={item.cfopSuffix}
              onValueChange={(suffix) => {
                const group = cfopGroups.find(g => g.suffix === suffix)
                const resolved = group && sameUf !== null ? resolveCfopForUf(group, sameUf) : null
                onChange(index, {cfopSuffix: suffix, cfop: resolved ?? ''})
              }}
              options={cfopOptions} placeholder="CFOP"/>
          ) : (
            <Input type="text" value={item.cfop} onChange={(e) => onChange(index, {cfop: e.target.value})}
                   maxLength={4} placeholder="5102"/>
          )}
          {cfopUfUnknown && (
            <span className="text-xs text-red-600">
              Selecione um destinatário com UF para definir o CFOP.
            </span>
          )}
          {cfopMissingVariant && (
            <span className="text-xs text-red-600">
              Configure o CFOP {sameUf ? '5' : '6'}xxx neste produto para esta UF de destino.
            </span>
          )}
        </div>
        <div className="col-span-1 md:col-span-2 flex-col gap-1">
          <Label className="text-xs font-medium text-gray-600">Qtd ({item.product.unit ?? 'UN'})</Label>
          <div className="flex items-center">
            <button type="button"
                    onClick={() => onChange(index, {qty: String(Math.max(0, (parseFloat(item.qty) || 0) - 1))})}
                    className="h-8 w-7 shrink-0 flex items-center justify-center rounded-l-lg border border-r-0 border-input bg-muted/30 text-gray-600 hover:bg-muted/60 font-medium select-none text-sm">−
            </button>
            <NumericInput decimal integerPlaces={7} decimalPlaces={4} value={item.qty}
                          onChange={(v) => onChange(index, {qty: v})} placeholder="1"
                          className="rounded-none border-x-0 text-center"/>
            <button type="button"
                    onClick={() => onChange(index, {qty: String((parseFloat(item.qty) || 0) + 1)})}
                    className="h-8 w-7 shrink-0 flex items-center justify-center rounded-r-lg border border-l-0 border-input bg-muted/30 text-gray-600 hover:bg-muted/60 font-medium select-none text-sm">+
            </button>
          </div>
        </div>
        <div className="col-span-1 md:col-span-2 flex flex-col gap-1">
          <Label className="text-xs font-medium text-gray-600">Valor unitário</Label>
          <CurrencyInput decimalPlaces={2} maxDecimalPlaces={10} value={item.unitValue}
                         onChange={(v) => onChange(index, {unitValue: v})} placeholder="0,00"/>
        </div>
        <div className="col-span-1 md:col-span-2 flex flex-col gap-1">
          <Label className="text-xs font-medium text-gray-600">Desconto</Label>
          <CurrencyInput decimalPlaces={2} value={item.discount}
                         onChange={(v) => onChange(index, {discount: v})} placeholder="0,00"/>
        </div>
      </div>

      {/* ── Veículo — dados por unidade ───────────────────────────── */}
      {isVeiculo && (
        <div className="rounded-md border border-indigo-100 bg-indigo-50/30 p-3 space-y-2">
          <p className="text-xs font-semibold text-indigo-700 uppercase tracking-wider">Dados da unidade</p>
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
            <div className="flex flex-col gap-1">
              <Label className="text-xs font-medium text-gray-600">Chassi (VIN, 17 chars) *</Label>
              <Input value={item.veic_chassi ?? ''}
                     onChange={(e) => onChange(index, {veic_chassi: e.target.value.toUpperCase()})}
                     placeholder="9BWZZZ377VT004251" maxLength={17} className="font-mono text-xs"/>
            </div>
            <div className="flex flex-col gap-1">
              <Label className="text-xs font-medium text-gray-600">Número de série *</Label>
              <Input value={item.veic_n_serie ?? ''}
                     onChange={(e) => onChange(index, {veic_n_serie: e.target.value})}
                     placeholder="nSerie" maxLength={9}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label className="text-xs font-medium text-gray-600">Número do motor *</Label>
              <Input value={item.veic_n_motor ?? ''}
                     onChange={(e) => onChange(index, {veic_n_motor: e.target.value})}
                     placeholder="nMotor" maxLength={21}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label className="text-xs font-medium text-gray-600">Código cor (montadora)</Label>
              <Input value={item.veic_c_cor ?? ''}
                     onChange={(e) => onChange(index, {veic_c_cor: e.target.value})}
                     placeholder={item.product.veic_c_cor ?? 'cCor'} maxLength={4}/>
            </div>
            <div className="flex flex-col gap-1 sm:col-span-2">
              <Label className="text-xs font-medium text-gray-600">Descrição da cor</Label>
              <Input value={item.veic_x_cor ?? ''}
                     onChange={(e) => onChange(index, {veic_x_cor: e.target.value})}
                     placeholder={item.product.veic_x_cor ?? 'xCor'} maxLength={40}/>
            </div>
          </div>
        </div>
      )}

      {/* ── Armamento — dados por unidade ─────────────────────────── */}
      {isArma && (
        <div className="rounded-md border border-red-100 bg-red-50/20 p-3 space-y-2">
          <p className="text-xs font-semibold text-red-700 uppercase tracking-wider">
            Armas desta NF-e ({(item.armas ?? []).length})
          </p>
          {(item.armas ?? []).length > 0 && (
            <div className="space-y-1">
              {(item.armas ?? []).map((a, ai) => (
                <div key={ai}
                     className="flex items-center justify-between rounded bg-white border border-red-100 px-3 py-1.5 text-xs">
                  <span className="font-mono text-gray-700">série: {a.n_serie} · cano: {a.n_cano}</span>
                  <Button type="button" variant="ghost" size="xs"
                          onClick={() => onChange(index, {armas: (item.armas ?? []).filter((_, i) => i !== ai)})}
                          className="text-red-500 hover:text-red-700">remover</Button>
                </div>
              ))}
            </div>
          )}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 items-end">
            <div className="flex flex-col gap-1">
              <Label className="text-xs font-medium text-gray-600">Série *</Label>
              <Input value={newArma.n_serie} placeholder="nSerie" maxLength={15}
                     onChange={(e) => setNewArma((a) => ({...a, n_serie: e.target.value}))}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label className="text-xs font-medium text-gray-600">Cano *</Label>
              <Input value={newArma.n_cano} placeholder="nCano" maxLength={15}
                     onChange={(e) => setNewArma((a) => ({...a, n_cano: e.target.value}))}/>
            </div>
            <div className="flex flex-col gap-1 sm:col-span-2">
              <Label className="text-xs font-medium text-gray-600">Descrição (opcional)</Label>
              <Input value={newArma.descr ?? ''} placeholder={item.product.arma_descr ?? 'Descrição da arma'}
                     maxLength={256}
                     onChange={(e) => setNewArma((a) => ({...a, descr: e.target.value}))}/>
            </div>
          </div>
          <Button type="button" variant="ghost" size="sm"
                  disabled={!newArma.n_serie || !newArma.n_cano}
                  onClick={() => {
                    onChange(index, {armas: [...(item.armas ?? []), newArma]})
                    setNewArma({n_serie: '', n_cano: '', descr: ''})
                  }}
                  className="text-brand-600 hover:text-brand-700 px-0">
            + Adicionar arma
          </Button>
        </div>
      )}

      <div className="text-right text-sm font-medium text-gray-700">
        Total: <span className="font-semibold">{fmt(total)}</span>
      </div>
    </div>
  )
}

// ─── Date / amount utilities ──────────────────────────────────────────────────

function addOneMonth(dateStr: string): string {
  if (!dateStr) return ''
  const [year, month, day] = dateStr.split('-').map(Number)
  let newMonth = month + 1
  let newYear = year
  if (newMonth > 12) {
    newMonth = 1;
    newYear++
  }
  const lastDay = new Date(newYear, newMonth, 0).getDate()
  const newDay = Math.min(day, lastDay)
  return `${newYear}-${String(newMonth).padStart(2, '0')}-${String(newDay).padStart(2, '0')}`
}

function generateDuplicatas(total: number, count: number, firstDate: string): EmitDuplicata[] {
  if (count <= 0 || total <= 0) return []
  const cents = Math.round(total * 100)
  const baseC = Math.floor(cents / count)
  const remainder = cents - baseC * count
  const result: EmitDuplicata[] = []
  for (let i = 0; i < count; i++) {
    const amount = (baseC + (i === count - 1 ? remainder : 0)) / 100
    const date = i === 0 ? firstDate : addOneMonth(result[i - 1].d_venc)
    result.push({n_dup: String(i + 1).padStart(3, '0'), d_venc: date, v_dup: amount.toFixed(2)})
  }
  return result
}

// ─── Step types ───────────────────────────────────────────────────────────────

type EmitStep = 'destinatario' | 'produtos' | 'pagamento' | 'revisao'
const STEPS: { id: EmitStep; label: string }[] = [
  {id: 'destinatario', label: 'Destinatário'},
  {id: 'produtos', label: 'Produtos'},
  {id: 'pagamento', label: 'Pagamento'},
  {id: 'revisao', label: 'Revisão'},
]
const STEP_IDS = STEPS.map(s => s.id)

// ─── Step indicator ───────────────────────────────────────────────────────────

function StepIndicator({current, steps}: { current: EmitStep; steps: typeof STEPS }) {
  const idx = steps.findIndex(s => s.id === current)
  return (
    <div className="flex items-center gap-0 mb-6">
      {steps.map((step, i) => {
        const done = i < idx
        const active = i === idx
        return (
          <div key={step.id} className="flex items-center flex-1 last:flex-none">
            <div className="flex flex-col items-center gap-1 shrink-0">
              <div
                className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-semibold transition-colors ${
                  done ? 'bg-brand-600 text-white' : active ? 'bg-brand-600 text-white ring-2 ring-brand-200' : 'bg-gray-100 text-gray-400'
                }`}>
                {done ? '✓' : i + 1}
              </div>
              <span
                className={`text-xs hidden sm:block ${active ? 'text-brand-600 font-medium' : done ? 'text-gray-500' : 'text-gray-400'}`}>
                {step.label}
              </span>
            </div>
            {i < steps.length - 1 && (
              <div className={`flex-1 h-0.5 mx-2 ${i < idx ? 'bg-brand-500' : 'bg-gray-200'}`}/>
            )}
          </div>
        )
      })}
    </div>
  )
}

// ─── Main form ────────────────────────────────────────────────────────────────

export function NfeEmitForm() {
  const {selectedOrg} = useAuth()
  const router = useRouter()

  const [currentStep, setCurrentStep] = useState<EmitStep>('destinatario')
  const [receiver, setReceiver] = useState<PersonItemOut | null>(null)
  const [selfIssuance, setSelfIssuance] = useState(false)
  const [entrega, setEntrega] = useState<NfeLocalIn | null>(null)
  const [saveEntregaLocation, setSaveEntregaLocation] = useState(false)
  const [retirada, setRetirada] = useState<NfeLocalIn | null>(null)
  const [saveRetiradaLocation, setSaveRetiradaLocation] = useState(false)
  const [prevReceiverSk, setPrevReceiverSk] = useState<string | null>(null)
  const [products, setProducts] = useState<EmitProduct[]>([])
  const [payments, setPayments] = useState<EmitPayment[]>([])
  const [additionalInfo, setAdditionalInfo] = useState('')
  const [natOpManual, setNatOpManual] = useState<string | null>(null)
  const [showProductPicker, setShowProductPicker] = useState(false)
  const [newPaymentType, setNewPaymentType] = useState('01')
  const [newPaymentValue, setNewPaymentValue] = useState('')
  const paymentValueLockedRef = useRef(false)
  const [newPaymentIndPag, setNewPaymentIndPag] = useState<'0' | '1'>('0')
  const [newPaymentCard, setNewPaymentCard] = useState<NfeCardIn | null>(null)
  const [showCardToggle, setShowCardToggle] = useState(false)
  // Cobrança
  const [cobrFat, setCobrFat] = useState<NfeFatIn>({n_fat: '', v_orig: '', v_desc: '', v_liq: ''})
  const [prevHasPrazoPayment, setPrevHasPrazoPayment] = useState(false)
  const [prevSameUf, setPrevSameUf] = useState<boolean | null>(null)
  const [prevHasNoPaymentCfop, setPrevHasNoPaymentCfop] = useState(false)
  const [duplicatas, setDuplicatas] = useState<EmitDuplicata[]>([])
  const [dupCount, setDupCount] = useState('1')
  const [dupFirstDate, setDupFirstDate] = useState('')
  // Transport
  const [showTransport, setShowTransport] = useState(false)
  const [transport, setTransport] = useState<EmitTransport>({
    mod_frete: '9', transporta_cnpj: '', transporta_nome: '', transporta_uf: '',
    veiculo_placa: '', veiculo_uf: '', veiculo_rntrc: '',
  })
  const [selectedCarrier, setSelectedCarrier] = useState<PersonItemOut | null>(null)
  const [selectedVehicle, setSelectedVehicle] = useState<VehicleOut | null>(null)
  const [showCarrierModal, setShowCarrierModal] = useState(false)
  const [createCarrierLoading, setCreateCarrierLoading] = useState(false)
  const [vehicleSearchQuery, setVehicleSearchQuery] = useState('')
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [loadingFavCpfCnpj, setLoadingFavCpfCnpj] = useState<string | null>(null)

  // ─── Queries ──────────────────────────────────────────────────────────────

  const {data: nfeConfig} = useQuery({
    queryKey: queryKeys.nfeConfig(selectedOrg!.pk),
    queryFn: () => apiClient.getNFeConfig(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })

  const {data: orgData} = useQuery({
    queryKey: queryKeys.organizations.detail(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getOrganization(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })

  const {data: vehiclesData} = useQuery({
    queryKey: queryKeys.vehicles.list(selectedOrg?.pk),
    queryFn: () => apiClient.getVehicles({limit: 50}),
    enabled: !!selectedOrg && showTransport,
  })

  const {data: productsData} = useQuery({
    queryKey: queryKeys.products.list(selectedOrg?.pk),
    queryFn: () => apiClient.getProducts({limit: 50}),
    enabled: !!selectedOrg,
  })

  const {data: recentNfes, isLoading: recentNfesLoading} = useQuery({
    queryKey: queryKeys.nfes.list(selectedOrg?.pk, {limit: 50}),
    queryFn: () => apiClient.getNfes({limit: 50}),
    enabled: !!selectedOrg,
    staleTime: 60_000,
  })

  const favoriteReceivers = useMemo((): Array<{ name: string; cpfCnpj: string; count: number }> => {
    if (!recentNfes?.items) return []
    const orgDoc = selectedOrg ? unformatCpfCnpj(selectedOrg.pk) : null
    const counts = new Map<string, { name: string; cpfCnpj: string; count: number }>()
    for (const nfe of recentNfes.items as NfeListOut[]) {
      if (nfe.incoming || !nfe.dest_cpf_cnpj) continue
      if (orgDoc && unformatCpfCnpj(nfe.dest_cpf_cnpj) === orgDoc) continue
      const cur = counts.get(nfe.dest_cpf_cnpj)
      if (cur) cur.count++
      else counts.set(nfe.dest_cpf_cnpj, {name: nfe.dest_name, cpfCnpj: nfe.dest_cpf_cnpj, count: 1})
    }
    return [...counts.values()].sort((a, b) => b.count - a.count).slice(0, 5)
  }, [recentNfes, selectedOrg])

  // ─── Totals ───────────────────────────────────────────────────────────────

  const totalProducts = products.reduce((s, p) => s + (parseFloat(p.qty) || 0) * (parseFloat(p.unitValue) || 0), 0)
  const totalDiscount = products.reduce((s, p) => s + (parseFloat(p.discount) || 0), 0)
  const totalNfe = Math.max(0, totalProducts - totalDiscount)
  const totalPaid = payments.some(it => it.payment_type === '90') ? totalNfe : payments.reduce((s, p) => s + (parseFloat(p.value) || 0), 0)
  const remaining = totalNfe - totalPaid

  // ─── CFOP direction (tp_nf) + nat_op ───────────────────────────────────────
  // Note type is set by the FIRST product's CFOP; mixing entrada (1/2/3) with
  // saída (5/6/7) CFOPs is not allowed in the same NF-e.
  const noteDirection = products.length > 0 ? cfopDirection(products[0].cfop) : null
  const cfopMixError = noteDirection !== null
    && products.some(p => cfopDirection(p.cfop) !== null && cfopDirection(p.cfop) !== noteDirection)
  const tpNf = products.length > 0 ? cfopTpNf(products[0].cfop) : '1'
  const computedNatOp = useMemo(() => buildNatOpFromCfops(products.map(p => p.cfop)), [products])
  const natOp = natOpManual ?? computedNatOp

  // Recipient in the issuer's UF? Self-issuance ⇒ always same UF.
  const issuerUf = selectedOrg?.state_federation ?? null
  const recipientUf = selfIssuance
    ? issuerUf
    : (receiver?.person.addresses?.[0]?.state_federation ?? null)
  const sameUf: boolean | null =
    issuerUf && recipientUf ? issuerUf === recipientUf : null

  // Grouped-CFOP products block emission until the destination UF is known
  // (sameUf === null) AND a same-scope variant is resolved (non-empty cfop).
  const cfopUnresolvedError = products.some(p => p.cfopSuffix && (!p.cfop || sameUf === null))

  // Re-resolve CFOPs when sameUf changes (same-render pattern to avoid effect setState warning)
  if (sameUf !== prevSameUf) {
    setPrevSameUf(sameUf)
    if (sameUf !== null) {
      setProducts(prev => prev.map(item => {
        if (!item.cfopSuffix) return item
        const groups = groupCfopConfigBySuffix(item.product.cfop_config)
        const group = groups.find(g => g.suffix === item.cfopSuffix)
        if (!group) return item
        return {...item, cfop: resolveCfopForUf(group, sameUf) ?? ''}
      }))
    }
  }

  // Products with a "sem pagamento" CFOP (e.g. remessa/bonificação) force the
  // payment to "Sem pagamento" (tPag 90). Same-render guard mirrors the CFOP
  // re-resolve above to avoid the React 19 passive-effect setState cascade.
  const hasNoPaymentCfop = products.some(p => (NO_PAYMENT_CFOPS as string[]).includes(p.cfop))
  if (hasNoPaymentCfop !== prevHasNoPaymentCfop) {
    setPrevHasNoPaymentCfop(hasNoPaymentCfop)
    if (hasNoPaymentCfop) {
      setNewPaymentType('90')
      setPayments([{payment_type: '90', value: '0.00', ind_pag: '0', card: null}])
    }
  }

  // Derived — cobrança only shown when there's an "a prazo" payment
  const hasPrazoPayment = payments.some(p => p.ind_pag === '1')
  const isPix = newPaymentType === '17'
  const isCardPayment = CARD_PAYMENT_TYPES.has(newPaymentType) || isPix

  // ─── Auto-fill new payment value from remaining ───────────────────────────

  useEffect(() => {
    if (!paymentValueLockedRef.current) {
      setNewPaymentValue(remaining > 0.005 ? remaining.toFixed(2) : '')
    }
  }, [remaining])

  // ─── Auto-fill fatura on first prazo payment (setState during render) ──────
  // Avoids the passive-effect setState cascade error in React 19.

  if (hasPrazoPayment !== prevHasPrazoPayment) {
    setPrevHasPrazoPayment(hasPrazoPayment)
    if (hasPrazoPayment) {
      setCobrFat(f => ({
        ...f,
        v_orig: f.v_orig || totalProducts.toFixed(2),
        v_desc: f.v_desc || (totalDiscount > 0 ? totalDiscount.toFixed(2) : ''),
        v_liq: f.v_liq || totalNfe.toFixed(2),
      }))
    }
  }

  // ─── Reset entrega when the destinatário changes (setState during render —
  // same pattern as prevHasPrazoPayment above, avoids the React 19 passive-
  // effect setState cascade). Saved locations are per-destinatário, so a
  // stale entrega from a previous receiver must not survive a receiver swap.

  const receiverSk = receiver?.sk ?? null
  if (receiverSk !== prevReceiverSk) {
    setPrevReceiverSk(receiverSk)
    setEntrega(null)
    setSaveEntregaLocation(false)
  }

  // ─── Step navigation ──────────────────────────────────────────────────────

  function canGoNext(step: EmitStep): boolean {
    if (step === 'destinatario') return selfIssuance || receiver !== null
    if (step === 'produtos') return products.length > 0 && !cfopMixError && !cfopUnresolvedError
    if (step === 'pagamento') {
      if (payments.length > 0) return true
      if (newPaymentType === '90') return true
      return !!newPaymentValue && parseFloat(newPaymentValue) > 0
    }
    return true
  }

  function handleNext() {
    const i = STEP_IDS.indexOf(currentStep)
    if (i < STEP_IDS.length - 1) {
      if (currentStep === 'destinatario' && products.length === 0) {
        const firstProduct = productsData?.items?.[0]
        if (firstProduct) handleSelectProduct(firstProduct)
      }
      if (currentStep === 'pagamento') {
        const isNoPay = newPaymentType === '90'
        const hasValidValue = isNoPay || (!!newPaymentValue && parseFloat(newPaymentValue) > 0)
        if (hasValidValue && payments.length === 0) handleAddPayment()
      }
      setCurrentStep(STEP_IDS[i + 1])
    }
  }

  function handleBack() {
    const i = STEP_IDS.indexOf(currentStep)
    if (i > 0) setCurrentStep(STEP_IDS[i - 1])
  }

  // ─── Product handlers ─────────────────────────────────────────────────────

  const handleSelectProduct = (product: ProductOut) => {
    const groups = groupCfopConfigBySuffix(product.cfop_config)
    const firstGroup = groups[0]
    const firstSuffix = firstGroup?.suffix ?? (product.cfop_nfce ? cfopSuffix(product.cfop_nfce) : '')
    const resolvedCfop = firstGroup && sameUf !== null
      ? (resolveCfopForUf(firstGroup, sameUf) ?? '')
      : (product.cfop_config[0]?.cfop ?? product.cfop_nfce ?? '')
    // NF-e: consumer-final price for CPF, resale price for CNPJ (self-issuance = org CNPJ).
    const recipientDoc = selfIssuance
      ? unformatCpfCnpj(selectedOrg?.pk ?? '')
      : unformatCpfCnpj(receiver?.sk ?? '')
    setProducts(prev => [...prev, {
      product, cfop: resolvedCfop, cfopSuffix: firstSuffix, qty: '1',
      unitValue: resolveUnitPrice(product, recipientDoc), discount: '0',
      armas: product.prod_type === 'arma' ? [] : undefined,
    }])
    setShowProductPicker(false)
  }

  const handleProductChange = (index: number, updated: Partial<EmitProduct>) =>
    setProducts(prev => prev.map((item, i) => (i === index ? {...item, ...updated} : item)))

  const handleProductRemove = (index: number) =>
    setProducts(prev => prev.filter((_, i) => i !== index))

  // ─── Payment handlers ─────────────────────────────────────────────────────

  const handleAddPayment = () => {
    const isNoPay = newPaymentType === '90'
    if (!isNoPay && (!newPaymentValue || parseFloat(newPaymentValue) <= 0)) return
    const value = isNoPay ? '0.00' : newPaymentValue
    setPayments(prev => [...prev, {
      payment_type: newPaymentType,
      value,
      ind_pag: newPaymentIndPag,
      card: showCardToggle ? newPaymentCard : null,
    }])
    paymentValueLockedRef.current = false
    setNewPaymentCard(null)
    setShowCardToggle(false)
    setNewPaymentIndPag('0')
  }

  const handleRemovePayment = (index: number) => {
    paymentValueLockedRef.current = false
    setPayments(prev => prev.filter((_, i) => i !== index))
  }

  // ─── Duplicata handlers ───────────────────────────────────────────────────

  const handleGenerateDuplicatas = () => {
    const n = parseInt(dupCount) || 1
    const total = parseFloat(cobrFat.v_liq || cobrFat.v_orig || totalNfe.toFixed(2)) || totalNfe
    const firstDate = dupFirstDate || (() => {
      // Default: 30 days from today
      const d = new Date()
      d.setMonth(d.getMonth() + 1)
      const lastDay = new Date(d.getFullYear(), d.getMonth() + 1, 0).getDate()
      d.setDate(Math.min(d.getDate(), lastDay))
      return d.toISOString().split('T')[0]
    })()
    setDuplicatas(generateDuplicatas(total, n, firstDate))
  }

  // ─── Carrier handler ──────────────────────────────────────────────────────

  const handleCreateCarrier = async (data: PersonCreate) => {
    setCreateCarrierLoading(true)
    try {
      const created = await apiClient.createPerson(data)
      setSelectedCarrier(created)
      setShowCarrierModal(false)
    } finally {
      setCreateCarrierLoading(false)
    }
  }

  // ─── Submit ───────────────────────────────────────────────────────────────

  const handleSubmit = async () => {
    setSubmitError(null)
    if (cfopMixError) {
      setSubmitError('Não é possível misturar CFOPs de entrada e saída na mesma NF-e.')
      return
    }
    if (cfopUnresolvedError) {
      setSubmitError('Há produtos sem CFOP válido para a UF do destinatário. Selecione um destinatário com UF e configure o CFOP de mesma natureza para a UF de destino.')
      return
    }
    const vTroco = totalPaid > totalNfe + 0.005 ? (totalPaid - totalNfe).toFixed(2) : null
    const hasCobr = hasPrazoPayment && (cobrFat.v_liq || duplicatas.length > 0)

    const payload: NfeEmit = {
      ...(selfIssuance ? {self_issuance: true} : {receiver_id: receiver!.sk}),
      products: products.map(p => ({
        product_id: p.product.sk, cfop: p.cfop, quantity: p.qty,
        unit_value: p.unitValue || null, discount: p.discount || '0',
        veic_chassi: p.veic_chassi || null, veic_n_serie: p.veic_n_serie || null,
        veic_n_motor: p.veic_n_motor || null, veic_c_cor: p.veic_c_cor || null,
        veic_x_cor: p.veic_x_cor || null,
        armas: p.armas && p.armas.length > 0 ? p.armas : null,
      })),
      payments: payments.map(p => ({
        payment_type: p.payment_type, value: p.value,
        ind_pag: p.ind_pag || undefined, card: p.card || undefined,
      })),
      additional_info: additionalInfo.trim() || null,
      nat_op: natOp || null,
      tp_nf: tpNf,
      transport: showTransport ? (() => {
        const mf = transport.mod_frete as NfeTransportIn['mod_frete']
        let carrierPk: string | null = null
        if (mf === '0' || mf === '1' || mf === '2') carrierPk = selectedCarrier?.sk ?? null
        return {
          mod_frete: mf, transporta_pk: carrierPk,
          veiculo_sk: selectedVehicle?.sk ?? null,
          veiculo_placa: !selectedVehicle ? (transport.veiculo_placa || null) : null,
          veiculo_uf: !selectedVehicle ? (transport.veiculo_uf || null) : null,
          veiculo_rntrc: !selectedVehicle ? (transport.veiculo_rntrc || null) : null,
        } as NfeTransportIn
      })() : null,
      cobr_fat: hasCobr ? {
        n_fat: cobrFat.n_fat || null, v_orig: cobrFat.v_orig || null,
        v_desc: cobrFat.v_desc || null, v_liq: cobrFat.v_liq || null,
      } : null,
      cobr_duplicatas: hasCobr && duplicatas.length > 0
        ? duplicatas.map(d => ({n_dup: d.n_dup || null, d_venc: d.d_venc || null, v_dup: d.v_dup} as NfeDuplicataIn))
        : null,
      v_troco: vTroco,
      retirada: retirada,
      entrega: entrega,
      save_retirada_location: retirada ? saveRetiradaLocation : false,
      save_entrega_location: entrega ? saveEntregaLocation : false,
    }

    setIsSubmitting(true)
    try {
      await apiClient.emitNfe(payload)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao emitir NF-e.')
      setIsSubmitting(false)
      return
    }
    setIsSubmitting(false)
    router.push('/nfe')
  }

  if (!selectedOrg) {
    return <div className="text-center py-12 text-sm text-gray-500">Selecione uma organização para emitir NF-e.</div>
  }

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <div className="max-w-3xl space-y-0 pb-4">
      <HomologationBanner environment={nfeConfig?.environment}/>

      {/* Step progress */}
      <StepIndicator current={currentStep} steps={STEPS}/>

      {/* Error */}
      {submitError && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 mb-4">
          {submitError}
        </div>
      )}

      {/* ──────────────── Step 1: Destinatário ──────────────────────────── */}
      {currentStep === 'destinatario' && (
        <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Para quem?</p>
            <button
              type="button"
              onClick={() => {
                setSelfIssuance(v => {
                  if (!v) setReceiver(null)
                  return !v
                })
              }}
              className={`text-xs font-medium px-3 py-1.5 rounded-full border transition-colors ${
                selfIssuance
                  ? 'border-brand-400 bg-brand-50 text-brand-700'
                  : 'border-gray-200 bg-white text-gray-500 hover:border-brand-300 hover:text-brand-700'
              }`}
            >
              Para si mesmo
            </button>
          </div>

          {selfIssuance ? (
            <div className="rounded-lg border border-brand-200 bg-brand-50/40 px-4 py-3">
              <p className="text-sm font-medium text-brand-800">Emissão própria</p>
              <p className="text-xs text-brand-600 mt-0.5">O destinatário será a própria organização emissora.</p>
            </div>
          ) : (
            <>
              {!receiver && (recentNfesLoading || favoriteReceivers.length > 0) && (
                <div className="space-y-2">
                  <p className="text-xs font-medium text-gray-400">Recentes</p>
                  <div className="flex flex-wrap gap-2">
                    {recentNfesLoading ? (
                      <>
                        <div className="h-7 w-24 rounded-full bg-gray-100 animate-pulse"/>
                        <div className="h-7 w-32 rounded-full bg-gray-100 animate-pulse"/>
                        <div className="h-7 w-20 rounded-full bg-gray-100 animate-pulse"/>
                      </>
                    ) : favoriteReceivers.map(fav => (
                      <button
                        key={fav.cpfCnpj}
                        type="button"
                        disabled={loadingFavCpfCnpj !== null}
                        onClick={async () => {
                          setLoadingFavCpfCnpj(fav.cpfCnpj)
                          try {
                            const person = await apiClient.getPersonByCpfCnpj(fav.cpfCnpj)
                            setReceiver(person)
                          } catch { /* person deleted — fall through to manual search */
                          } finally {
                            setLoadingFavCpfCnpj(null)
                          }
                        }}
                        className="rounded-full border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 hover:border-brand-300 hover:text-brand-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        {loadingFavCpfCnpj === fav.cpfCnpj ? (
                          <span className="flex items-center gap-1.5">
                            <span
                              className="inline-block w-3 h-3 border-2 border-gray-300 border-t-brand-500 rounded-full animate-spin"/>
                            {fav.name}
                          </span>
                        ) : fav.name}
                      </button>
                    ))}
                  </div>
                </div>
              )}
              <ReceiverSearch value={receiver} onChange={setReceiver}/>
              {receiver && (
                <LocationPicker
                  label="Local de entrega"
                  savedLocations={receiver.person.delivery_locations ?? []}
                  value={entrega}
                  onChange={setEntrega}
                  save={saveEntregaLocation}
                  onSaveChange={setSaveEntregaLocation}
                />
              )}
            </>
          )}
          <LocationPicker
            label="Local de retirada"
            savedLocations={orgData?.pickup_locations ?? []}
            value={retirada}
            onChange={setRetirada}
            save={saveRetiradaLocation}
            onSaveChange={setSaveRetiradaLocation}
          />
        </div>
      )}

      {/* ──────────────── Step 2: Produtos ──────────────────────────────── */}
      {currentStep === 'produtos' && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
              Produtos ({products.length})
            </p>
            <span className="text-sm font-semibold text-gray-900">{fmt(totalNfe)}</span>
          </div>
          {products.length > 0 && (
            <div className="space-y-2">
              {products.map((item, i) => (
                <ProductRow key={`${item.product.sk}-${i}`} item={item} index={i} sameUf={sameUf}
                            onChange={handleProductChange} onRemove={handleProductRemove}/>
              ))}
            </div>
          )}
          {cfopMixError && (
            <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              Não é possível misturar CFOPs de entrada (1/2/3) e de saída (5/6/7) na mesma NF-e.
              O tipo da nota é definido pelo CFOP do primeiro produto
              ({noteDirection === 'in' ? 'entrada' : 'saída'}).
            </div>
          )}
          {products.length > 0 && natOp && !cfopMixError && (
            <NatOpInlineEdit
              value={natOp}
              onChange={setNatOpManual}
              onReset={() => setNatOpManual(null)}
              canReset={natOpManual !== null}
              suffix={<span className="ml-1.5 text-gray-400">· {noteDirection === 'in' ? 'Entrada' : 'Saída'}</span>}
            />
          )}
          {showProductPicker ? (
            <ProductPicker onSelect={handleSelectProduct} onClose={() => setShowProductPicker(false)}/>
          ) : (
            <Button type="button" variant="ghost" size="sm" onClick={() => setShowProductPicker(true)}
                    className="text-brand-600 hover:text-brand-700 px-0">
              + Adicionar produto
            </Button>
          )}
        </div>
      )}

      {/* ──────────────── Step 3: Pagamento ─────────────────────────────── */}
      {currentStep === 'pagamento' && (
        <div className="space-y-4">
          {/* Payment list */}
          {payments.length > 0 && (
            <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Pagamentos</p>
              {payments.map((p, i) => (
                <div key={i} className="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2.5 text-sm">
                  <span className="text-gray-700">
                    {NF_PAYMENT_TYPES[p.payment_type] ?? p.payment_type}
                    {p.ind_pag === '1' && <span className="ml-1.5 text-xs text-amber-600">(prazo)</span>}
                    {p.card && <span className="ml-1.5 text-xs text-blue-600">· transação</span>}
                  </span>
                  <div className="flex items-center gap-3">
                    <span className="font-medium">{fmt(parseFloat(p.value) || 0)}</span>
                    <Button type="button" variant="ghost" size="xs" onClick={() => handleRemovePayment(i)}
                            className="text-red-500 hover:text-red-700">remover</Button>
                  </div>
                </div>
              ))}
              <p
                className={`text-sm pt-1 ${Math.abs(remaining) < 0.01 ? 'text-green-600' : remaining < 0 ? 'text-blue-600' : 'text-amber-600'}`}>
                {Math.abs(remaining) < 0.01 ? '✓ Total confere.' : remaining > 0 ? `Restam ${fmt(remaining)}.` : `Troco: ${fmt(-remaining)}`}
              </p>
            </div>
          )}

          {/* Add payment */}
          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Adicionar pagamento</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-[1fr_auto_auto_auto] gap-2 items-end">
              <div className="flex flex-col gap-1">
                <Label className="text-xs font-medium text-gray-600">Forma de pagamento</Label>
                <OptionsSelect value={newPaymentType}
                               onValueChange={(v) => {
                                 setNewPaymentType(v)
                                 setShowCardToggle(false)
                                 setNewPaymentCard(null)
                               }}
                               options={PAYMENT_OPTIONS}/>
              </div>
              {newPaymentType !== '90' && (
                <div className="flex flex-col gap-1">
                  <Label className="text-xs font-medium text-gray-600 whitespace-nowrap">À vista / Parcelado</Label>
                  <OptionsSelect value={newPaymentIndPag} onValueChange={v => setNewPaymentIndPag(v as '0' | '1')}
                                 options={[{value: '0', label: '0 – À vista'}, {value: '1', label: '1 – Parcelado'}]}/>
                </div>
              )}
              {newPaymentType !== '90' && (
                <div className="flex flex-col gap-1">
                  <Label className="text-xs font-medium text-gray-600">Valor</Label>
                  <CurrencyInput decimalPlaces={2} value={newPaymentValue}
                                 onChange={(v) => {
                                   paymentValueLockedRef.current = true
                                   setNewPaymentValue(v)
                                 }}
                                 placeholder="0,00"/>
                </div>
              )}
              <Button type="button" variant="brand" onClick={handleAddPayment}
                      className="self-end"
                      disabled={newPaymentType !== '90' && (!newPaymentValue || parseFloat(newPaymentValue) <= 0)}>
                Adicionar
              </Button>
            </div>
            {newPaymentType === '90' && (
              <p className="text-xs text-gray-400">Sem pagamento: valor zero será registrado na NF-e.</p>
            )}

            {/* Card/PIX toggle */}
            {isCardPayment && (
              <div className="pt-1 border-t border-gray-100 space-y-2">
                <div className="flex items-center gap-2">
                  <input type="checkbox" id="toggle-card" checked={showCardToggle}
                         onChange={e => {
                           setShowCardToggle(e.target.checked);
                           if (!e.target.checked) setNewPaymentCard(null)
                         }}
                         className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
                  <label htmlFor="toggle-card" className="text-xs font-medium text-gray-500 cursor-pointer">
                    {isPix ? 'Informar NSU/autorização (opcional)' : 'Informar dados do cartão'}
                  </label>
                </div>
                {showCardToggle && (
                  <PaymentCardFields card={newPaymentCard} onChange={setNewPaymentCard} isPix={isPix}/>
                )}
              </div>
            )}
          </div>

          {/* Cobrança — only when ind_pag=1 payment exists */}
          {hasPrazoPayment && (
            <div className="rounded-xl border border-amber-100 bg-amber-50/20 p-4 space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-xs font-semibold uppercase tracking-wider text-amber-700">Cobrança (duplicatas)</p>
                <Button type="button" variant="ghost" size="xs"
                        className="text-amber-600 hover:text-amber-800 text-xs"
                        onClick={() => setCobrFat({
                          n_fat: cobrFat.n_fat,
                          v_orig: totalProducts.toFixed(2),
                          v_desc: totalDiscount > 0 ? totalDiscount.toFixed(2) : '',
                          v_liq: totalNfe.toFixed(2),
                        })}>
                  Sincronizar com total
                </Button>
              </div>

              {/* Fatura */}
              <div>
                <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">Fatura</p>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                  <div className="flex flex-col gap-1">
                    <Label className="text-xs font-medium text-gray-600">Número</Label>
                    <Input value={cobrFat.n_fat ?? ''} onChange={e => setCobrFat(f => ({...f, n_fat: e.target.value}))}
                           placeholder="FAT-001" maxLength={60}/>
                  </div>
                  <div className="flex flex-col gap-1">
                    <Label className="text-xs font-medium text-gray-600">Valor original</Label>
                    <CurrencyInput decimalPlaces={2} value={cobrFat.v_orig ?? ''}
                                   onChange={v => setCobrFat(f => ({...f, v_orig: v}))} placeholder="0,00"/>
                  </div>
                  <div className="flex flex-col gap-1">
                    <Label className="text-xs font-medium text-gray-600">Desconto</Label>
                    <CurrencyInput decimalPlaces={2} value={cobrFat.v_desc ?? ''}
                                   onChange={v => setCobrFat(f => ({...f, v_desc: v}))} placeholder="0,00"/>
                  </div>
                  <div className="flex flex-col gap-1">
                    <Label className="text-xs font-medium text-gray-600">Valor líquido</Label>
                    <CurrencyInput decimalPlaces={2} value={cobrFat.v_liq ?? ''}
                                   onChange={v => setCobrFat(f => ({...f, v_liq: v}))} placeholder="0,00"/>
                  </div>
                </div>
              </div>

              {/* Duplicatas */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                    Parcelas {duplicatas.length > 0 && `(${duplicatas.length})`}
                  </p>
                  {duplicatas.length > 0 && (() => {
                    const allocated = duplicatas.reduce((s, d) => s + (parseFloat(d.v_dup) || 0), 0)
                    const expected = parseFloat(cobrFat.v_liq || cobrFat.v_orig || totalNfe.toFixed(2)) || totalNfe
                    const diff = Math.abs(allocated - expected)
                    return (
                      <span className={`text-xs font-medium ${diff < 0.01 ? 'text-green-600' : 'text-amber-600'}`}>
                        {diff < 0.01 ? '✓ Total conferido' : `${fmt(allocated)} de ${fmt(expected)}`}
                      </span>
                    )
                  })()}
                </div>

                {/* Generator */}
                <div className="rounded-lg bg-white border border-gray-100 p-3">
                  <div className="flex flex-wrap items-end gap-2">
                    <div className="flex flex-col gap-1 w-24">
                      <Label className="text-xs font-medium text-gray-600 whitespace-nowrap">Nº parcelas</Label>
                      <NumericInput value={dupCount} onChange={setDupCount} placeholder="1" integerPlaces={3}/>
                    </div>
                    <div className="flex flex-col gap-1 w-40">
                      <Label className="text-xs font-medium text-gray-600 whitespace-nowrap">1º vencimento</Label>
                      <Input type="date" value={dupFirstDate} onChange={e => setDupFirstDate(e.target.value)}/>
                    </div>
                    <Button type="button" variant="brand" size="sm" onClick={handleGenerateDuplicatas}>
                      Gerar parcelas
                    </Button>
                    {duplicatas.length > 0 && (
                      <Button type="button" variant="ghost" size="sm" onClick={() => setDuplicatas([])}
                              className="text-red-500 hover:text-red-700">Limpar</Button>
                    )}
                  </div>
                </div>

                {/* Generated list */}
                {duplicatas.length > 0 && (
                  <div className="rounded-lg bg-white border border-gray-100 overflow-hidden">
                    {duplicatas.map((d, i) => (
                      <div key={i}
                           className="flex items-center gap-2 px-3 py-2 border-b last:border-b-0 border-gray-50">
                        <span className="font-mono text-xs text-gray-400 w-7 shrink-0">{d.n_dup}</span>
                        <Input type="date" value={d.d_venc}
                               onChange={e => {
                                 const newDate = e.target.value
                                 setDuplicatas(prev => {
                                   const updated = [...prev]
                                   updated[i] = {...updated[i], d_venc: newDate}
                                   if (i === 0 && newDate) {
                                     for (let j = 1; j < updated.length; j++) {
                                       updated[j] = {...updated[j], d_venc: addOneMonth(updated[j - 1].d_venc)}
                                     }
                                   }
                                   return updated
                                 })
                               }}
                               className="w-36 text-xs h-7 shrink-0"/>
                        <CurrencyInput decimalPlaces={2} value={d.v_dup}
                                       onChange={v => setDuplicatas(prev => prev.map((x, j) => j === i ? {
                                         ...x,
                                         v_dup: v
                                       } : x))}
                                       className="flex-1 min-w-0 h-7"/>
                        <Button type="button" variant="ghost" size="xs"
                                onClick={() => setDuplicatas(prev => prev.filter((_, j) => j !== i))}
                                className="text-red-400 hover:text-red-600 shrink-0 px-1">×</Button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {/* ──────────────── Step 4: Revisão ───────────────────────────────── */}
      {currentStep === 'revisao' && (
        <div className="space-y-4">
          {/* Summary */}
          <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Resumo</p>
            <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
              <div>
                <span className="text-gray-500">Destinatário</span>
                <p className="font-medium text-gray-900">
                  {selfIssuance ? <span className="text-brand-700">Emissão própria</span> : receiver?.name}
                </p>
              </div>
              <div>
                <span className="text-gray-500">Total</span>
                <p className="font-semibold text-gray-900 text-base">{fmt(totalNfe)}</p>
              </div>
              <div>
                <span className="text-gray-500">Produtos</span>
                <p className="font-medium text-gray-900">{products.length} item(s)</p>
              </div>
              <div>
                <span className="text-gray-500">Pagamento</span>
                <p className="font-medium text-gray-900">
                  {payments.map(p => `${NF_PAYMENT_TYPES[p.payment_type] ?? p.payment_type} ${fmt(parseFloat(p.value) || 0)}`).join(' + ')}
                </p>
              </div>
            </div>
          </div>

          {/* Transport */}
          <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-3">
            <div className="flex items-center gap-2">
              <input type="checkbox" id="toggle-transport" checked={showTransport}
                     onChange={e => {
                       setShowTransport(e.target.checked);
                       if (!e.target.checked) {
                         setSelectedCarrier(null);
                         setSelectedVehicle(null)
                       }
                     }}
                     className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
              <label htmlFor="toggle-transport"
                     className="text-xs font-semibold uppercase tracking-wider text-gray-400 cursor-pointer">
                Transporte
              </label>
            </div>
            {showTransport && (
              <div className="space-y-4">
                <div className="flex flex-col gap-1 max-w-xs">
                  <Label className="text-xs font-medium text-gray-600">Modalidade</Label>
                  <OptionsSelect value={transport.mod_frete}
                                 onValueChange={v => setTransport(t => ({...t, mod_frete: v}))}
                                 options={MOD_FRETE_OPTIONS}/>
                </div>
                {transport.mod_frete !== '9' && (
                  <>
                    <div className="space-y-2">
                      <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">Transportadora</p>
                      {(transport.mod_frete === '3' || transport.mod_frete === '4') && (
                        <p className="text-sm text-gray-500 rounded-lg bg-gray-50 border border-gray-100 px-4 py-2.5">
                          Transporte próprio — sem transportadora externa.
                        </p>
                      )}
                      {(transport.mod_frete === '0' || transport.mod_frete === '1' || transport.mod_frete === '2') && (
                        selectedCarrier ? (
                          <div
                            className="flex items-center gap-3 rounded-lg border border-green-200 bg-green-50 px-4 py-2.5">
                            <div className="flex-1"><p
                              className="font-medium text-gray-900 text-sm">{selectedCarrier.name}</p></div>
                            <Button type="button" variant="ghost" size="xs" onClick={() => setSelectedCarrier(null)}
                                    className="text-red-500">Trocar</Button>
                          </div>
                        ) : (
                          <div className="space-y-2">
                            <CarrierSearch onSelect={setSelectedCarrier}/>
                            <Button type="button" variant="ghost" size="xs" onClick={() => setShowCarrierModal(true)}
                                    className="text-brand-600 px-0 text-xs">+ Cadastrar nova</Button>
                          </div>
                        )
                      )}
                    </div>
                    <div className="space-y-2">
                      <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">Veículo</p>
                      {selectedVehicle ? (
                        <div
                          className="flex items-center gap-3 rounded-lg border border-green-200 bg-green-50 px-4 py-2.5">
                          <div className="flex-1"><p
                            className="font-mono text-sm font-medium">{selectedVehicle.plate}</p><p
                            className="text-xs text-gray-500">{selectedVehicle.plate_uf}</p></div>
                          <Button type="button" variant="ghost" size="xs" onClick={() => setSelectedVehicle(null)}
                                  className="text-red-500">Trocar</Button>
                        </div>
                      ) : (
                        <VehicleSelect vehicles={vehiclesData?.items ?? []} onSelect={setSelectedVehicle}
                                       query={vehicleSearchQuery} onQueryChange={setVehicleSearchQuery}/>
                      )}
                      {!selectedVehicle && (
                        <details className="text-xs">
                          <summary className="cursor-pointer text-gray-400">Informar manualmente</summary>
                          <div className="grid grid-cols-3 gap-2 mt-2">
                            <div className="flex flex-col gap-1"><Label
                              className="text-xs font-medium text-gray-600">Placa</Label><Input
                              value={transport.veiculo_placa}
                              onChange={e => setTransport(t => ({...t, veiculo_placa: e.target.value.toUpperCase()}))}
                              placeholder="ABC1D23" maxLength={8}/></div>
                            <div className="flex flex-col gap-1"><Label
                              className="text-xs font-medium text-gray-600">UF</Label><Input
                              value={transport.veiculo_uf}
                              onChange={e => setTransport(t => ({...t, veiculo_uf: e.target.value.toUpperCase()}))}
                              placeholder="SP" maxLength={2}/></div>
                            <div className="flex flex-col gap-1"><Label
                              className="text-xs font-medium text-gray-600">RNTRC</Label><Input
                              value={transport.veiculo_rntrc}
                              onChange={e => setTransport(t => ({...t, veiculo_rntrc: e.target.value}))}
                              placeholder="Opcional"/></div>
                          </div>
                        </details>
                      )}
                    </div>
                  </>
                )}
              </div>
            )}
          </div>

          {/* Additional info */}
          <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-2">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Informações adicionais</p>
            <Textarea value={additionalInfo} onChange={e => setAdditionalInfo(e.target.value)}
                      placeholder="Observações, dados ao fisco, pedido, etc. (opcional)" rows={3}/>
          </div>
        </div>
      )}

      {/* ── Navigation bar ────────────────────────────────────────────────── */}
      <div
        className="sticky bottom-0 bg-gray-50 border-t border-gray-200 -mx-4 md:-mx-8 px-4 md:px-8 py-3 md:py-4 flex items-center justify-between gap-2">
        {/* Left: totals (steps 2+) */}
        <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-xs md:text-sm min-w-0">
          {currentStep !== 'destinatario' && totalProducts > 0 && (
            <>
              <span className="text-gray-500 hidden sm:inline">Produtos: <span
                className="font-medium text-gray-900">{fmt(totalProducts)}</span></span>
              {totalDiscount > 0 && <span className="text-gray-500 hidden sm:inline">Desc: <span
                  className="font-medium text-red-600">-{fmt(totalDiscount)}</span></span>}
              <span className="text-gray-700 font-semibold">Total: <span
                className="text-gray-900">{fmt(totalNfe)}</span></span>
            </>
          )}
        </div>

        {/* Right: navigation / submit */}
        <div className="flex items-center gap-2 shrink-0">
          {currentStep !== 'destinatario' && (
            <Button type="button" variant="outline" size="sm" onClick={handleBack}>← Voltar</Button>
          )}

          {currentStep !== 'revisao' ? (
            <Button type="button" variant="brand" size="sm" disabled={!canGoNext(currentStep)} onClick={handleNext}>
              Próximo →
            </Button>
          ) : (
            <Button type="button" variant="brand" size="sm" disabled={isSubmitting} onClick={handleSubmit}>
              {isSubmitting ? 'Emitindo...' : 'Emitir NF-e'}
            </Button>
          )}
        </div>
      </div>

      {/* Modals */}
      <Modal isOpen={showCarrierModal} title="Cadastrar nova transportadora"
             onClose={() => setShowCarrierModal(false)} size="xl">
        <PersonForm onSubmit={handleCreateCarrier} loading={createCarrierLoading}/>
      </Modal>
    </div>
  )
}
