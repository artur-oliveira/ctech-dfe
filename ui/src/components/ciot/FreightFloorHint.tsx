'use client';

import {useQuery} from '@tanstack/react-query';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {floorUnavailable} from '@/lib/utils/ciot';
import {formatCurrency} from '@/lib/utils/helpers';

export interface FloorParams {
  cargo_type?: number
  axles?: number
  distance_km?: number
  empty_return?: boolean
}

/**
 * Dica do piso mínimo de frete. Sem tabela verificada a api responde 503 e a dica diz "indisponível"; nunca
 * bloqueia a declaração, e a consulta não é repetida (um 503 aqui não é pane).
 */
export function FreightFloorHint({params, freight}: { params: FloorParams; freight: string }) {
  const ready = !!params.cargo_type && !!params.axles && !!params.distance_km;
  const query = useQuery({
    queryKey: queryKeys.ciots.floor(params),
    queryFn: () => apiClient.getFreightFloor({
      cargo_type: params.cargo_type, axles: params.axles, distance_km: params.distance_km,
      // ponytail: composition e high_performance não saem: o mapeamento da composição ainda não existe na api (fase C7).
      empty_return: params.empty_return,
    }),
    enabled: ready,
    retry: false,
  });

  if (!ready) {
    return <p className="text-xs text-gray-500">Piso mínimo de frete: informe tipo de carga, eixos e distância para consultar.</p>;
  }
  if (query.isLoading) return <p className="text-xs text-gray-500">Consultando o piso mínimo de frete…</p>;
  if (query.error) {
    return (
      <p className="text-xs text-gray-500" data-testid="floor-unavailable">
        {floorUnavailable(query.error)
          ? 'Piso mínimo de frete indisponível: a tabela oficial ainda não foi verificada. A declaração segue normalmente.'
          : 'Não foi possível consultar o piso mínimo de frete agora. A declaração segue normalmente.'}
      </p>
    );
  }
  const floor = query.data;
  if (!floor) return null;
  const below = freight !== '' && Number(freight) < Number(floor.value);
  return (
    <p className={`text-xs ${below ? 'text-amber-800' : 'text-gray-600'}`} data-testid="floor-value">
      Piso mínimo de frete: <b>{formatCurrency(floor.value)}</b> (tabela {floor.table_version}).
      {below && ' O frete informado está abaixo do piso.'}
    </p>
  );
}
