'use client'

import {useMemo, useState} from 'react'
import {useRouter} from 'next/navigation'
import {createPortal} from 'react-dom'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {queryKeys} from '@/lib/api/query-keys'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Label} from '@/components/ui/label'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Combobox} from '@/components/ui/combobox'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {VehicleForm} from '@/components/vehicles/VehicleForm'
import {UF_OPTIONS} from '@/lib/schemas/entity'
import {suggestRoute, ufsBorder} from '@/lib/utils/uf-graph'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency} from '@/lib/utils/helpers'
import {maskCpf} from '@/lib/utils/masks'
import {validateCPF} from '@/lib/utils/validators'
import type {
  MdfeCargoPreview,
  MdfeDriverIn,
  MdfeEmit,
  MdfeMunIn,
  NfeListOut,
  VehicleCreate,
  VehicleOut,
} from '@/lib/types/api'
import {Plane, Ship, TramFront, Truck} from 'lucide-react'

// ─── steps ──────────────────────────────────────────────────────────────────

type Step = 'modal' | 'documentos' | 'carga' | 'transporte' | 'seguro' | 'veiculo'

interface StepDef {
  id: Step
  label: string
}

const BASE_STEPS: StepDef[] = [
  {id: 'modal', label: 'Transporte'},
  {id: 'documentos', label: 'Documentos'},
  {id: 'carga', label: 'Carga'},
  {id: 'transporte', label: 'Trajeto'},
  {id: 'veiculo', label: 'Veículo'},
]

// Seguro só aparece quando há CT-e (MDF-e de NF-e não exige seguro).
const SEGURO_STEP: StepDef = {id: 'seguro', label: 'Seguro'}

const MODAIS = [
  {id: 'rodoviario', label: 'Rodoviário', icon: <Truck/>, enabled: true},
  {id: 'aereo', label: 'Aéreo', icon: <Plane/>, enabled: false},
  {id: 'aquaviario', label: 'Aquaviário', icon: <Ship/>, enabled: false},
  {id: 'ferroviario', label: 'Ferroviário', icon: <TramFront/>, enabled: false},
]

function StepIndicator({steps, current}: { steps: StepDef[]; current: Step }) {
  const idx = steps.findIndex((s) => s.id === current)
  return (
    <div className="flex items-center gap-0 mb-6">
      {steps.map((step, i) => {
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
            {i < steps.length - 1 && <div className={`flex-1 h-0.5 mx-2 ${i < idx ? 'bg-brand-500' : 'bg-gray-200'}`}/>}
          </div>
        )
      })}
    </div>
  )
}

// ─── document picker (NF-e) ───────────────────────────────────────────────────

