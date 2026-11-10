'use client';

import {useState} from 'react';
import type {StepIssue} from '@/lib/utils/cte';

/**
 * Pendências por passo dos formulários de CT-e em passos. Um passo só mostra as suas depois da primeira tentativa de
 * continuar, e a lista acompanha o formulário (some o que foi resolvido).
 */
export function useStepIssues(all: StepIssue[]) {
  const [attempted, setAttempted] = useState<ReadonlySet<string>>(() => new Set());
  return {
    /** `validate` do passo: marca como tentado e diz se está livre de pendências. */
    validate: (step: string) => {
      setAttempted((prev) => (prev.has(step) ? prev : new Set(prev).add(step)));
      return !all.some((i) => i.step === step);
    },
    /** Envio final: mostra as pendências de todos os passos e devolve o primeiro com pendência, na ordem dada. */
    firstInvalid: (order: readonly string[]) => {
      setAttempted(new Set(order));
      return order.find((s) => all.some((i) => i.step === s)) ?? null;
    },
    of: (step: string) => (attempted.has(step) ? all.filter((i) => i.step === step) : []),
  };
}

/** O erro leva `id="{campo}-error"` para o `focusFirstInvalid` da casca focar o campo dono. */
export function StepIssues({items}: { items: StepIssue[] }) {
  if (items.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-x-4 gap-y-0.5">
      {items.map((i, n) => (
        <p key={i.text} role="alert" id={`${i.field ?? `cte-issue-${n}`}-error`} className="text-sm text-red-600">{i.text}</p>
      ))}
    </div>
  );
}
