'use client';

import {cn} from '@/lib/utils';

interface CheckboxFieldProps {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
  label: string
  disabled?: boolean
  id?: string
  className?: string
  /**
   * Para ficar na MESMA linha de um input: a célula tem a altura do input (44px, 32px no
   * modo compacto), então `items-end` alinha a linha toda. Sem isso, o checkbox embaixo do
   * input deixa a coluna mais alta e o `items-end` empurra os campos vizinhos para baixo.
   */
  inline?: boolean
}

/** Checkbox com rótulo clicável; único lugar com o tamanho, a cor e a área de toque do padrão. */
export function CheckboxField({checked, onCheckedChange, label, disabled, id, className, inline}: CheckboxFieldProps) {
  return (
    <label
      htmlFor={id}
      className={cn(
        'flex items-center gap-2 text-xs text-gray-600',
        disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer',
        inline ? 'h-11 shrink-0 whitespace-nowrap in-data-[density=compact]:h-8' : 'min-h-11 sm:min-h-0',
        className,
      )}
    >
      <input type="checkbox" id={id} checked={checked} disabled={disabled}
             onChange={(e) => onCheckedChange(e.target.checked)}
             className="size-4 rounded border-gray-300 text-brand-600"/>
      {label}
    </label>
  );
}
