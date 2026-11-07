import {LENGTH_HINT_RATIO} from '@/lib/constants/layout-limits';

/**
 * Contador de caracteres que só aparece perto do limite do leiaute. Longe dele, um "12/60" embaixo de cada campo
 * é ruído; perto, é o que evita o 422 de texto longo na emissão.
 */
export function LengthHint({value, max}: { value: string | null | undefined; max: number }) {
  const length = value?.length ?? 0;
  if (length < max * LENGTH_HINT_RATIO) return null;
  return (
    <p className={`text-right text-xs tabular-nums ${length >= max ? 'text-warning' : 'text-gray-500'}`}>
      {length}/{max}
    </p>
  );
}
