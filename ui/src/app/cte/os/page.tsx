'use client';

import {CteFormPage} from '@/components/cte/CteFormPage';
import {CteOsForm} from '@/components/cte/CteOsForm';

export default function CteOsPage() {
  return (
    <CteFormPage crumb="Novo CT-e OS" title="Emitir CT-e OS"
                 subtitle="Conhecimento de Transporte para Outros Serviços (modelo 67): pessoas, valores ou excesso de bagagem"
                 what="emitir CT-e OS">
      <CteOsForm/>
    </CteFormPage>
  );
}
