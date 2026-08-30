'use client'

import {useEffect, useMemo, useRef, useState} from 'react'
import {useForm, useWatch, type FieldErrors} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {useInfiniteQuery, useQuery} from '@tanstack/react-query'
import {useRouter} from 'next/navigation'
import {toast} from 'sonner'
import {apiClient} from '@/lib/api/client'
import {EmitError} from '@/components/ui/emit-error'
import {emitFailure, type EmitFailure} from '@/lib/billing/notice'
import {useAuth} from '@/lib/hooks/useAuth'
import {useEmitDraft} from '@/lib/hooks/useEmitDraft'
import {queryKeys} from '@/lib/api/query-keys'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Textarea} from '@/components/ui/textarea'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Combobox, type ComboboxOption} from '@/components/ui/combobox'
import {Button} from '@/components/ui/button'
import {HomologationBanner} from '@/components/ui/homologation-banner'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import {EmitConfirmModal} from '@/components/ui/emit-confirm-modal'
import {DraftRecoveryBanner} from '@/components/ui/draft-recovery-banner'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {PersonPicker} from '@/components/persons/PersonPicker'
import {NfseServicePicker} from '@/components/nfse/NfseServicePicker'
import {type NfseEmitFormData, nfseEmitSchema} from '@/lib/schemas/nfse'
import type {NfseEmit, OrganizationOut, PersonItemOut, ServiceOut} from '@/lib/types/api'
import {NFSE_SUBSTITUTION_MOTIVES, NFSE_THIRD_PARTY_MOTIVES} from '@/lib/data/nfse_motives'
import {formatCpfCnpj, orgTaxId, personTaxId} from '@/lib/utils/document'
import {formatCurrency} from '@/lib/utils/helpers'
import {formatISODateBR} from '@/lib/utils/dfe'

const THIRD_PARTY_ISSUER_OPTIONS = [
  {value: '1', label: 'O próprio prestador'},
  {value: '2', label: 'O tomador'},
  {value: '3', label: 'O intermediário'},
]

const RETENTION_LABELS: Record<number, string> = {
  1: 'ISS não retido',
  2: 'ISS retido pelo tomador',
  3: 'ISS retido pelo intermediário',
}

const FORM_FIELD_ORDER: readonly (keyof NfseEmitFormData | `service.${keyof NfseEmitFormData['service']}`)[] = [
  'customer_id',
  'service.service_id',
  'service.value',
  'service.tax_rate',
  'competence',
  'tp_emit',
  'provider_person_id',
  'motivo_emis_ti',
  'ch_nfse_rej',
  'intermediary_id',
  'service.c_trib_mun',
  'additional_info',
  'substitutes_reason',
]

interface NfseEmitFormProps {
  mode?: 'emit' | 'substitute' | 'duplicate'
  sourceIdDps?: string
}

interface NfseDraftState {
  values: NfseEmitFormData
  provider: PersonItemOut | null
  customer: PersonItemOut | null
  intermediary: PersonItemOut | null
  service: ServiceOut | null
  moreOptionsOpen: boolean
}

