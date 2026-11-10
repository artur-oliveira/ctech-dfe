'use client';

import {useState} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {useAuth} from '@/lib/hooks/useAuth';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import {queryKeys} from '@/lib/api/query-keys';
import type {EmitFailure} from '@/lib/billing/notice';
import {PersonPicker} from '@/components/persons/PersonPicker';
import {FormWizard, ReviewSection, focusFirstInvalid, useWizard, type WizardStep} from '@/components/ui/form-wizard';
import {EmitError} from '@/components/ui/emit-error';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {
  CTE_CHARGE_ERROR_CODES,
  CTE_SUBSTITUTE_LIMIT_DAYS,
  CTE_SUBSTITUTE_SEFAZ_DAYS,
  CTE_SUBSTITUTE_TAKER_ROLES,
  CTE_TAKER_LABELS,
  CTE_SUBSTITUTE_NEEDS_PERSON,
  CTE_TAKER_ERROR_CODES,
} from '@/lib/constants/cte';
import {cteComplementBodySchema, cteSubstituteBodySchema} from '@/lib/schemas/cte';
import {
  buildChargeBody, chargeMissingItems, cteDocFailure, cteErrorStep, daysSince, ibsCbsComplete, stepOfCodes, substituteWindow,
  type StepIssue,
} from '@/lib/utils/cte';
import {cn} from '@/lib/utils';
import {formatCurrency} from '@/lib/utils/helpers';
import type {CteDetailOut, CteTakerRole, PersonItemOut} from '@/lib/types/api';
import {CteChargeExtras, useCteCharge} from './CteCharge';
import {CTE_SURFACE, CteSection} from './CteBlock';
import {StepIssues, useStepIssues} from './StepIssues';
import {CteFreightFields, icmsSummary} from './CteFreightFields';
import {OriginalSummary} from './CteOriginalGate';

export type CteChangeMode = 'complement' | 'substitute'

const STEP_ORIGINAL = 'original';
const STEP_TAKER = 'taker';
const STEP_VALUE = 'value';
const STEP_REVIEW = 'review';
const ERROR_STEP_BY_CODE = {
  ...stepOfCodes(STEP_TAKER, CTE_TAKER_ERROR_CODES),
  ...stepOfCodes(STEP_VALUE, CTE_CHARGE_ERROR_CODES),
  substitution_window_expired: STEP_ORIGINAL,
};
const ERROR_STEP_BY_TARGET = {aut_xml: STEP_VALUE} as const;

const COPY = {
  complement: {
    emitLabel: 'Emitir complemento',
    docLabel: 'CT-e complementar',
    success: 'Complemento enviado, aguardando autorização da SEFAZ.',
    fallback: 'Erro ao emitir o complemento.',
    freightTitle: 'Qual é o valor complementar?',
    freightHint: 'Só a diferença a cobrar além do CT-e original, não o frete todo. Componentes são opcionais e saem do total.',
    totalLabel: 'Valor complementar',
    barLabel: 'Valor complementar',
  },
  substitute: {
    emitLabel: 'Emitir substituto',
    docLabel: 'CT-e substituto',
    success: 'Substituto enviado, aguardando autorização da SEFAZ.',
    fallback: 'Erro ao emitir o substituto.',
    freightTitle: 'Quanto custa o frete agora?',
    freightHint: 'O valor total do CT-e substituto, que toma o lugar do original. Componentes são opcionais e saem do total.',
    totalLabel: 'Valor total do frete',
    barLabel: 'Frete',
  },
} as const;

/** Explica quando usar e manda para a outra ação, sem o usuário precisar saber a diferença de cabeça. */
function WhenToUse({mode, original}: { mode: CteChangeMode; original: CteDetailOut }) {
  const days = daysSince(original.dh_emi ?? original.created_at);
  const deadline = substituteWindow(days);
  const other = mode === 'complement' ? 'substitute' : 'complement';
  return (
    <section aria-label="Quando usar" className={`${CTE_SURFACE} space-y-2 text-sm text-gray-700`}>
      {mode === 'complement' ? (
        <>
          <h3 className="text-base font-semibold text-gray-900">Quando usar o complemento</h3>
          <p>
            Use quando o valor cobrado no CT-e original ficou abaixo do devido e falta cobrar a diferença. O complemento
            só soma valor: remetente, destinatário, percurso e tomador repetem os do original, lidos do XML dele.
          </p>
          <p className="text-gray-500">
            O original precisa estar autorizado e ter no máximo 12 meses. A SEFAZ confere o limite de 10 complementos por original.
          </p>
        </>
      ) : (
        <>
          <h3 className="text-base font-semibold text-gray-900">Quando usar o substituto</h3>
          <p>
            Use para trocar um CT-e autorizado por outro, com valor ou tributos corrigidos e, se preciso, outro tomador.
            Carga e NF-e repetem as do original, lidas do XML dele.
          </p>
          <p className="text-gray-500">
            O prazo é de {CTE_SUBSTITUTE_SEFAZ_DAYS} dias da emissão do original; até {CTE_SUBSTITUTE_LIMIT_DAYS} dias
            (60 dias mais 168 horas) a decisão é da SEFAZ, e depois disso a emissão é recusada.
          </p>
          {deadline === 'sefaz' && (
            <p role="status" className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-gray-900">
              O original tem {days} dias. Passou dos {CTE_SUBSTITUTE_SEFAZ_DAYS} dias, então a SEFAZ decide se aceita.
            </p>
          )}
          {deadline === 'expired' && (
            <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-danger">
              O original tem {days} dias e o prazo para substituir acabou.
            </p>
          )}
        </>
      )}
      <p>
        {mode === 'complement' ? 'Precisa corrigir dados em vez de cobrar mais? ' : 'Só falta cobrar uma diferença de valor? '}
        <Link href={`/cte/${other}?key=${original.sk}`} className="font-medium text-brand-600 hover:text-brand-700">
          {other === 'complement' ? 'Emitir complemento' : 'Emitir substituto'}
        </Link>
      </p>
    </section>
  );
}

