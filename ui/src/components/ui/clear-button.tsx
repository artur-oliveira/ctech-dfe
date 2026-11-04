import {XIcon} from 'lucide-react';

/** Espaço reservado à direita do TEXTO (não do botão: a seta não pode sair do lugar) para o X não cobrir o texto truncado. */
export const CLEARABLE_TEXT_PADDING = 'pr-7';
/** Mesmo espaço para o `Select` de @aoctech/ui, cujo texto é o primeiro span do trigger (literal inteiro para o Tailwind enxergar). */
export const CLEARABLE_SELECT_TEXT_PADDING = '[&>span:first-child]:pr-7';

/** Botão X que limpa um campo opcional; fica sobre o trigger, à esquerda da seta. */
export function ClearButton({onClear}: { onClear: () => void }) {
  return (
    <button
      type="button"
      aria-label="Limpar seleção"
      onClick={onClear}
      className="absolute right-8 top-1/2 z-10 -translate-y-1/2 rounded-sm p-1 text-muted-foreground hover:text-foreground"
    >
      <XIcon className="size-3.5"/>
    </button>
  );
}
