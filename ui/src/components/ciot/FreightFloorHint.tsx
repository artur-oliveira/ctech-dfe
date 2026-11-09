'use client';

import {useQuery} from '@tanstack/react-query';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {floorUnavailable} from '@/lib/utils/ciot';
import {formatISODateBR} from '@/lib/utils/dfe';
import {formatCurrency} from '@/lib/utils/helpers';

export interface FloorParams {
  cargo_type?: number
  axles?: number
  distance_km?: number
  empty_return?: boolean
  composition?: boolean
  high_performance?: boolean
}

/**
 * Dica do piso mínimo de frete: só aviso (a ANTT decide), nunca bloqueia a declaração. Se a api responder 503 a
 * dica diz "indisponível", e a consulta não é repetida (um 503 aqui não é pane). `operationDate` (yyyy-MM-dd)
 * anterior à vigência da tabela vira um aviso extra.
 */
export function FreightFloorHint({params, freight, operationDate}: { params: FloorParams; freight: string; operationDate?: string }) {
  const ready = !!params.cargo_type && !!params.axles && !!params.distance_km;
  const query = useQuery({
    queryKey: queryKeys.ciots.floor(params),
    queryFn: () => apiClient.getFreightFloor({
      cargo_type: params.cargo_type, axles: params.axles, distance_km: params.distance_km,
      empty_return: params.empty_return, composition: params.composition, high_performance: params.high_performance,
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
          ? 'Piso mínimo de frete indisponível: a tabela está marcada como não verificada. A declaração segue normalmente.'
          : 'Não foi possível consultar o piso mínimo de frete agora. A declaração segue normalmente.'}
      </p>
    );
  }
  const floor = query.data;
  if (!floor) return null;
  const below = freight !== '' && Number(freight) < Number(floor.value);
  const predates = !!operationDate && operationDate < floor.valid_from;
  return (
    <div className="space-y-1 text-xs" data-testid="floor-value-box">
      <p className={below ? 'text-amber-800' : 'text-gray-600'} data-testid="floor-value">
        Piso mínimo de frete: <b>{formatCurrency(floor.value)}</b> (tabela {floor.table_version}, vigente desde{' '}
        {formatISODateBR(floor.valid_from)}).
        {below && ' O frete informado está abaixo do piso; a ANTT decide na declaração.'}
      </p>
      <p className="text-gray-500" data-testid="floor-breakdown">
        Ida {formatCurrency(floor.outbound_value)}
        {floor.empty_return_cents > 0 && <>; retorno vazio {formatCurrency(floor.empty_return_value)}</>}
        ; coluna de {floor.axles_used} eixos.
      </p>
      {predates && (
        <p className="text-amber-800" data-testid="floor-predates">
          A data da viagem é anterior à vigência desta tabela ({formatISODateBR(floor.valid_from)}); o piso daquela data pode ser outro.
        </p>
      )}
    </div>
  );
}
