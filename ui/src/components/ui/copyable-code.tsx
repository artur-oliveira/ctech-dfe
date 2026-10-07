'use client';

import {useState} from 'react';
import {Copy, Check} from 'lucide-react';

interface CopyableCodeProps {
  /** Valor mostrado e copiado (chave de acesso, id_dps, etc.) — já sem formatação. */
  value: string
  className?: string
}

/** Monospace code line com ícone de copiar que aparece no hover (sempre visível no touch). */
export function CopyableCode({value, className = 'text-xs text-gray-400 font-mono mt-1'}: CopyableCodeProps) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard API indisponível (contexto não seguro, permissão negada): nada a fazer.
    }
  }

  return (
    <button type="button" onClick={handleCopy} title="Copiar"
            className={`group inline-flex max-w-full items-center gap-1.5 text-left ${className}`}>
      <span className="break-all">{value}</span>
      {copied
        ? <Check className="h-3.5 w-3.5 shrink-0 text-brand-600"/>
        : <Copy className="h-3.5 w-3.5 shrink-0 text-gray-300 opacity-100 sm:opacity-0 sm:group-hover:opacity-100"/>}
    </button>
  );
}
