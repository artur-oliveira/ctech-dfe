'use client';

import type {ReactNode} from 'react';
import Link from 'next/link';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig';
import {NoPermission} from '@/components/ui/no-permission';
import {useAuth} from '@/lib/hooks/useAuth';
import {PERMISSION_CTE_CREATE} from '@/lib/constants/cte';
import {hasPermission} from '@/lib/utils/delivery-events';

/** Casca das telas de emissão de CT-e (complemento, substituto, OS): trilha, título, permissão e configuração fiscal. */
export function CteFormPage({crumb, title, subtitle, what, children}: {
  crumb: string
  title: string
  subtitle: string
  /** Complemento da frase "Você não tem permissão para ...". */
  what: string
  children: ReactNode
}) {
  const {selectedOrg} = useAuth();
  const allowed = hasPermission(selectedOrg?.permissions, PERMISSION_CTE_CREATE);
  return (
    <ProtectedRoute>
      <RootLayout>
        <div className="p-4 md:p-8">
          <div className="mb-6">
            <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
              <Link href="/cte" className="hover:text-brand-600">CT-e</Link>
              <span>/</span>
              <span className="text-gray-600">{crumb}</span>
            </div>
            <h1 className="text-2xl font-semibold text-gray-900">{title}</h1>
            <p className="text-gray-500 text-sm mt-0.5">{subtitle}</p>
          </div>
          {selectedOrg && !allowed ? <NoPermission what={what}/> : (
            <RequireFiscalConfig variant="cte">{children}</RequireFiscalConfig>
          )}
        </div>
      </RootLayout>
    </ProtectedRoute>
  );
}
