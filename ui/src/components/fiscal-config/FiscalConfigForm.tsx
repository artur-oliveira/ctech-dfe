'use client';

import {useEffect, useState} from 'react';
import type {ReactNode} from 'react';
import type {Resolver} from 'react-hook-form';
import {useForm, useWatch} from 'react-hook-form';
import {zodResolver} from '@hookform/resolvers/zod';
import {Input} from '@/components/ui/input';
import {Textarea} from '@aoctech/ui';
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {formatDatetimeBR} from '@/lib/utils/dfe';
import {
  BRAZIL_TIMEZONES,
  type BrazilTimezone,
  type CTeConfigFormData,
  cteConfigSchema,
  type DocVariant,
  type MDFeConfigFormData,
  mdfeConfigSchema,
  type NFCeConfigFormData,
  nfceConfigSchema,
  QR_CODE_VERSION_WITHOUT_CSC,
  QR_CODE_VERSIONS,
  type NFeConfigFormData,
  nfeConfigSchema,
  TIMEZONE_LABELS,
} from '@/lib/schemas/fiscal-configs';
import type {MDFeConfigOut, NFCeConfigOut, NFeConfigOut, NfseConfigOut} from '@/lib/types/api';
import {SettingsRail, useSettingsRail, type RailSection} from '@/components/ui/settings-rail';
import {NfseConfigForm} from '@/components/fiscal-config/NfseConfigForm';

type AnyConfigOut = NFeConfigOut | NFCeConfigOut | MDFeConfigOut | null | undefined
type AnyFormData = NFeConfigFormData | NFCeConfigFormData | CTeConfigFormData | MDFeConfigFormData

interface FiscalConfigFormProps {
  variant: DocVariant
  initialData: AnyConfigOut | NfseConfigOut
  onSave: (data: Record<string, unknown>) => Promise<void>
  loading?: boolean
  /** Seção "Forma de emissão" (EmissionSection, com Aplicar próprio); entra no trilho sem se misturar ao salvar. */
  emission?: ReactNode
  /** Abre a configuração já na forma de emissão (documento em contingência). */
  emissionAlert?: boolean
}

const SCHEMA_BY_VARIANT = {
  nfe: nfeConfigSchema,
  cte: cteConfigSchema,
  nfce: nfceConfigSchema,
  mdfe: mdfeConfigSchema,
};

const QR_VERSION_LABELS: Record<(typeof QR_CODE_VERSIONS)[number], string> = {
  '2': 'Versão 2 (com CSC)',
  '3': 'Versão 3 (sem CSC)',
};

const LABEL_BY_VARIANT: Record<DocVariant, string> = {
  nfe: 'NF-e',
  nfce: 'NFC-e',
  cte: 'CT-e',
  mdfe: 'MDF-e',
  nfse: 'NFS-e',
};

function toFormValues(variant: DocVariant, data: AnyConfigOut): AnyFormData {
  const cfg = data as NFeConfigOut | undefined;
  const base = {
    timezone: (BRAZIL_TIMEZONES.includes(cfg?.timezone as BrazilTimezone)
      ? cfg!.timezone
      : 'America/Sao_Paulo') as BrazilTimezone,
    environment: String(cfg?.environment ?? 2),
    prod_current_serie: String(cfg?.prod_current_serie ?? 0),
    prod_current_number: String(cfg?.prod_current_number ?? 1),
    hom_current_serie: String(cfg?.hom_current_serie ?? 0),
    hom_current_number: String(cfg?.hom_current_number ?? 1),
    csrt_id: String(cfg?.csrt_id ?? ''),
    // O CSRT nunca volta da API: o campo nasce vazio a cada abertura, e vazio
    // no PUT significa "manter o que está gravado", não "apagar".
    csrt: '',
  };

  if (variant === 'nfce') {
    const nfce = data as NFCeConfigOut | undefined;
    return {
      ...base,
      qr_code_version: String(nfce?.qr_code_version ?? 2),
      prod_csc: nfce?.prod_csc ?? '',
      // 0 é "sem CSC" (mínimo da API é 1): o campo nasce vazio, não "0".
      prod_csc_id: nfce?.prod_csc_id ? String(nfce.prod_csc_id) : '',
      hom_csc: nfce?.hom_csc ?? '',
      hom_csc_id: nfce?.hom_csc_id ? String(nfce.hom_csc_id) : '',
    } as never;
  }

  if (variant === 'mdfe') {
    const mdfe = data as MDFeConfigOut | undefined;
    return {
      ...base,
      ind_canal_verde: mdfe?.ind_canal_verde ?? false,
      ind_carrega_posterior: mdfe?.ind_carrega_posterior ?? false,
      inf_ad_fisco: mdfe?.inf_ad_fisco ?? '',
      rntrc: mdfe?.rntrc ?? '',
    } as never;
  }

  return base as never;
}

