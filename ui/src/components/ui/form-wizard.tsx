'use client';

import {useCallback, useEffect, useRef, useState} from 'react';
import type {FormEvent, ReactNode} from 'react';
import {Button} from '@aoctech/ui';
import {cn} from '@/lib/utils';

export interface WizardStep {
  id: string
  label: string
  /** Conteúdo do passo. Todos os passos ficam montados (só o atual aparece), então nada se perde ao navegar. */
  content: ReactNode | ((wizard: Wizard) => ReactNode)
  /** Mostra os erros do passo e devolve se ele está válido. Sem ela o passo sempre deixa continuar. */
  validate?: () => boolean
}

export interface Wizard {
  index: number
  step: WizardStep
  isLast: boolean
  /** Passo mais distante já visitado: o indicador só deixa pular até ele. */
  reached: number
  /** Vai para um passo por id ou posição, sem validar (volta, "Editar" da revisão, erro da API). */
  goTo: (target: string | number) => void
  back: () => void
  /** Valida o passo atual e avança; devolve false (e fica) se houver erro. */
  next: () => boolean
}

/**
 * Estado do assistente. `steps` pode ser recriado a cada render (as `validate` fecham sobre o estado do
 * formulário); o hook sempre usa a versão mais recente.
 */
export function useWizard(steps: readonly WizardStep[]): Wizard {
  const [index, setIndex] = useState(0);
  const [reached, setReached] = useState(0);
  const latest = useRef(steps);
  useEffect(() => { latest.current = steps; });

  const goTo = useCallback((target: string | number) => {
    const i = typeof target === 'number' ? target : latest.current.findIndex((s) => s.id === target);
    if (i < 0 || i >= latest.current.length) return;
    setIndex(i);
    setReached((r) => Math.max(r, i));
  }, []);

  const next = useCallback(() => {
    if (latest.current[index].validate?.() === false) return false;
    if (index < latest.current.length - 1) goTo(index + 1);
    return true;
  }, [index, goTo]);

  const back = useCallback(() => setIndex((i) => Math.max(0, i - 1)), []);

  return {index, step: steps[index], isLast: index === steps.length - 1, reached, goTo, back, next};
}

/** Leva o foco ao primeiro campo com erro do passo visível (o erro `<p role="alert" id="{campo}-error">`). */
export function focusFirstInvalid() {
  requestAnimationFrame(() => {
    const alert = document.querySelector<HTMLElement>('[data-wizard-panel]:not([hidden]) [role="alert"]');
    if (!alert) return;
    const control = alert.id.endsWith('-error') ? document.getElementById(alert.id.slice(0, -'-error'.length)) : null;
    (control ?? alert).scrollIntoView?.({block: 'center'});
    control?.focus?.();
  });
}

function Progress({wizard, steps}: { wizard: Wizard; steps: readonly WizardStep[] }) {
  return (
    <ol className="flex items-center gap-1.5" aria-label="Passos">
      {steps.map((s, i) => {
        const done = i < wizard.index;
        const active = i === wizard.index;
        const canJump = i !== wizard.index && i <= wizard.reached;
        return (
          <li key={s.id} className="flex items-center gap-1.5" aria-current={active ? 'step' : undefined}>
            <button
              type="button"
              disabled={!canJump}
              onClick={() => wizard.goTo(i)}
              aria-label={`${i + 1}. ${s.label}${done ? ' (concluído)' : ''}`}
              className={cn(
                'flex h-7 items-center gap-1.5 rounded-full px-2.5 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
                active ? 'bg-brand-600 text-white' : done ? 'bg-brand-50 text-brand-700 hover:bg-brand-100' : 'bg-gray-100 text-gray-500',
                !canJump && !active && 'cursor-default',
              )}
            >
              <span aria-hidden="true">{done ? '✓' : i + 1}</span>
              <span className={cn(active ? 'sr-only' : 'sr-only md:not-sr-only')}>{s.label}</span>
            </button>
            {i < steps.length - 1 && <span aria-hidden="true" className={cn('h-px w-3 sm:w-5', done ? 'bg-brand-500' : 'bg-gray-200')}/>}
          </li>
        );
      })}
    </ol>
  );
}

