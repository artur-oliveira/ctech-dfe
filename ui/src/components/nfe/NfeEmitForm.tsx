'use client'

import {useCallback, useEffect, useMemo, useRef, useState} from 'react'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {useRouter} from 'next/navigation'
import {useQuery} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient} from '@/lib/api/client'
import {duplicataSumGap, paymentBalanceGap, SUM_TOLERANCE, unitDataGap} from '@/lib/utils/emit-guards'
import {emitFailure, type EmitFailure} from '@/lib/billing/notice'
import {Textarea} from '@/components/ui/textarea'
import {GlossaryTerm} from '@/components/ui/glossary-term'
import {NumericInput} from '@/components/ui/numeric-input'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {CollapsibleSection} from '@/components/ui/collapsible-section'
import {datetimeLocalToOffset} from '@/lib/utils/datetime'
import {AccessKeyPicker} from '@/components/nfe/AccessKeyPicker'
import {
  COMPRA_GOV_TP_OPER_COM_REFERENCIA,
  COMPRA_GOV_TP_OPER_REFERENCIA_UNICA,
} from '@/lib/data/ibs_cbs_reform'
import {
  EMPTY_NICHE_GROUPS,
  NicheGroupsFields,
  type NicheGroupsValue,
} from '@/components/nfe/NicheGroupsFields'
import {Modal} from '@/components/ui/modal'
import {EmitConfirmModal} from '@/components/ui/emit-confirm-modal'
import {EmitError} from '@/components/ui/emit-error'
import {DraftRecoveryBanner} from '@/components/ui/draft-recovery-banner'
import {useEmitDraft} from '@/lib/hooks/useEmitDraft'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import {StepIndicator} from '@/components/ui/step-indicator'
import type {
  NfeArmaIn,
  NfeIBSCBSPairIn,
  NfeCardIn,
  NfeDuplicataIn,
  NfeEmit,
  NfeFatIn,
  NfeListOut,
  NfeLocalIn,
  NfeProcRefIn,
  NfeRefIn,
  NfeReboqueIn,
  NfeTransportIn,
  NfeVolIn,
  PersonCreate,
  PersonItemOut,
  ProductOut,
  VehicleOut
} from '@/lib/types/api'
import {NF_PAYMENT_TYPES} from '@/lib/types/api'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {PersonForm} from '@/components/persons/PersonForm'
import {PersonPicker} from '@/components/persons/PersonPicker'
import {IND_PROC_OPTIONS, MOD_FRETE_OPTIONS} from '@/lib/data/nfe_fields'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {resolveCfopScope} from '@/lib/data/cfop'
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
import {CARD_PAYMENT_TYPES, isPixPaymentType, PaymentCardFields} from "@/components/nfe/PaymentCardFields"
import {ProductLineItem} from "@/components/ui/product-line-item"
import {ProductSearch} from "@/components/ui/product-search"
import {NO_PAYMENT_TYPE, PAYMENT_OPTIONS} from "@/lib/data/payment-options"
import {previewInstallments} from "@/lib/schemas/payment-terms"
import {NatOpInlineEdit} from "@/components/nfe/NatOpInlineEdit"
import {LocationPicker} from "@/components/nfe/LocationPicker"
import {NfeRefsPicker} from "@/components/nfe/NfeRefsPicker"
import {VolumesFields} from "@/components/nfe/VolumesFields"
import {finNFeRequiresRef} from "@/lib/schemas/nfe-refs"

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
  // Pedido de compra do cliente (prod/xPed, prod/nItemPed) — controle B2B.
  x_ped?: string
  n_item_ped?: string
  // Grupos apurados da reforma. transf_cred e ajuste_compet substituem a
  // apuração normal do item (choice do XSD); estorno_cred convive com ela.
  reform_mode?: ReformItemMode
  reform_v_ibs?: string
  reform_v_cbs?: string
  estorno_v_ibs?: string
  estorno_v_cbs?: string
}

/**
 * Ramo escolhido do choice de apuração da reforma no item. O radio existe para
 * que o operador não possa marcar dois — o XSD os declara alternativos, e a
 * alternativa seria descobrir isso na rejeição.
 */
type ReformItemMode = 'none' | 'transf_cred' | 'ajuste_compet'

const REFORM_ITEM_MODE_OPTIONS: { value: ReformItemMode; label: string }[] = [
  {value: 'none', label: 'Apuração normal'},
  {value: 'transf_cred', label: 'Transferência de crédito'},
  {value: 'ajuste_compet', label: 'Ajuste de competência'},
]

/** Par IBS/CBS do ramo escolhido; null quando o item não usa aquele ramo. */
function reformPair(item: EmitProduct, mode: ReformItemMode): NfeIBSCBSPairIn | null {
  if ((item.reform_mode ?? 'none') !== mode) return null
  if (!item.reform_v_ibs && !item.reform_v_cbs) return null
  return {v_ibs: item.reform_v_ibs || null, v_cbs: item.reform_v_cbs || null}
}

/** Estorno de crédito do item — convive com qualquer ramo da apuração. */
function estornoPair(item: EmitProduct): NfeIBSCBSPairIn | null {
  if (!item.estorno_v_ibs && !item.estorno_v_cbs) return null
  return {v_ibs: item.estorno_v_ibs || null, v_cbs: item.estorno_v_cbs || null}
}

