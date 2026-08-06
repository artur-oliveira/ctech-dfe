'use client'

import {useEffect, useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Input} from '@/components/ui/input'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Combobox} from '@/components/ui/combobox'
import {Button} from '@/components/ui/button'
import {formatDatetimeBR} from '@/lib/utils/dfe'
import {type NfseConfigFormData, nfseConfigSchema} from '@/lib/schemas/fiscal-configs'
import type {NfseConfigOut} from '@/lib/types/api'
import {CITY_OPTIONS} from '@/lib/data/cities'

interface NfseConfigFormProps {
  initialData: NfseConfigOut | null | undefined
  onSave: (data: Record<string, unknown>) => Promise<void>
  loading?: boolean
}

function toFormValues(cfg: NfseConfigOut | null | undefined): NfseConfigFormData {
  return {
    provider: cfg?.provider ?? 'nacional',
    environment: String(cfg?.environment ?? 2) as '1' | '2',
    c_loc_emi: cfg?.c_loc_emi ?? '',
    serie: cfg?.serie ?? '1',
    prod_current_number: String(cfg?.prod_current_number ?? 0),
    hom_current_number: String(cfg?.hom_current_number ?? 0),
    certificate_sk: cfg?.certificate_sk ?? '',
    abrasf_endpoint_url: cfg?.abrasf?.endpoint_url ?? '',
    abrasf_wsdl_version: cfg?.abrasf?.wsdl_version ?? '',
    abrasf_municipality_code: cfg?.abrasf?.municipality_code ?? '',
    abrasf_synchronous: cfg?.abrasf?.synchronous ?? false,
  }
}

function toApiPayload(d: NfseConfigFormData): Record<string, unknown> {
  return {
    provider: d.provider,
    environment: parseInt(d.environment, 10),
    c_loc_emi: d.c_loc_emi,
    serie: d.serie,
    prod_current_number: parseInt(d.prod_current_number, 10),
    hom_current_number: parseInt(d.hom_current_number, 10),
    certificate_sk: d.certificate_sk || null,
    abrasf: d.provider === 'abrasf204' ? {
      endpoint_url: d.abrasf_endpoint_url,
      wsdl_version: d.abrasf_wsdl_version,
      municipality_code: d.abrasf_municipality_code,
      synchronous: !!d.abrasf_synchronous,
    } : null,
  }
}

