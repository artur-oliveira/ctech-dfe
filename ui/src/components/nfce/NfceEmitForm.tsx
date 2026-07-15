'use client'

import {useCallback, useEffect, useMemo, useRef, useState} from 'react'
import {useRouter} from 'next/navigation'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {queryKeys} from '@/lib/api/query-keys'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Label} from '@/components/ui/label'
import {GlossaryTerm} from '@/components/ui/glossary-term'
import {CollapsibleSection} from '@/components/ui/collapsible-section'
import {Textarea} from '@/components/ui/textarea'
import {Modal} from '@/components/ui/modal'
import {CurrencyInput} from '@/components/ui/currency-input'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {PersonForm} from '@/components/persons/PersonForm'
import {CARD_PAYMENT_TYPES, isPixPaymentType, PaymentCardFields} from '@/components/nfe/PaymentCardFields'
import {NatOpInlineEdit} from '@/components/nfe/NatOpInlineEdit'
import type {CfopConfigItem, NfceEmit, NfeCardIn, PersonCreate, ProductOut} from '@/lib/types/api'
import {NF_PAYMENT_TYPES} from '@/lib/types/api'
import {buildNatOpFromCfops, getCfopDescription} from '@/lib/data/cfop'
import {formatCpfCnpj} from '@/lib/utils/document'
import {maskCpf} from '@/lib/utils/masks'
import {validateCPF} from '@/lib/utils/validators'

// ─── local types ──────────────────────────────────────────────────────────────

interface Consumer {
  cpf: string
  name: string | null
}

interface EmitProduct {
  product: ProductOut
  cfop: string
  qty: string
  unitValue: string
  discount: string
}

interface EmitPayment {
  payment_type: string
  value: string
  card: NfeCardIn | null
}

type Step = 'consumidor' | 'produtos' | 'pagamento'

const STEPS: { id: Step; label: string }[] = [
  {id: 'consumidor', label: 'Consumidor'},
  {id: 'produtos', label: 'Produtos'},
  {id: 'pagamento', label: 'Pagamento'},
]

const PAYMENT_OPTIONS = Object.entries(NF_PAYMENT_TYPES)
  .map(([value, label]) => ({value, label: `${value} – ${label}`, display: label}))
  .sort((a, b) => parseInt(a.value) - parseInt(b.value))

function fmt(n: number): string {
  return n.toLocaleString('pt-BR', {style: 'currency', currency: 'BRL'})
}

function computeTotal(p: EmitProduct): number {
  const qty = parseFloat(p.qty) || 0
  const unit = parseFloat(p.unitValue) || 0
  const disc = parseFloat(p.discount) || 0
  return Math.max(0, qty * unit - disc)
}

/** CFOPs configured on a product that are valid for NFC-e (internal saída — 5xxx). */
function nfceCfopsForProduct(product: ProductOut): string[] {
  const configured = (product.cfop_config as CfopConfigItem[] | undefined ?? [])
    .map((c) => c.cfop)
    .filter((c) => c.startsWith('5'))
  if (configured.length > 0) return configured
  if (product.cfop_nfce && product.cfop_nfce.startsWith('5')) return [product.cfop_nfce]
  return []
}

// ─── consumer search (CPF) ──────────────────────────────────────────────────────

