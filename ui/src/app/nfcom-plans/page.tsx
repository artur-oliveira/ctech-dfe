'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryListPage} from '@/components/registry/RegistryListPage';
import {CreditCardIcon} from '@/components/ui/icon';
import {SK_PREFIX} from '@/lib/constants/entity-keys';
import {NFCOM_UNIT_OPTIONS} from '@/lib/constants/nfcom';
import type {NfcomPlanItemOut} from '@/lib/types/api';

export default function NfcomPlansPage() {
  return (
    <RegistryListPage<NfcomPlanItemOut>
      title="Planos da NFCom"
      description="Classificação, unidade, valor e tributação cadastrados uma vez; a fatura só escolhe o plano e a quantidade"
      newLabel="Novo plano"
      newHref="/nfcom-plans/new"
      editHref={(id) => `/nfcom-plans/edit?id=${encodeURIComponent(id)}`}
      emptyTitle="Nenhum plano"
      emptyDescription="Cadastre cada plano ou serviço uma vez, com a classificação e os tributos; na fatura basta escolher o plano e informar a quantidade."
      icon={<CreditCardIcon width={20} height={20}/>}
      ariaLabel="Planos da NFCom"
      skPrefix={SK_PREFIX.NFCOM_PLAN}
      queryKey={queryKeys.nfcomPlans.list}
      fetchPage={(cursor) => apiClient.getNfcomPlans({cursor})}
      deleteItem={(id) => apiClient.deleteNfcomPlan(id)}
      columns={[
        {label: 'Nome', render: (e) => e.name},
        {label: 'Código', render: (e) => String(e.code ?? '-')},
        {label: 'Unidade', render: (e) => NFCOM_UNIT_OPTIONS.find((o) => o.value === e.unit)?.label ?? '-'},
        {label: 'Valor padrão', render: (e) => (typeof e.unit_value === 'string' && e.unit_value ? `R$ ${Number(e.unit_value).toFixed(2).replace('.', ',')}` : 'Informado na fatura')},
      ]}
    />
  );
}
