'use client';

import {useEffect, useState} from 'react';
import {useForm, useWatch} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {Input} from '@/components/ui/input';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {Combobox} from '@/components/ui/combobox';
import type {ReactNode} from 'react';
import {SettingsRail, useSettingsRail, type RailSection} from '@/components/ui/settings-rail';
import {formatDatetimeBR} from '@/lib/utils/dfe';
import {
  BRAZIL_TIMEZONES,
  type BrazilTimezone,
  type NfseConfigFormData,
  nfseConfigSchema,
  TIMEZONE_LABELS,
} from '@/lib/schemas/fiscal-configs';
import type {NfseConfigOut} from '@/lib/types/api';
import {CITY_OPTIONS} from '@/lib/data/cities';

interface NfseConfigFormProps {
  initialData: NfseConfigOut | null | undefined
  onSave: (data: Record<string, unknown>) => Promise<void>
  loading?: boolean
}

function toFormValues(cfg: NfseConfigOut | null | undefined): NfseConfigFormData {
  return {
    provider: cfg?.provider ?? 'nacional',
    environment: String(cfg?.environment ?? 2) as '1' | '2',
    timezone: (BRAZIL_TIMEZONES.includes(cfg?.timezone as BrazilTimezone)
      ? cfg?.timezone
      : 'America/Sao_Paulo') as BrazilTimezone,
    c_loc_emi: cfg?.c_loc_emi ?? '',
    serie: cfg?.serie ?? '1',
    prod_current_number: String(cfg?.prod_current_number ?? 0),
    hom_current_number: String(cfg?.hom_current_number ?? 0),
    certificate_sk: cfg?.certificate_sk ?? '',
    abrasf_endpoint_url: cfg?.abrasf?.endpoint_url ?? '',
    abrasf_wsdl_version: cfg?.abrasf?.wsdl_version ?? '',
    abrasf_municipality_code: cfg?.abrasf?.municipality_code ?? '',
    abrasf_synchronous: cfg?.abrasf?.synchronous ?? false,
  };
}

function toApiPayload(d: NfseConfigFormData): Record<string, unknown> {
  return {
    provider: d.provider,
    environment: parseInt(d.environment, 10),
    timezone: d.timezone,
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
  };
}

const SEC = {env: 'ambiente', num: 'numeracao', abrasf: 'abrasf'} as const;

function sectionOf(field: string): string {
  if (field.startsWith('abrasf_')) return SEC.abrasf;
  if (field === 'c_loc_emi' || field === 'serie' || field.endsWith('_current_number')) return SEC.num;
  return SEC.env;
}

export function NfseConfigForm({initialData, onSave, loading = false}: NfseConfigFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null);
  const savedAt = lastSavedAt ?? (initialData?.updated_at
    ? new Date(initialData.updated_at).toLocaleString('pt-BR')
    : null);

  const form = useForm<NfseConfigFormData>({
    resolver: zodResolver(nfseConfigSchema),
    defaultValues: toFormValues(initialData),
  });

  useEffect(() => {
    form.reset(toFormValues(initialData));
  }, [form, initialData]);

  const provider = useWatch({control: form.control, name: 'provider'});
  const environment = useWatch({control: form.control, name: 'environment'});
  const isAbrasf = provider === 'abrasf204';
  const isProd = environment === '1';
  const activeNsu = isProd ? initialData?.prod_nsu : initialData?.hom_nsu;
  const activeLastAt = isProd ? initialData?.prod_last_dist_nsu_at : initialData?.hom_last_dist_nsu_at;
  const order = [SEC.env, SEC.num, ...(isAbrasf ? [SEC.abrasf] : [])];
  const rail = useSettingsRail(form, order, sectionOf);
  const dirty = Object.values(rail.dirtyCount).some(Boolean);

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null);
    try {
      await onSave(toApiPayload(data));
      setLastSavedAt(new Date().toLocaleString('pt-BR'));
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar');
    }
  }, (errs) => rail.onInvalid(errs as Record<string, unknown>));

  const content: Record<string, ReactNode> = {
    [SEC.env]: (
      <>
        {initialData && activeNsu !== undefined && (
          <div className="space-y-0.5 rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-600">
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
              className="w-full sm:max-w-96"
            />
            <FormMessage/>
          </FormItem>
        )}/>

        {isAbrasf && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            O módulo web ainda não emite DANFSE nem consulta parâmetros municipais para ABRASF 2.04; essas ações
            ficam desabilitadas nas telas de NFS-e enquanto este provedor estiver ativo.
          </div>
        )}

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField control={form.control} name="environment" render={({field}) => (
            <FormItem>
              <FormLabel>Ambiente ativo</FormLabel>
              <OptionsSelect
                id={field.name}
                value={field.value}
                onValueChange={field.onChange}
                options={[
                  {value: '1', label: '1 · Produção'},
                  {value: '2', label: '2 · Homologação'},
                ]}
                className="w-full"
              />
              <FormMessage/>
            </FormItem>
          )}/>

          <FormField control={form.control} name="timezone" render={({field}) => (
            <FormItem>
              <FormLabel>Fuso horário da emissão</FormLabel>
              <OptionsSelect
                id={field.name}
                value={field.value}
                onValueChange={field.onChange}
                options={BRAZIL_TIMEZONES.map((timezone) => ({
                  value: timezone,
                  label: TIMEZONE_LABELS[timezone],
                }))}
                className="w-full"
              />
              <FormMessage/>
            </FormItem>
          )}/>
        </div>
      </>
    ),
    [SEC.num]: (
      <>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-[minmax(0,1fr)_10rem]">
          <FormField control={form.control} name="c_loc_emi" render={({field}) => (
            <FormItem>
              <FormLabel>Município emissor</FormLabel>
              <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                        options={CITY_OPTIONS} placeholder="Buscar município"/>
              <FormMessage/>
            </FormItem>
          )}/>

          {/* NFS-e tem uma única série para os dois ambientes; não é prod/hom split. */}
          <FormField control={form.control} name="serie" render={({field}) => (
            <FormItem>
              <FormLabel>Série</FormLabel>
              <NumericInput {...field} integerPlaces={5} placeholder="1" onChange={field.onChange}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6">
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
      </>
    ),
    [SEC.abrasf]: (
      <>
        <FormField control={form.control} name="abrasf_endpoint_url" render={({field}) => (
          <FormItem>
            <FormLabel>Endpoint do WSDL</FormLabel>
            <Input {...field} id={field.name} placeholder="https://..."/>
            <FormMessage/>
          </FormItem>
        )}/>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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
      </>
    ),
  };

  const labels: Record<string, string> = {
    [SEC.env]: 'Provedor e ambiente', [SEC.num]: 'Município e numeração', [SEC.abrasf]: 'ABRASF 2.04',
  };
  const sections: RailSection[] = order.map((id) => ({
    id,
    label: labels[id],
    content: content[id],
    dirty: !!rail.dirtyCount[id],
    errors: rail.errorCount[id],
  }));

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} noValidate>
        <SettingsRail
          sections={sections}
          active={rail.active}
          onSelect={rail.setActive}
          saving={loading}
          dirty={dirty}
          status={savedAt ? `Última atualização ${savedAt}` : 'NFS-e ainda não configurado'}
          banner={submitError && (
            <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {submitError}
            </div>
          )}
        />
      </form>
    </Form>
  );
}
