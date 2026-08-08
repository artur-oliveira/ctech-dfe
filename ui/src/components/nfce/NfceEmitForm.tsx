'use client'

import {useCallback, useEffect, useMemo, useRef, useState} from 'react'
import {useRouter} from 'next/navigation'
import {useQuery} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient, ApiError} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {queryKeys} from '@/lib/api/query-keys'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {GlossaryTerm} from '@/components/ui/glossary-term'
import {CollapsibleSection} from '@/components/ui/collapsible-section'
import {Textarea} from '@/components/ui/textarea'
import {Modal} from '@/components/ui/modal'
import {EmitConfirmModal} from '@/components/ui/emit-confirm-modal'
import {EmitError} from '@/components/ui/emit-error'
import {DraftRecoveryBanner} from '@/components/ui/draft-recovery-banner'
import {useEmitDraft} from '@/lib/hooks/useEmitDraft'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {ProductLineItem} from '@/components/ui/product-line-item'
import {ProductSearch} from '@/components/ui/product-search'
import {PersonForm} from '@/components/persons/PersonForm'
import {CARD_PAYMENT_TYPES, isPixPaymentType, PaymentCardFields} from '@/components/nfe/PaymentCardFields'
import {NatOpInlineEdit} from '@/components/nfe/NatOpInlineEdit'
import type {CfopConfigItem, NfceEmit, NfeCardIn, PersonCreate, ProductOut} from '@/lib/types/api'
import {NF_PAYMENT_TYPES} from '@/lib/types/api'
import {PAYMENT_OPTIONS, QUICK_PAYMENT_TYPES} from '@/lib/data/payment-options'
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

/** NFC-e is always an internal consumer sale — only 5xxx saída CFOPs apply. */
const NFCE_CFOP_PREFIX = '5'

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
    .filter((c) => c.startsWith(NFCE_CFOP_PREFIX))
  if (configured.length > 0) return configured
  if (product.cfop_nfce?.startsWith(NFCE_CFOP_PREFIX)) return [product.cfop_nfce]
  return []
}

// ─── consumer (CPF na nota) ───────────────────────────────────────────────────

/**
 * "CPF na nota?" — the question a cashier asks at payment, not before the sale.
 * Identification is optional for NFC-e, so this never blocks the flow.
 */
