'use client';

import {EmitPageHeader} from '@/components/ui/emit-page-header';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig';
import {NfeEmitForm} from '@/components/nfe/NfeEmitForm';

function NfeEmitContent() {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <EmitPageHeader parent="NF-e" parentHref="/nfe" crumb="Nova NF-e" title="Emitir NF-e"
                        subtitle="Preencha os dados para emitir uma Nota Fiscal Eletrônica"/>
        
        <RequireFiscalConfig variant="nfe">
          <NfeEmitForm/>
        </RequireFiscalConfig>
      </div>
    </RootLayout>
  );
}

export default function NfeEmitPage() {
  return (
    <ProtectedRoute>
      <NfeEmitContent/>
    </ProtectedRoute>
  );
}
