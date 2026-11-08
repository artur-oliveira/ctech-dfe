'use client';

import {Button} from '@aoctech/ui';
import {formatCurrency} from '@/lib/utils/helpers';

/** Barra de ação fixa das telas de emissão: o valor, o primeiro item que falta e o botão de emitir. */
export function CteActionBar({label, amount, missing, pending, buttonLabel, onEmit}: {
  label: string
  amount: string
  missing: string[]
  pending: boolean
  buttonLabel: string
  onEmit: () => void
}) {
  return (
    <div className="sticky bottom-(--bottomnav-height) -mx-4 flex items-center justify-between gap-3 border-t border-gray-200 bg-gray-50 px-4 py-3 md:-mx-8 md:px-8">
      <div className="min-w-0">
        <p className="text-xs text-gray-500">{label}</p>
        <p className="text-base font-semibold text-gray-900">{amount ? formatCurrency(amount) : 'R$ 0,00'}</p>
        {missing.length > 0 && (
          <p data-testid="cte-missing" className="truncate text-xs text-warning">
            {missing[0]}{missing.length > 1 ? ` (+${missing.length - 1})` : ''}
          </p>
        )}
      </div>
      <Button type="button" variant="brand" disabled={missing.length > 0 || pending} onClick={onEmit}>
        {pending ? 'Emitindo…' : buttonLabel}
      </Button>
    </div>
  );
}