function ConsumerSearch({value, onChange}: { value: Consumer | null; onChange: (c: Consumer | null) => void }) {
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)
  const [open, setOpen] = useState(false)
  const [docLoading, setDocLoading] = useState(false)
  const [notFound, setNotFound] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [createLoading, setCreateLoading] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const digits = query.replace(/\D/g, '')
  const isCpf = validateCPF(digits)

  const nameQuery = useQuery({
    queryKey: queryKeys.persons.search(debouncedQuery),
    queryFn: () => apiClient.searchPersonsByName(debouncedQuery),
    enabled: open && !!debouncedQuery && !isCpf && debouncedQuery.length >= 2,
  })

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const handleSearchByDoc = useCallback(async (docDigits: string) => {
    setNotFound(false)
    setDocLoading(true)
    try {
      const person = await apiClient.getPersonByCpfCnpj(docDigits)
      onChange({cpf: docDigits, name: person.name})
      setQuery('')
      setOpen(false)
    } catch {
      // Not registered — allow emitting with the raw CPF, and offer registration.
      onChange({cpf: docDigits, name: null})
      setNotFound(true)
    } finally {
      setDocLoading(false)
    }
  }, [onChange])

  const handleCreatePerson = async (data: PersonCreate) => {
    setCreateLoading(true)
    try {
      const created = await apiClient.createPerson(data)
      onChange({cpf: created.sk.replace(/\D/g, ''), name: created.name})
      setShowCreate(false)
      setNotFound(false)
      setQuery('')
    } finally {
      setCreateLoading(false)
    }
  }

  // Only CPF persons can be NFC-e consumers.
  const suggestions = (nameQuery.data?.items ?? []).filter((p) => p.sk.startsWith('CPF_'))

  if (value) {
    return (
      <div className="space-y-2">
        <div className="flex items-center gap-3 rounded-lg border border-green-200 bg-green-50 px-4 py-3">
          <div className="flex-1 min-w-0">
            <p className="font-medium text-gray-900 text-sm">{value.name ?? 'Consumidor não identificado'}</p>
            <p className="text-xs text-gray-500 font-mono mt-0.5">{formatCpfCnpj(value.cpf)}</p>
          </div>
          <Button type="button" variant="ghost" size="xs" onClick={() => {
            onChange(null)
            setNotFound(false)
          }} className="text-danger hover:text-red-700 shrink-0">Trocar</Button>
        </div>
        {!value.name && (
          <p className="text-xs text-amber-600">
            CPF não cadastrado — a NFC-e pode ser emitida assim mesmo.{' '}
            <button type="button" className="text-brand-600 hover:text-brand-700 underline"
                    onClick={() => setShowCreate(true)}>Deseja cadastrar? (opcional)
            </button>
          </p>
        )}
        <Modal isOpen={showCreate} title="Cadastrar consumidor"
               onClose={() => setShowCreate(false)} size="xl">
          <PersonForm lockTipo="pf" initialCpfCnpj={value.cpf} onSubmit={handleCreatePerson} loading={createLoading}/>
        </Modal>
      </div>
    )
  }

  return (
    <div ref={containerRef} className="space-y-3">
      <div className="relative">
        <div className="flex gap-2">
          <Input
            type="text"
            value={isCpf ? maskCpf(digits) : query}
            onChange={(e) => {
              const newQuery = e.target.value.toUpperCase()
              setQuery(newQuery)
              setNotFound(false)
              setOpen(true)
              const newDigits = newQuery.replace(/\D/g, '')
              if (validateCPF(newDigits)) handleSearchByDoc(newDigits)
            }}
            onFocus={() => setOpen(true)}
            placeholder="Nome ou CPF do consumidor (opcional)"
            className="flex-1"
          />
          {docLoading && (
            <span className="flex items-center px-3 text-xs text-gray-400 shrink-0">Buscando…</span>
          )}
        </div>

        {open && !isCpf && suggestions.length > 0 && (
          <div
            className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-popover overflow-hidden">
            {suggestions.map((p) => (
              <button key={p.sk} type="button" onMouseDown={(e) => e.preventDefault()}
                      onClick={() => {
                        onChange({cpf: p.sk.replace(/\D/g, ''), name: p.name})
                        setQuery('')
                        setOpen(false)
                      }}
                      className="w-full text-left px-4 py-2.5 hover:bg-gray-50 transition-colors">
                <p className="text-sm font-medium text-gray-900">{p.name}</p>
                <p className="text-xs text-gray-400 font-mono">{formatCpfCnpj(p.sk.replace(/\D/g, ''))}</p>
              </button>
            ))}
          </div>
        )}
      </div>

      {notFound && (
        <p className="text-xs text-amber-600">
          CPF não encontrado.{' '}
          <button type="button" className="text-brand-600 hover:text-brand-700 underline"
                  onClick={() => setShowCreate(true)}>Cadastrar pessoa (opcional)
          </button>
        </p>
      )}

      <p className="text-xs text-gray-400">
        A NFC-e pode ser emitida sem identificar o consumidor. Somente pessoa física.
      </p>

      <Modal isOpen={showCreate} title="Cadastrar consumidor (pessoa física)"
             onClose={() => setShowCreate(false)} size="xl">
        <PersonForm lockTipo="pf" initialCpfCnpj={digits} onSubmit={handleCreatePerson} loading={createLoading}/>
      </Modal>
    </div>
  )
}

// ─── product picker ─────────────────────────────────────────────────────────────

