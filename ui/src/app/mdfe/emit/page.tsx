'use client';

import {EmitPageHeader} from '@/components/ui/emit-page-header';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig';
import {MdfeEmitForm} from '@/components/mdfe/MdfeEmitForm';

function MdfeEmitContent() {
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <EmitPageHeader parent="MDF-e" parentHref="/mdfe" crumb="Novo MDF-e" title="Emitir MDF-e"
                        subtitle="Manifesto Eletrônico de Documentos Fiscais"/>
        
        <RequireFiscalConfig variant="mdfe">
          <MdfeEmitForm/>
        </RequireFiscalConfig>
      </div>
    </RootLayout>
  );
}

export default function MdfeEmitPage() {
  return (
    <ProtectedRoute>
      <MdfeEmitContent/>
    </ProtectedRoute>
  );
}