interface EmitPayment {
  payment_type: string
  value: string
  ind_pag: '0' | '1'
  card: NfeCardIn | null
  /** Terminal de captura que processou o pagamento, quando houver. */
  terminal_id: string | null
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

/** Data de hoje em ISO — piso de vencimento: duplicata vencida antes da emissão é rejeição. */
function todayIso(): string {
  return localIso().slice(0, 10)
}

/** Agora no formato do input datetime-local (hora local, não UTC). */
function localIso(): string {
  const now = new Date()
  const offsetMs = now.getTimezoneOffset() * 60_000
  return new Date(now.getTime() - offsetMs).toISOString().slice(0, 16)
}

function itemUnitDataGap(item: EmitProduct): string | null {
  return unitDataGap({
    prodType: item.product.prod_type,
    chassi: item.veic_chassi,
    nSerie: item.veic_n_serie,
    nMotor: item.veic_n_motor,
    armaCount: (item.armas ?? []).length,
  })
}

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
            aria-label="Buscar destinatário por nome, CPF ou CNPJ"
            role="combobox"
            aria-expanded={open && !isDoc && suggestions.length > 0}
            aria-controls="nfe-receiver-suggestions"
            aria-autocomplete="list"
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
          <p className="text-xs text-warning mt-1">{directError}</p>
        )}

        {open && !isDoc && suggestions.length > 0 && (
          <div
            id="nfe-receiver-suggestions"
            role="listbox"
            aria-label="Destinatários encontrados"
            className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-popover overflow-hidden">
            {suggestions.map((p) => {
              const cpfCnpj = unformatCpfCnpj(p.pk)
              return (
                <button
                  key={p.sk}
                  type="button"
                  role="option"
                  aria-selected={false}
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
            className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-popover px-4 py-3 space-y-2">
            <div className="h-4 w-3/4 rounded bg-gray-100 animate-pulse"/>
            <div className="h-4 w-1/2 rounded bg-gray-100 animate-pulse"/>
          </div>
        )}

        {open && !isDoc && debouncedQuery.length >= 2 && !nameQuery.isLoading && suggestions.length === 0 && !showCreate && (
          <div className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-popover px-4 py-3">
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

// ─── Product row ──────────────────────────────────────────────────────────────

interface ProductRowProps {
  item: EmitProduct
  index: number
  sameUf: boolean | null
  /** Natureza fiscal vinda da operação. Quando presente, o CFOP deixa de ser
   *  uma pergunta por item e vira texto resolvido. */
  operationCfopSuffix?: string
  onChange: (index: number, updated: Partial<EmitProduct>) => void
  onRemove: (index: number) => void
}

export function ProductRow({item, index, sameUf, operationCfopSuffix, onChange, onRemove}: ProductRowProps) {
  const cfopGroups = groupCfopConfigBySuffix(item.product.cfop_config)
  const cfopOptions = cfopGroups.map((g) => {
    const codes = cfopGroupCodes(g)
    const label = g.label ? `${codes} – ${g.label}` : codes
    return {value: g.suffix, label}
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
  const unitDataGap = itemUnitDataGap(item)

  const [newArma, setNewArma] = useState<NfeArmaIn>({n_serie: '', n_cano: '', descr: ''})

  return (
    <ProductLineItem
      idPrefix={`nfe-item-${index}`}
      description={item.product.description}
      brand={item.product.brand}
      unit={item.product.unit}
      qty={item.qty}
      unitValue={item.unitValue}
      discount={item.discount}
      total={total}
      onChange={(patch) => onChange(index, patch)}
      onRemove={() => onRemove(index)}
      badges={
        <>
          {isVeiculo && <span className="px-1.5 py-0.5 bg-indigo-100 text-indigo-700 rounded text-xs">Veículo</span>}
          {isArma && <span className="px-1.5 py-0.5 bg-red-100 text-red-700 rounded text-xs">Armamento</span>}
        </>
      }
      cfopSlot={
        <>
          <div className="flex items-center gap-1"><Label htmlFor={`nfe-item-${index}-cfop`} className="text-xs font-medium text-gray-600">CFOP</Label><GlossaryTerm term="cfop"/></div>
          {operationCfopSuffix ? (
            /* A operação já respondeu qual é a natureza fiscal: o CFOP vira
               informação, não pergunta. É o ganho real do passo 2 — o operador
               deixa de precisar saber a natureza fiscal item a item. */
            <p id={`nfe-item-${index}-cfop`} className="font-mono text-sm text-gray-700">
              {item.cfop || '—'}
              <span className="ml-2 font-sans text-xs text-gray-500">definido pela operação</span>
            </p>
          ) : cfopOptions.length > 0 ? (
            <OptionsSelect
              id={`nfe-item-${index}-cfop`}
              value={item.cfopSuffix}
              onValueChange={(suffix) => {
                const group = cfopGroups.find(g => g.suffix === suffix)
                const resolved = group && sameUf !== null ? resolveCfopForUf(group, sameUf) : null
                onChange(index, {cfopSuffix: suffix, cfop: resolved ?? ''})
              }}
              options={cfopOptions} placeholder="CFOP"/>
          ) : (
            <Input id={`nfe-item-${index}-cfop`} type="text" value={item.cfop} onChange={(e) => onChange(index, {cfop: e.target.value})}
                   maxLength={4} placeholder="5102"/>
          )}
          {cfopUfUnknown && (
            <span className="text-xs text-red-600">
              Selecione um destinatário com UF para definir o CFOP.
            </span>
          )}
          {cfopMissingVariant && (
            <span className="text-xs text-danger">
              Configure o CFOP {sameUf ? '5' : '6'}xxx neste produto para esta UF de destino.
            </span>
          )}
        </>
      }
    >

      {unitDataGap && (
        <p role="alert" className="text-[0.8rem] text-danger">{unitDataGap}</p>
      )}

      {/* ── Veículo — dados por unidade ───────────────────────────── */}
      {isVeiculo && (
        <div className="rounded-md border border-indigo-100 bg-indigo-50/30 p-3 space-y-2">
          <p className="text-sm font-medium text-indigo-800">Dados da unidade</p>
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

      {/* ── Reforma: apuração do item (choice do XSD) ─────────────── */}
      <details className="rounded-md border border-gray-200 p-3">
        <summary className="cursor-pointer text-sm font-medium text-gray-700">
          Apuração de IBS/CBS deste item (opcional)
        </summary>
        <div className="mt-2 space-y-2">
          <p className="text-xs text-gray-500">
            Transferência de crédito e ajuste de competência <strong>substituem</strong> a apuração
            normal do item — por isso a escolha é exclusiva. O estorno de crédito convive com ela.
          </p>
          <div className="flex flex-col sm:flex-row gap-2 sm:gap-4">
            {REFORM_ITEM_MODE_OPTIONS.map((opt) => (
              <label key={opt.value} htmlFor={`item-reform-${index}-${opt.value}`}
                     className="flex items-center gap-2 min-h-11 text-sm text-gray-700 cursor-pointer">
                <input type="radio" id={`item-reform-${index}-${opt.value}`}
                       name={`item-reform-${index}`} value={opt.value}
                       checked={(item.reform_mode ?? 'none') === opt.value}
                       onChange={() => onChange(index, {
                         reform_mode: opt.value,
                         // Trocar de modo zera os valores: eles significam
                         // coisas diferentes em cada ramo.
                         reform_v_ibs: '', reform_v_cbs: '',
                       })}
                       className="h-4 w-4 border-gray-300 text-brand-600"/>
                {opt.label}
              </label>
            ))}
          </div>
          {(item.reform_mode ?? 'none') !== 'none' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <div className="flex flex-col gap-1">
                <Label htmlFor={`item-reform-ibs-${index}`} className="text-xs font-medium text-gray-600">
                  Valor de IBS
                </Label>
                <CurrencyInput id={`item-reform-ibs-${index}`} value={item.reform_v_ibs ?? ''}
                               onChange={(v) => onChange(index, {reform_v_ibs: v})}/>
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor={`item-reform-cbs-${index}`} className="text-xs font-medium text-gray-600">
                  Valor de CBS
                </Label>
                <CurrencyInput id={`item-reform-cbs-${index}`} value={item.reform_v_cbs ?? ''}
                               onChange={(v) => onChange(index, {reform_v_cbs: v})}/>
              </div>
            </div>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 pt-2 border-t border-gray-100">
            <div className="flex flex-col gap-1">
              <Label htmlFor={`item-estorno-ibs-${index}`} className="text-xs font-medium text-gray-600">
                Estorno de crédito — IBS
              </Label>
              <CurrencyInput id={`item-estorno-ibs-${index}`} value={item.estorno_v_ibs ?? ''}
                             onChange={(v) => onChange(index, {estorno_v_ibs: v})}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor={`item-estorno-cbs-${index}`} className="text-xs font-medium text-gray-600">
                Estorno de crédito — CBS
              </Label>
              <CurrencyInput id={`item-estorno-cbs-${index}`} value={item.estorno_v_cbs ?? ''}
                             onChange={(v) => onChange(index, {estorno_v_cbs: v})}/>
            </div>
          </div>
        </div>
      </details>

      {/* ── Pedido de compra do cliente (prod/xPed, prod/nItemPed) ── */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
        <div className="flex flex-col gap-1">
          <Label htmlFor={`item-xped-${index}`} className="text-xs font-medium text-gray-600">
            Pedido do cliente
          </Label>
          <Input id={`item-xped-${index}`} value={item.x_ped ?? ''} maxLength={15} className="w-full"
                 placeholder="Opcional — controle B2B"
                 onChange={(e) => onChange(index, {x_ped: e.target.value})}/>
        </div>
        <div className="flex flex-col gap-1">
          <Label htmlFor={`item-nitemped-${index}`} className="text-xs font-medium text-gray-600">
            Item do pedido
          </Label>
          <NumericInput id={`item-nitemped-${index}`} value={item.n_item_ped ?? ''} maxLength={6}
                        disabled={!item.x_ped} placeholder="Só com pedido informado"
                        onChange={(v) => onChange(index, {n_item_ped: v})}/>
        </div>
      </div>

      {/* ── Armamento — dados por unidade ─────────────────────────── */}
      {isArma && (
        <div className="rounded-md border border-red-100 bg-red-50/20 p-3 space-y-2">
          <p className="text-sm font-medium text-danger">
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
                          className="text-danger hover:text-red-700">remover</Button>
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

    </ProductLineItem>
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

// ─── Review row ───────────────────────────────────────────────────────────────

/** One block of the pre-emission document preview, with a jump back to its step. */
function ReviewRow({label, onEdit, children}: {
  label: string
  onEdit: () => void
  children: React.ReactNode
}) {
  return (
    <div className="px-5 py-4">
      <div className="flex items-baseline justify-between gap-3">
        <p className="text-sm font-medium text-gray-700">{label}</p>
        <Button type="button" variant="ghost" size="xs" onClick={onEdit}
                className="text-brand-700 hover:text-brand-800 shrink-0">
          Editar
        </Button>
      </div>
      <div className="mt-1.5 text-sm">{children}</div>
    </div>
  )
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
  // rawProducts é o que o usuário montou; `products` (abaixo) é isso com o
  // CFOP da operação já aplicado, quando há operação.
  const [rawProducts, setProducts] = useState<EmitProduct[]>([])
  const [payments, setPayments] = useState<EmitPayment[]>([])
  const [additionalInfo, setAdditionalInfo] = useState('')
  const [nfRefs, setNfRefs] = useState<NfeRefIn[]>([])
  const [vols, setVols] = useState<NfeVolIn[]>([])
  const [reboques, setReboques] = useState<NfeReboqueIn[]>([])
  const [procRef, setProcRef] = useState<NfeProcRefIn[]>([])
  const [natOpManual, setNatOpManual] = useState<string | null>(null)
  // null = ainda não escolhido; a operação padrão vale como default.
  const [operationId, setOperationId] = useState<string | null>(null)
  // Grupos de nicho (compra, cana, agropecuario) — todos opcionais.
  const [nicheGroups, setNicheGroups] = useState<NicheGroupsValue>(EMPTY_NICHE_GROUPS)
  // Saída da mercadoria e previsão de entrega. Em branco, valem o prazo padrão
  // da natureza de operação (ou nenhuma tag, se ela não define prazo).
  const [dhSaiEnt, setDhSaiEnt] = useState('')
  const [dPrevEntrega, setDPrevEntrega] = useState('')
  // Chaves referenciadas da reforma: documentos anteriores da compra
  // governamental e NF-e de antecipação de pagamento a abater.
  const [compraGovRefs, setCompraGovRefs] = useState<string[]>([])
  const [pagAntecipadoRefs, setPagAntecipadoRefs] = useState<string[]>([])
  // '' = pagamento manual (comportamento de sempre).
  const [paymentTermId, setPaymentTermId] = useState('')
  const [showProductPicker, setShowProductPicker] = useState(false)
  const [newPaymentType, setNewPaymentType] = useState('01')
  const [newPaymentValue, setNewPaymentValue] = useState('')
  const paymentValueLockedRef = useRef(false)
  const [newPaymentIndPag, setNewPaymentIndPag] = useState<'0' | '1'>('0')
  const [newPaymentCard, setNewPaymentCard] = useState<NfeCardIn | null>(null)
  const [newPaymentTerminal, setNewPaymentTerminal] = useState('')
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
  const [vehicleSearchQuery, setVehicleSearchQuery] = useState('')
  const [submitError, setSubmitError] = useState<EmitFailure | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [showEmitConfirm, setShowEmitConfirm] = useState(false)
  const [loadingFavCpfCnpj, setLoadingFavCpfCnpj] = useState<string | null>(null)

  // ─── Queries ──────────────────────────────────────────────────────────────

  const {config: nfeConfig} = useFiscalConfig('nfe', selectedOrg?.pk)

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

  const {data: operationPage} = useQuery({
    queryKey: queryKeys.operations.list(selectedOrg?.pk),
    queryFn: () => apiClient.getOperations({limit: 100}),
    enabled: !!selectedOrg,
  })
  const operations = useMemo(
    () => (operationPage?.items ?? []).filter((op) => (op.doc_types ?? ['nfe']).includes('nfe')),
    [operationPage],
  )
  // A operação padrão da organização vem pré-selecionada, sem escrever estado:
  // `operationId` guarda só a escolha explícita do usuário — inclusive a
  // escolha de não usar operação nenhuma (string vazia após tocar no seletor).
  const defaultOperationId = operations.find((op) => op.is_default)
  const effectiveOperationId = operationId === null
    ? (defaultOperationId ? extractId(defaultOperationId.sk, SK_PREFIX.OPERATION) : '')
    : operationId
  const selectedOperation = operations.find(
    (op) => extractId(op.sk, SK_PREFIX.OPERATION) === effectiveOperationId,
  )

  const {data: paymentTermPage} = useQuery({
    queryKey: queryKeys.paymentTerms.list(selectedOrg?.pk),
    queryFn: () => apiClient.getPaymentTerms({limit: 100}),
    enabled: !!selectedOrg,
  })
  const paymentTerms = paymentTermPage?.items ?? []
  const selectedPaymentTerm = paymentTerms.find(
    (t) => extractId(t.sk, SK_PREFIX.PAYMENT_TERM) === paymentTermId,
  )

  const operationCfopSuffix = typeof selectedOperation?.cfop_suffix === 'string'
    ? selectedOperation.cfop_suffix
    : ''

  // Complementar, ajuste e devolução só existem contra um documento anterior:
  // a seção de referências aparece exatamente nessas finalidades.
  const operationFinNFe = typeof selectedOperation?.fin_nfe === 'string'
    ? selectedOperation.fin_nfe
    : null
  const requiresNfRefs = finNFeRequiresRef(operationFinNFe)

  // Safra da cana e CPF do responsável técnico agronômico: os dois vêm do
  // cadastro (operação e organização) e habilitam os grupos de nicho.
  const operationCanaSafra = typeof selectedOperation?.cana_safra === 'string'
    ? selectedOperation.cana_safra
    : null
  const orgTechnicalManagerCpf = typeof orgData?.person?.technical_manager_cpf === 'string'
    ? orgData.person.technical_manager_cpf
    : null
  const operationDhSaiEntOffsetDays = typeof selectedOperation?.dh_sai_ent_offset_days === 'number'
    ? selectedOperation.dh_sai_ent_offset_days
    : null

  // A regra do refDFeAnt é do leiaute: obrigatório nos tipos 2 e 3, vedado em 1
  // e 4, e no tipo 2 uma chave só. O formulário só mostra o campo quando ele é
  // aceito — assim a rejeição não é a primeira notícia da regra.
  const operationCompraGovTpOper = typeof selectedOperation?.compra_gov_tp_oper === 'string'
    ? selectedOperation.compra_gov_tp_oper
    : ''
  const compraGovNeedsRef = COMPRA_GOV_TP_OPER_COM_REFERENCIA.has(operationCompraGovTpOper)
  const compraGovRefMax = operationCompraGovTpOper === COMPRA_GOV_TP_OPER_REFERENCIA_UNICA ? 1 : undefined


  // Recipient in the issuer's UF? Self-issuance ⇒ always same UF.
  const issuerUf = selectedOrg?.state_federation ?? null
  const recipientUf = selfIssuance
    ? issuerUf
    : (receiver?.person.addresses?.[0]?.state_federation ?? null)
  const sameUf: boolean | null =
    issuerUf && recipientUf ? issuerUf === recipientUf : null

  // Com a operação escolhida, o CFOP de cada item é derivado das UFs — o
  // operador não responde item a item o que a operação já respondeu uma vez.
  // Derivado, não gravado: o estado dos produtos continua sendo o que o usuário
  // digitou, e trocar de operação não deixa CFOP velho para trás.
  const operationCfop = operationCfopSuffix && issuerUf && recipientUf
    ? resolveCfopScope(operationCfopSuffix, issuerUf, recipientUf)
    : null
  const products = operationCfop
    ? rawProducts.map((p) => ({...p, cfop: operationCfop, cfopSuffix: operationCfopSuffix}))
    : rawProducts

  // ─── Draft recovery ───────────────────────────────────────────────────────

  const draftState = useMemo(() => ({
    currentStep, receiver, selfIssuance, entrega, retirada, products: rawProducts, payments,
    additionalInfo, natOpManual, cobrFat, duplicatas, showTransport, transport,
    selectedCarrier, selectedVehicle,
  }), [currentStep, receiver, selfIssuance, entrega, retirada, rawProducts, payments,
    additionalInfo, natOpManual, cobrFat, duplicatas, showTransport, transport,
    selectedCarrier, selectedVehicle])
  const draft = useEmitDraft('nfe', selectedOrg?.pk, draftState,
    rawProducts.length > 0 || receiver !== null || selfIssuance)

  const restoreDraft = () => {
    const s = draft.recovered?.state
    if (s) {
      setCurrentStep(s.currentStep)
      setReceiver(s.receiver)
      setSelfIssuance(s.selfIssuance)
      setEntrega(s.entrega)
      setRetirada(s.retirada)
      setProducts(s.products)
      setPayments(s.payments)
      setAdditionalInfo(s.additionalInfo)
      setNatOpManual(s.natOpManual)
      setCobrFat(s.cobrFat)
      setDuplicatas(s.duplicatas)
      setShowTransport(s.showTransport)
      setTransport(s.transport)
      setSelectedCarrier(s.selectedCarrier)
      setSelectedVehicle(s.selectedVehicle)
    }
    draft.accept()
  }

  // ─── Totals ───────────────────────────────────────────────────────────────

  const totalProducts = products.reduce((s, p) => s + (parseFloat(p.qty) || 0) * (parseFloat(p.unitValue) || 0), 0)
  const totalDiscount = products.reduce((s, p) => s + (parseFloat(p.discount) || 0), 0)
  const totalNfe = Math.max(0, totalProducts - totalDiscount)
  const totalPaid = payments.some(it => it.payment_type === NO_PAYMENT_TYPE) ? totalNfe : payments.reduce((s, p) => s + (parseFloat(p.value) || 0), 0)
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


  // Grouped-CFOP products block emission until the destination UF is known
  // (sameUf === null) AND a same-scope variant is resolved (non-empty cfop).
  const cfopUnresolvedError = products.some(p => p.cfopSuffix && (!p.cfop || sameUf === null))

  // Dados por unidade (chassi, motor, arma) que a SEFAZ só cobra na emissão.
  const itemGaps = useMemo(
    () => products
      .map((item, index) => ({index, reason: itemUnitDataGap(item)}))
      .filter((g): g is {index: number; reason: string} => g.reason !== null),
    [products],
  )

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
      setNewPaymentType(NO_PAYMENT_TYPE)
      setPayments([{payment_type: NO_PAYMENT_TYPE, value: '0.00', ind_pag: '0', card: null, terminal_id: null}])
    }
  }

  // Derived — cobrança only shown when there's an "a prazo" payment
  const hasPrazoPayment = payments.some(p => p.ind_pag === '1')

  // A fatura e suas parcelas também têm que fechar: "somatório das duplicatas
  // difere do valor da fatura" é rejeição, não aviso.
  const faturaTotal = parseFloat(cobrFat.v_liq || cobrFat.v_orig || '') || totalNfe
  const duplicataGap: string | null = !hasPrazoPayment || duplicatas.length === 0
    ? null
    : duplicataSumGap(faturaTotal, duplicatas.reduce((sum, d) => sum + (parseFloat(d.v_dup) || 0), 0))

  // Saída anterior à emissão e entrega no passado são rejeições da SEFAZ; o
  // `min` do input cobre o caminho do calendário, esta regra cobre o resto.
  const dateGap: string | null = (() => {
    if (dhSaiEnt && dhSaiEnt < localIso().slice(0, 10)) {
      return 'A saída da mercadoria não pode ser anterior à emissão.'
    }
    if (dPrevEntrega && dPrevEntrega < todayIso()) {
      return 'A previsão de entrega não pode ser anterior à emissão.'
    }
    return null
  })()
  const isPix = isPixPaymentType(newPaymentType)
  const isCardPayment = CARD_PAYMENT_TYPES.has(newPaymentType)

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

  /**
   * Motivo pelo qual o passo ainda não pode avançar, ou null quando pode. É uma
   * frase, não um booleano, porque um botão desabilitado sem explicação faz o
   * operador procurar o problema no lugar errado.
   */
  function stepBlockReason(step: EmitStep): string | null {
    if (step === 'destinatario') {
      return selfIssuance || receiver !== null ? null : 'Selecione o destinatário da nota.'
    }
    if (step === 'produtos') {
      if (products.length === 0) return 'Adicione ao menos um produto.'
      if (cfopMixError) return 'A nota mistura CFOP de entrada e de saída.'
      if (cfopUnresolvedError) return 'Há item sem CFOP resolvido para a UF de destino.'
      const gap = itemGaps[0]
      if (gap) return `Item ${gap.index + 1}: ${gap.reason}`
      return null
    }
    if (step === 'pagamento') {
      // O prazo gera parcelas, fatura e duplicatas a partir do total na emissão:
      // por construção a soma fecha.
      if (paymentTermId) return null
      if (newPaymentType === NO_PAYMENT_TYPE) return null
      if (payments.some(p => p.payment_type === NO_PAYMENT_TYPE)) return null
      if (payments.length === 0 && !(parseFloat(newPaymentValue) > 0)) {
        return 'Informe o pagamento da nota.'
      }
      // O valor ainda não adicionado conta: handleNext o adiciona ao avançar.
      const balanceGap = paymentBalanceGap(remaining - (parseFloat(newPaymentValue) || 0), false)
      if (balanceGap !== null) return balanceGap
      if (duplicataGap !== null) return duplicataGap
      return null
    }
    return null
  }

  function canGoNext(step: EmitStep): boolean {
    return stepBlockReason(step) === null
  }

  /**
   * Joga a diferença na última parcela. O resíduo quase sempre é arredondamento
   * de rateio, e refazer a conta à mão é exatamente o trabalho que o produto
   * existe para tirar do operador.
   */
  function handleAbsorbRemainder() {
    setPayments(prev => {
      if (prev.length === 0) return prev
      const last = prev[prev.length - 1]
      const adjusted = (parseFloat(last.value) || 0) + remaining
      if (adjusted <= 0) return prev
      return [...prev.slice(0, -1), {...last, value: adjusted.toFixed(2)}]
    })
  }

  function handleNext() {
    const i = STEP_IDS.indexOf(currentStep)
    if (i < STEP_IDS.length - 1) {
      if (currentStep === 'pagamento' && !paymentTermId) {
        const isNoPay = newPaymentType === NO_PAYMENT_TYPE
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
    const isNoPay = newPaymentType === NO_PAYMENT_TYPE
    if (!isNoPay && (!newPaymentValue || parseFloat(newPaymentValue) <= 0)) return
    const value = isNoPay ? '0.00' : newPaymentValue
    setPayments(prev => [...prev, {
      payment_type: newPaymentType,
      value,
      ind_pag: newPaymentIndPag,
      card: showCardToggle ? newPaymentCard : null,
      terminal_id: showCardToggle ? (newPaymentTerminal || null) : null,
    }])
    paymentValueLockedRef.current = false
    setNewPaymentCard(null)
    setNewPaymentTerminal('')
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

  // ─── Submit ───────────────────────────────────────────────────────────────

  const handleSubmit = async () => {
    setSubmitError(null)
    if (cfopMixError) {
      setSubmitError({message: 'Não é possível misturar CFOPs de entrada e saída na mesma NF-e.'})
      return
    }
    if (cfopUnresolvedError) {
      setSubmitError({message: 'Há produtos sem CFOP válido para a UF do destinatário. Selecione um destinatário com UF e configure o CFOP de mesma natureza para a UF de destino.'})
      return
    }
    if (requiresNfRefs && nfRefs.length === 0) {
      setSubmitError({message: 'Esta finalidade exige ao menos um documento referenciado. Adicione a nota de origem em "Documentos referenciados".'})
      return
    }
    const vTroco = totalPaid > totalNfe + 0.005 ? (totalPaid - totalNfe).toFixed(2) : null
    const hasCobr = hasPrazoPayment && (cobrFat.v_liq || duplicatas.length > 0)

    const payload: NfeEmit = {
      ...(selfIssuance ? {self_issuance: true} : {receiver_id: receiver!.sk}),
      operation_id: effectiveOperationId || null,
      payment_term_id: paymentTermId || null,
      products: products.map(p => ({
        product_id: p.product.sk, cfop: p.cfop, quantity: p.qty,
        unit_value: p.unitValue || null, discount: p.discount || '0',
        veic_chassi: p.veic_chassi || null, veic_n_serie: p.veic_n_serie || null,
        veic_n_motor: p.veic_n_motor || null, veic_c_cor: p.veic_c_cor || null,
        veic_x_cor: p.veic_x_cor || null,
        armas: p.armas && p.armas.length > 0 ? p.armas : null,
        x_ped: p.x_ped || null,
        n_item_ped: p.x_ped ? (p.n_item_ped || null) : null,
        transf_cred: reformPair(p, 'transf_cred'),
        ajuste_compet: reformPair(p, 'ajuste_compet'),
        estorno_cred: estornoPair(p),
      })),
      payments: payments.map(p => ({
        payment_type: p.payment_type, value: p.value,
        ind_pag: p.ind_pag || undefined, card: p.card || undefined,
        terminal_id: p.terminal_id || undefined,
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
          vols: vols.length > 0 ? vols : null,
          reboques: reboques.length > 0 ? reboques : null,
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
      nf_refs: nfRefs.length > 0 ? nfRefs : null,
      proc_ref: procRef.length > 0 ? procRef : null,
      compra_x_ped: nicheGroups.compraXPed.trim() || null,
      compra_x_cont: nicheGroups.compraXCont.trim() || null,
      cana: nicheGroups.cana,
      agro: nicheGroups.agro,
      dh_sai_ent: datetimeLocalToOffset(dhSaiEnt) || null,
      d_prev_entrega: dPrevEntrega || null,
      compra_gov_refs: compraGovRefs.length > 0 ? compraGovRefs : null,
      pag_antecipado_refs: pagAntecipadoRefs.length > 0 ? pagAntecipadoRefs : null,
    }

    setIsSubmitting(true)
    try {
      const result = await apiClient.emitNfe(payload)
      draft.clear()
      toast.success('NF-e enviada para a SEFAZ', {
        description: result.sefaz_protocol
          ? `Protocolo ${result.sefaz_protocol} · chave ${result.sk}`
          : `Chave de acesso ${result.sk}`,
      })
      router.push(`/nfe/detail?key=${result.sk}`)
    } catch (err) {
      setSubmitError(emitFailure(err, 'Erro ao emitir NF-e.'))
      setIsSubmitting(false)
    }
  }

  if (!selectedOrg) {
    return <div className="text-center py-12 text-sm text-gray-500">Selecione uma organização para emitir NF-e.</div>
  }

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <div className="max-w-3xl space-y-0 pb-4">
      <HomologationBanner environment={nfeConfig?.environment}/>

      {draft.recovered && (
        <DraftRecoveryBanner savedAt={draft.recovered.savedAt} onRestore={restoreDraft} onDiscard={draft.discard}/>
      )}

      {/* Step progress */}
      <StepIndicator current={currentStep} steps={STEPS}/>

      {/* ──────────────── Step 1: Destinatário ──────────────────────────── */}
      {currentStep === 'destinatario' && (
        <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium text-gray-600">Para quem?</p>
            <button
              type="button"
              aria-pressed={selfIssuance}
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

          {operations.length > 0 && (
            <div className="space-y-1 border-b border-gray-100 pb-4">
              <label htmlFor="nfe-operation" className="text-sm font-medium text-gray-600">
                Natureza da operação
              </label>
              <OptionsSelect
                id="nfe-operation"
                value={effectiveOperationId}
                onValueChange={setOperationId}
                options={[
                  {value: '', label: 'Sem operação — preencher manualmente'},
                  ...operations.map((op) => ({
                    value: extractId(op.sk, SK_PREFIX.OPERATION),
                    label: op.is_default ? `${op.name} (padrão)` : op.name,
                  })),
                ]}
              />
              {selectedOperation && (
                <p className="text-xs text-gray-500">
                  Preenche natureza, finalidade e CFOP dos itens. Qualquer campo alterado aqui vence a operação.
                </p>
              )}
            </div>
          )}

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
            <p className="text-sm font-medium text-gray-600">
              Produtos ({products.length})
            </p>
            <span className="text-sm font-semibold text-gray-900">{fmt(totalNfe)}</span>
          </div>
          {products.length > 0 && (
            <div className="space-y-2">
              {products.map((item, i) => (
                <ProductRow key={`${item.product.sk}-${i}`} item={item} index={i} sameUf={sameUf}
                            operationCfopSuffix={operationCfopSuffix}
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
            <ProductSearch
              onSelect={handleSelectProduct}
              onClose={() => setShowProductPicker(false)}
              className="rounded-lg border border-brand-200 bg-brand-50/30 p-4"
            />
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
          {paymentTerms.length > 0 && (
            <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-2">
              <label htmlFor="nfe-payment-term" className="text-sm font-medium text-gray-600">
                Condição de pagamento
              </label>
              <OptionsSelect
                id="nfe-payment-term"
                value={paymentTermId}
                onValueChange={setPaymentTermId}
                options={[
                  {value: '', label: 'Informar pagamento manualmente'},
                  ...paymentTerms.map((t) => ({
                    value: extractId(t.sk, SK_PREFIX.PAYMENT_TERM),
                    label: `${t.name} — ${t.installments}×`,
                  })),
                ]}
              />
              {selectedPaymentTerm && (
                <>
                  <p className="text-xs text-gray-500">
                    Parcelas, fatura e duplicatas são geradas na emissão a partir do total da nota.
                  </p>
                  <div className="space-y-1 pt-1">
                    {previewInstallments(
                      {
                        installments: selectedPaymentTerm.installments,
                        interval_days: selectedPaymentTerm.interval_days ?? 0,
                        first_due_days: selectedPaymentTerm.first_due_days ?? 0,
                      },
                      totalNfe,
                      new Date(),
                    ).map((inst) => (
                      <div key={inst.number}
                           className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-sm">
                        <span className="font-mono text-xs text-gray-500">{inst.number}</span>
                        <span className="text-gray-700">{inst.dueDate}</span>
                        <span className="font-medium text-gray-900">{fmt(parseFloat(inst.value))}</span>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </div>
          )}

          {!paymentTermId && (<>
          {/* Payment list */}
          {payments.length > 0 && (
            <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-2">
              <p className="text-sm font-medium text-gray-600">Pagamentos</p>
              {payments.map((p, i) => (
                <div key={i} className="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2.5 text-sm">
                  <span className="text-gray-700">
                    {NF_PAYMENT_TYPES[p.payment_type] ?? p.payment_type}
                    {p.ind_pag === '1' && <span className="ml-1.5 text-xs text-warning">(prazo)</span>}
                    {p.card && <span className="ml-1.5 text-xs text-blue-700">· transação</span>}
                  </span>
                  <div className="flex items-center gap-3">
                    <span className="font-medium">{fmt(parseFloat(p.value) || 0)}</span>
                    <Button type="button" variant="ghost" size="xs" onClick={() => handleRemovePayment(i)}
                            className="text-danger hover:text-red-700">remover</Button>
                  </div>
                </div>
              ))}
              <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                <p className={`text-sm ${Math.abs(remaining) < SUM_TOLERANCE ? 'text-success' : 'text-warning'}`}>
                  {Math.abs(remaining) < SUM_TOLERANCE
                    ? '✓ Total confere.'
                    : remaining > 0
                      ? `⌛ Restam ${fmt(remaining)} para fechar o total.`
                      : `⚠ Pagamentos excedem o total em ${fmt(-remaining)}.`}
                </p>
                {Math.abs(remaining) >= SUM_TOLERANCE && (
                  <Button type="button" variant="outline" size="xs" onClick={handleAbsorbRemainder}>
                    Ajustar última parcela
                  </Button>
                )}
              </div>
            </div>
          )}

          {/* Add payment */}
          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <p className="text-sm font-medium text-gray-600">Adicionar pagamento</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-[1fr_auto_auto_auto] gap-2 items-end">
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-1"><Label htmlFor="nfe-payment-type" className="text-xs font-medium text-gray-600">Forma de pagamento</Label><GlossaryTerm term="ind_pag"/></div>
                <OptionsSelect id="nfe-payment-type" value={newPaymentType}
                               onValueChange={(v) => {
                                 setNewPaymentType(v)
                                 setShowCardToggle(false)
                                 setNewPaymentCard(null)
                               }}
                               options={PAYMENT_OPTIONS}/>
              </div>
              {newPaymentType !== NO_PAYMENT_TYPE && (
                <div className="flex flex-col gap-1">
                  <Label htmlFor="nfe-payment-ind-pag" className="text-xs font-medium text-gray-600 whitespace-nowrap">À vista / Parcelado</Label>
                  <OptionsSelect id="nfe-payment-ind-pag" value={newPaymentIndPag} onValueChange={v => setNewPaymentIndPag(v as '0' | '1')}
                                 options={[{value: '0', label: '0 – À vista'}, {value: '1', label: '1 – Parcelado'}]}/>
                </div>
              )}
              {newPaymentType !== NO_PAYMENT_TYPE && (
                <div className="flex flex-col gap-1">
                  <Label htmlFor="nfe-payment-value" className="text-xs font-medium text-gray-600">Valor</Label>
                  <CurrencyInput id="nfe-payment-value" decimalPlaces={2} value={newPaymentValue}
                                 onChange={(v) => {
                                   paymentValueLockedRef.current = true
                                   setNewPaymentValue(v)
                                 }}
                                 placeholder="0,00"/>
                </div>
              )}
              <Button type="button" variant="brand" onClick={handleAddPayment}
                      className="self-end"
                      disabled={newPaymentType !== NO_PAYMENT_TYPE && (!newPaymentValue || parseFloat(newPaymentValue) <= 0)}>
                Adicionar
              </Button>
            </div>
            {newPaymentType === NO_PAYMENT_TYPE && (
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
                  <PaymentCardFields card={newPaymentCard} onChange={setNewPaymentCard} isPix={isPix}
                                   terminalId={newPaymentTerminal} onTerminalChange={setNewPaymentTerminal}/>
                )}
              </div>
            )}
          </div>

          {/* Cobrança — only when ind_pag=1 payment exists */}
          {hasPrazoPayment && (
            <div className="rounded-xl border border-amber-100 bg-amber-50/20 p-4 space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium text-warning">Cobrança (duplicatas)</p>
                <Button type="button" variant="ghost" size="xs"
                        className="text-warning hover:text-amber-900 text-xs"
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
                <p className="text-sm font-medium text-gray-600 mb-2">Fatura</p>
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
                  <p className="text-sm font-medium text-gray-600">
                    Parcelas {duplicatas.length > 0 && `(${duplicatas.length})`}
                  </p>
                  {duplicatas.length > 0 && (() => {
                    const allocated = duplicatas.reduce((s, d) => s + (parseFloat(d.v_dup) || 0), 0)
                    const expected = parseFloat(cobrFat.v_liq || cobrFat.v_orig || totalNfe.toFixed(2)) || totalNfe
                    const diff = Math.abs(allocated - expected)
                    return (
                      <span className={`text-xs font-medium ${diff < 0.01 ? 'text-success' : 'text-warning'}`}>
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
                      <Input type="date" min={todayIso()} value={dupFirstDate}
                             onChange={e => setDupFirstDate(e.target.value)}/>
                    </div>
                    <Button type="button" variant="brand" size="sm" onClick={handleGenerateDuplicatas}>
                      Gerar parcelas
                    </Button>
                    {duplicatas.length > 0 && (
                      <Button type="button" variant="ghost" size="sm" onClick={() => setDuplicatas([])}
                              className="text-danger hover:text-red-700">Limpar</Button>
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
                        <Input type="date" min={todayIso()} value={d.d_venc}
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
                                className="text-danger hover:text-red-700 shrink-0 px-1">×</Button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
          </>)}
        </div>
      )}

      {/* ──────────────── Step 4: Revisão ───────────────────────────────── */}
      {currentStep === 'revisao' && (
        <div className="rounded-xl border border-gray-200 bg-white divide-y divide-gray-100">
          <ReviewRow label="Destinatário" onEdit={() => setCurrentStep('destinatario')}>
            {selfIssuance
              ? <span className="text-brand-700 font-medium">Emissão própria</span>
              : <span className="font-medium text-gray-900">{receiver?.name}</span>}
            <span className="block text-xs text-gray-500 mt-0.5">
              {natOp}{noteDirection && ` · ${noteDirection === 'in' ? 'Entrada' : 'Saída'}`}
            </span>
          </ReviewRow>

          <ReviewRow label={`Produtos (${products.length})`} onEdit={() => setCurrentStep('produtos')}>
            <ul className="space-y-1">
              {products.map((p, i) => (
                <li key={`${p.product.sk}-${i}`} className="flex items-baseline justify-between gap-3">
                  <span className="min-w-0 text-gray-900">
                    <span className="font-mono text-xs text-gray-500 mr-1.5">{p.qty}×</span>
                    {p.product.description}
                    <span className="ml-1.5 font-mono text-xs text-gray-500">CFOP {p.cfop || '—'}</span>
                  </span>
                  <span className="shrink-0 font-medium text-gray-900">{fmt(computeTotal(p))}</span>
                </li>
              ))}
            </ul>
            {totalDiscount > 0 && (
              <p className="mt-2 text-xs text-gray-500">Desconto total: −{fmt(totalDiscount)}</p>
            )}
          </ReviewRow>

          <ReviewRow label="Pagamento" onEdit={() => setCurrentStep('pagamento')}>
            {selectedPaymentTerm && (
              <p className="text-gray-900">
                {selectedPaymentTerm.name}
                <span className="ml-1.5 text-xs text-gray-500">
                  {selectedPaymentTerm.installments}× · parcelas geradas na emissão
                </span>
              </p>
            )}
            <ul className="space-y-1">
              {payments.map((p, i) => (
                <li key={i} className="flex items-baseline justify-between gap-3">
                  <span className="text-gray-900">
                    {NF_PAYMENT_TYPES[p.payment_type] ?? p.payment_type}
                    {p.ind_pag === '1' && <span className="ml-1.5 text-xs text-warning">a prazo</span>}
                  </span>
                  <span className="shrink-0 font-medium text-gray-900">{fmt(parseFloat(p.value) || 0)}</span>
                </li>
              ))}
            </ul>
            {duplicatas.length > 0 && (
              <ul className="mt-2 space-y-0.5 text-xs text-gray-500">
                {duplicatas.map((d, i) => (
                  <li key={i}>
                    Parcela {d.n_dup} · vence {d.d_venc || '—'} · {fmt(parseFloat(d.v_dup) || 0)}
                  </li>
                ))}
              </ul>
            )}
          </ReviewRow>

          {showTransport && (
            <ReviewRow label="Transporte" onEdit={() => setCurrentStep('pagamento')}>
              <span className="text-gray-900">
                {MOD_FRETE_OPTIONS.find(o => o.value === transport.mod_frete)?.label ?? transport.mod_frete}
              </span>
              {(selectedCarrier || selectedVehicle || transport.veiculo_placa) && (
                <span className="block text-xs text-gray-500 mt-0.5">
                  {selectedCarrier?.name}
                  {selectedCarrier && (selectedVehicle || transport.veiculo_placa) && ' · '}
                  {selectedVehicle?.plate ?? transport.veiculo_placa}
                </span>
              )}
            </ReviewRow>
          )}

          {additionalInfo.trim() && (
            <ReviewRow label="Informações adicionais" onEdit={() => setCurrentStep('pagamento')}>
              <span className="text-gray-700 whitespace-pre-wrap">{additionalInfo}</span>
            </ReviewRow>
          )}

          <div className="flex items-baseline justify-between gap-3 px-5 py-4">
            <span className="text-sm font-medium text-gray-700">Total da NF-e</span>
            <span className="text-lg font-semibold text-gray-900">{fmt(totalNfe)}</span>
          </div>
        </div>
      )}

      {/* Documentos referenciados (ide/NFref) — obrigatório na finalidade
          escolhida, então fica à vista e não dentro do grupo opcional. */}
      {currentStep === 'pagamento' && requiresNfRefs && (
        <div className="mt-4 space-y-2 rounded-lg border border-gray-200 p-3">
          <Label className="text-sm font-medium text-gray-700">Documentos referenciados</Label>
          <p className="text-xs text-gray-500">Esta finalidade de emissão exige a nota de origem.</p>
          <NfeRefsPicker value={nfRefs} onChange={setNfRefs}/>
        </div>
      )}

      {/* Optional groups — collected before the review that shows them */}
      {currentStep === 'pagamento' && (
        <CollapsibleSection title="Configurações avançadas" description="Transporte e informações adicionais (opcional)"
                            className="mt-4">
          <div className="space-y-3">
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
              <label htmlFor="toggle-transport" className="text-sm font-medium text-gray-700 cursor-pointer">
                Transporte
              </label>
            </div>
            {showTransport && (
              <div className="space-y-4">
                <div className="flex flex-col gap-1 max-w-xs">
                  <div className="flex items-center gap-1"><Label className="text-xs font-medium text-gray-600">Modalidade</Label><GlossaryTerm term="mod_frete"/></div>
                  <OptionsSelect value={transport.mod_frete}
                                 onValueChange={v => setTransport(t => ({...t, mod_frete: v}))}
                                 options={MOD_FRETE_OPTIONS}/>
                </div>
                {transport.mod_frete !== '9' && (
                  <>
                    <div className="space-y-2">
                      <p className="text-sm font-medium text-gray-600">Transportadora</p>
                      {(transport.mod_frete === '3' || transport.mod_frete === '4') && (
                        <p className="text-sm text-gray-500 rounded-lg bg-gray-50 border border-gray-100 px-4 py-2.5">
                          Transporte próprio — sem transportadora externa.
                        </p>
                      )}
                      {(transport.mod_frete === '0' || transport.mod_frete === '1' || transport.mod_frete === '2') && (
                        <PersonPicker value={selectedCarrier} onChange={setSelectedCarrier} role="carrier"
                                      placeholder="Buscar transportadora por nome ou CNPJ/CPF"/>
                      )}
                    </div>
                    <div className="space-y-2">
                      <p className="text-sm font-medium text-gray-600">Veículo</p>
                      {selectedVehicle ? (
                        <div
                          className="flex items-center gap-3 rounded-lg border border-green-200 bg-green-50 px-4 py-2.5">
                          <div className="flex-1"><p
                            className="font-mono text-sm font-medium">{selectedVehicle.plate}</p><p
                            className="text-xs text-gray-500">{selectedVehicle.plate_uf}</p></div>
                          <Button type="button" variant="ghost" size="xs" onClick={() => setSelectedVehicle(null)}
                                  className="text-red-600">Trocar</Button>
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
                              className="text-xs font-medium text-gray-600">RNTC</Label><Input
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
            {showTransport && (
              <div className="pt-3 border-t border-gray-100">
                <VolumesFields vols={vols} onVolsChange={setVols}
                               reboques={reboques} onReboquesChange={setReboques}/>
              </div>
            )}
          </div>

          {/* Processos referenciados (infAdic/procRef) */}
          <div className="space-y-2 pt-3 border-t border-gray-100">
            <div className="flex items-center justify-between">
              <Label className="text-sm font-medium text-gray-700">Processos referenciados</Label>
              <Button type="button" variant="ghost" size="xs"
                      onClick={() => setProcRef(p => [...p, {n_proc: '', ind_proc: '0'}])}>
                + Processo
              </Button>
            </div>
            {procRef.map((proc, i) => (
              <div key={i} className="grid grid-cols-1 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_auto] gap-2 items-end">
                <div className="flex flex-col gap-1">
                  <Label htmlFor={`proc-nproc-${i}`} className="text-xs font-medium text-gray-600">Número</Label>
                  <Input id={`proc-nproc-${i}`} maxLength={60} value={proc.n_proc} placeholder="0001/2026"
                         onChange={e => setProcRef(p => p.map((v, k) => k === i ? {...v, n_proc: e.target.value} : v))}/>
                </div>
                <div className="flex flex-col gap-1">
                  <Label htmlFor={`proc-indproc-${i}`} className="text-xs font-medium text-gray-600">Origem</Label>
                  <OptionsSelect id={`proc-indproc-${i}`} value={proc.ind_proc}
                                 options={[...IND_PROC_OPTIONS]}
                                 onValueChange={(v: string) => setProcRef(p => p.map((x, k) => k === i ? {...x, ind_proc: v as NfeProcRefIn['ind_proc']} : x))}/>
                </div>
                <Button type="button" variant="ghost" size="xs"
                        onClick={() => setProcRef(p => p.filter((_, k) => k !== i))}>
                  Remover
                </Button>
              </div>
            ))}
          </div>

          {/* Compra governamental e antecipação de pagamento (ide da reforma) */}
          {(compraGovNeedsRef || pagAntecipadoRefs.length > 0 || compraGovRefs.length > 0) && (
            <div className="space-y-3 pt-3 border-t border-gray-100">
              {compraGovNeedsRef && (
                <AccessKeyPicker
                  id="nfe-compra-gov-refs"
                  label="Documentos anteriores da compra governamental"
                  value={compraGovRefs}
                  onChange={setCompraGovRefs}
                  max={compraGovRefMax}
                  hint={compraGovRefMax === 1
                    ? 'A operação escolhida na natureza aceita uma chave só.'
                    : 'Obrigatório para o tipo de operação governamental cadastrado.'}/>
              )}
              <AccessKeyPicker
                id="nfe-pag-antecipado-refs"
                label="NF-e de antecipação de pagamento a abater"
                value={pagAntecipadoRefs}
                onChange={setPagAntecipadoRefs}
                hint="Opcional. Abate as parcelas já recebidas por antecipação."/>
            </div>
          )}

          {/* Saída da mercadoria e previsão de entrega (ide) */}
          <div className="space-y-2 pt-3 border-t border-gray-100">
            <Label className="text-sm font-medium text-gray-700">Saída e entrega</Label>
            <p className="text-xs text-gray-500">
              {operationDhSaiEntOffsetDays === null
                ? 'Em branco, a nota não declara data de saída.'
                : `Em branco, a saída é ${operationDhSaiEntOffsetDays} dia(s) após a emissão (prazo da operação).`}
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <div className="flex flex-col gap-1">
                <Label htmlFor="nfe-dh-sai-ent" className="text-xs font-medium text-gray-600">
                  Saída da mercadoria
                </Label>
                <Input id="nfe-dh-sai-ent" type="datetime-local" min={localIso()} value={dhSaiEnt}
                       onChange={(e) => setDhSaiEnt(e.target.value)}/>
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor="nfe-d-prev-entrega" className="text-xs font-medium text-gray-600">
                  Previsão de entrega
                </Label>
                <Input id="nfe-d-prev-entrega" type="date" min={todayIso()} value={dPrevEntrega}
                       onChange={(e) => setDPrevEntrega(e.target.value)}/>
              </div>
            </div>
          </div>

          {/* Grupos de nicho: compra pública, cana e agropecuário */}
          <div className="pt-3 border-t border-gray-100">
            <NicheGroupsFields value={nicheGroups} onChange={setNicheGroups}
                               canaSafra={operationCanaSafra}
                               technicalManagerCpf={orgTechnicalManagerCpf}/>
          </div>

          {/* Additional info */}
          <div className="space-y-2 pt-3 border-t border-gray-100">
            <Label htmlFor="nfe-additional-info" className="text-sm font-medium text-gray-700">
              Informações adicionais
            </Label>
            <Textarea id="nfe-additional-info" value={additionalInfo}
                      onChange={e => setAdditionalInfo(e.target.value)}
                      placeholder="Observações, dados ao fisco, pedido, etc. (opcional)" rows={3}/>
          </div>
        </CollapsibleSection>
      )}

      {/* Emission failure — rendered next to the action bar that triggers it */}
      <div className="mt-4 empty:mt-0">
        <EmitError failure={submitError}/>
      </div>

      {/* ── Navigation bar ────────────────────────────────────────────────── */}
      <div
        className="sticky bottom-0 bg-gray-50 border-t border-gray-200 -mx-4 px-4 md:mx-0 md:px-0 py-3 md:py-4 flex items-center justify-between gap-2">
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
            <Button type="button" variant="outline" onClick={handleBack}>Voltar</Button>
          )}

          {currentStep !== 'revisao' ? (
            <div className="flex items-center gap-3">
              {stepBlockReason(currentStep) && (
                <span className="text-right text-[0.8rem] text-warning">{stepBlockReason(currentStep)}</span>
              )}
              <Button type="button" variant="brand" disabled={!canGoNext(currentStep)} onClick={handleNext}>
                Próximo
              </Button>
            </div>
          ) : (
            <div className="flex items-center gap-3">
              {dateGap && <span className="text-right text-[0.8rem] text-warning">{dateGap}</span>}
              <Button type="button" variant="brand" disabled={isSubmitting || dateGap !== null}
                      onClick={() => setShowEmitConfirm(true)}>
                {isSubmitting ? 'Emitindo…' : 'Emitir NF-e'}
              </Button>
            </div>
          )}
        </div>
      </div>

      {/* Modals */}
      <EmitConfirmModal
        open={showEmitConfirm}
        onClose={() => setShowEmitConfirm(false)}
        onConfirm={() => {
          setShowEmitConfirm(false)
          void handleSubmit()
        }}
        docLabel="NF-e"
        summary={[
          {label: 'Destinatário', value: selfIssuance ? 'Emissão própria' : (receiver?.name ?? '—')},
          {label: 'Total', value: fmt(totalNfe)},
          {label: 'Produtos', value: `${products.length} item(s)`},
        ]}
      />
    </div>
  )
}
