import {XIcon} from 'lucide-react';

/** Largura reservada à direita do trigger para o X não cobrir o texto truncado (X + seta). */
export const CLEARABLE_TRIGGER_PADDING = 'pr-14!';

/** Botão X que limpa um campo opcional; fica sobre o trigger, à esquerda da seta. */
export function ClearButton({onClear}: { onClear: () => void }) {
  return (
    <button
      type="button"
      aria-label="Limpar seleção"
      onClick={onClear}
      className="absolute right-7 top-1/2 z-10 -translate-y-1/2 rounded-sm p-1 text-muted-foreground hover:text-foreground"
    >
      <XIcon className="size-3.5"/>
    </button>
  );
}
