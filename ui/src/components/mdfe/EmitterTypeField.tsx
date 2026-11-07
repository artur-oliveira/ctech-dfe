'use client';

import type {MdfeEmitterType} from '@/lib/types/api';

/** Rótulos do `tpEmit` no leiaute do MDF-e (MOC 3.00a, campo 7). */
export const EMITTER_TYPES: { value: MdfeEmitterType; label: string; description: string }[] = [
  {
    value: '1',
    label: 'Prestador de serviço de transporte',
    description: 'Transportadora que emitiu CT-e para a carga.',
  },
  {
    value: '2',
    label: 'Transportador de carga própria',
    description: 'Quem leva a própria mercadoria, com as NF-e dela. Também é o tipo de quem emite NF-e.',
  },
  {
    value: '3',
    label: 'Prestador que emitirá CT-e globalizado',
    description: 'Transportadora que leva NF-e e emite depois um CT-e globalizado relacionando as notas.',
  },
];

export interface EmitterContext {
  /** Tipo dos documentos da carga; este formulário manifesta NF-e. */
  docType: 'nfe' | 'cte'
  laterLoading: boolean
  /** UF de início diferente da de fim, ou uma delas no exterior. */
  interstate: boolean
}

/** O tipo que vale sem escolha: o do documento (NF-e é 2, CT-e é 1), como a api faz quando o campo vem vazio. */
export const defaultEmitterType = (ctx: EmitterContext): MdfeEmitterType =>
  ctx.docType === 'cte' && !ctx.laterLoading ? '1' : '2';

/**
 * Por que o tipo não vale nesta viagem (regras F14 a F17 e F24 do MOC), ou null. Espelha
 * `api/internal/services/mdfes/emitter_type.go`; a api confere de novo.
 */
export function emitterTypeBlock(type: MdfeEmitterType, ctx: EmitterContext): string | null {
  if (ctx.laterLoading && type !== '2') return 'Carregamento posterior exige carga própria (2).';
  if (type === '1' && ctx.docType === 'nfe') return 'Exige CT-e na carga; este manifesto leva NF-e.';
  if (type !== '1' && ctx.docType === 'cte') return 'Manifesta só NF-e; esta carga é de CT-e.';
  if (type === '3' && ctx.interstate) return 'Vale só para viagem dentro de um estado.';
  return null;
}

/**
 * Regra F113 (NT 2026.001): rodoviário com tipo 1 ou 3 e RNTRC informado exige ao menos um CIOT. O caso do tipo 2
 * com `tpTransp` e o do TAC dependem do cadastro da ANTT e ficam com a SEFAZ.
 */
export const ciotRequired = (type: MdfeEmitterType, rodoviario: boolean, rntrc: string | null | undefined): boolean =>
  rodoviario && !!rntrc && (type === '1' || type === '3');

export interface EmitterTypeFieldProps {
  value: MdfeEmitterType
  onChange: (value: MdfeEmitterType) => void
  context: EmitterContext
  /** Recusas da api para o tipo de emitente (F14 a F17), mostradas junto do campo. */
  errors?: string[]
}

/** Tipo do emitente (`ide/tpEmit`): três opções fixas, cada uma com o motivo quando não vale para esta viagem. */
export function EmitterTypeField({value, onChange, context, errors = []}: EmitterTypeFieldProps) {
  const current = emitterTypeBlock(value, context);
  return (
    <fieldset className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <legend className="sr-only">Tipo de emitente</legend>
      <p aria-hidden="true" className="text-xs font-semibold uppercase tracking-wider text-gray-400">Tipo de emitente</p>
      <div className="space-y-2">
        {EMITTER_TYPES.map((t) => {
          const blocked = emitterTypeBlock(t.value, context);
          const checked = t.value === value;
          return (
            <label key={t.value}
                   className={`flex min-h-11 items-start gap-3 rounded-lg border px-3 py-2.5 ${
                     checked ? 'border-brand-500 bg-brand-50' : 'border-gray-200'} ${
                     blocked && !checked ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer'}`}>
              <input type="radio" name="mdfe-emitter-type" value={t.value} checked={checked}
                     disabled={!!blocked && !checked} onChange={() => onChange(t.value)}
                     className="mt-1 size-4 shrink-0 accent-brand-600"/>
              <span className="min-w-0 text-sm">
                <span className="block font-medium text-gray-900">{t.value}. {t.label}</span>
                <span className="block text-xs text-gray-500">{blocked ?? t.description}</span>
              </span>
            </label>
          );
        })}
      </div>
      {current && <p role="alert" className="text-xs text-red-600">{current}</p>}
      {errors.map((e) => <p key={e} role="alert" className="text-xs text-red-600">{e}</p>)}
    </fieldset>
  );
}
