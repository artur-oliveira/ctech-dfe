'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryEditPage} from '@/components/registry/RegistryFormPage';
import {BpeLineForm} from '@/components/bpe/BpeLineForm';
import type {BpeLineCreate, BpeLineItemOut} from '@/lib/types/api';

export default function EditBpeLinePage() {
  return (
    <RegistryEditPage<BpeLineItemOut, BpeLineCreate>
      listHref="/bpe-lines"
      listLabel="Linhas do BP-e"
      singular="linha"
      listKey={queryKeys.bpeLines.list}
      detailKey={queryKeys.bpeLines.detail}
      fetchOne={(id) => apiClient.getBpeLine(id)}
      update={(id, d) => apiClient.updateBpeLine(id, d)}
      renderForm={(props) => <BpeLineForm {...props}/>}
    />
  );
}