function todayCompetence(): string {
  const now = new Date()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${now.getFullYear()}-${month}-${day}`
}

function nextMonthCompetence(value: string): string {
  const [year, month, day] = value.split('-').map(Number)
  if (!year || !month || !day) return todayCompetence()
  const lastDay = new Date(year, month + 1, 0).getDate()
  const next = new Date(year, month, Math.min(day, lastDay))
  return `${next.getFullYear()}-${String(next.getMonth() + 1).padStart(2, '0')}-${String(next.getDate()).padStart(2, '0')}`
}

function orgAsPerson(org: OrganizationOut): PersonItemOut {
  return {
    pk: org.pk,
    sk: org.pk,
    cpf_or_cnpj: orgTaxId(org),
    name: org.name,
    person: org.person as unknown as PersonItemOut['person'],
    created_at: org.created_at,
    updated_at: org.updated_at,
  }
}

function singleLine(value: string): string {
  return value.replace(/[\r\n]+/g, ' ')
}

function hasFieldError(errors: FieldErrors<NfseEmitFormData>, path: string): boolean {
  return path.split('.').reduce<unknown>((value, key) => {
    if (!value || typeof value !== 'object') return undefined
    return (value as Record<string, unknown>)[key]
  }, errors) != null
}

export function NfseEmitForm({mode = 'emit', sourceIdDps}: NfseEmitFormProps) {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const orgPk = selectedOrg?.pk ?? ''

  const [selectedProvider, setSelectedProvider] = useState<PersonItemOut | null>(null)
  const [selectedCustomer, setSelectedCustomer] = useState<PersonItemOut | null>(null)
  const [selectedIntermediary, setSelectedIntermediary] = useState<PersonItemOut | null>(null)
  const [selectedService, setSelectedService] = useState<ServiceOut | null>(null)
  const [moreOptionsOpen, setMoreOptionsOpen] = useState(mode === 'substitute')
  const [submitError, setSubmitError] = useState<EmitFailure | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [showEmitConfirm, setShowEmitConfirm] = useState(false)
  const appliedSourceRef = useRef<string | null>(null)

  const {config: nfseConfig} = useFiscalConfig('nfse', orgPk)

  const {data: org} = useQuery({
    queryKey: queryKeys.organizations.detail(orgPk),
    queryFn: () => apiClient.getOrganization(orgPk),
    enabled: !!orgPk,
  })

  const sourceQuery = useQuery({
    queryKey: queryKeys.nfses.detail(sourceIdDps ?? ''),
    queryFn: () => apiClient.getNfse(sourceIdDps!),
    enabled: mode !== 'emit' && !!sourceIdDps,
  })

  const sourceInput = sourceQuery.data?.emit_input
  const sourceServiceQuery = useQuery({
    queryKey: queryKeys.services.detail(sourceInput?.service.service_id ?? ''),
    queryFn: () => apiClient.getService(sourceInput!.service.service_id),
    enabled: !!sourceInput?.service.service_id,
  })
  const sourceProviderQuery = useQuery({
    queryKey: queryKeys.persons.detail(sourceInput?.provider_person_id ?? ''),
    queryFn: () => apiClient.getPerson(sourceInput!.provider_person_id!),
    enabled: !!sourceInput?.provider_person_id && sourceInput.provider_person_id !== orgPk,
  })
  const sourceCustomerQuery = useQuery({
    queryKey: queryKeys.persons.detail(sourceInput?.customer_id ?? ''),
    queryFn: () => apiClient.getPerson(sourceInput!.customer_id!),
    enabled: !!sourceInput?.customer_id && sourceInput.customer_id !== orgPk,
  })
  const sourceIntermediaryQuery = useQuery({
    queryKey: queryKeys.persons.detail(sourceInput?.intermediary_id ?? ''),
    queryFn: () => apiClient.getPerson(sourceInput!.intermediary_id!),
    enabled: !!sourceInput?.intermediary_id && sourceInput.intermediary_id !== orgPk,
  })

  const rejectedNfsesQuery = useInfiniteQuery({
    queryKey: queryKeys.nfses.list(orgPk, {status: 'rejected', limit: 100}),
    queryFn: ({pageParam}) => apiClient.getNfses({status: 'rejected', limit: 100, sort: 'desc', cursor: pageParam}),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.has_next ? (lastPage.next_cursor ?? undefined) : undefined,
    enabled: !!orgPk,
  })
  const {fetchNextPage: fetchNextRejectedPage, hasNextPage: hasNextRejectedPage,
    isFetchingNextPage: isFetchingNextRejectedPage} = rejectedNfsesQuery

  useEffect(() => {
    if (hasNextRejectedPage && !isFetchingNextRejectedPage) {
      void fetchNextRejectedPage()
    }
  }, [fetchNextRejectedPage, hasNextRejectedPage, isFetchingNextRejectedPage])

  const form = useForm<NfseEmitFormData>({
    resolver: zodResolver(nfseEmitSchema),
    mode: 'onBlur',
    defaultValues: {
      tp_emit: '1',
      competence: todayCompetence(),
      provider_person_id: '',
      customer_id: '',
      intermediary_id: '',
      service: {service_id: '', description: '', value: '', tax_rate: '', c_trib_mun: ''},
      motivo_emis_ti: '',
      ch_nfse_rej: '',
      substitutes_access_key: '',
      substitutes_reason: '',
      additional_info: '',
    },
  })

  const values = useWatch({control: form.control}) as NfseEmitFormData
  const tpEmit = values.tp_emit
  const motivoEmisTi = values.motivo_emis_ti

  useEffect(() => {
    if (mode === 'substitute' && sourceQuery.data?.access_key) {
      form.setValue('substitutes_access_key', sourceQuery.data.access_key)
    }
  }, [form, mode, sourceQuery.data])

  useEffect(() => {
    if (!sourceIdDps || !sourceQuery.data || !sourceInput || !sourceServiceQuery.data
      || appliedSourceRef.current === sourceIdDps) return

    const resolvePerson = (id: string | null | undefined, fetched: PersonItemOut | undefined) => {
      if (!id) return null
      if (id === orgPk && org) return orgAsPerson(org)
      return fetched ?? null
    }
    const provider = resolvePerson(sourceInput.provider_person_id, sourceProviderQuery.data)
    const customer = resolvePerson(sourceInput.customer_id, sourceCustomerQuery.data)
    const intermediary = resolvePerson(sourceInput.intermediary_id, sourceIntermediaryQuery.data)
    if ((sourceInput.provider_person_id && !provider) || (sourceInput.customer_id && !customer)
      || (sourceInput.intermediary_id && !intermediary)) return

    const service = sourceServiceQuery.data
    const timer = window.setTimeout(() => {
      form.reset({
        tp_emit: String(sourceInput.tp_emit) as NfseEmitFormData['tp_emit'],
        competence: mode === 'duplicate' ? nextMonthCompetence(sourceQuery.data.competence) : sourceQuery.data.competence,
        provider_person_id: sourceInput.provider_person_id ?? '',
        customer_id: sourceInput.customer_id ?? '',
        intermediary_id: sourceInput.intermediary_id ?? '',
        service: {
          service_id: sourceInput.service.service_id,
          description: sourceInput.service.description ?? service.description,
          value: sourceInput.service.value ?? service.value,
          tax_rate: sourceInput.service.tax_rate ?? service.iss.tax_rate,
          c_trib_mun: sourceInput.service.c_trib_mun ?? service.trib_municipal_code ?? '',
        },
        motivo_emis_ti: sourceInput.motivo_emis_ti
          ? String(sourceInput.motivo_emis_ti) as NfseEmitFormData['motivo_emis_ti'] : '',
        ch_nfse_rej: sourceInput.ch_nfse_rej ?? '',
        substitutes_access_key: mode === 'substitute' ? (sourceQuery.data.access_key ?? '') : '',
        substitutes_reason: '',
        additional_info: sourceInput.additional_info ?? '',
      })
      setSelectedProvider(provider)
      setSelectedCustomer(customer)
      setSelectedIntermediary(intermediary)
      setSelectedService(service)
      appliedSourceRef.current = sourceIdDps
    }, 0)
    return () => window.clearTimeout(timer)
  }, [form, mode, org, orgPk, sourceCustomerQuery.data, sourceIdDps, sourceInput,
    sourceIntermediaryQuery.data, sourceProviderQuery.data, sourceQuery.data, sourceServiceQuery.data])

  const draftState = useMemo<NfseDraftState>(() => ({
    values,
    provider: selectedProvider,
    customer: selectedCustomer,
    intermediary: selectedIntermediary,
    service: selectedService,
    moreOptionsOpen,
  }), [moreOptionsOpen, selectedCustomer, selectedIntermediary, selectedProvider, selectedService, values])

  const draft = useEmitDraft(mode === 'emit' ? 'nfse' : `nfse-${mode}`, selectedOrg?.pk, draftState,
    !!values.service?.service_id || selectedCustomer !== null || selectedProvider !== null)

  const restoreDraft = () => {
    const recovered = draft.recovered?.state
    if (recovered) {
      form.reset(recovered.values)
      setSelectedProvider(recovered.provider)
      setSelectedCustomer(recovered.customer)
      setSelectedIntermediary(recovered.intermediary)
      setSelectedService(recovered.service)
      setMoreOptionsOpen(recovered.moreOptionsOpen)
    }
    draft.accept()
  }

  const rejectedOptions: ComboboxOption[] = (rejectedNfsesQuery.data?.pages.flatMap((page) => page.items) ?? [])
    .filter((item) => item.access_key)
    .map((item) => ({
      value: item.access_key!,
      label: `NFS-e ${item.number} / ${item.serie} – ${item.dest_name ?? 'Sem tomador'} – ${formatCurrency(item.total)}`,
    }))

  const handleSelectService = (service: ServiceOut) => {
    setSelectedService(service)
    form.setValue('service.service_id', service.sk, {shouldValidate: true})
    form.setValue('service.description', service.description)
    form.setValue('service.value', service.value)
    form.setValue('service.tax_rate', service.iss.tax_rate)
    form.setValue('service.c_trib_mun', service.trib_municipal_code ?? '')
  }

  const handleClearService = () => {
    setSelectedService(null)
    form.setValue('service.service_id', '', {shouldValidate: true})
  }

  const handleProviderChange = (person: PersonItemOut | null) => {
    setSelectedProvider(person)
    form.setValue('provider_person_id', person?.sk ?? '', {shouldValidate: true})
  }

  const handleCustomerChange = (person: PersonItemOut | null) => {
    setSelectedCustomer(person)
    form.setValue('customer_id', person?.sk ?? '', {shouldValidate: true})
  }

  const handleIntermediaryChange = (person: PersonItemOut | null) => {
    setSelectedIntermediary(person)
    form.setValue('intermediary_id', person?.sk ?? '', {shouldValidate: true})
  }

  const onInvalid = (errors: FieldErrors<NfseEmitFormData>) => {
    const firstError = FORM_FIELD_ORDER.find((field) => hasFieldError(errors, field))
    setSubmitError({message: 'Revise o campo destacado antes de emitir a NFS-e.'})
    if (!firstError) return
    const isAdvancedField = !['customer_id', 'service.service_id', 'service.value', 'service.tax_rate', 'competence']
      .includes(firstError)
    if (isAdvancedField) setMoreOptionsOpen(true)
    window.setTimeout(() => document.getElementById(firstError)?.focus(), 0)
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
      draft.clear()
      toast.success(mode === 'substitute' ? 'Substituição enviada para processamento' : 'NFS-e enviada para processamento', {
        description: 'Acompanhe a autorização e o retorno do fisco na tela de detalhes.',
        action: {label: 'Emitir outra', onClick: () => router.push('/nfse/emit')},
      })
      router.push(`/nfse/detail?id=${encodeURIComponent(result.sk)}`)
    } catch (error) {
      setSubmitError(emitFailure(error, 'Não foi possível enviar a NFS-e. Revise os dados e tente novamente.'))
      setIsSubmitting(false)
    }
  }, onInvalid)

  const serviceValue = values.service?.value ?? ''
  const taxRate = values.service?.tax_rate ?? ''
  const issValue = ((Number(serviceValue) || 0) * (Number(taxRate) || 0) / 100).toFixed(2)
  const retention = selectedService?.iss.tp_ret_issqn
    ? (RETENTION_LABELS[selectedService.iss.tp_ret_issqn] ?? 'Retenção não informada')
    : 'Retenção não informada'

  if (!selectedOrg) {
    return <div className="py-12 text-center text-sm text-gray-500">Selecione uma organização para emitir NFS-e.</div>
  }

  if (nfseConfig?.provider === 'abrasf204') {
    return (
      <div role="alert" className="max-w-3xl rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
        A emissão por ABRASF 2.04 ainda não está disponível aqui. Em Configuração Fiscal, selecione o provedor
        Nacional (ADN) para emitir NFS-e.
      </div>
    )
  }

  const sourceDependenciesLoading = sourceServiceQuery.isLoading || sourceProviderQuery.isLoading
    || sourceCustomerQuery.isLoading || sourceIntermediaryQuery.isLoading

  if (mode !== 'emit' && (sourceQuery.isLoading || sourceDependenciesLoading)) {
    return <LoadingSkeleton count={2} height="h-32" rounded="rounded-xl"/>
  }

  const sourceDependencyError = sourceServiceQuery.isError || sourceProviderQuery.isError
    || sourceCustomerQuery.isError || sourceIntermediaryQuery.isError
  const sourceUnavailable = mode !== 'emit' && (sourceQuery.isError || !sourceInput || sourceDependencyError
    || (mode === 'substitute' && !sourceQuery.data?.access_key))

  return (
    <Form {...form}>
      <form onSubmit={(event) => event.preventDefault()} className="max-w-4xl space-y-4 pb-4">
        <HomologationBanner environment={nfseConfig?.environment}/>

        {draft.recovered && mode === 'emit' && (
          <DraftRecoveryBanner savedAt={draft.recovered.savedAt} onRestore={restoreDraft} onDiscard={draft.discard}/>
        )}

        {mode === 'substitute' && sourceQuery.data && (
          <div className="rounded-lg border border-brand-200 bg-brand-50 px-4 py-3 text-sm text-brand-800">
            Você está substituindo a NFS-e nº {sourceQuery.data.number}. A nota original será cancelada pelo fisco
            somente quando a substituta for autorizada.
          </div>
        )}

        {mode === 'duplicate' && sourceQuery.data && !sourceUnavailable && (
          <div className="rounded-lg border border-brand-200 bg-brand-50 px-4 py-3 text-sm text-brand-800">
            Cópia da NFS-e nº {sourceQuery.data.number}. Confira os dados; a competência avançou um mês.
          </div>
        )}

        {sourceUnavailable && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {mode === 'duplicate'
              ? 'Esta NFS-e não possui as referências de catálogo necessárias para uma cópia segura.'
              : 'Não foi possível carregar a NFS-e original e suas referências. Volte aos detalhes e tente novamente.'}
          </div>
        )}

        <section aria-labelledby="nfse-essential-title" className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
          <div className="mb-5">
            <h2 id="nfse-essential-title" className="text-lg font-semibold text-gray-900">Dados da NFS-e</h2>
            <p className="mt-0.5 text-sm text-gray-500">Escolha o tomador e o serviço; os valores do catálogo podem ser ajustados nesta emissão.</p>
          </div>

          <div className="space-y-5">
            <div>
              <div className="mb-2 flex items-center justify-between gap-3">
                <label className="text-sm font-medium text-gray-700">Tomador</label>
                {!selectedCustomer && org && (
                  <Button type="button" variant="ghost" size="xs" className="text-brand-700"
                          onClick={() => handleCustomerChange(orgAsPerson(org))}>
                    Usar a própria empresa
                  </Button>
                )}
              </div>
              <PersonPicker value={selectedCustomer} onChange={handleCustomerChange} role="customer" autoFocus/>
            </div>

            <FormField control={form.control} name="service.service_id" render={() => (
              <FormItem>
                <FormLabel>Serviço</FormLabel>
                <NfseServicePicker id="service.service_id" value={values.service?.service_id}
                                   onSelect={handleSelectService} onClear={handleClearService}/>
                <FormMessage/>
              </FormItem>
            )}/>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
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
              <FormField control={form.control} name="competence" render={({field}) => (
                <FormItem className="sm:col-span-2">
                  <FormLabel>Data de competência</FormLabel>
                  <Input {...field} id={field.name} type="date" className="max-w-64"/>
                  <FormMessage/>
                </FormItem>
              )}/>
            </div>
          </div>
        </section>

        <details open={moreOptionsOpen} onToggle={(event) => setMoreOptionsOpen(event.currentTarget.open)}
                 className="rounded-xl border border-gray-200 bg-white">
          <summary className="flex min-h-11 cursor-pointer items-center px-4 py-3 text-sm font-medium text-brand-700 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 md:px-6">
            Mais opções
          </summary>
          <div className="space-y-5 border-t border-gray-100 p-4 md:p-6">
            <FormField control={form.control} name="tp_emit" render={({field}) => (
              <FormItem>
                <FormLabel>Responsável pela emissão</FormLabel>
                <OptionsSelect id={field.name} value={field.value} onValueChange={(value) => {
                  field.onChange(value)
                  if (value === '1') handleProviderChange(null)
                }} options={THIRD_PARTY_ISSUER_OPTIONS}/>
                <FormMessage/>
              </FormItem>
            )}/>

            {tpEmit !== '1' && (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <FormField control={form.control} name="provider_person_id" render={() => (
                  <FormItem>
                    <FormLabel>Prestador do serviço</FormLabel>
                    <PersonPicker value={selectedProvider} onChange={handleProviderChange} role="provider"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="motivo_emis_ti" render={({field}) => (
                  <FormItem>
                    <FormLabel>Motivo da emissão por terceiro</FormLabel>
                    <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                                   options={[...NFSE_THIRD_PARTY_MOTIVES]} placeholder="Selecione o motivo"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
            )}

            {tpEmit !== '1' && motivoEmisTi === '4' && (
              <FormField control={form.control} name="ch_nfse_rej" render={({field}) => (
                <FormItem>
                  <FormLabel>NFS-e rejeitada pelo prestador</FormLabel>
                  <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                            options={rejectedOptions} placeholder="Selecione uma NFS-e rejeitada desta organização"/>
                  <FormMessage/>
                </FormItem>
              )}/>
            )}

            <FormField control={form.control} name="intermediary_id" render={() => (
              <FormItem>
                <FormLabel>Intermediário</FormLabel>
                <PersonPicker value={selectedIntermediary} onChange={handleIntermediaryChange}/>
                <FormMessage/>
              </FormItem>
            )}/>

            <FormField control={form.control} name="service.description" render={({field}) => (
              <FormItem>
                <FormLabel>Descrição do serviço</FormLabel>
                <Textarea {...field} id={field.name} maxLength={2000} rows={3}
                          onChange={(event) => field.onChange(singleLine(event.target.value))}/>
                <FormMessage/>
              </FormItem>
            )}/>

            {values.service?.c_trib_mun && (
              <div className="rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-700">
                Código de tributação municipal herdado do serviço: <span className="font-mono font-medium">{values.service.c_trib_mun}</span>
              </div>
            )}

            <FormField control={form.control} name="additional_info" render={({field}) => (
              <FormItem>
                <FormLabel>Informações complementares</FormLabel>
                <Textarea {...field} id={field.name} maxLength={2000} rows={3}
                          onChange={(event) => field.onChange(singleLine(event.target.value))}/>
                <FormMessage/>
              </FormItem>
            )}/>

            {mode === 'substitute' && (
              <FormField control={form.control} name="substitutes_reason" render={({field}) => (
                <FormItem>
                  <FormLabel>Motivo da substituição</FormLabel>
                  <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                                 options={[...NFSE_SUBSTITUTION_MOTIVES]} placeholder="Selecione o motivo"/>
                  <FormMessage/>
                </FormItem>
              )}/>
            )}
          </div>
        </details>

        <section aria-labelledby="nfse-preview-title" className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
          <div className="mb-4 flex items-start justify-between gap-4">
            <div>
              <h2 id="nfse-preview-title" className="text-lg font-semibold text-gray-900">Prévia da DPS</h2>
              <p className="mt-0.5 text-sm text-gray-500">Resumo que será enviado para autorização.</p>
            </div>
            {nfseConfig?.serie && <span className="text-xs text-gray-500">Série {nfseConfig.serie}</span>}
          </div>
          <dl className="divide-y divide-gray-100 text-sm">
            <div className="grid gap-1 py-2 sm:grid-cols-[10rem_1fr]"><dt className="text-gray-500">Tomador</dt><dd className="font-medium text-gray-900">{selectedCustomer ? `${selectedCustomer.name} · ${formatCpfCnpj(personTaxId(selectedCustomer))}` : '—'}</dd></div>
            <div className="grid gap-1 py-2 sm:grid-cols-[10rem_1fr]"><dt className="text-gray-500">Serviço</dt><dd className="font-medium text-gray-900 wrap-break-word">{selectedService?.description ?? values.service?.description ?? '—'}</dd></div>
            <div className="grid gap-1 py-2 sm:grid-cols-[10rem_1fr]"><dt className="text-gray-500">Tributação nacional</dt><dd className="font-mono text-gray-900">{selectedService?.trib_nacional_code ?? '—'}</dd></div>
            <div className="grid grid-cols-2 gap-4 py-2 sm:grid-cols-4">
              <div><dt className="text-gray-500">Valor</dt><dd className="mt-0.5 font-medium text-gray-900">{serviceValue ? formatCurrency(serviceValue) : '—'}</dd></div>
              <div><dt className="text-gray-500">Alíquota</dt><dd className="mt-0.5 font-medium text-gray-900">{taxRate ? `${taxRate}%` : '—'}</dd></div>
              <div><dt className="text-gray-500">ISS calculado</dt><dd className="mt-0.5 font-medium text-gray-900">{serviceValue && taxRate ? formatCurrency(issValue) : '—'}</dd></div>
              <div><dt className="text-gray-500">Competência</dt><dd className="mt-0.5 font-medium text-gray-900">{formatISODateBR(values.competence ?? '')}</dd></div>
            </div>
            <div className="grid gap-1 py-2 sm:grid-cols-[10rem_1fr]"><dt className="text-gray-500">Retenção</dt><dd className="font-medium text-gray-900">{retention}</dd></div>
          </dl>
        </section>

        <EmitError failure={submitError}/>

        <div className="sticky bottom-0 -mx-4 flex flex-col gap-2 border-t border-gray-200 bg-gray-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between md:-mx-8 md:px-8">
          <p className="text-sm text-gray-600">{serviceValue ? `Total ${formatCurrency(serviceValue)}` : 'Selecione um serviço para emitir'}</p>
          <Button type="button" variant="brand" disabled={isSubmitting || sourceUnavailable}
                  onClick={() => setShowEmitConfirm(true)}>
            {isSubmitting ? 'Enviando…' : mode === 'substitute' ? 'Substituir NFS-e'
              : mode === 'duplicate' ? 'Emitir cópia' : 'Emitir NFS-e'}
          </Button>
        </div>

        <EmitConfirmModal
          open={showEmitConfirm}
          onClose={() => setShowEmitConfirm(false)}
          onConfirm={() => {
            setShowEmitConfirm(false)
            void submit()
          }}
          docLabel="NFS-e"
          summary={[]}
        />
      </form>
    </Form>
  )
}
