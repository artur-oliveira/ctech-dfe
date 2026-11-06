'use client';

import {PersonPicker} from '@/components/persons/PersonPicker';
import {
  CTE_TAKER_LABELS,
  CTE_TAKER_NEEDS_PERSON,
  CTE_TAKER_REASON_LABELS,
  CTE_TAKER_ROLES,
} from '@/lib/constants/cte';
import {cn} from '@/lib/utils';
import type {CteDeriveWarning, CteTakerRole, CteTakerSuggestion, PersonItemOut} from '@/lib/types/api';
import {CteSection, CteWarnings} from './CteBlock';

interface Props {
  role: CteTakerRole | null
  suggestion: CteTakerSuggestion | null
  senderName: string | null
  recipientName: string | null
  person: PersonItemOut | null
  onRole: (role: CteTakerRole) => void
  onPerson: (person: PersonItemOut | null) => void
  warnings: CteDeriveWarning[]
  /** `derive` já respondeu; antes disso remetente e destinatário ainda não são conhecidos. */
  loaded: boolean
}

/** Bloco 2: quem paga o frete (tomador). Escolha segmentada; a sugestão das notas vem marcada. */
export function CteTakerChoice({role, suggestion, senderName, recipientName, person, onRole, onPerson, warnings, loaded}: Props) {
  const who = (r: CteTakerRole): string | null => {
    if (r === 'remetente') return senderName;
    if (r === 'destinatario') return recipientName;
    return r === role && person ? person.name : 'Escolher do cadastro';
  };

  return (
    <CteSection id="cte-taker" title="Quem paga o frete?"
                hint="O tomador do serviço. Expedidor, recebedor e outro vêm do cadastro de pessoas.">
      <CteWarnings warnings={warnings}/>
      <div role="radiogroup" aria-labelledby="cte-taker" className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-5">
        {CTE_TAKER_ROLES.map((r) => {
          const checked = role === r;
          const name = who(r);
          // Remetente/destinatário sem nome: as notas divergem e nada foi escolhido
          // em Dados do transporte. Não há quem marcar.
          const unavailable = !CTE_TAKER_NEEDS_PERSON.has(r) && !name;
          const suggested = suggestion?.role === r;
          return (
            <label key={r}
                   className={cn(
                     'flex min-h-11 cursor-pointer flex-col justify-center rounded-lg border px-3 py-2 transition-colors',
                     'has-[:focus-visible]:ring-3 has-[:focus-visible]:ring-ring/50',
                     checked ? 'border-brand-500 bg-brand-50' : 'border-gray-200 hover:border-brand-300',
                     unavailable && 'cursor-not-allowed opacity-50 hover:border-gray-200',
                   )}>
              <input type="radio" name="cte-taker" value={r} checked={checked} disabled={unavailable}
                     onChange={() => onRole(r)} className="sr-only"/>
              <span className="text-sm font-medium text-gray-900">{CTE_TAKER_LABELS[r]}</span>
              <span className="truncate text-xs text-gray-500">{name ?? (!loaded ? 'Escolha as notas' : r === 'remetente'
                ? 'Notas com remetentes diferentes' : 'Notas com destinatários diferentes')}</span>
              {suggested && (
                <span className="mt-1 text-xs font-medium text-brand-700">
                  Sugerido · {CTE_TAKER_REASON_LABELS[suggestion.reason]}
                </span>
              )}
            </label>
          );
        })}
      </div>

      {role && CTE_TAKER_NEEDS_PERSON.has(role) && (
        <div className="space-y-1.5">
          <p className="text-sm font-medium text-gray-700">{CTE_TAKER_LABELS[role]} que paga o frete</p>
          <PersonPicker value={person} onChange={onPerson} placeholder="Buscar pessoa cadastrada"/>
        </div>
      )}
    </CteSection>
  );
}
