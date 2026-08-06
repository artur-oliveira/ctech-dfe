'use client'

import {useEffect, useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {useQuery} from '@tanstack/react-query'
import {useRouter} from 'next/navigation'
import {toast} from 'sonner'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Textarea} from '@/components/ui/textarea'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {EmitConfirmModal} from '@/components/ui/emit-confirm-modal'
import {StepIndicator} from '@/components/ui/step-indicator'
import {NfsePersonSearch} from '@/components/nfse/NfsePersonSearch'
import {NfseServicePicker} from '@/components/nfse/NfseServicePicker'
import {type NfseEmitFormData, nfseEmitSchema} from '@/lib/schemas/nfse'
import type {NfseEmit, PersonItemOut, ServiceOut} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'

// ─── Steps ────────────────────────────────────────────────────────────────────

export const NFSE_STEPS = [
  {id: 'prestador', label: 'Prestador'},
  {id: 'tomador', label: 'Tomador'},
  {id: 'servico', label: 'Serviço'},
  {id: 'valores', label: 'Valores'},
  {id: 'revisao', label: 'Revisão'},
] as const satisfies readonly { id: string; label: string }[]

export type NfseStep = typeof NFSE_STEPS[number]['id']
const STEP_IDS = NFSE_STEPS.map((s) => s.id)

function todayCompetence(): string {
  const now = new Date()
  return `01/${String(now.getMonth() + 1).padStart(2, '0')}/${now.getFullYear()}`
}

const MOTIVO_EMIS_TI_OPTIONS = [
  {value: '1', label: '1'},
  {value: '2', label: '2'},
  {value: '3', label: '3'},
  {value: '4', label: '4 – DPS rejeitada pelo prestador'},
]

interface NfseEmitFormProps {
  mode?: 'emit' | 'substitute'
  sourceIdDps?: string
}