interface TakerState {
  change: boolean
  role: CteTakerRole | null
  person: PersonItemOut | null
}

/** Troca opcional do tomador (`indAlteraToma`): papel do original diferente do atual, ou outra pessoa do cadastro. */
function TakerChange({original, value, onChange}: { original: CteDetailOut; value: TakerState; onChange: (v: TakerState) => void }) {
  const current = original.taker?.role;
  const names: Partial<Record<CteTakerRole, string | undefined>> = {
    remetente: original.sender?.name, destinatario: original.recipient?.name,
    expedidor: original.expedidor?.name, recebedor: original.recebedor?.name,
  };
  // Expedidor e recebedor só existem se o original os traz (senão a api responde 422 taker_role_missing).
  const roles = CTE_SUBSTITUTE_TAKER_ROLES.filter((r) => r !== current
    && (r !== 'expedidor' || !!original.expedidor) && (r !== 'recebedor' || !!original.recebedor));
  return (
    <CteSection id="cte-taker-change" title="Trocar o tomador?"
                hint={`Hoje paga o frete: ${original.taker?.name ?? 'o tomador do original'}. Sem troca, o tomador continua o mesmo.`}>
      <label className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-gray-800">
        <input type="checkbox" className="size-5 accent-brand-600" checked={value.change}
               onChange={(e) => onChange({change: e.target.checked, role: null, person: null})}/>
        Informar outro tomador neste substituto
      </label>
      {value.change && (
        <>
          <div role="radiogroup" aria-labelledby="cte-taker-change" className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            {roles.map((r) => {
              const checked = value.role === r;
              return (
                <label key={r} className={cn(
                  'flex min-h-11 cursor-pointer flex-col justify-center rounded-lg border px-3 py-2 transition-colors',
                  'has-[:focus-visible]:ring-3 has-[:focus-visible]:ring-ring/50',
                  checked ? 'border-brand-500 bg-brand-50' : 'border-gray-200 hover:border-brand-300')}>
                  <input type="radio" name="cte-taker-change" value={r} checked={checked} className="sr-only"
                         onChange={() => onChange({...value, role: r, person: r === value.role ? value.person : null})}/>
                  <span className="text-sm font-medium text-gray-900">{CTE_TAKER_LABELS[r]}</span>
                  <span className="truncate text-xs text-gray-500">
                    {CTE_SUBSTITUTE_NEEDS_PERSON.has(r) ? (checked && value.person ? value.person.name : 'Escolher do cadastro')
                      : names[r] ?? 'Participante do original'}
                  </span>
                </label>
              );
            })}
          </div>
          {value.role && CTE_SUBSTITUTE_NEEDS_PERSON.has(value.role) && (
            <div className="space-y-1.5">
              <p className="text-sm font-medium text-gray-700">Pessoa que paga o frete</p>
              <PersonPicker value={value.person} onChange={(person) => onChange({...value, person})}
                            placeholder="Buscar pessoa cadastrada"/>
            </div>
          )}
        </>
      )}
    </CteSection>
  );
}