function toApiPayload(variant: DocVariant, data: AnyFormData): Record<string, unknown> {
  const d = data as Record<string, string>;
  const base = {
    timezone: d.timezone,
    environment: parseInt(d.environment, 10),
    prod_current_serie: parseInt(d.prod_current_serie, 10),
    prod_current_number: parseInt(d.prod_current_number, 10),
    hom_current_serie: parseInt(d.hom_current_serie, 10),
    hom_current_number: parseInt(d.hom_current_number, 10),
    csrt_id: d.csrt_id || null,
    // Chave ausente, não null: vazio significa "manter o gravado". Enviar null
    // apagaria o segredo que a tela nunca chegou a exibir.
    ...(d.csrt ? {csrt: d.csrt} : {}),
  };

  if (variant === 'nfce') {
    // Com a versão 3 o CSC é opcional: campo vazio sai do corpo em vez de virar 0 ou NaN.
    return {
      ...base,
      qr_code_version: parseInt(d.qr_code_version, 10),
      ...(d.prod_csc ? {prod_csc: d.prod_csc} : {}),
      ...(d.prod_csc_id ? {prod_csc_id: parseInt(d.prod_csc_id, 10)} : {}),
      ...(d.hom_csc ? {hom_csc: d.hom_csc} : {}),
      ...(d.hom_csc_id ? {hom_csc_id: parseInt(d.hom_csc_id, 10)} : {}),
    };
  }

  if (variant === 'mdfe') {
    const m = data as unknown as MDFeConfigFormData;
    return {
      ...base,
      ind_canal_verde: m.ind_canal_verde,
      ind_carrega_posterior: m.ind_carrega_posterior,
      inf_ad_fisco: m.inf_ad_fisco || null,
      rntrc: m.rntrc || null,
    };
  }

  return base;
}

export function FiscalConfigForm({variant, initialData, onSave, loading = false, emission, emissionAlert}: FiscalConfigFormProps) {
  // NFS-e não cabe no layout prod/hom-split: série única (não uma por
  // ambiente), com timezone próprio, e o provider troca o conjunto de campos
  // (nacional x abrasf204) em vez de csc x sem-csc. Componente próprio em vez
  // de forçar o formato aqui; ver NfseConfigForm.tsx.
  if (variant === 'nfse') {
    return <NfseConfigForm initialData={initialData as NfseConfigOut | null} onSave={onSave} loading={loading}/>;
  }

  return (
    <FiscalConfigFormInner variant={variant} initialData={initialData as AnyConfigOut} onSave={onSave} loading={loading}
                           emission={emission} emissionAlert={emissionAlert}/>
  );
}

const SEC = {env: 'ambiente', num: 'numeracao', csc: 'csc', manifest: 'manifesto', csrt: 'csrt', emission: 'emissao'} as const;

function sectionOf(field: string): string {
  if (/_current_(serie|number)$/.test(field)) return SEC.num;
  if (/_csc(_id)?$/.test(field)) return SEC.csc;
  if (['rntrc', 'ind_canal_verde', 'ind_carrega_posterior', 'inf_ad_fisco'].includes(field)) return SEC.manifest;
  if (field === 'csrt' || field === 'csrt_id') return SEC.csrt;
  return SEC.env;
}

const ENV_TONE = {
  prod: {label: 'Produção', cls: 'bg-emerald-50 text-emerald-700'},
  hom: {label: 'Homologação', cls: 'bg-blue-50 text-blue-700'},
} as const;

/** Coluna Produção | Homologação: o mesmo conteúdo para cada ambiente. */
function EnvColumns({children}: { children: (env: 'prod' | 'hom') => ReactNode }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6">
      {(['prod', 'hom'] as const).map((env) => (
        <section key={env} className="space-y-3">
          <p className={`rounded-md px-2 py-1 text-xs font-semibold uppercase tracking-wider ${ENV_TONE[env].cls}`}>
            {ENV_TONE[env].label}
          </p>
          {children(env)}
        </section>
      ))}
    </div>
  );
}

