'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Combobox, type ComboboxOption} from '@/components/ui/combobox'
import {Button} from '@/components/ui/button'
import {type ServiceFormData, serviceSchema} from '@/lib/schemas/services'
import type {ServiceCreate, ServiceOut} from '@/lib/types/api'
import {NFSE_TRIB_NACIONAL} from '@/lib/data/nfse_trib_nacional'
import {PIS_COFINS_OPTIONS} from '@/lib/data/pis_cofins'
import {UNIT_OPTIONS} from '@/lib/data/unit'
import {generateEntityCode} from '@/lib/utils/code'

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
  {value: '0', label: '0 – Não informado'},
  {value: '1', label: '1 – Hipótese constitucional 1'},
  {value: '2', label: '2 – Hipótese constitucional 2'},
  {value: '3', label: '3 – Hipótese constitucional 3'},
  {value: '4', label: '4 – Hipótese constitucional 4'},
  {value: '5', label: '5 – Hipótese constitucional 5'},
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
  display: t.code,
}))

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
  }
}

function nullify(v: string | undefined): string | null | undefined {
  return v === '' ? null : v
}

function toApiPayload(data: ServiceFormData): ServiceCreate {
  return {
    code: data.code,
    description: data.description,
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
  }
}

export function ServiceForm({initialData, onSubmit, loading = false}: ServiceFormProps) {
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
    },
  })

  const trIssqn = useWatch({control: form.control, name: 'iss.trib_issqn'})

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    try {
      await onSubmit(toApiPayload(data))
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar')
    }
  })

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} className="space-y-4">
        {submitError && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {submitError}
          </div>
        )}

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Identificação</p>

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

          <FormField control={form.control} name="trib_nacional_code" render={({field}) => (
            <FormItem>
              <FormLabel>Código de tributação nacional *</FormLabel>
              <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                        options={TRIB_NACIONAL_OPTIONS} placeholder="Buscar código nacional"/>
              <FormMessage/>
            </FormItem>
          )}/>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <FormField control={form.control} name="trib_municipal_code" render={({field}) => (
              <FormItem>
                <FormLabel>Código de tributação municipal</FormLabel>
                <Input {...field} id={field.name} maxLength={20}/>
                <FormMessage/>
              </FormItem>
            )}/>
            <FormField control={form.control} name="nbs_code" render={({field}) => (
              <FormItem>
                <FormLabel>Código NBS</FormLabel>
                <Input {...field} id={field.name} maxLength={9} placeholder="9 dígitos"/>
                <FormMessage/>
              </FormItem>
            )}/>
            <FormField control={form.control} name="cnae" render={({field}) => (
              <FormItem>
                <FormLabel>CNAE</FormLabel>
                <Input {...field} id={field.name}/>
                <FormMessage/>
              </FormItem>
            )}/>
          </div>

          <FormField control={form.control} name="value" render={({field}) => (
            <FormItem>
              <FormLabel>Valor unitário *</FormLabel>
              <CurrencyInput id={field.name} value={field.value} onChange={field.onChange}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">ISSQN</p>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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
                <Input {...field} id={field.name} maxLength={2} placeholder="Ex: US"
                       onChange={(e) => field.onChange(e.target.value.toUpperCase())}/>
                <FormMessage/>
              </FormItem>
            )}/>
          )}
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <button
            type="button"
            onClick={() => setShowFederal((v) => !v)}
            className="text-sm font-medium text-brand-600 hover:text-brand-700"
          >
            {showFederal ? '− Ocultar tributos federais (opcional)' : '+ Tributos federais (PIS/COFINS, opcional)'}
          </button>

          {showFederal && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1 border-t border-gray-100">
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
          <Button type="submit" disabled={loading}>
            {loading ? 'Salvando...' : 'Salvar'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