/** Complemento (tpCTe 1) e substituto (tpCTe 3): só o que o usuário decide; o resto sai do XML do original. */
export function CteChangeForm({mode, original}: { mode: CteChangeMode; original: CteDetailOut }) {
  const {selectedOrg} = useAuth();
  const router = useRouter();
  const qc = useQueryClient();
  const {config} = useFiscalConfig('cte', selectedOrg?.pk);
  const copy = COPY[mode];
  const charge = useCteCharge(original.origin ?? null, original.destination ?? null);
  const [taker, setTaker] = useState<TakerState>({change: false, role: null, person: null});
  const [failure, setFailure] = useState<EmitFailure | null>(null);
  const [failureStep, setFailureStep] = useState(STEP_REVIEW);

  const deadline = substituteWindow(daysSince(original.dh_emi ?? original.created_at));
  const substitute = mode === 'substitute';
  const items: StepIssue[] = [
    ...(substitute && deadline === 'expired' ? [{step: STEP_ORIGINAL, text: 'O prazo para substituir acabou'}] : []),
    ...(substitute && taker.change && !taker.role ? [{step: STEP_TAKER, text: 'Escolha o novo tomador'}] : []),
    ...(substitute && taker.role && CTE_SUBSTITUTE_NEEDS_PERSON.has(taker.role) && !taker.person
      ? [{step: STEP_TAKER, text: 'Escolha a pessoa que paga o frete'}] : []),
    ...chargeMissingItems(charge.state, mode === 'complement' ? 'valor complementar' : 'frete', STEP_VALUE),
    ...(charge.state.ibsCbs && !ibsCbsComplete(charge.state.ibsCbs)
      ? [{step: STEP_VALUE, text: 'Complete o IBS/CBS ou desligue o bloco'}] : []),
  ];
  const issues = useStepIssues(items);

  const emit = useMutation({
    mutationFn: () => {
      const base = buildChargeBody(charge.state);
      if (mode === 'complement') {
        return apiClient.complementCte(cteComplementBodySchema.parse({...base, original_keys: [original.sk]}));
      }
      return apiClient.substituteCte(cteSubstituteBodySchema.parse({
        ...base, original_key: original.sk,
        ...(taker.change && taker.role ? {
          change_taker: true,
          taker: CTE_SUBSTITUTE_NEEDS_PERSON.has(taker.role) && taker.person ? {role: taker.role, person_id: taker.person.sk} : {role: taker.role},
        } : {}),
      }));
    },
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.ctes.lists(selectedOrg?.pk)});
      void qc.invalidateQueries({queryKey: queryKeys.ctes.detail(original.sk)});
      toast.success(copy.success);
      router.push('/cte');
    },
    onError: (err) => {
      const f = cteDocFailure(err, copy.fallback);
      const step = cteErrorStep(err, f, ERROR_STEP_BY_CODE, ERROR_STEP_BY_TARGET, STEP_REVIEW);
      setFailure(f);
      setFailureStep(step);
      wizard.goTo(step);
    },
  });

  const failureAt = (step: string) => <EmitError failure={failureStep === step ? failure : null}/>;
  const cfopLabel = charge.freight.cfopOptions.find((o) => o.value === charge.freight.cfop)?.label;
  const takerText = taker.change && taker.role
    ? `${CTE_TAKER_LABELS[taker.role]}${taker.person ? ` · ${taker.person.name}` : ''}` : 'O mesmo do original';

  const steps: WizardStep[] = [
    // O alerta do prazo já está no próprio passo (WhenToUse), então não repete a pendência.
    {id: STEP_ORIGINAL, label: 'Original', validate: () => issues.validate(STEP_ORIGINAL), content: (
      <>
        <OriginalSummary doc={original}/>
        <WhenToUse mode={mode} original={original}/>
        {failureAt(STEP_ORIGINAL)}
      </>
    )},
    ...(substitute ? [{id: STEP_TAKER, label: 'Tomador', validate: () => issues.validate(STEP_TAKER), content: (
      <>
        <TakerChange original={original} value={taker} onChange={setTaker}/>
        <StepIssues items={issues.of(STEP_TAKER)}/>
        {failureAt(STEP_TAKER)}
      </>
    )}] : []),
    {id: STEP_VALUE, label: 'Valores', validate: () => issues.validate(STEP_VALUE), content: (
      <>
        <CteFreightFields {...charge.freight} title={copy.freightTitle} hint={copy.freightHint} totalLabel={copy.totalLabel}/>
        <CteChargeExtras charge={charge} xmlErrors={failure?.issues?.filter((i) => i.target === 'aut_xml').map((i) => i.text)}/>
        <StepIssues items={issues.of(STEP_VALUE)}/>
        {failureAt(STEP_VALUE)}
      </>
    )},
    {id: STEP_REVIEW, label: 'Revisão', content: (w) => (
      <>
        <p className="text-sm text-gray-600">Confira os dados antes de emitir o {copy.docLabel}.</p>
        <div className="grid gap-x-8 gap-y-4 md:grid-cols-2">
          <ReviewSection title="CT-e original" stepId={STEP_ORIGINAL} wizard={w} items={[
            ['Número', `${original.number} · série ${original.serie}`],
            ['Tomador atual', original.taker?.name ?? ''],
          ]}/>
          {substitute && <ReviewSection title="Tomador" stepId={STEP_TAKER} wizard={w} items={[['Novo tomador', takerText]]}/>}
          <ReviewSection title="Valores" stepId={STEP_VALUE} wizard={w} items={[
            [copy.totalLabel, charge.freight.total ? formatCurrency(charge.freight.total) : ''],
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
                  submitLabel={copy.emitLabel} onCancel={() => router.push('/cte')}/>
    </div>
  );
}
