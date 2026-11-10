'use client';

import {useState} from 'react';
import {useRouter} from 'next/navigation';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {Label} from '@aoctech/ui';
import {ChevronDown} from 'lucide-react';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import type {EmitFailure} from '@/lib/billing/notice';
import {PersonPicker} from '@/components/persons/PersonPicker';
import {Combobox} from '@/components/ui/combobox';
import {CurrencyInput} from '@/components/ui/currency-input';
import {FormWizard, ReviewSection, focusFirstInvalid, useWizard, type WizardStep} from '@/components/ui/form-wizard';
import {EmitError} from '@/components/ui/emit-error';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {Input} from '@/components/ui/input';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {
  CTE_ICMS_CST_OPTIONS,
  CTE_CHARGE_ERROR_CODES,
  CTE_OS_DESCRIPTION_MAX,
  CTE_OS_DOCS_ERROR_CODES,
  CTE_OS_ROAD_ERROR_CODES,
  CTE_OS_ROUTE_ERROR_CODES,
  CTE_OS_FEDERAL_TAXES,
  CTE_OS_ICMS_CST_EXCLUDED,
  CTE_OS_QUANTITY_LABELS,
  CTE_OS_ROUTE_UFS_MAX,
  CTE_OS_SERVICES,
} from '@/lib/constants/cte';
import {CITY_OPTIONS} from '@/lib/data/cities';
import {UF_OPTIONS} from '@/lib/schemas/entity';
import {cteOsBodySchema} from '@/lib/schemas/cte';
import {cteDocFailure, cteErrorStep, stepOfCodes} from '@/lib/utils/cte';
import {
  buildCteOsBody,
  cteOsMissingItems,
  OS_STEP_CHARGE,
  OS_STEP_DOCS,
  OS_STEP_ROAD,
  OS_STEP_ROUTE,
  OS_STEP_SERVICE,
  emptyCteOs,
  osHasRoad,
  osInssRequired,
  osPlace,
  osRegistrationKind,
  type CteOsState,
} from '@/lib/utils/cte-os';
import {cn} from '@/lib/utils';
import {formatCurrency} from '@/lib/utils/helpers';
import type {CteOsServiceType} from '@/lib/types/api';
import {CteSection} from './CteBlock';
import {CteChargeExtras, useCteCharge} from './CteCharge';
import {StepIssues, useStepIssues} from './StepIssues';
import {CteFreightFields, icmsSummary} from './CteFreightFields';
import {CteOsDocRefs, CteOsGtve, CteOsRoad} from './CteOsFields';

const STEP_REVIEW = 'review';
const ERROR_STEP_BY_CODE = {
  ...stepOfCodes(OS_STEP_ROUTE, CTE_OS_ROUTE_ERROR_CODES),
  ...stepOfCodes(OS_STEP_ROAD, CTE_OS_ROAD_ERROR_CODES),
  ...stepOfCodes(OS_STEP_DOCS, CTE_OS_DOCS_ERROR_CODES),
  ...stepOfCodes(OS_STEP_CHARGE, CTE_CHARGE_ERROR_CODES),
};
const ERROR_STEP_BY_TARGET = {aut_xml: OS_STEP_CHARGE, documentos: OS_STEP_DOCS, partes: OS_STEP_ROUTE} as const;
/** Rótulo do passo dos dados do tipo (o de documentos) por serviço. */
const DOCS_LABEL = {baggage: 'Bilhetes', values: 'GTV-e'} as const;

const FIELD = 'space-y-1.5';
const LABEL = 'text-sm font-medium text-gray-700';
const OS_CST_OPTIONS = CTE_ICMS_CST_OPTIONS.filter((o) => o.value !== CTE_OS_ICMS_CST_EXCLUDED);

/**
 * CT-e OS (modelo 67): pessoas, valores ou excesso de bagagem, em passos que mudam com o tipo (serviço, partes e rota,
 * dados do tipo, valores, revisão). Sem notas: o usuário informa o serviço, o tomador,
 * o percurso e os dados do tipo. Nada é presumido (CFOP, ICMS, tomador). Contrato: `POST /v1.0/ctes/os`.
 */
