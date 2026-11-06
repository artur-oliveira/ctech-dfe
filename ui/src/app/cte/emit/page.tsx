'use client';

import Link from 'next/link';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig';
import {CteEmitForm} from '@/components/cte/CteEmitForm';

function CteEmitContent() {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/cte" className="hover:text-brand-600">CT-e</Link>
            <span>/</span>
            <span className="text-gray-600">Novo CT-e</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">Emitir CT-e</h1>
          <p className="text-gray-500 text-sm mt-0.5">Conhecimento de Transporte rodoviário a partir de NF-e</p>
        </div>

        <RequireFiscalConfig variant="cte">
          <CteEmitForm/>
        </RequireFiscalConfig>
      </div>
    </RootLayout>
  );
}

export default function CteEmitPage() {
  return (
    <ProtectedRoute>
      <CteEmitContent/>
    </ProtectedRoute>
  );
}
