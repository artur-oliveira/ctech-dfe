'use client';

import {Suspense} from 'react';
import {useAuth} from '@/lib/hooks/useAuth';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {NoOrgBanner} from '@/components/ui/no-org-banner';
import {UnclosedList} from '@/components/mdfe/UnclosedList';

function UnclosedContent() {
  const {selectedOrg} = useAuth();
  return (
    <RootLayout>
      <div className="p-4 md:p-8 max-w-3xl">
        <div className="mb-6">
          <h1 className="text-2xl font-semibold text-gray-900">MDF-e não encerrados</h1>
          <p className="text-gray-500 text-sm mt-0.5">
            O que a SEFAZ ainda tem como aberto para este emitente, emitido aqui ou em outro sistema.
          </p>
        </div>
        {selectedOrg ? <UnclosedList orgPk={selectedOrg.pk}/> : <NoOrgBanner/>}
      </div>
    </RootLayout>
  );
}

export default function MdfeUnclosedPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <UnclosedContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