export function CteOsForm() {
  const {selectedOrg} = useAuth();
  const router = useRouter();
  const qc = useQueryClient();
  const {config} = useFiscalConfig('cte', selectedOrg?.pk);
  const [raw, setRaw] = useState<CteOsState>(emptyCteOs);
  const [failure, setFailure] = useState<EmitFailure | null>(null);
  const [failureStep, setFailureStep] = useState(STEP_REVIEW);

  const origin = osPlace(raw.origin);
  const destination = osPlace(raw.destination);
  const forcedKind = osRegistrationKind(raw.serviceType, origin, destination);
  // O trajeto manda no documento do modal rodoviário de pessoas; senão vale a escolha do usuário.
  const state: CteOsState = {...raw, registrationKind: forcedKind ?? raw.registrationKind};
  const charge = useCteCharge(origin, destination);

  const patch = (p: Partial<CteOsState>) => setRaw((s) => ({...s, ...p}));
  const chooseService = (serviceType: CteOsServiceType) => setRaw((s) => s.serviceType === serviceType ? s : ({
    ...emptyCteOs(), serviceType, taker: s.taker, description: s.description, origin: s.origin, destination: s.destination,
  }));

  const t = state.serviceType;
  const issues = useStepIssues(cteOsMissingItems(state, charge.state));
  const inss = osInssRequired(state);

  const emit = useMutation({
    mutationFn: () => apiClient.emitCteOs(cteOsBodySchema.parse(buildCteOsBody(state, charge.state)) as ReturnType<typeof buildCteOsBody>),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.ctes.lists(selectedOrg?.pk)});
      toast.success('CT-e OS enviado, aguardando autorização da SEFAZ.');
      router.push('/cte');
    },
    onError: (err) => {
      const f = cteDocFailure(err, 'Erro ao emitir o CT-e OS.');
      const step = cteErrorStep(err, f, ERROR_STEP_BY_CODE, ERROR_STEP_BY_TARGET, STEP_REVIEW);
      setFailure(f);
      setFailureStep(step);
      wizard.goTo(step);
    },
  });

  const addUf = (uf: string) => uf && !raw.routeUfs.includes(uf) && patch({routeUfs: [...raw.routeUfs, uf]});

  const failureAt = (step: string) => <EmitError failure={failureStep === step ? failure : null}/>;
  const stepEnd = (step: string) => (
    <>
      <StepIssues items={issues.of(step)}/>
      {failureAt(step)}
    </>
  );
  const own = (step: string): Pick<WizardStep, 'id' | 'validate'> => ({id: step, validate: () => issues.validate(step)});
  const cfopLabel = charge.freight.cfopOptions.find((o) => o.value === charge.freight.cfop)?.label;
  const serviceLabel = CTE_OS_SERVICES.find((s) => s.value === t)?.label;

  const steps: WizardStep[] = [
    {...own(OS_STEP_SERVICE), label: 'Serviço', content: (
      <>
        <CteSection id="cte-os-service-type" title="Que serviço você presta?"
                          hint="O tipo decide quais dados vêm a seguir.">
                <div role="radiogroup" aria-labelledby="cte-os-service-type" className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                  {CTE_OS_SERVICES.map((s) => {
                    const checked = t === s.value;
                    return (
                      <label key={s.value} className={cn(
                        'flex min-h-11 cursor-pointer flex-col justify-center rounded-lg border px-3 py-2 transition-colors',
                        'has-[:focus-visible]:ring-3 has-[:focus-visible]:ring-ring/50',
                        checked ? 'border-brand-500 bg-brand-50' : 'border-gray-200 hover:border-brand-300')}>
                        <input type="radio" name="cte-os-service" value={s.value} checked={checked} className="sr-only"
                               onChange={() => chooseService(s.value)}/>
                        <span className="text-sm font-medium text-gray-900">{s.label}</span>
                        <span className="text-xs text-gray-500">{s.hint}</span>
                      </label>
                    );
                  })}
                </div>
              </CteSection>
        {t && (
          <CteSection id="cte-os-description" title="O que está sendo prestado?">
                      <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
                      <div className={FIELD}>
                        <Label htmlFor="cte-os-desc" className={LABEL}>Descrição do serviço</Label>
                        <Input id="cte-os-desc" maxLength={CTE_OS_DESCRIPTION_MAX} value={state.description} className="w-full"
                               onChange={(e) => patch({description: e.target.value.replace(/[\r\n]/g, '')})}/>
                        <p className="text-xs text-gray-500">Até {CTE_OS_DESCRIPTION_MAX} caracteres.</p>
                      </div>
                      <div className={FIELD}>
                        <Label htmlFor="cte-os-qty" className={LABEL}>{CTE_OS_QUANTITY_LABELS[t]} (opcional)</Label>
                        <NumericInput id="cte-os-qty" decimal decimalPlaces={4} integerPlaces={11} value={state.quantity}
                                      onChange={(quantity) => patch({quantity})} className="w-full"/>
                      </div>
                      </div>
                    </CteSection>
        )}
        {stepEnd(OS_STEP_SERVICE)}
      </>
    )},
    {...own(OS_STEP_ROUTE), label: 'Partes e rota', content: (
      <>
        <CteSection id="cte-os-taker" title="Quem paga o serviço?"
                              hint="O tomador sai sempre do cadastro de pessoas.">
                    <PersonPicker value={state.taker} onChange={(taker) => patch({taker})} placeholder="Buscar pessoa cadastrada"/>
                  </CteSection>
        <CteSection id="cte-os-route" title="Por onde vai?"
                              hint={t === 'people' ? 'Origem e destino são obrigatórios.' : t === 'values' ? 'O destino é obrigatório.' : 'Origem e destino são opcionais.'}>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <div className={FIELD}>
                        <Label htmlFor="cte-os-origin" className={LABEL}>Origem</Label>
                        <Combobox id="cte-os-origin" fuzzySearch options={CITY_OPTIONS} value={state.origin || null} clearable={t !== 'people'}
                                  onValueChange={(origin) => patch({origin})} placeholder="Município de origem" searchPlaceholder="Buscar município..."/>
                      </div>
                      <div className={FIELD}>
                        <Label htmlFor="cte-os-destination" className={LABEL}>Destino</Label>
                        <Combobox id="cte-os-destination" fuzzySearch options={CITY_OPTIONS} value={state.destination || null} clearable={t === 'baggage'}
                                  onValueChange={(destination) => patch({destination})} placeholder="Município de destino" searchPlaceholder="Buscar município..."/>
                      </div>
                    </div>
                    <div className={FIELD}>
                      <Label htmlFor="cte-os-route-uf" className={LABEL}>UFs do percurso (opcional, em ordem)</Label>
                      <OptionsSelect id="cte-os-route-uf" value="" placeholder="Adicionar UF" options={UF_OPTIONS.filter((u) => !raw.routeUfs.includes(u.value))}
                                     disabled={raw.routeUfs.length >= CTE_OS_ROUTE_UFS_MAX} onValueChange={addUf} className="w-full sm:max-w-xs"/>
                      {raw.routeUfs.length > 0 && (
                        <ul aria-label="UFs do percurso" className="flex flex-wrap gap-2 pt-1">
                          {raw.routeUfs.map((uf, i) => (
                            <li key={uf} className="inline-flex items-center gap-1 rounded-full bg-gray-100 py-0.5 pl-3 pr-1 text-sm text-gray-800">
                              {i + 1}. {uf}
                              <button type="button" aria-label={`Remover ${uf}`} className="flex size-8 items-center justify-center rounded-full text-gray-500 hover:text-danger"
                                      onClick={() => patch({routeUfs: raw.routeUfs.filter((x) => x !== uf)})}>×</button>
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>
                  </CteSection>
        {stepEnd(OS_STEP_ROUTE)}
      </>
    )},
    ...(osHasRoad(t) ? [{...own(OS_STEP_ROAD), label: 'Rodoviário', content: (
      <>
        <CteOsRoad state={state} onChange={patch} forcedKind={forcedKind}/>
        {stepEnd(OS_STEP_ROAD)}
      </>
    )}] : []),
    ...(t === 'baggage' || t === 'values' ? [{...own(OS_STEP_DOCS), label: DOCS_LABEL[t], content: (
      <>
        {t === 'baggage'
          ? <CteOsDocRefs lines={state.docRefs} onChange={(docRefs) => patch({docRefs})}/>
          : <CteOsGtve lines={state.gtve} onChange={(gtve) => patch({gtve})} freightTotal={charge.freight.total}/>}
        {stepEnd(OS_STEP_DOCS)}
      </>
    )}] : []),
    {...own(OS_STEP_CHARGE), label: 'Valores', content: (
      <>
        <CteFreightFields {...charge.freight} cstOptions={OS_CST_OPTIONS}
                          title="Quanto custa o serviço?" hint="O total da prestação. Componentes são opcionais e saem do total."
                          totalLabel="Valor total do serviço"/>
        <details className="group rounded-lg border border-gray-200 bg-white" open={inss || undefined}>
                    <summary className="flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 px-3 py-2 [&::-webkit-details-marker]:hidden">
                      <span className="text-sm font-medium text-gray-900">Tributos federais{inss ? '' : ' (opcional)'}</span>
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="truncate text-sm text-gray-600">
                          {inss ? 'INSS obrigatório' : CTE_OS_FEDERAL_TAXES.some((f) => state.federal[f.key]) ? 'Informado' : 'Não informado'}
                        </span>
                        <ChevronDown className="size-4 shrink-0 text-gray-400 transition-transform group-open:rotate-180" aria-hidden="true"/>
                      </span>
                    </summary>
                    <div className="space-y-3 border-t border-gray-100 p-3">
                      {inss && <p className="text-sm text-gray-700">Transporte de pessoas com tomador pessoa jurídica exige o valor do INSS.</p>}
                      <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
                        {CTE_OS_FEDERAL_TAXES.map((f) => (
                          <div key={f.key} className={FIELD}>
                            <Label htmlFor={`cte-os-fed-${f.key}`} className={LABEL}>{f.label}{f.key === 'inss' && inss ? ' *' : ''}</Label>
                            <CurrencyInput id={`cte-os-fed-${f.key}`} value={state.federal[f.key]} placeholder="0,00" className="w-full"
                                           onChange={(v) => patch({federal: {...state.federal, [f.key]: v}})}/>
                          </div>
                        ))}
                      </div>
                    </div>
                  </details>
        <CteChargeExtras charge={charge} xmlErrors={failure?.issues?.filter((i) => i.target === 'aut_xml').map((i) => i.text)}/>
        {stepEnd(OS_STEP_CHARGE)}
      </>
    )},
    {id: STEP_REVIEW, label: 'Revisão', content: (w) => (
      <>
        <p className="text-sm text-gray-600">Confira os dados antes de emitir o CT-e OS.</p>
        <div className="grid gap-x-8 gap-y-4 md:grid-cols-2">
          <ReviewSection title="Serviço" stepId={OS_STEP_SERVICE} wizard={w} items={[
            ['Tipo', serviceLabel ?? ''],
            ['Descrição', state.description],
            ...(state.quantity && t ? [[CTE_OS_QUANTITY_LABELS[t], state.quantity] as const] : []),
          ]}/>
          <ReviewSection title="Partes e rota" stepId={OS_STEP_ROUTE} wizard={w} items={[
            ['Tomador', state.taker?.name ?? ''],
            ['Trajeto', [origin, destination].filter(Boolean).map((p) => `${p!.city} / ${p!.uf}`).join(' para ')],
            ...(raw.routeUfs.length > 0 ? [['Percurso', raw.routeUfs.join(', ')] as const] : []),
          ]}/>
          {osHasRoad(t) && (
            <ReviewSection title="Transporte rodoviário" stepId={OS_STEP_ROAD} wizard={w} items={[
              ['Documento', state.registrationKind === 'taf' ? `TAF ${state.taf}` : state.registrationKind === 'state_registration' ? `Registro estadual ${state.stateRegistration}` : ''],
              ...(state.vehicle ? [['Veículo', state.vehicle.plate] as const] : []),
            ]}/>
          )}
          {t === 'baggage' && (
            <ReviewSection title={DOCS_LABEL.baggage} stepId={OS_STEP_DOCS} wizard={w}
                           items={[['Documentos', String(state.docRefs.length)]]}/>
          )}
          {t === 'values' && (
            <ReviewSection title={DOCS_LABEL.values} stepId={OS_STEP_DOCS} wizard={w}
                           items={[['GTV-e', String(state.gtve.length)]]}/>
          )}
          <ReviewSection title="Valores" stepId={OS_STEP_CHARGE} wizard={w} items={[
            ['Valor', charge.freight.total ? formatCurrency(charge.freight.total) : ''],
            ['CFOP', cfopLabel ?? ''],
            ['ICMS', icmsSummary(charge.freight.total, charge.freight.icms)],
          ]}/>
        </div>
        {failureAt(STEP_REVIEW)}
      </>
    )},
  ];
  const wizard = useWizard(steps);

  const submit = () => {
    setFailure(null);
    const bad = issues.firstInvalid(steps.slice(0, -1).map((s) => s.id));
    if (bad) {
      toast.error('Confira os campos destacados.');
      wizard.goTo(bad);
      focusFirstInvalid();
      return;
    }
    emit.mutate();
  };

  return (
    <div className="space-y-4">
      <HomologationBanner environment={config?.environment}/>
      <FormWizard wizard={wizard} steps={steps} onSubmit={submit} submitting={emit.isPending}
                  submitLabel="Emitir CT-e OS" onCancel={() => router.push('/cte')}/>
    </div>
  );
}
