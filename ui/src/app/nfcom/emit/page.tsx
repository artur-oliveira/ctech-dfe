'use client';

import {Suspense} from 'react';
import Link from 'next/link';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig';
import {NfcomEmitForm} from '@/components/nfcom/NfcomEmitForm';

function NfcomEmitContent() {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/nfcom" className="hover:text-brand-600">NFCom</Link>
            <span>/</span>
            <span className="text-gray-600">Nova NFCom</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">Emitir NFCom</h1>
          <p className="text-gray-500 text-sm mt-0.5">
            Preencha o cliente, o assinante, os itens e a fatura. O servidor calcula valores, tributos e totais.
          </p>
        </div>
        <RequireFiscalConfig variant="nfcom">
          <NfcomEmitForm/>
        </RequireFiscalConfig>
      </div>
    </RootLayout>
  );
}

export default function NfcomEmitPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfcomEmitContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
