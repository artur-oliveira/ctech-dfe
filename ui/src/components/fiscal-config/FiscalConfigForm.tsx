'use client'

import {useEffect, useState} from 'react'
import {useForm} from 'react-hook-form'
import type {Resolver} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Input} from '@/components/ui/input'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {formatDatetimeBR} from '@/lib/utils/dfe'
import {
  type BrazilTimezone,
  BRAZIL_TIMEZONES,
  type CTeConfigFormData,
  cteConfigSchema,
  type DocVariant,
  type MDFeConfigFormData,
  mdfeConfigSchema,
  type NFCeConfigFormData,
  nfceConfigSchema,
  type NFeConfigFormData,
  nfeConfigSchema,
  TIMEZONE_LABELS,
} from '@/lib/schemas/fiscal-configs'
import type {MDFeConfigOut, NFCeConfigOut, NFeConfigOut} from '@/lib/types/api'

type AnyConfigOut = NFeConfigOut | NFCeConfigOut | MDFeConfigOut | null | undefined
type AnyFormData = NFeConfigFormData | NFCeConfigFormData | CTeConfigFormData | MDFeConfigFormData

interface FiscalConfigFormProps {
  variant: DocVariant
  initialData: AnyConfigOut
  onSave: (data: Record<string, unknown>) => Promise<void>
  loading?: boolean
}

const SCHEMA_BY_VARIANT = {
  nfe: nfeConfigSchema,
  cte: cteConfigSchema,
  nfce: nfceConfigSchema,
  mdfe: mdfeConfigSchema,
}

const LABEL_BY_VARIANT: Record<DocVariant, string> = {
  nfe: 'NF-e',
  nfce: 'NFC-e',
  cte: 'CT-e',
  mdfe: 'MDF-e',
}

function toFormValues(variant: DocVariant, data: AnyConfigOut): AnyFormData {
  const cfg = data as NFeConfigOut | undefined
  const base = {
    timezone: (BRAZIL_TIMEZONES.includes(cfg?.timezone as BrazilTimezone)
      ? cfg!.timezone
      : 'America/Sao_Paulo') as BrazilTimezone,
    environment: String(cfg?.environment ?? 2),
    prod_current_serie: String(cfg?.prod_current_serie ?? 0),
    prod_current_number: String(cfg?.prod_current_number ?? 1),
    hom_current_serie: String(cfg?.hom_current_serie ?? 0),
    hom_current_number: String(cfg?.hom_current_number ?? 1),
  }

  if (variant === 'nfce') {
    const nfce = data as NFCeConfigOut | undefined
    return {
      ...base,
      prod_csc: nfce?.prod_csc ?? '',
      prod_csc_id: String(nfce?.prod_csc_id ?? ''),
      hom_csc: nfce?.hom_csc ?? '',
      hom_csc_id: String(nfce?.hom_csc_id ?? ''),
    } as never;
  }

  return base as never;
}

function toApiPayload(variant: DocVariant, data: AnyFormData): Record<string, unknown> {
  const d = data as Record<string, string>
  const base = {
    timezone: d.timezone,
    environment: parseInt(d.environment, 10),
    prod_current_serie: parseInt(d.prod_current_serie, 10),
    prod_current_number: parseInt(d.prod_current_number, 10),
    hom_current_serie: parseInt(d.hom_current_serie, 10),
    hom_current_number: parseInt(d.hom_current_number, 10),
  }

  if (variant === 'nfce') {
    return {
      ...base,
      prod_csc: d.prod_csc,
      prod_csc_id: parseInt(d.prod_csc_id, 10),
      hom_csc: d.hom_csc,
      hom_csc_id: parseInt(d.hom_csc_id, 10),
    }
  }

  return base
}