function ProductPicker({onSelect, onClose}: { onSelect: (p: ProductOut) => void; onClose: () => void }) {
  const {selectedOrg} = useAuth()
  const [query, setQuery] = useState('')
  const debounced = useDebounce(query, 300)

  const {data, isLoading} = useQuery({
    queryKey: queryKeys.products.list(selectedOrg?.pk),
    queryFn: () => apiClient.getProducts({limit: 50}),
    enabled: !!selectedOrg,
  })

  const all = data?.items ?? []
  const filtered = debounced
    ? all.filter((p) =>
      p.description.toLowerCase().includes(debounced.toLowerCase()) ||
      p.code.toLowerCase().includes(debounced.toLowerCase()))
    : all

  return (
    <div className="rounded-lg border border-brand-200 bg-brand-50/30 p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Buscar produto</p>
        <Button type="button" variant="ghost" size="xs" onClick={onClose} className="text-gray-400 hover:text-gray-600">
          Fechar
        </Button>
      </div>
      <Input type="text" autoFocus value={query} onChange={(e) => setQuery(e.target.value)}
             placeholder="Código ou descrição..." className="w-full"/>
      <div className="max-h-48 overflow-y-auto space-y-0.5">
        {isLoading ? (
          <div className="py-1">
            <LoadingSkeleton count={3} height="h-8" rounded="rounded-md"/>
          </div>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-gray-500 py-2">Nenhum produto encontrado.</p>
        ) : (
          filtered.map((p) => {
            const valid = nfceCfopsForProduct(p).length > 0
            return (
              <button key={p.sk} type="button" disabled={!valid} onClick={() => onSelect(p)}
                      className={`w-full text-left px-3 py-2 rounded-md transition-colors flex items-center justify-between gap-2 ${valid ? 'hover:bg-white' : 'opacity-40 cursor-not-allowed'}`}>
              <span className="text-sm text-gray-900 min-w-0 truncate">
                {p.description}
                {p.brand && <span className="ml-1.5 text-xs text-gray-400">{p.brand}</span>}
                {!valid && <span className="ml-1.5 text-xs text-red-600">sem CFOP de NFC-e</span>}
              </span>
                <span className="text-xs text-gray-400 shrink-0">
                  {parseFloat(p.value).toLocaleString('pt-BR', {minimumFractionDigits: 2, maximumFractionDigits: 2})}
                </span>
              </button>
            )
          })
        )}
      </div>
    </div>
  )
}

// ─── product row ────────────────────────────────────────────────────────────────

function ProductRow({item, index, onChange, onRemove}: {
  item: EmitProduct
  index: number
  onChange: (i: number, u: Partial<EmitProduct>) => void
  onRemove: (i: number) => void
}) {
  const cfopOptions = nfceCfopsForProduct(item.product).map((cfop) => {
    const desc = getCfopDescription(cfop)
    const label = desc ? `${cfop} – ${desc}` : cfop
    return {value: cfop, label, display: label}
  })
  const total = computeTotal(item)

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-3 md:p-4 space-y-3">
      <div className="flex items-start justify-between gap-2">
        <p className="font-medium text-gray-900 text-sm min-w-0 truncate">
          {item.product.description}
          {item.product.brand && (
            <span className="ml-1.5 text-xs text-gray-400 font-normal">{item.product.brand}</span>
          )}
        </p>
        <Button type="button" variant="ghost" size="xs" onClick={() => onRemove(index)}
                className="shrink-0 text-danger hover:text-red-700">Remover</Button>
      </div>
      <div className="grid grid-cols-3 md:grid-cols-12 gap-2 items-end">
        <div className="col-span-3 md:col-span-6 flex flex-col gap-1">
          <div className="flex items-center gap-1"><Label className="text-xs font-medium text-gray-600">CFOP</Label><GlossaryTerm term="cfop"/></div>
          {cfopOptions.length > 0 ? (
            <OptionsSelect value={item.cfop} onValueChange={(v) => onChange(index, {cfop: v})}
                           options={cfopOptions} placeholder="CFOP"/>
          ) : (
            <Input type="text" value={item.cfop} maxLength={4} placeholder="5102"
                   onChange={(e) => onChange(index, {cfop: e.target.value})}/>
          )}
        </div>
        <div className="col-span-1 md:col-span-2 flex flex-col gap-1">
          <Label className="text-xs font-medium text-gray-600">Qtd ({item.product.unit ?? 'UN'})</Label>
          <NumericInput decimal integerPlaces={7} decimalPlaces={4} value={item.qty}
                        onChange={(v) => onChange(index, {qty: v})} placeholder="1" className="w-full"/>
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
      <div className="text-right text-sm font-medium text-gray-700">
        Total: <span className="font-semibold">{fmt(total)}</span>
      </div>
    </div>
  )
}

