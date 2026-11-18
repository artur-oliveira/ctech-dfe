'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryEditPage} from '@/components/registry/RegistryFormPage';
import {NfcomSubscriberForm} from '@/components/nfcom/NfcomSubscriberForm';
import type {NfcomSubscriberCreate, NfcomSubscriberItemOut} from '@/lib/types/api';

export default function EditNfcomSubscriberPage() {
  return (
    <RegistryEditPage<NfcomSubscriberItemOut, NfcomSubscriberCreate>
      listHref="/nfcom-subscribers"
      listLabel="Assinantes da NFCom"
      singular="assinante"
      listKey={queryKeys.nfcomSubscribers.list}
      detailKey={queryKeys.nfcomSubscribers.detail}
      fetchOne={(id) => apiClient.getNfcomSubscriber(id)}
      update={(id, d) => apiClient.updateNfcomSubscriber(id, d)}
      renderForm={(props) => <NfcomSubscriberForm {...props}/>}
    />
  );
}
