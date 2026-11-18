'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryNewPage} from '@/components/registry/RegistryFormPage';
import {NfcomSubscriberForm} from '@/components/nfcom/NfcomSubscriberForm';
import type {NfcomSubscriberCreate, NfcomSubscriberItemOut} from '@/lib/types/api';

export default function NewNfcomSubscriberPage() {
  return (
    <RegistryNewPage<NfcomSubscriberItemOut, NfcomSubscriberCreate>
      listHref="/nfcom-subscribers"
      listLabel="Assinantes da NFCom"
      singular="assinante"
      listKey={queryKeys.nfcomSubscribers.list}
      create={(d) => apiClient.createNfcomSubscriber(d)}
      renderForm={(props) => <NfcomSubscriberForm {...props}/>}
    />
  );
}