export function NfseEmitForm({mode = 'emit', sourceIdDps}: NfseEmitFormProps) {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const pk = selectedOrg?.pk ?? ''

  const [currentStep, setCurrentStep] = useState<NfseStep>('prestador')
  const [selectedProvider, setSelectedProvider] = useState<PersonItemOut | null>(null)
  const [selectedCustomer, setSelectedCustomer] = useState<PersonItemOut | null>(null)
  const [selectedIntermediary, setSelectedIntermediary] = useState<PersonItemOut | null>(null)
  const [selectedService, setSelectedService] = useState<ServiceOut | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [showEmitConfirm, setShowEmitConfirm] = useState(false)

  const {data: nfseConfig} = useQuery({
    queryKey: queryKeys.nfseConfig(pk),
    queryFn: () => apiClient.getNfseConfig(pk),
    enabled: !!pk,
  })

  const {data: org} = useQuery({
    queryKey: queryKeys.organizations.detail(pk),
    queryFn: () => apiClient.getOrganization(pk),
    enabled: !!pk,
  })

  const {data: sourceNfse} = useQuery({
    queryKey: queryKeys.nfses.detail(sourceIdDps ?? ''),
    queryFn: () => apiClient.getNfse(sourceIdDps!),
    enabled: mode === 'substitute' && !!sourceIdDps,
  })

  const form = useForm<NfseEmitFormData>({
    resolver: zodResolver(nfseEmitSchema),
    mode: 'onBlur',
    defaultValues: {
      tp_emit: '1',
      competence: todayCompetence(),
      service: {service_id: ''},
    },
  })

  useEffect(() => {
    if (mode === 'substitute' && sourceNfse?.access_key) {
      form.setValue('substitutes_access_key', sourceNfse.access_key)
    }
  }, [mode, sourceNfse, form])

  const tpEmit = useWatch({control: form.control, name: 'tp_emit'})
  const motivoEmisTi = useWatch({control: form.control, name: 'motivo_emis_ti'})
  const serviceIdWatch = useWatch({control: form.control, name: 'service.service_id'})
  const competenceWatch = useWatch({control: form.control, name: 'competence'})
  const serviceDescriptionWatch = useWatch({control: form.control, name: 'service.description'})
  const serviceValueWatch = useWatch({control: form.control, name: 'service.value'})

  function canGoNext(step: NfseStep): boolean {
    if (step === 'prestador') {
      if (tpEmit !== '1' && (!selectedProvider || !form.getValues('motivo_emis_ti'))) return false
      return !!form.getValues('competence')
    }
    if (step === 'servico') return !!form.getValues('service.service_id')
    if (step === 'valores') return !!form.getValues('service.value')
    if (step === 'revisao' && mode === 'substitute') return !!form.getValues('substitutes_reason')
    return true
  }

  function handleNext() {
    const i = STEP_IDS.indexOf(currentStep)
    if (i < STEP_IDS.length - 1) setCurrentStep(STEP_IDS[i + 1])
  }

  function handleBack() {
    const i = STEP_IDS.indexOf(currentStep)
    if (i > 0) setCurrentStep(STEP_IDS[i - 1])
  }

  const handleSelectService = (service: ServiceOut) => {
    setSelectedService(service)
    // ServiceRepository.Get aceita a SK completa (buildServiceSK é idempotente
    // no prefixo SERVICE_) — mesma convenção de minimalInput() em document_test.go.
    form.setValue('service.service_id', service.sk)
    form.setValue('service.description', service.description)
    form.setValue('service.value', service.value)
    form.setValue('service.tax_rate', service.iss.tax_rate)
  }

  const handleClearService = () => {
    setSelectedService(null)
    form.setValue('service.service_id', '')
    form.setValue('service.description', '')
    form.setValue('service.value', '')
    form.setValue('service.tax_rate', '')
  }

  const submit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    const payload: NfseEmit = {
      tp_emit: Number(data.tp_emit) as 1 | 2 | 3,
      motivo_emis_ti: data.motivo_emis_ti ? (Number(data.motivo_emis_ti) as 1 | 2 | 3 | 4) : undefined,
      ch_nfse_rej: data.ch_nfse_rej || undefined,
      competence: data.competence,
      provider_person_id: data.tp_emit !== '1' ? (selectedProvider?.sk ?? null) : null,
      customer_id: selectedCustomer?.sk ?? null,
      intermediary_id: selectedIntermediary?.sk ?? null,
      service: {
        service_id: data.service.service_id,
        description: data.service.description || undefined,
        value: data.service.value || undefined,
        tax_rate: data.service.tax_rate || undefined,
        c_trib_mun: data.service.c_trib_mun || undefined,
      },
      substitutes_access_key: mode === 'substitute' ? data.substitutes_access_key : undefined,
      substitutes_reason: mode === 'substitute' ? data.substitutes_reason : undefined,
      additional_info: data.additional_info || undefined,
    }

    setIsSubmitting(true)
    try {
      const result = mode === 'substitute' && sourceIdDps
        ? await apiClient.substituteNfse(sourceIdDps, payload)
        : await apiClient.emitNfse(payload)
      toast.success('NFS-e enviada para processamento', {description: `id_dps ${result.sk}`})
      router.push(`/nfse/detail?id=${encodeURIComponent(result.sk)}`)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao emitir NFS-e.')
      setIsSubmitting(false)
    }
  })

  if (!selectedOrg) {
    return <div className="text-center py-12 text-sm text-gray-500">Selecione uma organização para emitir NFS-e.</div>
  }

  if (nfseConfig?.provider === 'abrasf204') {
    return (
      <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 max-w-3xl">
        Emissão por ABRASF 2.04 ainda não é suportada pelo front. Configure o provedor Nacional (ADN) em
        Configuração Fiscal para emitir NFS-e.
      </div>
    )
  }

  return (
    <Form {...form}>
      <div className="max-w-3xl space-y-0 pb-4">
        <HomologationBanner environment={nfseConfig?.environment}/>

        {mode === 'substitute' && sourceNfse && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 mb-4 text-sm text-amber-800">
            Substituindo a NFS-e nº {sourceNfse.number} ({sourceNfse.access_key ?? sourceNfse.sk}). O fisco cancela a
            nota original automaticamente ao autorizar esta.
          </div>
        )}

        <StepIndicator current={currentStep} steps={NFSE_STEPS}/>

        {submitError && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 mb-4">
            {submitError}
          </div>
        )}

        {currentStep === 'prestador' && (
          <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4">
            <FormField control={form.control} name="tp_emit" render={({field}) => (
              <FormItem>
                <FormLabel>Quem emite</FormLabel>
                <OptionsSelect
                  id={field.name} value={field.value} onValueChange={field.onChange}
                  options={[
                    {value: '1', label: '1 – O próprio prestador'},
                    {value: '2', label: '2 – O tomador'},
                    {value: '3', label: '3 – O intermediário'},
                  ]}
                />
                <FormMessage/>
              </FormItem>
            )}/>

            {tpEmit === '1' ? (
              <div className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3">
                <p className="text-sm font-medium text-gray-900">{org?.name}</p>
                <p className="text-xs text-gray-500 font-mono mt-0.5">
                  {org?.pk ? formatCpfCnpj(org.pk) : ''}
                </p>
              </div>
            ) : (
              <>
                <div>
                  <p className="text-sm font-medium text-gray-700 mb-2">Prestador do serviço</p>
                  <NfsePersonSearch value={selectedProvider} onChange={setSelectedProvider}/>
                </div>
                <FormField control={form.control} name="motivo_emis_ti" render={({field}) => (
                  <FormItem>
                    <FormLabel>Motivo da emissão por terceiro</FormLabel>
                    <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                                   options={MOTIVO_EMIS_TI_OPTIONS} placeholder="Consulte o manual do contribuinte"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                {motivoEmisTi === '4' && (
                  <FormField control={form.control} name="ch_nfse_rej" render={({field}) => (
                    <FormItem>
                      <FormLabel>Chave da NFS-e rejeitada</FormLabel>
                      <Input {...field} id={field.name} maxLength={50} className="font-mono text-xs"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                )}
              </>
            )}

            <FormField control={form.control} name="competence" render={({field}) => (
              <FormItem>
                <FormLabel>Competência</FormLabel>
                <Input {...field} id={field.name} placeholder="DD/MM/AAAA" className="max-w-40"/>
                <FormMessage/>
              </FormItem>
            )}/>

            {nfseConfig && (
              <p className="text-xs text-gray-400">Série {nfseConfig.serie} · {nfseConfig.environment === 1 ? 'Produção' : 'Homologação'}</p>
            )}
          </div>
        )}

        {currentStep === 'tomador' && (
          <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4">
            <div>
              <p className="text-sm font-medium text-gray-700 mb-2">Tomador (opcional)</p>
              <NfsePersonSearch value={selectedCustomer} onChange={setSelectedCustomer}/>
            </div>
            <div className="pt-2 border-t border-gray-100">
              <p className="text-sm font-medium text-gray-700 mb-2">Intermediário (opcional)</p>
              <NfsePersonSearch value={selectedIntermediary} onChange={setSelectedIntermediary}/>
            </div>
          </div>
        )}

        {currentStep === 'servico' && (
          <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4">
            <div>
              <p className="text-sm font-medium text-gray-700 mb-2">Serviço do catálogo</p>
              <NfseServicePicker
                value={serviceIdWatch}
                onSelect={handleSelectService}
                onClear={handleClearService}
              />
              {selectedService && (
                <p className="text-xs text-gray-400 mt-1">
                  Cód. tributação nacional {selectedService.trib_nacional_code}
                </p>
              )}
            </div>

            <FormField control={form.control} name="service.description" render={({field}) => (
              <FormItem>
                <FormLabel>Descrição do serviço</FormLabel>
                <Textarea {...field} id={field.name} maxLength={2000} rows={3}/>
                <FormMessage/>
              </FormItem>
            )}/>

            <FormField control={form.control} name="service.c_trib_mun" render={({field}) => (
              <FormItem>
                <FormLabel>Código de tributação municipal (opcional)</FormLabel>
                <Input {...field} id={field.name} maxLength={20}/>
                <FormMessage/>
              </FormItem>
            )}/>
          </div>
        )}

        {currentStep === 'valores' && (
          <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4">
            <FormField control={form.control} name="service.value" render={({field}) => (
              <FormItem>
                <FormLabel>Valor do serviço</FormLabel>
                <CurrencyInput id={field.name} value={field.value ?? ''} onChange={field.onChange}/>
                <FormMessage/>
              </FormItem>
            )}/>
            <FormField control={form.control} name="service.tax_rate" render={({field}) => (
              <FormItem>
                <FormLabel>Alíquota ISS (%)</FormLabel>
                <CurrencyInput id={field.name} value={field.value ?? ''} onChange={field.onChange}
                               decimalPlaces={2} maxDecimalPlaces={4}/>
                <FormMessage/>
              </FormItem>
            )}/>
            <FormField control={form.control} name="additional_info" render={({field}) => (
              <FormItem>
                <FormLabel>Informações complementares</FormLabel>
                <Textarea {...field} id={field.name} maxLength={2000} rows={3}/>
                <FormMessage/>
              </FormItem>
            )}/>
          </div>
        )}

        {currentStep === 'revisao' && (
          <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6 space-y-4">
            <dl className="divide-y divide-gray-100">
              <div className="flex items-center justify-between py-2 text-sm">
                <dt className="text-gray-500">Prestador</dt>
                <dd className="font-medium text-gray-900">{tpEmit === '1' ? org?.name : (selectedProvider?.name ?? '—')}</dd>
              </div>
              <div className="flex items-center justify-between py-2 text-sm">
                <dt className="text-gray-500">Competência</dt>
                <dd className="font-medium text-gray-900">{competenceWatch}</dd>
              </div>
              <div className="flex items-center justify-between py-2 text-sm">
                <dt className="text-gray-500">Tomador</dt>
                <dd className="font-medium text-gray-900">{selectedCustomer?.name ?? '—'}</dd>
              </div>
              <div className="flex items-center justify-between py-2 text-sm">
                <dt className="text-gray-500">Serviço</dt>
                <dd className="font-medium text-gray-900">{selectedService?.description ?? serviceDescriptionWatch ?? '—'}</dd>
              </div>
              <div className="flex items-center justify-between py-2 text-sm">
                <dt className="text-gray-500">Valor</dt>
                <dd className="font-medium text-gray-900">R$ {serviceValueWatch}</dd>
              </div>
            </dl>

            {mode === 'substitute' && (
              <FormField control={form.control} name="substitutes_reason" render={({field}) => (
                <FormItem>
                  <FormLabel>Motivo da substituição</FormLabel>
                  <Input {...field} id={field.name} maxLength={2}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            )}
          </div>
        )}

        <div className="sticky bottom-0 bg-gray-50 border-t border-gray-200 -mx-4 md:-mx-8 px-4 md:px-8 py-3 md:py-4 flex items-center justify-end gap-2">
          {currentStep !== 'prestador' && (
            <Button type="button" variant="outline" size="sm" onClick={handleBack}>← Voltar</Button>
          )}
          {currentStep !== 'revisao' ? (
            <Button type="button" variant="brand" size="sm" disabled={!canGoNext(currentStep)} onClick={handleNext}>
              Próximo →
            </Button>
          ) : (
            <Button type="button" variant="brand" size="sm" disabled={isSubmitting || !canGoNext('revisao')}
                    onClick={() => setShowEmitConfirm(true)}>
              {isSubmitting ? 'Emitindo...' : mode === 'substitute' ? 'Substituir NFS-e' : 'Emitir NFS-e'}
            </Button>
          )}
        </div>

        <EmitConfirmModal
          open={showEmitConfirm}
          onClose={() => setShowEmitConfirm(false)}
          onConfirm={() => {
            setShowEmitConfirm(false)
            void submit()
          }}
          docLabel="NFS-e"
          summary={[
            {label: 'Tomador', value: selectedCustomer?.name ?? '—'},
            {label: 'Serviço', value: selectedService?.description ?? serviceDescriptionWatch ?? '—'},
            {label: 'Valor', value: `R$ ${serviceValueWatch}`},
          ]}
        />
      </div>
    </Form>
  )
}
