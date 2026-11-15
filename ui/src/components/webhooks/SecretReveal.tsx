'use client';

import Link from 'next/link';
import {Button} from '@aoctech/ui';
import {CopyableCode} from '@/components/ui/copyable-code';

interface SecretRevealProps {
  secret: string
  /** "created" right after registering, "rotated" after a change; only the opening line differs. */
  reason: 'created' | 'rotated'
  onDone: () => void
}

/** The signing secret, shown once. There is no way back to it after "Guardei o segredo". */
export function SecretReveal({secret, reason, onDone}: SecretRevealProps) {
  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 space-y-4 overflow-y-auto px-6 py-5">
        <p className="text-sm text-gray-700">
          {reason === 'created' ? 'Webhook ativo. ' : 'Segredo trocado; o anterior deixou de valer. '}
          Este é o segredo que assina cada aviso. <strong>Guarde-o agora: ele não será mostrado de novo.</strong>
        </p>
        <div className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3">
          <p className="text-xs font-medium text-gray-500">Segredo de assinatura</p>
          <CopyableCode value={secret} className="mt-1 font-mono text-sm text-gray-900"/>
        </div>
        <p className="text-sm text-gray-600">
          Use-o no seu sistema para conferir a assinatura de cada chamada (cabeçalho <code className="font-mono text-xs">X-Ctech-Signature</code>).
          O guia mostra como em poucas linhas.{' '}
          <Link href="/guide/webhooks" className="font-medium text-brand-700 underline underline-offset-2" target="_blank">
            Ver no guia
          </Link>
        </p>
      </div>
      <div className="border-t border-gray-200 px-6 py-4 sm:flex sm:justify-end">
        <Button className="h-11 w-full sm:h-10 sm:w-auto" onClick={onDone}>Guardei o segredo</Button>
      </div>
    </div>
  );
}
