'use client';

import {Suspense} from 'react';
import {CteChangeForm} from '@/components/cte/CteChangeForm';
import {CteFormPage} from '@/components/cte/CteFormPage';
import {CteOriginalGate} from '@/components/cte/CteOriginalGate';

export default function CteSubstitutePage() {
  return (
    <CteFormPage crumb="Substituto de CT-e" title="Emitir substituto" subtitle="Substitui um CT-e autorizado por outro, com valor ou tomador corrigidos" what="emitir substituto de CT-e">
      <Suspense>
        <CteOriginalGate>{(original) => <CteChangeForm mode="substitute" original={original}/>}</CteOriginalGate>
      </Suspense>
    </CteFormPage>
  );
}