export function NfseConfigForm({initialData, onSave, loading = false}: NfseConfigFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null)
  const savedAt = lastSavedAt ?? (initialData?.updated_at
    ? new Date(initialData.updated_at).toLocaleString('pt-BR')
    : null)

  const form = useForm<NfseConfigFormData>({
    resolver: zodResolver(nfseConfigSchema),
    defaultValues: toFormValues(initialData),
  })

  useEffect(() => {
    form.reset(toFormValues(initialData))
  }, [form, initialData])

  const provider = useWatch({control: form.control, name: 'provider'})
  const environment = useWatch({control: form.control, name: 'environment'})
  const isAbrasf = provider === 'abrasf204'
  const isProd = environment === '1'
  const activeNsu = isProd ? initialData?.prod_nsu : initialData?.hom_nsu
  const activeLastAt = isProd ? initialData?.prod_last_dist_nsu_at : initialData?.hom_last_dist_nsu_at

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    try {
      await onSave(toApiPayload(data))
      setLastSavedAt(new Date().toLocaleString('pt-BR'))
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar')
    }
  })

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} className="space-y-6">
        {submitError && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {submitError}
          </div>
        )}

        {initialData && activeNsu !== undefined && (
          <div className="rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-600 space-y-0.5">
            <div>
              <span className="font-medium">Último NSU:</span>{' '}
              {activeNsu.toLocaleString('pt-BR')}
              <span className="ml-2 text-xs text-gray-400">(atualizado automaticamente pela distribuição)</span>
            </div>
            {activeLastAt && (
              <div className="text-xs text-gray-400">
                Última consulta: {formatDatetimeBR(activeLastAt)}
              </div>
            )}
          </div>
        )}

        <FormField control={form.control} name="provider" render={({field}) => (
          <FormItem>
            <FormLabel>Provedor</FormLabel>
            <OptionsSelect
              id={field.name}
              value={field.value}
              onValueChange={field.onChange}
              options={[
                {value: 'nacional', label: 'Nacional (ADN)'},
                {value: 'abrasf204', label: 'ABRASF 2.04 (municipal)'},
              ]}
              className="max-w-96"
            />
            <FormMessage/>
          </FormItem>
        )}/>

        {isAbrasf && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            O módulo web ainda não emite DANFSE nem consulta parâmetros municipais para ABRASF 2.04 — essas ações
            ficam desabilitadas nas telas de NFS-e enquanto este provedor estiver ativo.
          </div>
        )}

        <FormField control={form.control} name="environment" render={({field}) => (
          <FormItem>
            <FormLabel>Ambiente ativo</FormLabel>
            <OptionsSelect
              id={field.name}
              value={field.value}
              onValueChange={field.onChange}
              options={[
                {value: '1', label: '1 – Produção'},
                {value: '2', label: '2 – Homologação'},
              ]}
              className="max-w-64"
            />
            <FormMessage/>
          </FormItem>
        )}/>

        <FormField control={form.control} name="c_loc_emi" render={({field}) => (
          <FormItem>
            <FormLabel>Município emissor</FormLabel>
            <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                      options={CITY_OPTIONS} placeholder="Buscar município"/>
            <FormMessage/>
          </FormItem>
        )}/>

        {/* NFS-e tem uma única série para os dois ambientes — não é prod/hom split. */}
        <FormField control={form.control} name="serie" render={({field}) => (
          <FormItem>
            <FormLabel>Série</FormLabel>
            <NumericInput {...field} integerPlaces={5} placeholder="1" onChange={field.onChange} className="max-w-40"/>
            <FormMessage/>
          </FormItem>
        )}/>

        <div className="grid grid-cols-2 gap-6">
          <section className="space-y-3">
            <p className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold uppercase tracking-wider text-emerald-700">
              Produção
            </p>
            <FormField control={form.control} name="prod_current_number" render={({field}) => (
              <FormItem>
                <FormLabel>Número atual a ser emitido</FormLabel>
                <NumericInput {...field} integerPlaces={9} placeholder="0" onChange={field.onChange}/>
                <FormMessage/>
              </FormItem>
            )}/>
          </section>

          <section className="space-y-3">
            <p className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold uppercase tracking-wider text-blue-700">
              Homologação
            </p>
            <FormField control={form.control} name="hom_current_number" render={({field}) => (
              <FormItem>
                <FormLabel>Número atual a ser emitido</FormLabel>
                <NumericInput {...field} integerPlaces={9} placeholder="0" onChange={field.onChange}/>
                <FormMessage/>
              </FormItem>
            )}/>
          </section>
        </div>

        {isAbrasf && (
          <section className="space-y-3 border-t border-gray-100 pt-4">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">ABRASF 2.04</p>
            <FormField control={form.control} name="abrasf_endpoint_url" render={({field}) => (
              <FormItem>
                <FormLabel>Endpoint do WSDL</FormLabel>
                <Input {...field} id={field.name} placeholder="https://..."/>
                <FormMessage/>
              </FormItem>
            )}/>
            <div className="grid grid-cols-2 gap-3">
              <FormField control={form.control} name="abrasf_wsdl_version" render={({field}) => (
                <FormItem>
                  <FormLabel>Versão do WSDL</FormLabel>
                  <Input {...field} id={field.name} maxLength={10}/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="abrasf_municipality_code" render={({field}) => (
                <FormItem>
                  <FormLabel>Código IBGE do município</FormLabel>
                  <Input {...field} id={field.name} maxLength={7}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            </div>
          </section>
        )}

        <div className="flex items-center justify-between pt-2">
          {savedAt ? (
            <p className="text-xs text-muted-foreground">Última atualização {savedAt}</p>
          ) : (
            <p className="text-xs text-muted-foreground">NFS-e ainda não configurado</p>
          )}
          <Button type="submit" disabled={loading}>
            {loading ? 'Salvando…' : 'Salvar configuração'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
