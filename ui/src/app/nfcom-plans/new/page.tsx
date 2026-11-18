'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryNewPage} from '@/components/registry/RegistryFormPage';
import {NfcomPlanForm} from '@/components/nfcom/NfcomPlanForm';
import type {NfcomPlanCreate, NfcomPlanItemOut} from '@/lib/types/api';

export default function NewNfcomPlanPage() {
  return (
    <RegistryNewPage<NfcomPlanItemOut, NfcomPlanCreate>
      listHref="/nfcom-plans"
      listLabel="Planos da NFCom"
      singular="plano"
      listKey={queryKeys.nfcomPlans.list}
      create={(d) => apiClient.createNfcomPlan(d)}
      renderForm={(props) => <NfcomPlanForm {...props}/>}
    />
  );
}