function ConsumerField({value, onChange}: { value: Consumer | null; onChange: (c: Consumer | null) => void }) {
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
        <div className="flex items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 px-4 py-3">
          <div className="flex-1 min-w-0">
            <p className="font-medium text-gray-900 text-sm">{value.name ?? 'Consumidor não identificado'}</p>
            <p className="text-xs text-gray-500 font-mono mt-0.5">{formatCpfCnpj(value.cpf)}</p>
          </div>
          <Button type="button" variant="ghost" size="xs" onClick={() => {
            onChange(null)
            setNotFound(false)
          }} className="text-danger hover:text-red-700 shrink-0">Remover</Button>
        </div>
        {!value.name && (
          <p className="text-xs text-warning">
            CPF não cadastrado — a NFC-e pode ser emitida assim mesmo.{' '}
            <button type="button" className="text-brand-700 hover:text-brand-800 underline"
                    onClick={() => setShowCreate(true)}>Cadastrar (opcional)
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
    <div ref={containerRef} className="space-y-2">
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
            placeholder="CPF ou nome (opcional)"
            aria-label="CPF ou nome do consumidor"
            role="combobox"
            aria-expanded={open && !isCpf && suggestions.length > 0}
            aria-controls="nfce-consumer-suggestions"
            aria-autocomplete="list"
            className="flex-1"
          />
          {docLoading && (
            <span className="flex items-center px-3 text-xs text-gray-500 shrink-0">Buscando…</span>
          )}
        </div>

        {open && !isCpf && suggestions.length > 0 && (
          <div
            id="nfce-consumer-suggestions"
            role="listbox"
            aria-label="Consumidores encontrados"
            className="absolute z-20 mt-1 w-full rounded-lg border border-gray-200 bg-white shadow-popover overflow-hidden">
            {suggestions.map((p) => (
              <button key={p.sk} type="button" role="option" aria-selected={false}
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={() => {
                        onChange({cpf: p.sk.replace(/\D/g, ''), name: p.name})
                        setQuery('')
                        setOpen(false)
                      }}
                      className="w-full text-left px-4 py-2.5 hover:bg-gray-50 transition-colors">
                <p className="text-sm font-medium text-gray-900">{p.name}</p>
                <p className="text-xs text-gray-500 font-mono">{formatCpfCnpj(p.sk.replace(/\D/g, ''))}</p>
              </button>
            ))}
          </div>
        )}
      </div>

      {notFound && (
        <p className="text-xs text-warning">
          CPF não encontrado.{' '}
          <button type="button" className="text-brand-700 hover:text-brand-800 underline"
                  onClick={() => setShowCreate(true)}>Cadastrar pessoa (opcional)
          </button>
        </p>
      )}

      <Modal isOpen={showCreate} title="Cadastrar consumidor (pessoa física)"
             onClose={() => setShowCreate(false)} size="xl">
        <PersonForm lockTipo="pf" initialCpfCnpj={digits} onSubmit={handleCreatePerson} loading={createLoading}/>
      </Modal>
    </div>
  )
}

// ─── main form ────────────────────────────────────────────────────────────────

/**
 * NFC-e issuance — a counter sale, not a wizard.
 *
 * NF-e is a considered document (recipient, transport, duplicatas) and earns a
 * stepped flow. NFC-e is issued dozens of times an hour with a queue waiting,
 * so everything lives on one screen: the scan field keeps focus and adds on
 * Enter, the running total is always visible, and identification is asked once,
 * optionally, next to the payment — where a cashier actually asks it.
 */
export function NfceEmitForm() {
  const {selectedOrg} = useAuth()
  const router = useRouter()

  const [consumer, setConsumer] = useState<Consumer | null>(null)
  const [products, setProducts] = useState<EmitProduct[]>([])
  const [payments, setPayments] = useState<EmitPayment[]>([])
  const [newPaymentType, setNewPaymentType] = useState(QUICK_PAYMENT_TYPES[0] as string)
  const [newPaymentValue, setNewPaymentValue] = useState('')
  const [newPaymentCard, setNewPaymentCard] = useState<NfeCardIn | null>(null)
  const [showCardToggle, setShowCardToggle] = useState(false)
  const paymentLocked = useRef(false)
  const [natOpManual, setNatOpManual] = useState<string | null>(null)
  const [additionalInfo, setAdditionalInfo] = useState('')
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [showEmitConfirm, setShowEmitConfirm] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const {data: nfceConfig} = useQuery({
    queryKey: queryKeys.nfceConfig(selectedOrg!.pk),
    queryFn: () => apiClient.getNFCeConfig(selectedOrg!.pk),
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

  // ─── draft recovery ───────────────────────────────────────────────────────

  const draftState = useMemo(
    () => ({products, payments, consumer, additionalInfo, natOpManual}),
    [products, payments, consumer, additionalInfo, natOpManual])
  const draft = useEmitDraft('nfce', selectedOrg?.pk, draftState, products.length > 0)

  const restoreDraft = () => {
    const s = draft.recovered?.state
    if (s) {
      setProducts(s.products)
      setPayments(s.payments)
      setConsumer(s.consumer)
      setAdditionalInfo(s.additionalInfo)
      setNatOpManual(s.natOpManual)
    }
    draft.accept()
  }

  useEffect(() => {
    if (!paymentLocked.current) setNewPaymentValue(remaining > 0.005 ? remaining.toFixed(2) : '')
  }, [remaining])

  // ─── products ─────────────────────────────────────────────────────────────

  const addProduct = (product: ProductOut) => {
    const cfop = nfceCfopsForProduct(product)[0] ?? ''
    setProducts((prev) => {
      // Scanning the same item twice bumps the quantity instead of stacking rows.
      const existing = prev.findIndex((p) => p.product.sk === product.sk && p.cfop === cfop)
      if (existing >= 0) {
        return prev.map((p, i) =>
          i === existing ? {...p, qty: String((parseFloat(p.qty) || 0) + 1)} : p)
      }
      return [...prev, {product, cfop, qty: '1', unitValue: product.value, discount: '0'}]
    })
  }
  const changeProduct = (i: number, u: Partial<EmitProduct>) =>
    setProducts((prev) => prev.map((it, idx) => (idx === i ? {...it, ...u} : it)))
  const removeProduct = (i: number) => setProducts((prev) => prev.filter((_, idx) => idx !== i))

  const productDisabledReason = (p: ProductOut) =>
    nfceCfopsForProduct(p).length > 0 ? null : 'sem CFOP de NFC-e'

  // ─── payments ─────────────────────────────────────────────────────────────

  /** A typed-but-not-yet-added payment still counts towards emission. */
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

  const emitBlockedReason = products.length === 0
    ? 'Adicione pelo menos um produto.'
    : products.some((p) => !p.cfop.startsWith(NFCE_CFOP_PREFIX))
      ? 'Há produtos sem CFOP de saída (5xxx).'
      : effectivePayments().length === 0
        ? 'Informe pelo menos uma forma de pagamento.'
        : null
  const canEmit = !emitBlockedReason

  // ─── submit ───────────────────────────────────────────────────────────────

  const handleSubmit = async () => {
    setSubmitError(null)
    if (emitBlockedReason) {
      setSubmitError(emitBlockedReason)
      return
    }
    const allPayments = effectivePayments()
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
      draft.clear()
      toast.success('NFC-e enviada, aguardando autorização da SEFAZ.')
      router.push('/nfce')
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.detail : 'Erro ao emitir NFC-e.')
    } finally {
      setIsSubmitting(false)
    }
  }

  // ─── render ───────────────────────────────────────────────────────────────

  return (
    <div className="max-w-3xl pb-4">
      <HomologationBanner environment={nfceConfig?.environment}/>

      {draft.recovered && (
        <DraftRecoveryBanner savedAt={draft.recovered.savedAt} onRestore={restoreDraft} onDiscard={draft.discard}/>
      )}

      {/* Scan / search — always available, always focused */}
      <ProductSearch
        onSelect={addProduct}
        disabledReason={productDisabledReason}
        placeholder="Escaneie o código de barras ou busque o produto…"
        autoFocus
        className="rounded-xl border border-gray-200 bg-white p-4"
      />

      {/* Venda */}
      <div className="mt-4 space-y-2">
        {products.length === 0 ? (
          <div className="rounded-xl border border-dashed border-gray-200 px-4 py-10 text-center">
            <p className="text-sm font-medium text-gray-900">Nenhum item na venda</p>
            <p className="mt-1 text-sm text-gray-500">
              Escaneie um código de barras ou busque o produto acima. Enter adiciona o item destacado.
            </p>
          </div>
        ) : (
          products.map((item, i) => {
            const cfopOptions = nfceCfopsForProduct(item.product).map((cfop) => {
              const desc = getCfopDescription(cfop)
              const label = desc ? `${cfop} – ${desc}` : cfop
              return {value: cfop, label}
            })
            return (
              <ProductLineItem
                key={`${item.product.sk}-${i}`}
                idPrefix={`nfce-item-${i}`}
                description={item.product.description}
                brand={item.product.brand}
                unit={item.product.unit}
                qty={item.qty}
                unitValue={item.unitValue}
                discount={item.discount}
                total={computeTotal(item)}
                onChange={(patch) => changeProduct(i, patch)}
                onRemove={() => removeProduct(i)}
                cfopSlot={
                  <>
                    <div className="flex items-center gap-1">
                      <Label htmlFor={`nfce-item-${i}-cfop`} className="text-xs font-medium text-gray-600">CFOP</Label>
                      <GlossaryTerm term="cfop"/>
                    </div>
                    {cfopOptions.length > 0 ? (
                      <OptionsSelect id={`nfce-item-${i}-cfop`} value={item.cfop}
                                     onValueChange={(v) => changeProduct(i, {cfop: v})}
                                     options={cfopOptions} placeholder="CFOP"/>
                    ) : (
                      <Input id={`nfce-item-${i}-cfop`} type="text" value={item.cfop} maxLength={4} placeholder="5102"
                             aria-invalid={!item.cfop.startsWith(NFCE_CFOP_PREFIX)}
                             onChange={(e) => changeProduct(i, {cfop: e.target.value})}/>
                    )}
                  </>
                }
              />
            )
          })
        )}

        {products.length > 0 && natOp && (
          <NatOpInlineEdit value={natOp} onChange={setNatOpManual}
                           onReset={() => setNatOpManual(null)} canReset={natOpManual !== null}/>
        )}
      </div>

      {/* Pagamento */}
      {products.length > 0 && (
        <div className="mt-4 rounded-xl border border-gray-200 bg-white p-4 space-y-3">
          <p className="text-sm font-medium text-gray-600">Pagamento</p>

          {payments.length > 0 && (
            <div className="space-y-2">
              {payments.map((p, i) => (
                <div key={i} className="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2.5 text-sm">
                  <span className="text-gray-700">
                    {NF_PAYMENT_TYPES[p.payment_type] ?? p.payment_type}
                    {p.card && <span className="ml-1.5 text-xs text-blue-700">· transação</span>}
                  </span>
                  <div className="flex items-center gap-3">
                    <span className="font-medium">{fmt(parseFloat(p.value) || 0)}</span>
                    <Button type="button" variant="ghost" size="xs" onClick={() => removePayment(i)}
                            className="text-danger hover:text-red-700">remover</Button>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Quick picks — the three an operator reaches for */}
          <div className="flex flex-wrap gap-2">
            {QUICK_PAYMENT_TYPES.map((code) => {
              const active = newPaymentType === code
              return (
                <button
                  key={code}
                  type="button"
                  aria-pressed={active}
                  onClick={() => {
                    setNewPaymentType(code)
                    setShowCardToggle(false)
                    setNewPaymentCard(null)
                  }}
                  className={`min-h-11 sm:min-h-9 flex-1 sm:flex-none rounded-lg border px-4 text-sm font-medium transition-colors ${
                    active
                      ? 'border-brand-600 bg-brand-600 text-white'
                      : 'border-gray-200 bg-white text-gray-700 hover:border-brand-300 hover:text-brand-700'
                  }`}
                >
                  {NF_PAYMENT_TYPES[code] ?? code}
                </button>
              )
            })}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto_auto] gap-2 items-end">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-1">
                <Label htmlFor="nfce-payment-type" className="text-xs font-medium text-gray-600">
                  Outra forma de pagamento
                </Label>
                <GlossaryTerm term="ind_pag"/>
              </div>
              <OptionsSelect id="nfce-payment-type" value={newPaymentType} onValueChange={(v) => {
                setNewPaymentType(v)
                setShowCardToggle(false)
                setNewPaymentCard(null)
              }} options={PAYMENT_OPTIONS}/>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="nfce-payment-value" className="text-xs font-medium text-gray-600">Valor</Label>
              <CurrencyInput id="nfce-payment-value" decimalPlaces={2} value={newPaymentValue}
                             onChange={(v) => {
                               paymentLocked.current = true
                               setNewPaymentValue(v)
                             }} placeholder="0,00"/>
            </div>
            <Button type="button" variant="outline" onClick={addPayment} className="self-end"
                    disabled={!newPaymentValue || parseFloat(newPaymentValue) <= 0}>
              Dividir pagamento
            </Button>
          </div>

          {(isCardPayment || isPix) && (
            <div className="pt-1 border-t border-gray-100 space-y-2">
              <label htmlFor="nfce-toggle-card" className="flex items-center gap-2 min-h-11 sm:min-h-0 cursor-pointer">
                <input type="checkbox" id="nfce-toggle-card" checked={showCardToggle}
                       onChange={(e) => {
                         setShowCardToggle(e.target.checked)
                         if (!e.target.checked) setNewPaymentCard(null)
                       }}
                       className="h-3.5 w-3.5 rounded border-gray-300 text-brand-600"/>
                <span className="text-xs font-medium text-gray-600">
                  {isPix ? 'Informar NSU/autorização (opcional)' : 'Informar dados do cartão'}
                </span>
              </label>
              {showCardToggle && (
                <PaymentCardFields card={newPaymentCard} onChange={setNewPaymentCard} isPix={isPix}/>
              )}
            </div>
          )}

          {payments.length > 0 && (
            <p className={`text-sm ${
              Math.abs(remaining) < 0.01 ? 'text-success' : remaining < 0 ? 'text-blue-700' : 'text-warning'
            }`}>
              {Math.abs(remaining) < 0.01
                ? '✓ Total confere.'
                : remaining > 0 ? `⌛ Restam ${fmt(remaining)}.` : `↩ Troco: ${fmt(-remaining)}`}
            </p>
          )}

          {/* CPF na nota — asked where a cashier asks it */}
          <div className="pt-3 border-t border-gray-100 space-y-2">
            <Label className="text-xs font-medium text-gray-600">CPF na nota? (opcional)</Label>
            <ConsumerField value={consumer} onChange={setConsumer}/>
          </div>
        </div>
      )}

      {products.length > 0 && (
        <CollapsibleSection title="Configurações avançadas" description="Informações adicionais (opcional)"
                            className="mt-4">
          <Label htmlFor="nfce-additional-info" className="text-xs font-medium text-gray-600">
            Informações adicionais
          </Label>
          <Textarea id="nfce-additional-info" value={additionalInfo}
                    onChange={(e) => setAdditionalInfo(e.target.value)} rows={3}
                    maxLength={2000} placeholder="Observações…" className="w-full mt-1"/>
        </CollapsibleSection>
      )}

      <div className="mt-4 empty:mt-0">
        <EmitError message={submitError}/>
      </div>

      {/* Action bar — the total never leaves the screen */}
      <div
        className="sticky bottom-0 -mx-4 px-4 md:mx-0 md:px-0 py-3 mt-6 bg-gray-50 border-t border-gray-200 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs text-gray-500">Total{products.length > 0 && ` · ${products.length} item(s)`}</p>
          <p className="text-lg font-semibold text-gray-900 leading-tight">{fmt(totalNfce)}</p>
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          <Button type="button" variant="brand" onClick={() => setShowEmitConfirm(true)}
                  disabled={isSubmitting || !canEmit}>
            {isSubmitting ? 'Emitindo…' : 'Emitir NFC-e'}
          </Button>
          {emitBlockedReason && <span className="text-xs text-warning">{emitBlockedReason}</span>}
        </div>
      </div>

      <EmitConfirmModal
        open={showEmitConfirm}
        onClose={() => setShowEmitConfirm(false)}
        onConfirm={() => {
          setShowEmitConfirm(false)
          void handleSubmit()
        }}
        docLabel="NFC-e"
        summary={[
          {label: 'Consumidor', value: consumer ? formatCpfCnpj(consumer.cpf) : 'Não identificado'},
          {label: 'Itens', value: `${products.length} item(s)`},
          {
            label: 'Pagamento',
            value: effectivePayments()
              .map((p) => `${NF_PAYMENT_TYPES[p.payment_type] ?? p.payment_type} ${fmt(parseFloat(p.value) || 0)}`)
              .join(' + ') || '—',
          },
          {label: 'Total', value: fmt(totalNfce)},
        ]}
      />
    </div>
  )
}
