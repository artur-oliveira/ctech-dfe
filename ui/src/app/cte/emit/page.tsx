'use client';

import {CteEmitForm} from '@/components/cte/CteEmitForm';
import {CteFormPage} from '@/components/cte/CteFormPage';

export default function CteEmitPage() {
  return (
    <CteFormPage crumb="Novo CT-e" title="Emitir CT-e"
                 subtitle="Conhecimento de Transporte rodoviário a partir de NF-e"
                 what="emitir CT-e">
      <CteEmitForm/>
    </CteFormPage>
  );
}
