import type React from 'react';
import {TriangleAlert} from 'lucide-react';
import {CTE_WARNING_ACTIONS, CTE_WARNING_BLOCK, type CteBlock} from '@/lib/constants/cte';
import type {CteDeriveWarning} from '@/lib/types/api';

/** Avisos do `derive` que pertencem a este bloco; código desconhecido cai no bloco das notas. */
export const warningsFor = (warnings: CteDeriveWarning[] | undefined, block: CteBlock) =>
  (warnings ?? []).filter((w) => (CTE_WARNING_BLOCK[w.code] ?? 'notes') === block);

/** Callouts dentro do bloco a que o aviso se refere, com a ação que o resolve. */
export function CteWarnings({warnings}: { warnings: CteDeriveWarning[] }) {
  if (warnings.length === 0) return null;
  return (
    <ul className="space-y-2">
      {warnings.map((w, i) => (
        <li key={`${w.code}-${w.access_key ?? i}`} role="status"
            className="flex gap-2.5 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-sm">
          <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warning" aria-hidden="true"/>
          <div className="min-w-0">
            <p className="font-medium text-gray-900">{w.message}</p>
            {CTE_WARNING_ACTIONS[w.code] && <p className="text-gray-700">{CTE_WARNING_ACTIONS[w.code]}</p>}
          </div>
        </li>
      ))}
    </ul>
  );
}

/**
 * Superfície do bloco: cartão na página, plana dentro de um passo do assistente (o passo já é a superfície).
 */
export const CTE_SURFACE = 'rounded-xl border border-gray-200 bg-white p-4 md:p-5 in-data-[wizard-panel]:rounded-none in-data-[wizard-panel]:border-0 in-data-[wizard-panel]:bg-transparent in-data-[wizard-panel]:p-0 md:in-data-[wizard-panel]:p-0';

/**
 * Bloco da tela de emissão: título como pergunta (não eyebrow), apoio opcional
 * e o conteúdo. Mesma superfície dos cartões do app: branco, hairline, 14px.
 */
export function CteSection({id, title, hint, aside, children}: {
  id: string
  title: string
  hint?: string
  /** Fica à direita do título no desktop e abaixo dele no mobile. */
  aside?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section aria-labelledby={id} className={`${CTE_SURFACE} space-y-4 in-data-[wizard-panel]:space-y-3`}>
      <div className="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
        <div className="min-w-0">
          <h3 id={id} className="text-base font-semibold text-gray-900">{title}</h3>
          {hint && <p className="mt-0.5 text-sm text-gray-500">{hint}</p>}
        </div>
        {aside}
      </div>
      {children}
    </section>
  );
}
