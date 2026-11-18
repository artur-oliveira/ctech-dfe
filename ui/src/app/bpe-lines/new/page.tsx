'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryNewPage} from '@/components/registry/RegistryFormPage';
import {BpeLineForm} from '@/components/bpe/BpeLineForm';
import type {BpeLineCreate, BpeLineItemOut} from '@/lib/types/api';

export default function NewBpeLinePage() {
  return (
    <RegistryNewPage<BpeLineItemOut, BpeLineCreate>
      listHref="/bpe-lines"
      listLabel="Linhas do BP-e"
      singular="linha"
      listKey={queryKeys.bpeLines.list}
      create={(d) => apiClient.createBpeLine(d)}
      renderForm={(props) => <BpeLineForm {...props}/>}
    />
  );
}
