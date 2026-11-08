'use client';

import {Suspense} from 'react';
import Link from 'next/link';
import {useSearchParams} from 'next/navigation';
import {useQuery} from '@tanstack/react-query';
import {Skeleton} from '@aoctech/ui';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {canRectifyCiot} from '@/lib/utils/ciot';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {CiotEmitForm} from '@/components/ciot/CiotEmitForm';
import {NoOrgBanner} from '@/components/ui/no-org-banner';
import {NoPermission} from '@/components/ui/no-permission';
import {useAuth} from '@/lib/hooks/useAuth';
import {useCiotPermissions} from '@/lib/hooks/useCiotPermissions';

/** Carrega o CIOT a retificar e só então monta o formulário (o estado inicial vem da declaração gravada). */
function RectifyForm({id}: { id: string }) {
  const {data, isLoading, error} = useQuery({queryKey: queryKeys.ciots.detail(id), queryFn: () => apiClient.getCiot(id)});
  if (isLoading) return <Skeleton className="h-64 max-w-3xl rounded-xl"/>;
  if (error || !data) return <p role="alert" className="text-sm text-red-700">CIOT não encontrado.</p>;
  if (!canRectifyCiot(data)) {
    return <p role="alert" className="text-sm text-red-700">Este CIOT não pode ser retificado agora (carga lotação, em andamento ou fora de vigor).</p>;
  }
  return <CiotEmitForm key={data.id} rectifyOf={data}/>;
}

function CiotEmitContent() {
  const {selectedOrg} = useAuth();
  const {canCreate, canUpdate} = useCiotPermissions();
  const rectifyId = useSearchParams().get('rectify');
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/ciot" className="hover:text-brand-600">CIOT</Link>
            <span>/</span>
            <span className="text-gray-600">{rectifyId ? 'Retificar CIOT' : 'Declarar CIOT'}</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">{rectifyId ? 'Retificar CIOT' : 'Declarar CIOT'}</h1>
          <p className="text-gray-500 text-sm mt-0.5">Código Identificador da Operação de Transporte (ANTT)</p>
        </div>
        {!selectedOrg ? <NoOrgBanner/>
          : rectifyId ? (canUpdate ? <RectifyForm id={rectifyId}/> : <NoPermission what="retificar CIOT"/>)
            : canCreate ? <CiotEmitForm/> : <NoPermission what="declarar CIOT"/>}
      </div>
    </RootLayout>
  );
}

export default function CiotEmitPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <CiotEmitContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