export function FiscalConfigForm({variant, initialData, onSave, loading = false}: FiscalConfigFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null)
  const savedAt = lastSavedAt ?? (initialData?.updated_at
    ? new Date(initialData.updated_at).toLocaleString('pt-BR')
    : null)

  const form = useForm<AnyFormData>({
    resolver: zodResolver(SCHEMA_BY_VARIANT[variant]) as Resolver<AnyFormData>,
    defaultValues: toFormValues(variant, initialData),
  })

  useEffect(() => {
    form.reset(toFormValues(variant, initialData))
  }, [form, initialData, variant])

  const showCsc = variant === 'nfce'
  const showNsu = variant !== 'nfce'
  const nsuConfig = showNsu ? (initialData as NFeConfigOut | null) : null
  const isProd = nsuConfig?.environment === 1
  const activeNsu = nsuConfig ? (isProd ? nsuConfig.prod_nsu : nsuConfig.hom_nsu) : undefined
  const activeLastAt = nsuConfig ? (isProd ? nsuConfig.prod_last_dist_nsu_at : nsuConfig.hom_last_dist_nsu_at) : null

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    try {
      await onSave(toApiPayload(variant, data))
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

        {/* NSU read-only info */}
        {showNsu && nsuConfig && activeNsu !== undefined && (
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

        {/* Fuso horário */}
        <FormField
          control={form.control}
          name="timezone"
          render={({field}) => (
            <FormItem>
              <FormLabel>Fuso horário</FormLabel>
              <OptionsSelect
                id={field.name}
                value={field.value}
                onValueChange={field.onChange}
                options={BRAZIL_TIMEZONES.map((tz) => ({
                  value: tz,
                  label: TIMEZONE_LABELS[tz],
                }))}
                className="max-w-96"
              />
              <FormMessage/>
            </FormItem>
          )}
        />

        {/* Ambiente ativo */}
        <FormField
          control={form.control}
          name="environment"
          render={({field}) => (
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
          )}
        />

        {/* Two-column layout: Produção | Homologação */}
        <div className="grid grid-cols-2 gap-6">
          {/* Produção */}
          <section className="space-y-3">
            <p
              className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold uppercase tracking-wider text-emerald-700">
              Produção
            </p>

            <FormField
              control={form.control}
              name="prod_current_serie"
              render={({field}) => (
                <FormItem>
                  <FormLabel>Série</FormLabel>
                  <NumericInput {...field} integerPlaces={3} placeholder="1" onChange={field.onChange}/>
                  <FormMessage/>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="prod_current_number"
              render={({field}) => (
                <FormItem>
                  <FormLabel>Número atual a ser emitido</FormLabel>
                  <NumericInput {...field} integerPlaces={9} placeholder="0" onChange={field.onChange}/>
                  <FormMessage/>
                </FormItem>
              )}
            />

            {showCsc && (
              <>
                <FormField
                  control={form.control}
                  name="prod_csc"
                  render={({field}) => (
                    <FormItem>
                      <FormLabel>CSC (token)</FormLabel>
                      <Input
                        {...field}
                        placeholder="000100000000000000000000000000000001"
                        maxLength={36}
                        className="font-mono text-xs"
                      />
                      <FormMessage/>
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="prod_csc_id"
                  render={({field}) => (
                    <FormItem>
                      <FormLabel>Identificador do CSC</FormLabel>
                      <NumericInput {...field} integerPlaces={10} placeholder="1" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}
                />
              </>
            )}
          </section>

          {/* Homologação */}
          <section className="space-y-3">
            <p className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold uppercase tracking-wider text-blue-700">
              Homologação
            </p>

            <FormField
              control={form.control}
              name="hom_current_serie"
              render={({field}) => (
                <FormItem>
                  <FormLabel>Série</FormLabel>
                  <NumericInput {...field} integerPlaces={3} placeholder="1" onChange={field.onChange}/>
                  <FormMessage/>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="hom_current_number"
              render={({field}) => (
                <FormItem>
                  <FormLabel>Número atual a ser emitido</FormLabel>
                  <NumericInput {...field} integerPlaces={9} placeholder="0" onChange={field.onChange}/>
                  <FormMessage/>
                </FormItem>
              )}
            />

            {showCsc && (
              <>
                <FormField
                  control={form.control}
                  name="hom_csc"
                  render={({field}) => (
                    <FormItem>
                      <FormLabel>CSC (token)</FormLabel>
                      <Input
                        {...field}
                        placeholder="000100000000000000000000000000000001"
                        maxLength={36}
                        className="font-mono text-xs"
                      />
                      <FormMessage/>
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="hom_csc_id"
                  render={({field}) => (
                    <FormItem>
                      <FormLabel>Identificador do CSC</FormLabel>
                      <NumericInput {...field} integerPlaces={10} placeholder="1" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}
                />
              </>
            )}
          </section>
        </div>

        <div className="flex items-center justify-between pt-2">
          {savedAt ? (
            <p className="text-xs text-muted-foreground">Última atualização {savedAt}</p>
          ) : (
            <p className="text-xs text-muted-foreground">
              {LABEL_BY_VARIANT[variant]} ainda não configurado
            </p>
          )}
          <Button type="submit" disabled={loading}>
            {loading ? 'Salvando…' : 'Salvar configuração'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
