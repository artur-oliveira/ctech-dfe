'use client';

import {useCallback, useEffect, useRef, useState} from 'react';
import type {ReactNode} from 'react';
import type {FieldValues, UseFormReturn} from 'react-hook-form';
import {useFormState} from 'react-hook-form';
import {Button} from '@aoctech/ui';
import {cn} from '@/lib/utils';

export interface RailSection {
  id: string
  label: string
  /** Conteúdo da seção. Todas ficam montadas (só a ativa aparece), então campo e rascunho sobrevivem à navegação. */
  content: ReactNode
  /** Campos com alteração não salva. */
  dirty?: boolean
  /** Campos com erro de validação. */
  errors?: number
  /** Marca de atenção sem erro (ex.: em contingência). */
  alert?: boolean
  /** Seção com ação própria (Aplicar): a barra "Salvar alterações" some enquanto ela está aberta. */
  ownsSave?: boolean
}

function marker(s: RailSection): string {
  if (s.errors) return `${s.errors} ${s.errors === 1 ? 'erro' : 'erros'}`;
  if (s.dirty) return 'alterado';
  return '';
}

/**
 * Estado da seção ativa + roteamento de erro para formulários react-hook-form.
 * `sectionOf(campo)` diz a qual seção o campo pertence; `order` é a ordem das seções (a primeira com erro vence).
 */
export function useSettingsRail<T extends FieldValues>(
  form: UseFormReturn<T>,
  order: readonly string[],
  sectionOf: (field: string) => string,
  initial?: string,
) {
  const [active, setActive] = useState(initial ?? order[0]);
  const {errors, dirtyFields} = useFormState({control: form.control});
  // Seção condicional que sumiu (ex.: ABRASF ao trocar de provedor): volta à primeira.
  const current = order.includes(active) ? active : order[0];

  const tally = (names: string[]) => {
    const by: Record<string, number> = {};
    for (const n of names) by[sectionOf(n)] = (by[sectionOf(n)] ?? 0) + 1;
    return by;
  };
  const errorCount = tally(Object.keys(errors));
  const dirtyCount = tally(Object.keys(dirtyFields).filter((k) => !!(dirtyFields as Record<string, unknown>)[k]));

  /** Abre a seção e leva o foco ao campo (depois do render que a destampa). */
  const goToField = useCallback((name: string, section: string) => {
    setActive(section);
    requestAnimationFrame(() => {
      const el = document.querySelector<HTMLElement>(`[name="${name}"]`) ?? document.getElementById(name);
      el?.scrollIntoView?.({block: 'center'});
      el?.focus?.();
    });
  }, []);

  /** Para `handleSubmit(onValid, onInvalid)`: pula para a primeira seção com erro. */
  const onInvalid = useCallback((errs: Record<string, unknown>) => {
    const names = Object.keys(errs);
    const section = order.find((id) => names.some((n) => sectionOf(n) === id));
    if (!section) return;
    goToField(names.find((n) => sectionOf(n) === section) as string, section);
  }, [order, sectionOf, goToField]);

  return {active: current, setActive, errorCount, dirtyCount, goToField, onInvalid};
}

/**
 * Casca de configuração editada no lugar (revisitada, fora de ordem): trilho de seções à esquerda no desktop,
 * seletor no mobile, UMA seção visível por vez e barra de salvar fixa. Quem usa: `useSettingsRail` + `<SettingsRail>`.
 */
export function SettingsRail({sections, active, onSelect, status, saving, dirty, banner, className}: {
  sections: readonly RailSection[]
  active: string
  onSelect: (id: string) => void
  /** Texto da barra ("Última atualização ..."). */
  status: ReactNode
  saving?: boolean
  dirty?: boolean
  banner?: ReactNode
  className?: string
}) {
  const current = sections.find((s) => s.id === active) ?? sections[0];
  const heading = useFocusOnChange(current.id);

  return (
    <div className={cn('flex flex-col gap-3', className)}>
      {banner}
      <div className="grid gap-3 md:grid-cols-[13rem_minmax(0,1fr)] md:gap-6">
        <nav aria-label="Seções da configuração" className="min-w-0">
          <div className="md:hidden">
            <label htmlFor="settings-rail-select" className="mb-1 block text-xs font-medium text-gray-500">Seção</label>
            <select
              id="settings-rail-select"
              value={current.id}
              onChange={(e) => onSelect(e.target.value)}
              className="h-11 w-full rounded-md border border-gray-300 bg-white px-3 text-sm text-gray-900 focus-visible:outline-2 focus-visible:outline-ring"
            >
              {sections.map((s) => {
                const m = marker(s);
                return <option key={s.id} value={s.id}>{s.label}{m && ` (${m})`}</option>;
              })}
            </select>
          </div>
          <ul className="hidden flex-col gap-0.5 md:flex">
            {sections.map((s) => {
              const isActive = s.id === current.id;
              return (
                <li key={s.id}>
                  <button
                    type="button"
                    onClick={() => onSelect(s.id)}
                    aria-current={isActive ? 'true' : undefined}
                    className={cn(
                      'flex min-h-9 w-full items-center justify-between gap-2 rounded-md px-3 text-left text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring',
                      isActive ? 'bg-brand-50 text-brand-800' : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900',
                    )}
                  >
                    <span className="truncate">{s.label}</span>
                    {s.errors ? (
                      <span aria-label={marker(s)} className="shrink-0 rounded-full bg-red-100 px-1.5 text-xs font-semibold text-red-700">{s.errors}</span>
                    ) : s.dirty ? (
                      <span role="img" aria-label="alterado" className="size-2 shrink-0 rounded-full bg-brand-500"/>
                    ) : s.alert ? (
                      <span role="img" aria-label="atenção" className="size-2 shrink-0 rounded-full bg-amber-500"/>
                    ) : null}
                  </button>
                </li>
              );
            })}
          </ul>
        </nav>

        <div className="min-w-0">
          <h2 ref={heading} tabIndex={-1} className="mb-3 text-base font-semibold text-gray-900 outline-none">{current.label}</h2>
          {sections.map((s) => (
            <section key={s.id} data-rail-panel={s.id} hidden={s.id !== current.id} aria-label={s.label} className="flex flex-col gap-3">
              {s.content}
            </section>
          ))}
        </div>
      </div>

      {!current.ownsSave && (
        <div className="sticky bottom-(--bottomnav-height) z-10 -mx-4 flex items-center justify-between gap-3 border-t border-gray-200 bg-gray-50 px-4 py-3 md:static md:mx-0 md:bg-transparent md:px-0">
          <p className="min-w-0 text-xs text-muted-foreground" aria-live="polite">
            {dirty ? 'Alterações não salvas' : status}
          </p>
          <Button type="submit" disabled={saving} className="min-h-11 shrink-0 sm:min-h-0">
            {saving ? 'Salvando…' : 'Salvar alterações'}
          </Button>
        </div>
      )}
    </div>
  );
}

/** Leva o foco ao título quando a seção muda (não na primeira montagem). */
function useFocusOnChange(id: string) {
  const ref = useRef<HTMLHeadingElement>(null);
  const prev = useRef(id);
  useEffect(() => {
    if (prev.current === id) return;
    prev.current = id;
    // Se um erro roteou o foco para um campo, ele já está dentro do painel: não o roube.
    requestAnimationFrame(() => {
      if (!document.querySelector(`[data-rail-panel="${id}"] :focus`)) ref.current?.focus({preventScroll: true});
    });
  }, [id]);
  return ref;
}
