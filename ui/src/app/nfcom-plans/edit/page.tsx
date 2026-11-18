'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryEditPage} from '@/components/registry/RegistryFormPage';
import {NfcomPlanForm} from '@/components/nfcom/NfcomPlanForm';
import type {NfcomPlanCreate, NfcomPlanItemOut} from '@/lib/types/api';

export default function EditNfcomPlanPage() {
  return (
    <RegistryEditPage<NfcomPlanItemOut, NfcomPlanCreate>
      listHref="/nfcom-plans"
      listLabel="Planos da NFCom"
      singular="plano"
      listKey={queryKeys.nfcomPlans.list}
      detailKey={queryKeys.nfcomPlans.detail}
      fetchOne={(id) => apiClient.getNfcomPlan(id)}
      update={(id, d) => apiClient.updateNfcomPlan(id, d)}
      renderForm={(props) => <NfcomPlanForm {...props}/>}
    />
  );
}