function FiscalConfigFormInner({variant, initialData, onSave, loading = false, emission, emissionAlert}: {
  variant: Exclude<DocVariant, 'nfse'>
  initialData: AnyConfigOut
  onSave: (data: Record<string, unknown>) => Promise<void>
  loading?: boolean
  emission?: ReactNode
  emissionAlert?: boolean
}) {
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null);
  const savedAt = lastSavedAt ?? (initialData?.updated_at
    ? new Date(initialData.updated_at).toLocaleString('pt-BR')
    : null);

  const form = useForm<AnyFormData>({
    resolver: zodResolver(SCHEMA_BY_VARIANT[variant]) as Resolver<AnyFormData>,
    defaultValues: toFormValues(variant, initialData),
  });

  useEffect(() => {
    form.reset(toFormValues(variant, initialData));
  }, [form, initialData, variant]);

  const showCsc = variant === 'nfce';
  const showMdfeFields = variant === 'mdfe';
  const showNsu = variant !== 'nfce';
  const qrVersion = useWatch({control: form.control, name: 'qr_code_version' as never}) as unknown as string | undefined;
  const cscOptional = qrVersion === QR_CODE_VERSION_WITHOUT_CSC;
  const nsuConfig = showNsu ? (initialData as NFeConfigOut | null) : null;
  const isProd = nsuConfig?.environment === 1;
  const activeNsu = nsuConfig ? (isProd ? nsuConfig.prod_nsu : nsuConfig.hom_nsu) : undefined;
  const activeLastAt = nsuConfig ? (isProd ? nsuConfig.prod_last_dist_nsu_at : nsuConfig.hom_last_dist_nsu_at) : null;

  const order = [
    SEC.env, SEC.num,
    ...(showCsc ? [SEC.csc] : []),
    ...(showMdfeFields ? [SEC.manifest] : []),
    SEC.csrt,
    ...(emission ? [SEC.emission] : []),
  ];
  const rail = useSettingsRail(form, order, sectionOf, emissionAlert ? SEC.emission : undefined);
  const dirty = Object.values(rail.dirtyCount).some(Boolean);

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null);
    try {
      await onSave(toApiPayload(variant, data));
      setLastSavedAt(new Date().toLocaleString('pt-BR'));
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar');
    }
  }, (errs) => rail.onInvalid(errs as Record<string, unknown>));

  const numberField = (env: 'prod' | 'hom') => (
    <>
      <FormField
        control={form.control}
        name={`${env}_current_serie`}
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
        name={`${env}_current_number`}
        render={({field}) => (
          <FormItem>
            <FormLabel>Número atual a ser emitido</FormLabel>
            <NumericInput {...field} integerPlaces={9} placeholder="0" onChange={field.onChange}/>
            <FormMessage/>
          </FormItem>
        )}
      />
    </>
  );

  const cscFields = (env: 'prod' | 'hom') => (
    <>
      <FormField
        control={form.control}
        name={`${env}_csc` as never}
        render={({field}) => (
          <FormItem>
            <FormLabel>CSC (token){cscOptional && ' (opcional)'}</FormLabel>
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
        name={`${env}_csc_id` as never}
        render={({field}) => (
          <FormItem>
            <FormLabel>Identificador do CSC{cscOptional && ' (opcional)'}</FormLabel>
            <NumericInput {...field} integerPlaces={10} placeholder="1" onChange={field.onChange}/>
            <FormMessage/>
          </FormItem>
        )}
      />
    </>
  );

  const checkbox = (name: string, title: string, hint: string) => (
    <FormField
      control={form.control}
      name={name as never}
      render={({field}) => (
        <FormItem>
          <label className="flex min-h-11 cursor-pointer items-start gap-2 py-2">
            <input type="checkbox" checked={!!field.value}
                   onChange={(e) => field.onChange(e.target.checked)}
                   className="mt-0.5 size-4 cursor-pointer rounded border-gray-300 text-brand-600"/>
            <span className="text-sm text-gray-700">
              {title}
              <span className="block text-xs text-gray-500">{hint}</span>
            </span>
          </label>
          <FormMessage/>
        </FormItem>
      )}
    />
  );

  const content: Record<string, ReactNode> = {
    [SEC.env]: (
      <>
        {showNsu && nsuConfig && activeNsu !== undefined && (
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
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
                  options={BRAZIL_TIMEZONES.map((tz) => ({value: tz, label: TIMEZONE_LABELS[tz]}))}
                  className="w-full"
                />
                <FormMessage/>
              </FormItem>
            )}
          />
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
                    {value: '1', label: '1 · Produção'},
                    {value: '2', label: '2 · Homologação'},
                  ]}
                  className="w-full"
                />
                <FormMessage/>
              </FormItem>
            )}
          />
        </div>
        {showCsc && (
          <FormField
            control={form.control}
            name={'qr_code_version' as never}
            render={({field}) => (
              <FormItem>
                <FormLabel>Versão do QR Code</FormLabel>
                <OptionsSelect
                  id={field.name}
                  value={String(field.value ?? '2')}
                  onValueChange={field.onChange}
                  options={QR_CODE_VERSIONS.map((v) => ({value: v, label: QR_VERSION_LABELS[v]}))}
                  clearable={false}
                  className="w-full sm:max-w-80"
                />
                <p className="text-xs text-gray-500">
                  A versão 3 não usa CSC, mas a SEFAZ da sua UF precisa aceitá-la; se recusar, volte para a 2.
                  É obrigatória para produtor rural pessoa física, exceto no Paraná.
                </p>
                <FormMessage/>
              </FormItem>
            )}
          />
        )}
      </>
    ),
    [SEC.num]: <EnvColumns>{numberField}</EnvColumns>,
    [SEC.csc]: <EnvColumns>{cscFields}</EnvColumns>,
    // Campos que só existem no leiaute do MDF-e e recorrem em toda emissão.
    [SEC.manifest]: (
      <>
        <p className="text-xs text-gray-500">
          Valem para toda emissão da organização. A observação de uma viagem específica continua
          sendo preenchida na emissão do manifesto.
        </p>
        <div className="grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2">
          <div className="space-y-3">
            <FormField
              control={form.control}
              name={'rntrc' as never}
              render={({field}) => (
                <FormItem>
                  <FormLabel>RNTRC da transportadora (ANTT)</FormLabel>
                  <NumericInput {...field} value={(field.value as string) ?? ''} integerPlaces={8}
                                inputMode="numeric" placeholder="00000000" className="w-full"
                                onChange={field.onChange}/>
                  <p className="text-xs text-gray-500">
                    8 dígitos. Opcional no MDF-e; obrigatório para emitir CT-e rodoviário.
                  </p>
                  <FormMessage/>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name={'inf_ad_fisco' as never}
              render={({field}) => (
                <FormItem>
                  <FormLabel>Mensagem ao fisco</FormLabel>
                  <Textarea {...field} value={(field.value as string) ?? ''} maxLength={2000} rows={3}
                            className="w-full"
                            placeholder="Ex.: regime especial concedido pelo processo nº …"/>
                  <FormMessage/>
                </FormItem>
              )}
            />
          </div>
          <div>
            {checkbox('ind_canal_verde', 'Participa do Canal Verde',
              'Programa de circulação simplificada de cargas nas fronteiras estaduais.')}
            {checkbox('ind_carrega_posterior', 'Inclui carga depois de emitir',
              'Os documentos entram por evento de inclusão de DF-e após a autorização.')}
          </div>
        </div>
      </>
    ),
    // Responsável técnico; CSRT (NT 2018.005). Vale para os dois ambientes.
    [SEC.csrt]: (
      <>
        <p className="text-xs text-gray-500">
          Algumas UFs exigem o CSRT. O código é secreto: ele nunca é devolvido pela API, então este
          campo volta em branco a cada abertura; deixá-lo vazio mantém o que já está gravado.
        </p>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <FormField
            control={form.control}
            name="csrt_id"
            render={({field}) => (
              <FormItem>
                <FormLabel>Identificador do CSRT</FormLabel>
                <NumericInput {...field} value={field.value ?? ''} integerPlaces={2} placeholder="01"
                              onChange={field.onChange}/>
                <FormMessage/>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="csrt"
            render={({field}) => (
              <FormItem>
                <FormLabel>CSRT (36 caracteres)</FormLabel>
                <Input {...field} value={field.value ?? ''} maxLength={36}
                       autoComplete="off" placeholder="Deixe vazio para manter"
                       className="font-mono text-xs"/>
                <FormMessage/>
              </FormItem>
            )}
          />
        </div>
      </>
    ),
    [SEC.emission]: emission,
  };

  const labels: Record<string, string> = {
    [SEC.env]: 'Ambiente', [SEC.num]: 'Numeração e série', [SEC.csc]: 'CSC e QR Code',
    [SEC.manifest]: 'Manifesto', [SEC.csrt]: 'Responsável técnico', [SEC.emission]: 'Forma de emissão',
  };
  const sections: RailSection[] = order.map((id) => ({
    id,
    label: labels[id],
    content: content[id],
    dirty: !!rail.dirtyCount[id],
    errors: rail.errorCount[id],
    alert: id === SEC.emission && emissionAlert,
    ownsSave: id === SEC.emission,
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
          unchangedLocked={!!initialData}
          status={savedAt ? `Última atualização ${savedAt}` : `${LABEL_BY_VARIANT[variant]} ainda não configurado`}
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
