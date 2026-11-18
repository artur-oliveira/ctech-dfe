'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryListPage} from '@/components/registry/RegistryListPage';
import {CreditCardIcon} from '@/components/ui/icon';
import {SK_PREFIX} from '@/lib/constants/entity-keys';
import {NFCOM_SERVICE_TYPES, NFCOM_SUBSCRIBER_TYPES} from '@/lib/constants/nfcom';
import type {NfcomSubscriberItemOut} from '@/lib/types/api';

const label = (options: readonly { value: string; label: string }[], value: unknown) =>
  options.find((o) => o.value === value)?.label ?? '-';

export default function NfcomSubscribersPage() {
  return (
    <RegistryListPage<NfcomSubscriberItemOut>
      title="Assinantes da NFCom"
      description="Pessoa, código, tipo, serviço e contrato cadastrados uma vez; a fatura só escolhe o assinante"
      newLabel="Novo assinante"
      newHref="/nfcom-subscribers/new"
      editHref={(id) => `/nfcom-subscribers/edit?id=${encodeURIComponent(id)}`}
      emptyTitle="Nenhum assinante"
      emptyDescription="Cadastre o assinante uma vez, com a pessoa, o código e o contrato; na emissão da fatura basta escolhê-lo."
      icon={<CreditCardIcon width={20} height={20}/>}
      ariaLabel="Assinantes da NFCom"
      skPrefix={SK_PREFIX.NFCOM_SUBSCRIBER}
      queryKey={queryKeys.nfcomSubscribers.list}
      fetchPage={(cursor) => apiClient.getNfcomSubscribers({cursor})}
      deleteItem={(id) => apiClient.deleteNfcomSubscriber(id)}
      columns={[
        {label: 'Nome', render: (e) => e.name},
        {label: 'Código', render: (e) => String(e.code ?? '-')},
        {label: 'Tipo', render: (e) => label(NFCOM_SUBSCRIBER_TYPES, e.type)},
        {label: 'Serviço', render: (e) => label(NFCOM_SERVICE_TYPES, e.service_used)},
      ]}
    />
  );
}
