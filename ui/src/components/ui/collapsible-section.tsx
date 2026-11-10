'use client';

import {useId, useState} from 'react';
import {ChevronDown} from 'lucide-react';
import {cn} from '@/lib/utils';

/**
 * Único estilo de disclosure dos formulários e passos de wizard: linha com título em caixa de
 * frase, resumo opcional à direita (o que está dentro, sem abrir), marca de erro/preenchidos e
 * chevron. Fechado por padrão, para o fluxo principal ficar limpo; o botão nativo com
 * aria-expanded deixa a região alcançável por teclado e leitor de tela.
 */
export function CollapsibleSection({
  title,
  description,
  defaultOpen = false,
  badge,
  summary,
  summaryTestId,
  keepMounted = false,
  children,
  className,
}: {
  title: string
  description?: string
  /** Estado inicial; enquanto o usuário não clica, mudanças nele valem (ex.: campo que passa a ser obrigatório). */
  defaultOpen?: boolean
  /** Marca fechada: quantos campos com erro ou preenchidos há aqui dentro. Sem
   *  ela, uma seção colapsada esconde o erro que impede o formulário de salvar. */
  badge?: React.ReactNode
  /** Resumo curto do conteúdo, ao lado do chevron (ex.: "Simples Nacional", "Não informado"). */
  summary?: React.ReactNode
  summaryTestId?: string
  /** Mantém os filhos montados quando fechada (estado local, foco no primeiro erro). */
  keepMounted?: boolean
  children: React.ReactNode
  className?: string
}) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const open = userOpen ?? defaultOpen;
  const panelId = useId();

  return (
    <div className={cn('rounded-lg border border-gray-200 bg-white', className)}>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setUserOpen(!open)}
        className="flex min-h-11 w-full items-center justify-between gap-3 px-3 py-2 text-left focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
      >
        <span className="min-w-0">
          <span className="flex items-center gap-1.5">
            <span className="text-sm font-medium text-gray-900">{title}</span>
            {badge}
          </span>
          {description && <span className="mt-0.5 block text-xs text-gray-500">{description}</span>}
        </span>
        <span className="flex min-w-0 items-center gap-2">
          {summary != null && <span data-testid={summaryTestId} className="truncate text-sm text-gray-600">{summary}</span>}
          <ChevronDown className={cn('size-4 shrink-0 text-gray-400 transition-transform duration-200', open && 'rotate-180')} aria-hidden="true"/>
        </span>
      </button>
      {(open || keepMounted) && (
        <div id={panelId} className={cn('space-y-3 border-t border-gray-100 p-3', !open && 'hidden')}>
          {children}
        </div>
      )}
    </div>
  );
}
