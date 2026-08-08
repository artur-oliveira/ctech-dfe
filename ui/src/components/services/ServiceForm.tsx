'use client'

import {useEffect, useMemo, useState} from 'react'
import {useRouter} from 'next/navigation'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {useQuery} from '@tanstack/react-query'
import {Form, FormDescription, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Combobox, type ComboboxOption} from '@/components/ui/combobox'
import {Button} from '@/components/ui/button'
import {type ServiceFormData, serviceSchema} from '@/lib/schemas/services'
import type {ServiceCreate, ServiceOut} from '@/lib/types/api'
import {NFSE_TRIB_NACIONAL} from '@/lib/data/nfse_trib_nacional'
import {NFSE_NBS} from '@/lib/data/nfse_nbs'
import {NFSE_COUNTRIES} from '@/lib/data/nfse_countries'
import {PIS_COFINS_OPTIONS} from '@/lib/data/pis_cofins'
import {IBS_CBS_CLASS_BY_CST, IBS_CBS_CST_OPTIONS} from '@/lib/data/ibs_cbs_cst'
import {NFSE_INDOP} from '@/lib/data/nfse_indop'
import {UNIT_OPTIONS} from '@/lib/data/unit'
import {ALL_CNAES} from '@/lib/data/cnae'
import {generateEntityCode} from '@/lib/utils/code'
import {ApiError, apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {getMunicipalTaxCodes} from '@/lib/data/municipal_tax_codes'
import {cn} from '@/lib/utils'

interface ServiceFormProps {
  initialData?: ServiceOut
  onSubmit: (data: ServiceCreate) => Promise<void>
  loading?: boolean
}

const TRIB_ISSQN_OPTIONS = [
  {value: '1', label: '1 – Operação tributável'},
  {value: '2', label: '2 – Imunidade'},
  {value: '3', label: '3 – Exportação de serviço'},
  {value: '4', label: '4 – Não incidência'},
]

const TP_RET_ISSQN_OPTIONS = [
  {value: '1', label: '1 – Não retido'},
  {value: '2', label: '2 – Retido pelo tomador'},
  {value: '3', label: '3 – Retido pelo intermediário'},
]

const TP_IMUNIDADE_OPTIONS = [
  {value: '0', label: '0 – Tipo não informado na nota de origem'},
  {value: '1', label: '1 – Imunidade recíproca entre entes públicos'},
  {value: '2', label: '2 – Templos de qualquer culto'},
  {value: '3', label: '3 – Partidos, sindicatos, educação e assistência sem fins lucrativos'},
  {value: '4', label: '4 – Livros, jornais, periódicos e papel de impressão'},
  {value: '5', label: '5 – Fonogramas e videofonogramas musicais brasileiros'},
]

// TSTipoRetPISCofins — rótulos em api/internal/api/v1/dto.go (ServiceFederalBody).
const TP_RET_PIS_COFINS_OPTIONS = [
  {value: '0', label: '0 – Nenhum retido'},
  {value: '1', label: '1 – PIS/COFINS retidos'},
  {value: '2', label: '2 – PIS/COFINS não retidos'},
  {value: '3', label: '3 – PIS/COFINS/CSLL retidos'},
  {value: '4', label: '4 – PIS/COFINS retidos, CSLL não'},
  {value: '5', label: '5 – PIS retido, COFINS/CSLL não'},
  {value: '6', label: '6 – COFINS retido, PIS/CSLL não'},
  {value: '7', label: '7 – PIS não retido, COFINS/CSLL retidos'},
  {value: '8', label: '8 – PIS/COFINS não retidos, CSLL retido'},
  {value: '9', label: '9 – COFINS não retido, PIS/CSLL retidos'},
]

const TRIB_NACIONAL_OPTIONS: ComboboxOption[] = NFSE_TRIB_NACIONAL.map((t) => ({
  value: t.code,
  label: `${t.code} – ${t.description}`,
}))

const NBS_OPTIONS: ComboboxOption[] = NFSE_NBS.map((entry) => ({
  value: entry.code,
  label: `${entry.code} – ${entry.description}`,
}))

const COUNTRY_OPTIONS: ComboboxOption[] = NFSE_COUNTRIES.map((country) => ({
  value: country.code,
  label: `${country.code} – ${country.name}`,
}))

const CNAE_CLASS_END = 4
const CNAE_CHECK_DIGIT_END = 5

function formatCnae(code: string): string {
  return `${code.slice(0, CNAE_CLASS_END)}-${code.slice(CNAE_CLASS_END, CNAE_CHECK_DIGIT_END)}/${code.slice(CNAE_CHECK_DIGIT_END)}`
}

const CNAE_OPTIONS: ComboboxOption[] = ALL_CNAES.map((entry) => ({
  value: entry.code,
  label: `${formatCnae(entry.code)} – ${entry.description}`,
}))

const IND_OP_OPTIONS: ComboboxOption[] = NFSE_INDOP.map((entry) => ({
  value: entry.code,
  label: `${entry.code} – ${entry.tipo_operacao} · ${entry.local_fornecimento}`,
}))

const IND_DEST_OPTIONS = [
  {value: '0', label: '0 – O destinatário é o tomador'},
  {value: '1', label: '1 – O destinatário é diferente do tomador'},
]

const TP_OPER_OPTIONS = [
  {value: '1', label: '1 – Fornecimento com pagamento posterior'},
  {value: '2', label: '2 – Recebimento após o fornecimento'},
  {value: '3', label: '3 – Fornecimento com pagamento já realizado'},
  {value: '4', label: '4 – Recebimento antes do fornecimento'},
  {value: '5', label: '5 – Fornecimento e recebimento concomitantes'},
]

function toFormData(s: ServiceOut): ServiceFormData {
  return {
    code: s.code,
    description: s.description,
    trib_nacional_code: s.trib_nacional_code,
    trib_municipal_code: s.trib_municipal_code ?? '',
    nbs_code: s.nbs_code ?? '',
    cnae: s.cnae ?? '',
    unit: s.unit,
    value: s.value,
    iss: {
      trib_issqn: String(s.iss.trib_issqn) as ServiceFormData['iss']['trib_issqn'],
      tax_rate: s.iss.tax_rate,
      tp_ret_issqn: s.iss.tp_ret_issqn != null ? (String(s.iss.tp_ret_issqn) as '1' | '2' | '3') : '',
      tp_imunidade: s.iss.tp_imunidade != null ? (String(s.iss.tp_imunidade) as ServiceFormData['iss']['tp_imunidade']) : '',
      c_pais_resultado: s.iss.c_pais_resultado ?? '',
    },
    federal: s.federal ? {
      cst_pis_cofins: s.federal.cst_pis_cofins ?? '',
      aliq_pis: s.federal.aliq_pis ?? '',
      aliq_cofins: s.federal.aliq_cofins ?? '',
      tp_ret_pis_cofins: s.federal.tp_ret_pis_cofins != null ? (String(s.federal.tp_ret_pis_cofins) as NonNullable<ServiceFormData['federal']>['tp_ret_pis_cofins']) : '',
      v_ret_cp: s.federal.v_ret_cp ?? '',
      v_ret_irrf: s.federal.v_ret_irrf ?? '',
      v_ret_csll: s.federal.v_ret_csll ?? '',
    } : undefined,
    ibs_cbs: {
      c_ind_op: s.ibs_cbs?.c_ind_op ?? '',
      cst: s.ibs_cbs?.cst ?? '',
      c_class_trib: s.ibs_cbs?.c_class_trib ?? '',
      ind_dest: String(s.ibs_cbs?.ind_dest ?? 0) as '0' | '1',
      tp_oper: s.ibs_cbs?.tp_oper != null ? String(s.ibs_cbs.tp_oper) as '1' | '2' | '3' | '4' | '5' : '',
      fin_nfse: '0',
    },
    tot_trib: s.tot_trib ? {
      ind_tot_trib: '0',
      p_tot_trib_sn: s.tot_trib.p_tot_trib_sn ?? '',
    } : undefined,
  }
}

function nullify(v: string | undefined): string | null | undefined {
  const normalized = v?.trim()
  return normalized === '' ? null : normalized
}

function toApiPayload(data: ServiceFormData): ServiceCreate {
  return {
    code: data.code.trim(),
    description: data.description.trim(),
    trib_nacional_code: data.trib_nacional_code,
    trib_municipal_code: nullify(data.trib_municipal_code),
    nbs_code: nullify(data.nbs_code),
    cnae: nullify(data.cnae),
    unit: data.unit,
    value: data.value,
    iss: {
      trib_issqn: Number(data.iss.trib_issqn),
      // tax_rate é required no backend: sem tributação de ISSQN vai zerada.
      tax_rate: (data.iss.trib_issqn === '1' && data.iss.tax_rate) || '0',
      tp_ret_issqn: data.iss.tp_ret_issqn ? Number(data.iss.tp_ret_issqn) : null,
      tp_imunidade: data.iss.tp_imunidade ? Number(data.iss.tp_imunidade) : null,
      c_pais_resultado: nullify(data.iss.c_pais_resultado),
    },
    federal: data.federal && (data.federal.aliq_pis || data.federal.aliq_cofins || data.federal.cst_pis_cofins
      || data.federal.tp_ret_pis_cofins) ? {
      cst_pis_cofins: nullify(data.federal.cst_pis_cofins),
      aliq_pis: nullify(data.federal.aliq_pis),
      aliq_cofins: nullify(data.federal.aliq_cofins),
      tp_ret_pis_cofins: data.federal.tp_ret_pis_cofins ? Number(data.federal.tp_ret_pis_cofins) : null,
      v_ret_cp: nullify(data.federal.v_ret_cp),
      v_ret_irrf: nullify(data.federal.v_ret_irrf),
      v_ret_csll: nullify(data.federal.v_ret_csll),
    } : undefined,
    ibs_cbs: {
      c_ind_op: data.ibs_cbs.c_ind_op,
      cst: data.ibs_cbs.cst,
      c_class_trib: data.ibs_cbs.c_class_trib,
      ind_dest: Number(data.ibs_cbs.ind_dest),
      tp_oper: data.ibs_cbs.tp_oper ? Number(data.ibs_cbs.tp_oper) : null,
      fin_nfse: 0,
    },
    tot_trib: data.tot_trib ? {
      ind_tot_trib: 0,
      p_tot_trib_sn: nullify(data.tot_trib.p_tot_trib_sn),
    } : undefined,
  }
}

export function ServiceForm({initialData, onSubmit, loading = false}: ServiceFormProps) {
  const {selectedOrg} = useAuth()
  const [showFederal, setShowFederal] = useState(!!initialData?.federal)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [defaultCode] = useState(generateEntityCode)

  const form = useForm<ServiceFormData>({
    resolver: zodResolver(serviceSchema),
    defaultValues: initialData ? toFormData(initialData) : {
      // Código é identificação interna: gerado por padrão, editável se o usuário
      // quiser o próprio (ver lib/utils/code.ts).
      code: defaultCode, description: '', trib_nacional_code: '', trib_municipal_code: '',
      nbs_code: '', cnae: '', unit: 'UN', value: '',
      iss: {trib_issqn: '1', tax_rate: '', tp_ret_issqn: '', tp_imunidade: '', c_pais_resultado: ''},
      ibs_cbs: {c_ind_op: '', cst: '', c_class_trib: '', ind_dest: '0', tp_oper: '', fin_nfse: '0'},
    },
  })

  const router = useRouter()

  useEffect(() => {
    if (!form.formState.isDirty) return
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault()
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [form.formState.isDirty])

  const trIssqn = useWatch({control: form.control, name: 'iss.trib_issqn'})
  const ibsCbsCst = useWatch({control: form.control, name: 'ibs_cbs.cst'})
  const currentMunicipalCode = useWatch({control: form.control, name: 'trib_municipal_code'})
  const currentCnae = useWatch({control: form.control, name: 'cnae'})
  const classTribOptions = IBS_CBS_CLASS_BY_CST[ibsCbsCst] ?? []

  const {data: nfseConfig, isLoading: isMunicipalityLoading} = useQuery({
    queryKey: queryKeys.nfseConfig(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getNfseConfig(selectedOrg?.pk ?? ''),
    enabled: !!selectedOrg,
    retry: false,
  })
  const municipalTaxCodes = getMunicipalTaxCodes(nfseConfig?.c_loc_emi)
  const municipalTaxOptions = useMemo<ComboboxOption[]>(() => {
    const options = municipalTaxCodes.map((entry) => ({
      value: entry.municipalCode,
      label: `${entry.municipalCode} · ${entry.nationalItem} — ${entry.description} · ${entry.taxRate}%`,
    }))
    if (currentMunicipalCode && !options.some(({value}) => value === currentMunicipalCode)) {
      options.unshift({
        value: currentMunicipalCode,
        label: `${currentMunicipalCode} — código atual (fora do catálogo municipal)`,
      })
    }
    return options
  }, [currentMunicipalCode, municipalTaxCodes])
  const cnaeOptions = useMemo<ComboboxOption[]>(() => {
    if (!currentCnae || CNAE_OPTIONS.some(({value}) => value === currentCnae)) return CNAE_OPTIONS
    return [{
      value: currentCnae,
      label: `${formatCnae(currentCnae)} — código atual (fora do catálogo CNAE)`,
    }, ...CNAE_OPTIONS]
  }, [currentCnae])

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    try {
      await onSubmit(toApiPayload(data))
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.detail : 'Não foi possível salvar o serviço. Revise os dados e tente novamente.')
    }
  })

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} className="space-y-4">
        {submitError && (
          <div role="alert" aria-live="assertive" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {submitError}
          </div>
        )}

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <h2 className="text-sm font-semibold text-gray-900">Identificação do serviço</h2>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <FormField control={form.control} name="code" render={({field}) => (
              <FormItem>
                <FormLabel>Código *</FormLabel>
                <Input {...field} id={field.name} placeholder="SVC001" maxLength={60}
                       onChange={(e) => field.onChange(e.target.value.toUpperCase())}/>
                <FormMessage/>
              </FormItem>
            )}/>
            <FormField control={form.control} name="unit" render={({field}) => (
              <FormItem>
                <FormLabel>Unidade *</FormLabel>
                <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                               options={UNIT_OPTIONS}/>
                <FormMessage/>
              </FormItem>
            )}/>
          </div>

          <FormField control={form.control} name="description" render={({field}) => (
            <FormItem>
              <FormLabel>Descrição *</FormLabel>
              <Input {...field} id={field.name} placeholder="Descrição do serviço" maxLength={2000}/>
              <FormMessage/>
            </FormItem>
          )}/>

          <FormField control={form.control} name="value" render={({field}) => (
            <FormItem>
              <FormLabel>Valor unitário *</FormLabel>
              <CurrencyInput id={field.name} value={field.value} onChange={field.onChange}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </div>

        <div className="space-y-4 rounded-xl border border-gray-200 bg-white p-5">
          <h2 className="text-sm font-semibold text-gray-900">Classificação fiscal</h2>

          <FormField control={form.control} name="trib_nacional_code" render={({field}) => (
            <FormItem>
              <FormLabel>Código de tributação nacional *</FormLabel>
              <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                        options={TRIB_NACIONAL_OPTIONS} placeholder="Buscar código nacional"/>
              <FormMessage/>
            </FormItem>
          )}/>

          <div className="grid grid-cols-1 items-start gap-3 sm:grid-cols-2">
            <FormField control={form.control} name="trib_municipal_code" render={({field}) => (
              <FormItem>
                <FormLabel>Código de tributação municipal</FormLabel>
                {municipalTaxCodes.length > 0 ? (
                  <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                            options={municipalTaxOptions} placeholder="Buscar código municipal"
                            searchPlaceholder="Código, item ou descrição..." fuzzySearch/>
                ) : (
                  <Input {...field} id={field.name} maxLength={20} disabled={isMunicipalityLoading}
                         placeholder={isMunicipalityLoading ? 'Carregando município…' : 'Informe o código'}/>
                )}
                <FormDescription>
                  {municipalTaxCodes.length > 0
                    ? 'Catálogo do município emissor configurado para a organização.'
                    : 'Informe o código adotado pelo município emissor.'}
                </FormDescription>
                <FormMessage/>
              </FormItem>
            )}/>
            <FormField control={form.control} name="nbs_code" render={({field}) => (
              <FormItem>
                <FormLabel>Código NBS</FormLabel>
                <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                          options={NBS_OPTIONS} placeholder="Buscar NBS"/>
                <FormMessage/>
              </FormItem>
            )}/>
          </div>

          <FormField control={form.control} name="cnae" render={({field}) => (
            <FormItem>
              <FormLabel>CNAE</FormLabel>
              <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                        options={cnaeOptions} placeholder="Buscar CNAE"
                        searchPlaceholder="Código ou descrição..." fuzzySearch/>
              <FormMessage/>
            </FormItem>
          )}/>

        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <h2 className="text-sm font-semibold text-gray-900">ISSQN</h2>

          <div className={cn('grid grid-cols-1 items-start gap-3', trIssqn === '1' && 'sm:grid-cols-2')}>
            <FormField control={form.control} name="iss.trib_issqn" render={({field}) => (
              <FormItem>
                <FormLabel>Tributação *</FormLabel>
                <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                               options={TRIB_ISSQN_OPTIONS}/>
                <FormMessage/>
              </FormItem>
            )}/>
            {/* Imunidade, exportação e não incidência não têm alíquota — o DPS
                vai com 0 e o campo some. */}
            {trIssqn === '1' && (
              <FormField control={form.control} name="iss.tax_rate" render={({field}) => (
                <FormItem>
                  <FormLabel>Alíquota (%) *</FormLabel>
                  <CurrencyInput id={field.name} value={field.value ?? ''} onChange={field.onChange}
                                 decimalPlaces={2} maxDecimalPlaces={4}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            )}
          </div>

          <div className={cn('grid grid-cols-1 items-start gap-3', trIssqn === '2' && 'sm:grid-cols-2')}>
            <FormField control={form.control} name="iss.tp_ret_issqn" render={({field}) => (
              <FormItem>
                <FormLabel>Retenção</FormLabel>
                <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                               options={TP_RET_ISSQN_OPTIONS} placeholder="Não informado"/>
                <FormMessage/>
              </FormItem>
            )}/>
            {trIssqn === '2' && (
              <FormField control={form.control} name="iss.tp_imunidade" render={({field}) => (
                <FormItem>
                  <FormLabel>Hipótese de imunidade</FormLabel>
                  <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                                 options={TP_IMUNIDADE_OPTIONS}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            )}
          </div>

          {trIssqn === '3' && (
            <FormField control={form.control} name="iss.c_pais_resultado" render={({field}) => (
              <FormItem>
                <FormLabel>País do resultado</FormLabel>
                <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                          options={COUNTRY_OPTIONS} placeholder="Buscar país"/>
                <FormMessage/>
              </FormItem>
            )}/>
          )}
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <div className="space-y-1">
            <h2 className="text-sm font-semibold text-gray-900">IBS e CBS</h2>
            <p className="text-sm text-gray-500">
              Classificação exigida no leiaute nacional. Confirme os códigos com a sua assessoria fiscal.
            </p>
          </div>

          <FormField control={form.control} name="ibs_cbs.c_ind_op" render={({field}) => (
            <FormItem>
              <FormLabel>Indicador da operação *</FormLabel>
              <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                        options={IND_OP_OPTIONS} placeholder="Buscar natureza e local da operação"/>
              <FormMessage/>
            </FormItem>
          )}/>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <FormField control={form.control} name="ibs_cbs.cst" render={({field}) => (
              <FormItem>
                <FormLabel>CST IBS/CBS *</FormLabel>
                <OptionsSelect id={field.name} value={field.value} onValueChange={(value) => {
                  field.onChange(value)
                  form.setValue('ibs_cbs.c_class_trib', '')
                }} options={IBS_CBS_CST_OPTIONS}/>
                <FormMessage/>
              </FormItem>
            )}/>
            <FormField control={form.control} name="ibs_cbs.c_class_trib" render={({field}) => (
              <FormItem>
                <FormLabel>Classificação tributária *</FormLabel>
                <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                          options={classTribOptions} placeholder={ibsCbsCst ? 'Selecionar classificação' : 'Selecione o CST primeiro'}
                          disabled={!ibsCbsCst}/>
                <FormMessage/>
              </FormItem>
            )}/>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <FormField control={form.control} name="ibs_cbs.ind_dest" render={({field}) => (
              <FormItem>
                <FormLabel>Destinatário *</FormLabel>
                <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                               options={IND_DEST_OPTIONS}/>
                <FormMessage/>
              </FormItem>
            )}/>
            <FormField control={form.control} name="ibs_cbs.tp_oper" render={({field}) => (
              <FormItem>
                <FormLabel>Momento da operação</FormLabel>
                <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                               options={TP_OPER_OPTIONS} placeholder="Não informado"/>
                <FormMessage/>
              </FormItem>
            )}/>
          </div>
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <button
            type="button"
            onClick={() => setShowFederal((v) => !v)}
            aria-expanded={showFederal}
            aria-controls="federal-taxes-panel"
            className="text-sm font-medium text-brand-600 hover:text-brand-700"
          >
            {showFederal ? '− Ocultar tributos federais (opcional)' : '+ Tributos federais (PIS/COFINS, opcional)'}
          </button>

          {showFederal && (
            <div id="federal-taxes-panel" className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1 border-t border-gray-100">
              <FormField control={form.control} name="federal.cst_pis_cofins" render={({field}) => (
                <FormItem>
                  <FormLabel>CST PIS/COFINS</FormLabel>
                  <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                 options={PIS_COFINS_OPTIONS} placeholder="Não informado"/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="federal.tp_ret_pis_cofins" render={({field}) => (
                <FormItem>
                  <FormLabel>Retenção PIS/COFINS/CSLL</FormLabel>
                  <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                 options={TP_RET_PIS_COFINS_OPTIONS} placeholder="Não informado"/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="federal.aliq_pis" render={({field}) => (
                <FormItem>
                  <FormLabel>Alíquota PIS (%)</FormLabel>
                  <CurrencyInput id={field.name} value={field.value ?? ''} onChange={field.onChange}
                                 decimalPlaces={2} maxDecimalPlaces={4}/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="federal.aliq_cofins" render={({field}) => (
                <FormItem>
                  <FormLabel>Alíquota COFINS (%)</FormLabel>
                  <CurrencyInput id={field.name} value={field.value ?? ''} onChange={field.onChange}
                                 decimalPlaces={2} maxDecimalPlaces={4}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            </div>
          )}
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="outline" disabled={loading} onClick={() => router.push('/services')}>
            Cancelar
          </Button>
          <Button type="submit" disabled={loading}>
            {loading ? 'Salvando…' : 'Salvar serviço'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