/**
 * Casca de formulário em passos: cabeçalho compacto (indicador + nome do passo), um passo por tela e rodapé fixo
 * com Voltar/Continuar; no último passo o botão primário envia (`onSubmit`). Enter num campo avança em vez de enviar.
 * Quem usa: `const wizard = useWizard(steps)`, `<FormWizard wizard={wizard} steps={steps} .../>`.
 */
export function FormWizard({wizard, steps, onSubmit, submitLabel, submitting, onCancel, className, children}: {
  wizard: Wizard
  steps: readonly WizardStep[]
  /** Chamado no último passo; o formulário valida tudo e usa `wizard.goTo` para ir ao passo com erro. */
  onSubmit: () => void
  submitLabel: string
  submitting?: boolean
  onCancel?: () => void
  className?: string
  /** Fica dentro do `<form>`, depois do rodapé (modais de confirmação). */
  children?: ReactNode
}) {
  const heading = useRef<HTMLHeadingElement>(null);
  const first = useRef(true);
  useEffect(() => {
    if (first.current) { first.current = false; return; }
    heading.current?.focus({preventScroll: true});
  }, [wizard.index]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!wizard.isLast) {
      if (!wizard.next()) focusFirstInvalid();
      return;
    }
    onSubmit();
  };

  return (
    <form onSubmit={submit} noValidate className={cn('flex max-w-3xl flex-col gap-4 pb-4', className)}>
      <header className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h2 ref={heading} tabIndex={-1} className="text-base font-semibold text-gray-900 outline-none">
          <span className="mr-2 text-sm font-normal text-gray-500">Passo {wizard.index + 1} de {steps.length}</span>
          {wizard.step.label}
        </h2>
        <Progress wizard={wizard} steps={steps}/>
      </header>
      <p role="status" aria-live="polite" className="sr-only">Passo {wizard.index + 1} de {steps.length}: {wizard.step.label}</p>

      {steps.map((s, i) => (
        <section
          key={s.id}
          data-wizard-panel={s.id}
          hidden={i !== wizard.index}
          aria-label={s.label}
          className="flex flex-col gap-4 rounded-xl border border-gray-200 bg-white p-4 sm:p-5 motion-safe:animate-in motion-safe:fade-in-0 motion-safe:slide-in-from-bottom-1 motion-safe:duration-150"
        >
          {typeof s.content === 'function' ? s.content(wizard) : s.content}
        </section>
      ))}

      <div className="sticky bottom-(--bottomnav-height) -mx-4 flex items-center gap-2 border-t border-gray-200 bg-gray-50 px-4 pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] md:-mx-8 md:px-8">
        {onCancel && wizard.index === 0 ? (
          <Button type="button" variant="outline" className="min-h-11 sm:min-h-0" onClick={onCancel}>Cancelar</Button>
        ) : (
          <Button type="button" variant="outline" className="min-h-11 sm:min-h-0" onClick={wizard.back}>Voltar</Button>
        )}
        <Button type="submit" variant="brand" className="ml-auto min-h-11 flex-1 sm:flex-none sm:min-h-0" disabled={submitting}>
          {wizard.isLast ? (submitting ? 'Enviando…' : submitLabel) : 'Continuar'}
        </Button>
      </div>
      {children}
    </form>
  );
}

/** Bloco da revisão: título do passo, "Editar" que volta a ele e os pares rótulo/valor. */
export function ReviewSection({title, stepId, wizard, items}: {
  title: string
  stepId: string
  wizard: Wizard
  items: readonly (readonly [string, ReactNode])[]
}) {
  return (
    <section className="min-w-0">
      <div className="mb-1.5 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-900">{title}</h3>
        <button type="button" onClick={() => wizard.goTo(stepId)}
                className="py-2 text-xs font-medium text-brand-700 underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-ring sm:py-0">
          Editar<span className="sr-only"> {title}</span>
        </button>
      </div>
      <dl className="flex flex-col gap-y-1 text-sm">
        {items.map(([k, v]) => (
          <div key={k} className="flex min-w-0 gap-2">
            <dt className="shrink-0 text-gray-500">{k}</dt>
            <dd className="min-w-0 truncate text-gray-900">{v || 'Não informado'}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
