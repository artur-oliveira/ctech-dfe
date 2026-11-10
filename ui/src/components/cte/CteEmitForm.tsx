'use client';

import {useState} from 'react';
import {useRouter} from 'next/navigation';
import {keepPreviousData, useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {useDebounce} from '@/lib/hooks/useDebounce';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import type {EmitFailure} from '@/lib/billing/notice';
import {FormWizard, ReviewSection, focusFirstInvalid, useWizard, type WizardStep} from '@/components/ui/form-wizard';
import {EmitError} from '@/components/ui/emit-error';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {CTE_CHARGE_ERROR_CODES, CTE_DERIVE_DEBOUNCE_MS, CTE_TAKER_LABELS} from '@/lib/constants/cte';
import {cteEmitBodySchema} from '@/lib/schemas/cte';
import {
  buildCteEmitBody,
  cteEmitFailure,
  cteErrorStep,
  cteMissingItems,
  cteSummary,
  cteTransport,
  CTE_STEP_FREIGHT,
  CTE_STEP_NOTES,
  CTE_STEP_PARTIES,
  effectiveTakerRole,
  placeLabel,
  stepOfCodes,
  type CteFormState,
  type CteOverrides,
} from '@/lib/utils/cte';
import {formatCurrency} from '@/lib/utils/helpers';
import {formatWeightKg} from '@/lib/utils/cte';
import type {CteTakerRole, PersonItemOut} from '@/lib/types/api';
import {warningsFor} from './CteBlock';
import {CteNotesPicker} from './CteNotesPicker';
import {CteTakerChoice} from './CteTakerChoice';
import {CteFreightFields, icmsSummary} from './CteFreightFields';
import {StepIssues, useStepIssues} from './StepIssues';
import {CteTransportData} from './CteTransportData';
import {CteChargeExtras, useCteCharge} from './CteCharge';

const STEP_REVIEW = 'review';
const STEP_ORDER = [CTE_STEP_NOTES, CTE_STEP_PARTIES, CTE_STEP_FREIGHT];
/** Recusas da api que um passo resolve; o resto aparece na revisão. */
const ERROR_STEP_BY_CODE = stepOfCodes(CTE_STEP_FREIGHT, CTE_CHARGE_ERROR_CODES);
const ERROR_STEP_BY_TARGET = {aut_xml: CTE_STEP_FREIGHT, partes: CTE_STEP_PARTIES, documento: CTE_STEP_NOTES} as const;

const sameKeys = (a: string[], b: string[]) => a.length === b.length && a.every((k, i) => k === b[i]);

/**
 * Emissão de CT-e rodoviário a partir de NF-e, em passos (notas, tomador e partes, frete, revisão). Três decisões
 * do usuário (notas, tomador, valor); o resto vem do `derive` e aceita override.
 * Contrato: docs/specs/2026-11-05-cte-emissao-rodoviaria.md.
 */
export function CteEmitForm() {
  const {selectedOrg} = useAuth();
  const router = useRouter();
  const qc = useQueryClient();
  const {config} = useFiscalConfig('cte', selectedOrg?.pk);

  const [nfeKeys, setNfeKeys] = useState<string[]>([]);
  const [takerRole, setTakerRole] = useState<CteTakerRole | null>(null);
  const [takerPerson, setTakerPerson] = useState<PersonItemOut | null>(null);
  const [overrides, setOverrides] = useState<CteOverrides>({});
  const [failure, setFailure] = useState<EmitFailure | null>(null);
  const [failureStep, setFailureStep] = useState(STEP_REVIEW);

  // Derivar a cada mudança de notas, com debounce: adicionar três notas seguidas
  // faz uma chamada, não três. A resposta anterior fica na tela (esmaecida)
  // enquanto a nova chega.
  const debouncedKeys = useDebounce(nfeKeys, CTE_DERIVE_DEBOUNCE_MS);
  const deriveQuery = useQuery({
    queryKey: queryKeys.ctes.derive(selectedOrg?.pk, debouncedKeys),
    queryFn: () => apiClient.deriveCte(debouncedKeys),
    enabled: !!selectedOrg && debouncedKeys.length > 0,
    placeholderData: keepPreviousData,
    retry: false,
  });
  const derive = nfeKeys.length > 0 ? deriveQuery.data : undefined;
  const deriving = nfeKeys.length > 0 && (!sameKeys(nfeKeys, debouncedKeys) || deriveQuery.isFetching);
  const deriveError = deriveQuery.isError && !deriving
    ? (deriveQuery.error instanceof Error ? deriveQuery.error.message : 'Não foi possível ler as notas.')
    : null;

  const transport = cteTransport(derive, overrides);
  // Origem e destino decidem o escopo do CFOP (5 ou 6).
  const charge = useCteCharge(transport.origin, transport.destination);
  const freightTotal = charge.freight.total;
  const form: CteFormState = {nfeKeys, takerRole, takerPerson, overrides, ...charge.state};
  const role = effectiveTakerRole(form, derive);
  const items = deriveError
    ? [{step: CTE_STEP_NOTES, text: 'Revise as notas escolhidas'}]
    : cteMissingItems(form, derive, deriving);
  const issues = useStepIssues(items);

  const emit = useMutation({
    mutationFn: () => {
      const body = cteEmitBodySchema.parse(buildCteEmitBody(form, derive));
      return apiClient.emitCte(body);
    },
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.ctes.lists(selectedOrg?.pk)});
      toast.success('CT-e enviado, aguardando autorização da SEFAZ.');
      router.push('/cte');
    },
    onError: (err) => {
      const f = cteEmitFailure(err);
      const step = cteErrorStep(err, f, ERROR_STEP_BY_CODE, ERROR_STEP_BY_TARGET, STEP_REVIEW);
      setFailure(f);
      setFailureStep(step);
      wizard.goTo(step);
    },
  });

  const chooseRole = (r: CteTakerRole) => {
    setTakerRole(r);
    if (r !== takerRole) setTakerPerson(null);
  };

  const failureAt = (step: string) => <EmitError failure={failureStep === step ? failure : null}/>;
  const cfopLabel = charge.freight.cfopOptions.find((o) => o.value === charge.freight.cfop)?.label;
  const takerName = role === 'remetente' ? transport.senderName
    : role === 'destinatario' ? transport.recipientName : takerPerson?.name ?? null;

  const steps: WizardStep[] = [
    {id: CTE_STEP_NOTES, label: 'Notas', validate: () => issues.validate(CTE_STEP_NOTES), content: (
      <>
        <CteNotesPicker keys={nfeKeys} onChange={setNfeKeys} derive={derive} deriving={deriving}
                        error={deriveError} onRetry={() => void deriveQuery.refetch()} overrides={overrides}
                        warnings={warningsFor(derive?.warnings, 'notes')}/>
        <StepIssues items={issues.of(CTE_STEP_NOTES)}/>
        {failureAt(CTE_STEP_NOTES)}
      </>
    )},
    {id: CTE_STEP_PARTIES, label: 'Tomador e partes', validate: () => issues.validate(CTE_STEP_PARTIES), content: (
      <>
        <CteTakerChoice role={role} suggestion={derive?.taker_suggestion ?? null} loaded={!!derive}
                        senderName={transport.senderName} recipientName={transport.recipientName}
                        person={takerPerson} onRole={chooseRole} onPerson={setTakerPerson}
                        warnings={warningsFor(derive?.warnings, 'taker')}/>
        <CteTransportData derive={derive} overrides={overrides} onChange={setOverrides}
                          warnings={warningsFor(derive?.warnings, 'transport')}/>
        <StepIssues items={issues.of(CTE_STEP_PARTIES)}/>
        {failureAt(CTE_STEP_PARTIES)}
      </>
    )},
    {id: CTE_STEP_FREIGHT, label: 'Frete', validate: () => issues.validate(CTE_STEP_FREIGHT), content: (
      <>
        <CteFreightFields {...charge.freight}/>
        <CteChargeExtras charge={charge} xmlErrors={failure?.issues?.filter((i) => i.target === 'aut_xml').map((i) => i.text)}/>
        <StepIssues items={issues.of(CTE_STEP_FREIGHT)}/>
        {failureAt(CTE_STEP_FREIGHT)}
      </>
    )},
    {id: STEP_REVIEW, label: 'Revisão', content: (w) => (
      <>
        <p className="text-sm text-gray-600">Confira os dados antes de emitir o CT-e.</p>
        <div className="grid gap-x-8 gap-y-4 md:grid-cols-2">
          <ReviewSection title="Notas" stepId={CTE_STEP_NOTES} wizard={w} items={[
            ['Resumo', derive ? cteSummary(derive, overrides) : ''],
          ]}/>
          <ReviewSection title="Tomador e partes" stepId={CTE_STEP_PARTIES} wizard={w} items={[
            ['Tomador', role ? `${CTE_TAKER_LABELS[role]}${takerName ? ` · ${takerName}` : ''}` : ''],
            ['Remetente', transport.senderName ?? ''],
            ['Destinatário', transport.recipientName ?? ''],
            ['Trajeto', [placeLabel(transport.origin), placeLabel(transport.destination)].filter(Boolean).join(' para ')],
            ['Peso', transport.weightKg ? formatWeightKg(transport.weightKg) : ''],
          ]}/>
          <ReviewSection title="Frete" stepId={CTE_STEP_FREIGHT} wizard={w} items={[
            ['Valor', freightTotal ? formatCurrency(freightTotal) : ''],
            ['CFOP', cfopLabel ?? ''],
            ['ICMS', icmsSummary(freightTotal, charge.freight.icms)],
          ]}/>
        </div>
        {failureAt(STEP_REVIEW)}
      </>
    )},
  ];
  const wizard = useWizard(steps);

  const submit = () => {
    setFailure(null);
    const bad = issues.firstInvalid(STEP_ORDER);
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
                  submitLabel="Emitir CT-e" onCancel={() => router.push('/cte')}/>
    </div>
  );
}
