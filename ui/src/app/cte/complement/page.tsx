'use client';

import {Suspense} from 'react';
import {CteChangeForm} from '@/components/cte/CteChangeForm';
import {CteFormPage} from '@/components/cte/CteFormPage';
import {CteOriginalGate} from '@/components/cte/CteOriginalGate';

export default function CteComplementPage() {
  return (
    <CteFormPage crumb="Complemento de CT-e" title="Emitir complemento" subtitle="Cobra uma diferença de valor sobre um CT-e já autorizado" what="emitir complemento de CT-e">
      <Suspense>
        <CteOriginalGate>{(original) => <CteChangeForm mode="complement" original={original}/>}</CteOriginalGate>
      </Suspense>
    </CteFormPage>
  );
}