// ─── step indicator ─────────────────────────────────────────────────────────────

function StepIndicator({current}: { current: Step }) {
  const idx = STEPS.findIndex((s) => s.id === current)
  return (
    <div className="flex items-center gap-0 mb-6">
      {STEPS.map((step, i) => {
        const done = i < idx
        const active = i === idx
        return (
          <div key={step.id} className="flex items-center flex-1 last:flex-none">
            <div className="flex flex-col items-center gap-1 shrink-0">
              <div className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-semibold ${
                done || active ? 'bg-brand-600 text-white' : 'bg-gray-100 text-gray-400'}`}>
                {done ? '✓' : i + 1}
              </div>
              <span className={`text-xs hidden sm:block ${active ? 'text-brand-600 font-medium' : 'text-gray-400'}`}>
                {step.label}
              </span>
            </div>
            {i < STEPS.length - 1 && <div className={`flex-1 h-0.5 mx-2 ${i < idx ? 'bg-brand-500' : 'bg-gray-200'}`}/>}
          </div>
        )
      })}
    </div>
  )
}

// ─── main form ──────────────────────────────────────────────────────────────────

export function NfceEmitForm() {
  const {selectedOrg} = useAuth()
  const router = useRouter()

  const [step, setStep] = useState<Step>('consumidor')
  const [consumer, setConsumer] = useState<Consumer | null>(null)
  const [products, setProducts] = useState<EmitProduct[]>([])
  const [showPicker, setShowPicker] = useState(false)
  const [payments, setPayments] = useState<EmitPayment[]>([])
  const [newPaymentType, setNewPaymentType] = useState('01')
  const [newPaymentValue, setNewPaymentValue] = useState('')
  const [newPaymentCard, setNewPaymentCard] = useState<NfeCardIn | null>(null)
  const [showCardToggle, setShowCardToggle] = useState(false)
  const paymentLocked = useRef(false)
  const [natOpManual, setNatOpManual] = useState<string | null>(null)
  const [additionalInfo, setAdditionalInfo] = useState('')
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const {data: nfceConfig} = useQuery({
    queryKey: queryKeys.nfceConfig(selectedOrg!.pk),
    queryFn: () => apiClient.getNFCeConfig(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })

  // Products fetched here too (besides the picker) so we can auto-add the first one.
  const {data: productsData} = useQuery({
    queryKey: queryKeys.products.list(selectedOrg?.pk),
    queryFn: () => apiClient.getProducts({limit: 50}),
    enabled: !!selectedOrg,
  })

  const totalProducts = products.reduce((s, p) => s + (parseFloat(p.qty) || 0) * (parseFloat(p.unitValue) || 0), 0)
  const totalDiscount = products.reduce((s, p) => s + (parseFloat(p.discount) || 0), 0)
  const totalNfce = Math.max(0, totalProducts - totalDiscount)
  const totalPaid = payments.reduce((s, p) => s + (parseFloat(p.value) || 0), 0)
  const remaining = totalNfce - totalPaid
  const computedNatOp = useMemo(() => buildNatOpFromCfops(products.map((p) => p.cfop)), [products])
  const natOp = natOpManual ?? computedNatOp

  const isPix = isPixPaymentType(newPaymentType)
  const isCardPayment = CARD_PAYMENT_TYPES.has(newPaymentType)

  useEffect(() => {
    if (!paymentLocked.current) setNewPaymentValue(remaining > 0.005 ? remaining.toFixed(2) : '')
  }, [remaining])

  const addProduct = (product: ProductOut) => {
    const cfop = nfceCfopsForProduct(product)[0] ?? ''
    setProducts((prev) => [...prev, {product, cfop, qty: '1', unitValue: product.value, discount: '0'}])
    setShowPicker(false)
  }
  const changeProduct = (i: number, u: Partial<EmitProduct>) =>
    setProducts((prev) => prev.map((it, idx) => (idx === i ? {...it, ...u} : it)))
  const removeProduct = (i: number) => setProducts((prev) => prev.filter((_, idx) => idx !== i))

  // A pending (not yet "added") payment input that is still valid for emission.
  const pendingPayment = (): EmitPayment | null => {
    if (!newPaymentValue || parseFloat(newPaymentValue) <= 0) return null
    return {payment_type: newPaymentType, value: newPaymentValue, card: showCardToggle ? newPaymentCard : null}
  }

  const addPayment = () => {
    const p = pendingPayment()
    if (!p) return
    setPayments((prev) => [...prev, p])
    paymentLocked.current = false
    setNewPaymentCard(null)
    setShowCardToggle(false)
  }
  const removePayment = (i: number) => {
    paymentLocked.current = false
    setPayments((prev) => prev.filter((_, idx) => idx !== i))
  }

  const effectivePayments = (): EmitPayment[] => {
    const pending = pendingPayment()
    return pending ? [...payments, pending] : payments
  }

  const canNext = (s: Step): boolean => {
    if (s === 'consumidor') return true
    if (s === 'produtos') return products.length > 0 && products.every((p) => p.cfop.startsWith('5'))
    return true
  }

  const stepIdx = STEPS.findIndex((s) => s.id === step)

  const goNext = () => {
    if (stepIdx >= STEPS.length - 1 || !canNext(step)) return
    // Auto-add the first product when leaving the consumer step with none selected.
    if (step === 'consumidor' && products.length === 0 && productsData?.items?.length) {
      const first = productsData.items.find((p) => nfceCfopsForProduct(p).length > 0)
      if (first) addProduct(first)
    }
    setStep(STEPS[stepIdx + 1].id)
  }
  const goBack = () => stepIdx > 0 && setStep(STEPS[stepIdx - 1].id)

  const handleSubmit = async () => {
    setSubmitError(null)
    if (products.length === 0) {
      setSubmitError('Adicione pelo menos um produto.')
      return
    }
    const allPayments = effectivePayments()
    if (allPayments.length === 0) {
      setSubmitError('Informe pelo menos uma forma de pagamento.')
      return
    }
    const paid = allPayments.reduce((s, p) => s + (parseFloat(p.value) || 0), 0)
    if (paid + 0.005 < totalNfce) {
      setSubmitError('O total dos pagamentos é menor que o total da NFC-e.')
      return
    }
    const payload: NfceEmit = {
      consumer_cpf: consumer?.cpf || null,
      products: products.map((p) => ({
        product_id: p.product.sk, cfop: p.cfop, quantity: p.qty,
        unit_value: p.unitValue || null, discount: p.discount || '0',
      })),
      payments: allPayments.map((p) => ({
        payment_type: p.payment_type, value: p.value, card: p.card ?? undefined,
      })),
      additional_info: additionalInfo.trim() || null,
      nat_op: natOp || null,
    }
    setIsSubmitting(true)
    try {
      await apiClient.emitNfce(payload)
      router.push('/nfce')
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao emitir NFC-e.')
    } finally {
      setIsSubmitting(false)
    }
  }

  const canEmit = products.length > 0 && effectivePayments().length > 0

  return (
    <div className="max-w-3xl">
      <HomologationBanner environment={nfceConfig?.environment}/>
      <StepIndicator current={step}/>

      {step === 'consumidor' && (
        <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-5">
          <ConsumerSearch value={consumer} onChange={setConsumer}/>
        </div>
      )}

      {step === 'produtos' && (
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-xl border border-gray-200 bg-white px-4 py-3">
            <span className="text-sm text-gray-500">Total da NFC-e</span>
            <span className="text-sm font-semibold text-gray-900">{fmt(totalNfce)}</span>
          </div>
          {products.map((item, i) => (
            <ProductRow key={`${item.product.sk}-${i}`} item={item} index={i}
                        onChange={changeProduct} onRemove={removeProduct}/>
          ))}
          {products.length > 0 && natOp && (
            <NatOpInlineEdit value={natOp} onChange={setNatOpManual}
                             onReset={() => setNatOpManual(null)} canReset={natOpManual !== null}/>
          )}
          {showPicker ? (
            <ProductPicker onSelect={addProduct} onClose={() => setShowPicker(false)}/>
          ) : (
            <Button type="button" variant="ghost" size="sm" onClick={() => setShowPicker(true)}
                    className="text-brand-600 hover:text-brand-700 px-0">+ Adicionar produto</Button>
          )}
        </div>
      )}

      {step === 'pagamento' && (
        <div className="space-y-4">
          {payments.length > 0 && (
            <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Pagamentos</p>
              {payments.map((p, i) => (
                <div key={i} className="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2.5 text-sm">
                  <span className="text-gray-700">
                    {NF_PAYMENT_TYPES[p.payment_type] ?? p.payment_type}
                    {p.card && <span className="ml-1.5 text-xs text-blue-600">· transação</span>}
                  </span>
                  <div className="flex items-center gap-3">
                    <span className="font-medium">{fmt(parseFloat(p.value) || 0)}</span>
                    <Button type="button" variant="ghost" size="xs" onClick={() => removePayment(i)}
                            className="text-danger hover:text-red-700">remover</Button>
                  </div>
                </div>
              ))}
              <p
                className={`text-sm pt-1 ${Math.abs(remaining) < 0.01 ? 'text-green-600' : remaining < 0 ? 'text-blue-600' : 'text-amber-600'}`}>
                {Math.abs(remaining) < 0.01 ? '✓ Total confere.' : remaining > 0 ? `Restam ${fmt(remaining)}.` : `Troco: ${fmt(-remaining)}`}
              </p>
            </div>
          )}

          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Pagamento</p>
            <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto_auto] gap-2 items-end">
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-1"><Label className="text-xs font-medium text-gray-600">Forma de pagamento</Label><GlossaryTerm term="ind_pag"/></div>
                <OptionsSelect value={newPaymentType} onValueChange={(v) => {
                  setNewPaymentType(v)
                  setShowCardToggle(false)
                  setNewPaymentCard(null)
                }} options={PAYMENT_OPTIONS}/>
              </div>
              <div className="flex flex-col gap-1">
                <Label className="text-xs font-medium text-gray-600">Valor</Label>
                <CurrencyInput decimalPlaces={2} value={newPaymentValue}
                               onChange={(v) => {
                                 paymentLocked.current = true
                                 setNewPaymentValue(v)
                               }} placeholder="0,00"/>
              </div>
              <Button type="button" variant="brand" onClick={addPayment} className="self-end"
                      disabled={!newPaymentValue || parseFloat(newPaymentValue) <= 0}>Adicionar</Button>
            </div>

            {(isCardPayment || isPix) && (
              <div className="pt-1 border-t border-gray-100 space-y-2">
                <div className="flex items-center gap-2">
                  <input type="checkbox" id="nfce-toggle-card" checked={showCardToggle}
                         onChange={(e) => {
                           setShowCardToggle(e.target.checked)
                           if (!e.target.checked) setNewPaymentCard(null)
                         }}
                         className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
                  <label htmlFor="nfce-toggle-card" className="text-xs font-medium text-gray-500 cursor-pointer">
                    {isPix ? 'Informar NSU/autorização (opcional)' : 'Informar dados do cartão'}
                  </label>
                </div>
                {showCardToggle && (
                  <PaymentCardFields card={newPaymentCard} onChange={setNewPaymentCard} isPix={isPix}/>
                )}
              </div>
            )}
          </div>

          <CollapsibleSection title="Configurações avançadas" description="Informações adicionais (opcional)">
            <Label className="text-xs font-medium text-gray-600">Informações adicionais (opcional)</Label>
            <Textarea value={additionalInfo} onChange={(e) => setAdditionalInfo(e.target.value)} rows={3}
                      maxLength={2000} placeholder="Observações…" className="w-full mt-1"/>
          </CollapsibleSection>

          {submitError && (
            <div
              className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{submitError}</div>
          )}
        </div>
      )}

      {/* Action bar */}
      <div
        className="sticky bottom-0 -mx-4 px-4 md:-mx-8 md:px-8 py-3 mt-6 bg-gray-50 border-t border-gray-200 flex items-center justify-between gap-2">
        <Button type="button" variant="outline" size="sm" onClick={goBack} disabled={stepIdx === 0}>Voltar</Button>
        {step !== 'pagamento' ? (
          <Button type="button" variant="brand" size="sm" onClick={goNext} disabled={!canNext(step)}>Próximo</Button>
        ) : (
          <Button type="button" variant="brand" size="sm" onClick={handleSubmit}
                  disabled={isSubmitting || !canEmit}>
            {isSubmitting ? 'Emitindo…' : 'Emitir NFC-e'}
          </Button>
        )}
      </div>
    </div>
  )
}
