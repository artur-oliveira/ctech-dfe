'use client';

import type {ReactNode} from 'react';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig';
import {NoPermission} from '@/components/ui/no-permission';
import {EmitPageHeader} from '@/components/ui/emit-page-header';
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
          <EmitPageHeader parent="CT-e" parentHref="/cte" crumb={crumb} title={title} subtitle={subtitle}/>
          {selectedOrg && !allowed ? <NoPermission what={what}/> : (
            <RequireFiscalConfig variant="cte">{children}</RequireFiscalConfig>
          )}
        </div>
      </RootLayout>
    </ProtectedRoute>
  );
}