function DocumentPicker({selected, onToggle}: {
  selected: NfeListOut[]
  onToggle: (n: NfeListOut) => void
}) {
  const {selectedOrg} = useAuth()
  const [incoming, setIncoming] = useState<0 | 1>(0)
  const [numberSearch, setNumberSearch] = useState('')
  const debouncedNumber = useDebounce(numberSearch, 300)

  const params = {
    sort: 'desc' as const,
    incoming,
    limit: 50,
    ...(debouncedNumber ? {number: parseInt(debouncedNumber, 10)} : {}),
  }

  const {data, isLoading} = useQuery({
    queryKey: queryKeys.nfes.list(selectedOrg?.pk, params),
    queryFn: () => apiClient.getNfes(params),
    enabled: !!selectedOrg,
  })

  // Only authorized NF-es have an XML available for manifestation.
  const items = (data?.items ?? []).filter((n) => n.status === 'authorized')
  const selectedKeys = new Set(selected.map((s) => s.sk))

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Button type="button" size="xs" variant={incoming === 0 ? 'brand' : 'outline'}
                onClick={() => setIncoming(0)}>Emitidas</Button>
        <Button type="button" size="xs" variant={incoming === 1 ? 'brand' : 'outline'}
                onClick={() => setIncoming(1)}>Recebidas</Button>
        <div className="ml-auto w-32">
          <NumericInput value={numberSearch} onChange={setNumberSearch} integerPlaces={9}
                        placeholder="Nº da nota" className="w-full"/>
        </div>
      </div>

      <div className="max-h-72 overflow-y-auto rounded-lg border border-gray-200 divide-y divide-gray-100">
        {isLoading ? (
          <div className="p-3">
            <LoadingSkeleton count={4} height="h-10" rounded="rounded-md"/>
          </div>
        ) : items.length === 0 ? (
          <p className="p-4 text-sm text-gray-500 text-center">Nenhuma NF-e autorizada encontrada.</p>
        ) : (
          items.map((n) => {
            const checked = selectedKeys.has(n.sk)
            const counterparty = incoming === 0 ? n.dest_name : n.emit_name
            const doc = incoming === 0 ? n.dest_cpf_cnpj : n.emit_cpf_cnpj
            return (
              <button key={n.sk} type="button" onClick={() => onToggle(n)}
                      className={`flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors ${checked ? 'bg-brand-50' : 'hover:bg-gray-50'}`}>
                <span
                  className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border ${checked ? 'border-brand-600 bg-brand-600 text-white' : 'border-gray-300'}`}>
                  {checked && <span className="text-xs leading-none">✓</span>}
                </span>
                <span className="flex-1 min-w-0">
                  <span className="block text-sm font-medium text-gray-900 truncate">
                    Nº {n.number} · {counterparty || 'Sem destinatário'}
                  </span>
                  {doc && <span className="block text-xs text-gray-400 font-mono">{formatCpfCnpj(doc)}</span>}
                </span>
                <span className="text-xs text-gray-500 shrink-0">{formatCurrency(n.total)}</span>
              </button>
            )
          })
        )}
      </div>
    </div>
  )
}

// ─── municipality reorder list ────────────────────────────────────────────────

function MunReorderList({title, hint, muns, onReorder}: {
  title: string
  hint: string
  muns: MdfeMunIn[]
  onReorder: (next: MdfeMunIn[]) => void
}) {
  const move = (from: number, to: number) => {
    if (to < 0 || to >= muns.length) return
    const next = [...muns]
    const [item] = next.splice(from, 1)
    next.splice(to, 0, item)
    onReorder(next)
  }
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">{title}</p>
      <p className="text-xs text-gray-500">{hint}</p>
      <div className="space-y-1.5">
        {muns.map((m, i) => (
          <div key={m.ibge_code} className="flex items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm">
            <span className="w-5 text-center text-xs font-semibold text-gray-400">{i + 1}</span>
            <span className="flex-1 min-w-0 truncate text-gray-700">{m.city || m.ibge_code}</span>
            <Button type="button" variant="ghost" size="icon-sm" disabled={i === 0}
                    onClick={() => move(i, i - 1)} aria-label="Subir">↑</Button>
            <Button type="button" variant="ghost" size="icon-sm" disabled={i === muns.length - 1}
                    onClick={() => move(i, i + 1)} aria-label="Descer">↓</Button>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── vehicle modal (reuses VehicleForm) — handles both "no vehicle yet" and
// "vehicle selected but incomplete for this doc-type/role" ──────────────────

function VehicleRegisterModal({open, onClose, onSaved, editing, missing}: {
  open: boolean
  onClose: () => void
  onSaved: (v: VehicleOut) => void
  editing?: VehicleOut
  missing?: string[]
}) {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()
  const createMutation = useMutation({
    mutationFn: (d: VehicleCreate) => apiClient.createVehicle(d),
    onSuccess: (v) => {
      void qc.invalidateQueries({queryKey: queryKeys.vehicles.list(selectedOrg?.pk)})
      onSaved(v)
    },
  })
  const updateMutation = useMutation({
    mutationFn: (d: VehicleCreate) => apiClient.updateVehicle(editing!.sk, d),
    onSuccess: (v) => {
      void qc.invalidateQueries({queryKey: queryKeys.vehicles.list(selectedOrg?.pk)})
      onSaved(v)
    },
  })
  const mutation = editing ? updateMutation : createMutation
  if (!open || typeof document === 'undefined') return null
  return createPortal(
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center">
      <div className="bg-white rounded-xl shadow-modal max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div
          className="sticky top-0 bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between rounded-t-xl">
          <h2 className="text-lg font-semibold text-gray-900">
            {editing ? 'Completar dados do veículo' : 'Cadastrar veículo'}
          </h2>
          <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Fechar"
                  className="text-gray-400 hover:text-gray-600">×</Button>
        </div>
        <div className="p-6">
          <VehicleForm initialData={editing} highlightFields={missing}
                       onSubmit={async (d) => {
                         await mutation.mutateAsync(d)
                       }}
                       loading={mutation.isPending}/>
        </div>
      </div>
    </div>,
    document.body,
  )
}

// ─── cargo step ───────────────────────────────────────────────────────────────

function CargoStep({preview, isLoading, error, weightOverrides, onWeightChange}: {
  preview: MdfeCargoPreview | undefined
  isLoading: boolean
  error: string | null
  weightOverrides: Record<string, string>
  onWeightChange: (key: string, weight: string) => void
}) {
  if (isLoading) {
    return <LoadingSkeleton count={3} height="h-20" rounded="rounded-xl"/>
  }
  if (error) {
    return <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div>
  }
  if (!preview) return null

  // Recompute total weight including user-supplied overrides for visual feedback.
  const totalWeight = preview.documents.reduce((sum, d) => {
    const w = d.has_weight ? parseFloat(d.weight) : parseFloat(weightOverrides[d.access_key] || '0')
    return sum + (isNaN(w) ? 0 : w)
  }, 0)

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div className="rounded-xl border border-gray-200 bg-white p-3">
          <p className="text-xs text-gray-400">Documentos</p>
          <p className="text-lg font-semibold text-gray-900">{preview.documents.length}</p>
        </div>
        <div className="rounded-xl border border-gray-200 bg-white p-3">
          <p className="text-xs text-gray-400">Valor total</p>
          <p className="text-lg font-semibold text-gray-900">{formatCurrency(preview.total_value)}</p>
        </div>
        <div className="rounded-xl border border-gray-200 bg-white p-3">
          <p className="text-xs text-gray-400">Peso total</p>
          <p className="text-lg font-semibold text-gray-900">{totalWeight.toLocaleString('pt-BR')} kg</p>
        </div>
      </div>

      <div className="rounded-xl border border-gray-200 bg-white p-3">
        <p className="text-xs text-gray-400">Produto predominante</p>
        <p className="text-sm font-medium text-gray-900">{preview.predominant.x_prod || '—'}</p>
        {preview.predominant.ncm && <p className="text-xs text-gray-400 font-mono">NCM {preview.predominant.ncm}</p>}
      </div>

      <div className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Cargas por documento</p>
        {preview.documents.map((d) => (
          <div key={d.access_key} className="rounded-xl border border-gray-200 bg-white p-3 space-y-2">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <p className="text-sm font-medium text-gray-900 truncate">{d.emit_name} → {d.dest_name}</p>
                <p className="text-xs text-gray-400">
                  {d.loading.city}/{d.uf_start} → {d.unloading.city}/{d.uf_end} · {formatCurrency(d.value)}
                </p>
              </div>
            </div>
            {d.has_weight ? (
              <p className="text-xs text-gray-500">Peso: {parseFloat(d.weight).toLocaleString('pt-BR')} kg</p>
            ) : (
              <div className="flex flex-col gap-1">
                <Label className="text-xs font-medium text-amber-700">
                  Documento sem peso — informe o peso da carga (kg)
                </Label>
                <NumericInput value={weightOverrides[d.access_key] || ''}
                              onChange={(v) => onWeightChange(d.access_key, v)}
                              decimalPlaces={3} placeholder="0,000" className="w-full sm:w-48"/>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── main form ────────────────────────────────────────────────────────────────

export function MdfeEmitForm() {
  const {selectedOrg} = useAuth()
  const router = useRouter()

  const [step, setStep] = useState<Step>('modal')
  const [docs, setDocs] = useState<NfeListOut[]>([])

  // Cargo preview state.
  const [weightOverrides, setWeightOverrides] = useState<Record<string, string>>({})

  // Trajeto state — kept as nullable overrides so the effective values can be
  // derived from the cargo preview without setState-in-effect.
  const [loadingsOverride, setLoadingsOverride] = useState<MdfeMunIn[] | null>(null)
  const [unloadingsOverride, setUnloadingsOverride] = useState<MdfeMunIn[] | null>(null)
  const [ufIniOverride, setUfIniOverride] = useState('')
  const [ufFimOverride, setUfFimOverride] = useState('')
  const [routeOverride, setRouteOverride] = useState<string[] | null>(null)
  const [newRouteUf, setNewRouteUf] = useState('')
  const [tripStart, setTripStart] = useState('')

  // Bulk cargo (single document).
  const [cepCarrega, setCepCarrega] = useState('')
  const [cepDescarrega, setCepDescarrega] = useState('')

  // Vehicles (registered only) + register/edit modal.
  const [vehicleSk, setVehicleSk] = useState<string | null>(null)
  const [trailerSks, setTrailerSks] = useState<string[]>([])
  const [gateModal, setGateModal] = useState<{ vehicle: VehicleOut; missing: string[] } | null>(null)
  const [registerOpen, setRegisterOpen] = useState(false)

  // Drivers.
  const [drivers, setDrivers] = useState<MdfeDriverIn[]>([])
  const [newDriverName, setNewDriverName] = useState('')
  const [newDriverCpf, setNewDriverCpf] = useState('')

  const [submitError, setSubmitError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const {data: mdfeConfig} = useQuery({
    queryKey: queryKeys.mdfeConfig(selectedOrg!.pk),
    queryFn: () => apiClient.getMDFeConfig(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })

  const {data: tractorsData} = useQuery({
    queryKey: queryKeys.vehicles.list(selectedOrg?.pk, 'tractor'),
    queryFn: () => apiClient.getVehicles({role: 'tractor', limit: 50}),
    enabled: !!selectedOrg,
  })
  const {data: trailersData} = useQuery({
    queryKey: queryKeys.vehicles.list(selectedOrg?.pk, 'trailer'),
    queryFn: () => apiClient.getVehicles({role: 'trailer', limit: 50}),
    enabled: !!selectedOrg,
  })

  const tractorOptions = (tractorsData?.items ?? []).map((v: VehicleOut) => ({
    value: v.sk, label: `${v.plate} · ${v.plate_uf}`, display: v.plate,
  }))
  const trailerOptions = (trailersData?.items ?? []).map((v: VehicleOut) => ({
    value: v.sk, label: `${v.plate} · ${v.plate_uf}`, display: v.plate,
  }))

  const checkVehicle = async (v: VehicleOut, role: 'tractor' | 'trailer') => {
    const {missing} = await apiClient.getVehicleRequirements(v.sk, 'mdfe', role)
    if (missing.length > 0) setGateModal({vehicle: v, missing})
  }

  const onSelectTractor = (sk: string | null) => {
    setVehicleSk(sk)
    const v = tractorsData?.items.find((x) => x.sk === sk)
    if (v) void checkVehicle(v, 'tractor')
  }

  const onSelectTrailer = (sk: string) => {
    setTrailerSks((prev) => prev.includes(sk) ? prev : [...prev, sk])
    const v = trailersData?.items.find((x) => x.sk === sk)
    if (v) void checkVehicle(v, 'trailer')
  }

  const removeTrailer = (sk: string) => setTrailerSks((prev) => prev.filter((s) => s !== sk))

  // Seguro só aparece com CT-e. O seletor de documentos é NF-e-only no MVP, logo
  // hasCte é sempre falso; mantido explícito para quando o CT-e for habilitado.
  const hasCte = false
  const STEPS = useMemo(
    () => (hasCte ? [...BASE_STEPS.slice(0, 4), SEGURO_STEP, BASE_STEPS[4]] : BASE_STEPS),
    [hasCte],
  )

  // Cargo preview: fetched once documents are chosen and we reach the carga step.
  const docKeys = docs.map((d) => d.sk)
  const {data: preview, isLoading: previewLoading, error: previewErr} = useQuery({
    queryKey: queryKeys.mdfes.cargoPreview(selectedOrg?.pk, docKeys),
    queryFn: () => apiClient.previewMdfeCargo(docs.map((d) => ({type: 'nfe' as const, access_key: d.sk}))),
    enabled: !!selectedOrg && docKeys.length > 0 && (step === 'carga' || step === 'transporte' || step === 'veiculo'),
  })

  // Effective trajeto values: user override, else derived from the preview.
  const loadings = loadingsOverride ?? preview?.loadings ?? []
  const unloadings = unloadingsOverride ?? preview?.unloadings ?? []
  const ufIni = ufIniOverride || preview?.uf_start || ''
  const ufFim = ufFimOverride || preview?.uf_end || ''
  const routeNeeded = !!ufIni && !!ufFim && !ufsBorder(ufIni, ufFim)
  const suggestedRoute = routeNeeded ? suggestRoute(ufIni, ufFim) : []
  const route = routeOverride ?? suggestedRoute

  const toggleDoc = (n: NfeListOut) =>
    setDocs((prev) => prev.some((d) => d.sk === n.sk) ? prev.filter((d) => d.sk !== n.sk) : [...prev, n])

  const addRouteUf = () => {
    if (newRouteUf && !route.includes(newRouteUf)) setRouteOverride([...route, newRouteUf])
    setNewRouteUf('')
  }

  const addDriver = () => {
    const cpf = newDriverCpf.replace(/\D/g, '')
    if (!newDriverName.trim() || !validateCPF(cpf)) return
    if (drivers.some((c) => c.cpf === cpf)) return
    setDrivers((prev) => [...prev, {name: newDriverName.trim(), cpf}])
    setNewDriverName('')
    setNewDriverCpf('')
  }

  // UF options limited to the states present in the referenced documents.
  const docUfs = useMemo(() => {
    const set = new Set<string>()
    preview?.documents.forEach((d) => {
      if (d.uf_start) set.add(d.uf_start)
      if (d.uf_end) set.add(d.uf_end)
    })
    return set
  }, [preview])
  const ufIniOptions = UF_OPTIONS.filter((o) => docUfs.size === 0 || docUfs.has(o.value))
  const ufFimOptions = ufIniOptions

  const isSingleDoc = docs.length === 1
  const needsBulk = isSingleDoc
  const allWeightsKnown = (preview?.documents ?? []).every(
    (d) => d.has_weight || (weightOverrides[d.access_key]?.trim() ?? '') !== '',
  )

  const canNext = (s: Step): boolean => {
    if (s === 'modal') return true
    if (s === 'documentos') return docs.length > 0
    if (s === 'carga') return !!preview && allWeightsKnown
    if (s === 'transporte') {
      const bulkOk = !needsBulk || (cepCarrega.replace(/\D/g, '').length === 8 && cepDescarrega.replace(/\D/g, '').length === 8)
      const routeOk = !routeNeeded || route.length > 0
      return !!ufIni && !!ufFim && bulkOk && routeOk
    }
    if (s === 'seguro') return true
    return false
  }

  const stepIdx = STEPS.findIndex((s) => s.id === step)
  const goNext = () => {
    if (stepIdx < STEPS.length - 1 && canNext(step)) setStep(STEPS[stepIdx + 1].id)
  }
  const goBack = () => {
    if (stepIdx > 0) setStep(STEPS[stepIdx - 1].id)
  }

  const canEmit = docs.length > 0 && !!vehicleSk && drivers.length > 0 && allWeightsKnown
    && (!needsBulk || (cepCarrega.replace(/\D/g, '').length === 8 && cepDescarrega.replace(/\D/g, '').length === 8))

  const handleSubmit = async () => {
    setSubmitError(null)
    if (!canEmit) {
      setSubmitError('Preencha carga, trajeto, veículo cadastrado e ao menos um condutor.')
      return
    }
    const payload: MdfeEmit = {
      modal: 'rodoviario',
      documents: docs.map((d) => {
        const override = weightOverrides[d.sk]?.trim()
        return {type: 'nfe', access_key: d.sk, ...(override ? {weight: override} : {})}
      }),
      uf_start: ufIni || undefined,
      uf_end: ufFim || undefined,
      route: route.length ? route : undefined,
      loadings: loadings.length ? loadings : undefined,
      unloadings: unloadings.length ? unloadings : undefined,
      drivers,
      vehicle: {sk: vehicleSk},
      trailers: trailerSks.length ? trailerSks.map((sk) => ({sk})) : undefined,
      trip_start: tripStart ? `${tripStart}:00-03:00` : undefined,
      bulk_cargo: needsBulk
        ? {cep_loading: cepCarrega.replace(/\D/g, ''), cep_unloading: cepDescarrega.replace(/\D/g, '')}
        : undefined,
    }
    setIsSubmitting(true)
    try {
      await apiClient.emitMdfe(payload)
      router.push('/mdfe')
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao emitir MDF-e.')
    } finally {
      setIsSubmitting(false)
    }
  }

  const previewError = previewErr instanceof Error ? previewErr.message : (previewErr ? 'Erro ao analisar documentos.' : null)

  return (
    <div className="max-w-3xl">
      <HomologationBanner environment={mdfeConfig?.environment}/>
      <StepIndicator steps={STEPS} current={step}/>

      {/* Step 1 — modal */}
      {step === 'modal' && (
        <div className="space-y-3">
          <p className="text-sm text-gray-500">Selecione o tipo de transporte.</p>
          <div className="grid grid-cols-2 gap-3">
            {MODAIS.map((m) => (
              <button key={m.id} type="button" disabled={!m.enabled}
                      className={`relative rounded-xl border p-4 text-left transition-colors ${
                        m.id === 'rodoviario'
                          ? 'border-brand-500 bg-brand-50 ring-2 ring-brand-200'
                          : 'border-gray-200 opacity-50 cursor-not-allowed'}`}>
                <span className="text-2xl">{m.icon}</span>
                <p className="mt-2 text-sm font-medium text-gray-900">{m.label}</p>
                {!m.enabled && <span
                    className="absolute top-2 right-2 text-xs font-medium text-amber-600 bg-amber-50 px-1.5 py-0.5 rounded">em breve</span>}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Step 2 — documents */}
      {step === 'documentos' && (
        <div className="space-y-3">
          {docs.length > 0 && (
            <div className="rounded-xl border border-gray-200 bg-white p-3 space-y-1.5">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                {docs.length} documento{docs.length !== 1 ? 's' : ''} selecionado{docs.length !== 1 ? 's' : ''}
              </p>
              {docs.map((d) => (
                <div key={d.sk} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-sm">
                  <span className="text-gray-700 min-w-0 truncate">Nº {d.number} · {d.dest_name || d.emit_name}</span>
                  <Button type="button" variant="ghost" size="xs" onClick={() => toggleDoc(d)}
                          className="text-danger hover:text-red-700 shrink-0">remover</Button>
                </div>
              ))}
            </div>
          )}
          <DocumentPicker selected={docs} onToggle={toggleDoc}/>
          <p className="text-xs text-gray-400">
            Apenas NF-e por enquanto. Na próxima etapa mostramos a carga e os valores extraídos das notas.
          </p>
        </div>
      )}

      {/* Step 3 — carga */}
      {step === 'carga' && (
        <CargoStep preview={preview} isLoading={previewLoading} error={previewError}
                   weightOverrides={weightOverrides}
                   onWeightChange={(k, w) => setWeightOverrides((p) => ({...p, [k]: w}))}/>
      )}

      {/* Step 4 — transporte */}
      {step === 'transporte' && (
        <div className="space-y-4">
          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Trajeto</p>
            <div className="grid grid-cols-2 gap-3">
              <div className="flex flex-col gap-1">
                <Label className="text-xs font-medium text-gray-600">Onde começa? (UF)</Label>
                <OptionsSelect value={ufIni} onValueChange={setUfIniOverride} options={ufIniOptions}
                               placeholder="UF de início"/>
              </div>
              <div className="flex flex-col gap-1">
                <Label className="text-xs font-medium text-gray-600">Onde termina? (UF)</Label>
                <OptionsSelect value={ufFim} onValueChange={setUfFimOverride} options={ufFimOptions}
                               placeholder="UF de fim"/>
              </div>
            </div>
            <p className="text-xs text-gray-400">Preenchido com a origem/destino das notas. Ajuste se necessário.</p>
          </div>

          {routeNeeded && (
            <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Estados do percurso</p>
              <p className="text-xs text-gray-500">
                {ufIni} e {ufFim} não fazem fronteira — informe os estados intermediários (sugerimos abaixo).
              </p>
              {route.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {route.map((uf) => (
                    <span key={uf}
                          className="inline-flex items-center gap-1 rounded-full bg-brand-50 px-2.5 py-1 text-xs font-medium text-brand-700">
                      {uf}
                      <button type="button" onClick={() => setRouteOverride(route.filter((x) => x !== uf))}
                              className="text-brand-500 hover:text-brand-800">×</button>
                    </span>
                  ))}
                </div>
              )}
              <div className="flex items-end gap-2">
                <div className="w-32">
                  <OptionsSelect value={newRouteUf} onValueChange={setNewRouteUf} options={UF_OPTIONS}
                                 placeholder="UF"/>
                </div>
                <Button type="button" variant="outline" size="sm" onClick={addRouteUf}
                        disabled={!newRouteUf}>Adicionar</Button>
              </div>
            </div>
          )}

          <MunReorderList title="Onde será carregado?" hint="Ordene os municípios de carregamento na ordem da viagem."
                          muns={loadings} onReorder={setLoadingsOverride}/>
          <MunReorderList title="Onde será o destino?"
                          hint="Ordene os municípios de descarregamento na ordem da viagem."
                          muns={unloadings} onReorder={setUnloadingsOverride}/>

          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Início da viagem (opcional)</p>
            <input type="datetime-local" value={tripStart} onChange={(e) => setTripStart(e.target.value)}
                   className="w-full sm:w-64 h-11 rounded-md border border-gray-300 px-3 text-sm"/>
          </div>

          {needsBulk && (
            <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Carga lotação (documento
                único)</p>
              <p className="text-xs text-gray-500">Com um único documento, informe o CEP de carregamento e
                descarregamento.</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="flex flex-col gap-1">
                  <Label className="text-xs font-medium text-gray-600">CEP de carregamento</Label>
                  <NumericInput value={cepCarrega} onChange={setCepCarrega} integerPlaces={8} placeholder="00000000"
                                className="w-full"/>
                </div>
                <div className="flex flex-col gap-1">
                  <Label className="text-xs font-medium text-gray-600">CEP de descarregamento</Label>
                  <NumericInput value={cepDescarrega} onChange={setCepDescarrega} integerPlaces={8}
                                placeholder="00000000" className="w-full"/>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Step (optional) — seguro */}
      {step === 'seguro' && (
        <div className="rounded-xl border border-gray-200 bg-white p-6 text-center">
          <p className="text-sm font-medium text-gray-900">Seguro — em breve</p>
          <p className="mt-1.5 text-sm text-gray-500">
            Os dados de seguro são exigidos para MDF-e de CT-e. O suporte completo será disponibilizado em breve.
          </p>
        </div>
      )}

      {/* Step — veículo / condutor */}
      {step === 'veiculo' && (
        <div className="space-y-4">
          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Veículo (tração)</p>
              <Button type="button" size="xs" variant="outline"
                      onClick={() => setRegisterOpen(true)}>+ Cadastrar veículo</Button>
            </div>
            {tractorOptions.length > 0 ? (
              <Combobox value={vehicleSk} onValueChange={onSelectTractor} options={tractorOptions}
                        placeholder="Selecione um veículo" searchPlaceholder="Buscar placa..."/>
            ) : (
              <p className="text-sm text-gray-500">
                Nenhum veículo cadastrado. Cadastre um para continuar.
              </p>
            )}
          </div>

          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Reboques (opcional, até 3)</p>
            {trailerSks.length > 0 && (
              <div className="space-y-1.5">
                {trailerSks.map((sk) => {
                  const t = trailerOptions.find((o) => o.value === sk)
                  return (
                    <div key={sk} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-sm">
                      <span className="text-gray-700">{t?.label ?? sk}</span>
                      <Button type="button" variant="ghost" size="xs" onClick={() => removeTrailer(sk)}
                              className="text-danger hover:text-red-700">remover</Button>
                    </div>
                  )
                })}
              </div>
            )}
            {trailerSks.length < 3 && trailerOptions.length > 0 && (
              <Combobox value={null} onValueChange={(sk) => sk && onSelectTrailer(sk)}
                        options={trailerOptions.filter((o) => !trailerSks.includes(o.value))}
                        placeholder="Adicionar reboque" searchPlaceholder="Buscar placa..."/>
            )}
          </div>

          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Condutores</p>
            {drivers.length > 0 && (
              <div className="space-y-1.5">
                {drivers.map((c) => (
                  <div key={c.cpf}
                       className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-sm">
                    <span className="text-gray-700">{c.name} · <span
                      className="font-mono text-gray-400">{formatCpfCnpj(c.cpf)}</span></span>
                    <Button type="button" variant="ghost" size="xs"
                            onClick={() => setDrivers((p) => p.filter((x) => x.cpf !== c.cpf))}
                            className="text-danger hover:text-red-700">remover</Button>
                  </div>
                ))}
              </div>
            )}
            <div className="grid grid-cols-1 sm:grid-cols-[1fr_140px_auto] gap-2 items-end">
              <div className="flex flex-col gap-1">
                <Label className="text-xs font-medium text-gray-600">Nome</Label>
                <Input value={newDriverName} onChange={(e) => setNewDriverName(e.target.value)}
                       placeholder="Nome do condutor" className="w-full"/>
              </div>
              <div className="flex flex-col gap-1">
                <Label className="text-xs font-medium text-gray-600">CPF</Label>
                <Input value={maskCpf(newDriverCpf.replace(/\D/g, ''))}
                       onChange={(e) => setNewDriverCpf(e.target.value)}
                       placeholder="000.000.000-00" className="w-full"/>
              </div>
              <Button type="button" variant="brand" onClick={addDriver}
                      disabled={!newDriverName.trim() || !validateCPF(newDriverCpf.replace(/\D/g, ''))}>Adicionar</Button>
            </div>
          </div>

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
        {step !== 'veiculo' ? (
          <Button type="button" variant="brand" size="sm" onClick={goNext} disabled={!canNext(step)}>Próximo</Button>
        ) : (
          <Button type="button" variant="brand" size="sm" onClick={handleSubmit} disabled={isSubmitting || !canEmit}>
            {isSubmitting ? 'Emitindo…' : 'Emitir MDF-e'}
          </Button>
        )}
      </div>

      <VehicleRegisterModal open={registerOpen} onClose={() => setRegisterOpen(false)}
                            onSaved={(v) => {
                              setVehicleSk(v.sk);
                              setRegisterOpen(false)
                            }}/>
      <VehicleRegisterModal open={!!gateModal} onClose={() => setGateModal(null)}
                            editing={gateModal?.vehicle} missing={gateModal?.missing}
                            onSaved={() => setGateModal(null)}/>
    </div>
  )
}
