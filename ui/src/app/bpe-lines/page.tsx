'use client';

import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {RegistryListPage} from '@/components/registry/RegistryListPage';
import {TruckIcon} from '@/components/ui/icon';
import {SK_PREFIX} from '@/lib/constants/entity-keys';
import {BPE_VARIANTS} from '@/lib/constants/bpe';
import type {BpeLineItemOut} from '@/lib/types/api';

/** O que identifica a linha na lista: o percurso (comum), o voo (TA) ou o veículo (TM). */
function detail(l: BpeLineItemOut): string {
  if (l.variant === 'ta') return [l.flight_number, [l.from_airport, l.to_airport].filter(Boolean).join(' → ')].filter(Boolean).join(' ');
  if (l.variant === 'tm') return [l.prefix, l.plate].filter(Boolean).join(' ') || `${l.legs?.length ?? 0} trecho(s)`;
  return String(l.route_name ?? '-');
}

export default function BpeLinesPage() {
  return (
    <RegistryListPage<BpeLineItemOut>
      title="Linhas do BP-e"
      description="Percurso, voo ou grupo de viagem cadastrados uma vez; a venda só escolhe a linha e informa data, poltrona e valores"
      newLabel="Nova linha"
      newHref="/bpe-lines/new"
      editHref={(id) => `/bpe-lines/edit?id=${encodeURIComponent(id)}`}
      emptyTitle="Nenhuma linha"
      emptyDescription="Cadastre cada linha uma vez, com o percurso, o serviço e o imposto; na venda do bilhete basta escolher a linha e informar a data."
      icon={<TruckIcon width={20} height={20}/>}
      ariaLabel="Linhas do BP-e"
      skPrefix={SK_PREFIX.BPE_LINE}
      queryKey={queryKeys.bpeLines.list}
      fetchPage={(cursor) => apiClient.getBpeLines({cursor})}
      deleteItem={(id) => apiClient.deleteBpeLine(id)}
      columns={[
        {label: 'Nome', render: (e) => e.name},
        {label: 'Tipo', render: (e) => BPE_VARIANTS.find((v) => v.value === e.variant)?.label ?? '-'},
        {label: 'Percurso ou voo', render: detail},
      ]}
    />
  );
}
