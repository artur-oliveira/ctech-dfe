'use client';

import {Suspense} from 'react';
import Link from 'next/link';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig';
import {BpeEmitForm} from '@/components/bpe/BpeEmitForm';

function BpeEmitContent() {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/bpe" className="hover:text-brand-600">BP-e</Link>
            <span>/</span>
            <span className="text-gray-600">Novo BP-e</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">Emitir BP-e</h1>
          <p className="text-gray-500 text-sm mt-0.5">
            Escolha o tipo de bilhete e preencha a viagem, os valores e o pagamento. O servidor calcula totais, troco e tributos.
          </p>
        </div>
        <RequireFiscalConfig variant="bpe">
          <BpeEmitForm/>
        </RequireFiscalConfig>
      </div>
    </RootLayout>
  );
}

export default function BpeEmitPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <BpeEmitContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
