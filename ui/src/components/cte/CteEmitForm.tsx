'use client';

import {useState} from 'react';
import {useRouter} from 'next/navigation';
import {keepPreviousData, useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {Button} from '@aoctech/ui';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {useDebounce} from '@/lib/hooks/useDebounce';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import type {EmitFailure} from '@/lib/billing/notice';
import {EmitConfirmModal} from '@/components/ui/emit-confirm-modal';
import {EmitError} from '@/components/ui/emit-error';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {CTE_DERIVE_DEBOUNCE_MS, CTE_TAKER_LABELS} from '@/lib/constants/cte';
import {cteEmitBodySchema} from '@/lib/schemas/cte';
import {
  buildCteEmitBody,
  cteEmitFailure,
  cteMissing,
  cteSummary,
  cteTransport,
  effectiveTakerRole,
  type CteFormState,
  type CteOverrides,
} from '@/lib/utils/cte';
import {formatCurrency} from '@/lib/utils/helpers';
import type {CteTakerRole, PersonItemOut} from '@/lib/types/api';
import {warningsFor} from './CteBlock';
import {CteNotesPicker} from './CteNotesPicker';
import {CteTakerChoice} from './CteTakerChoice';
import {CteFreightFields} from './CteFreightFields';
import {CteTransportData} from './CteTransportData';
import {CteChargeExtras, useCteCharge} from './CteCharge';

const sameKeys = (a: string[], b: string[]) => a.length === b.length && a.every((k, i) => k === b[i]);

/**
 * Emissão de CT-e rodoviário a partir de NF-e, numa tela só. Três decisões do
 * usuário (notas, tomador, valor); o resto vem do `derive` e aceita override.
 * Contrato: docs/specs/2026-11-05-cte-emissao-rodoviaria.md.
 */
export function CteEmitForm() {
  const {selectedOrg} = useAuth();
  const router = useRouter();
  const qc = useQueryClient();
  const {config} = useFiscalConfig('cte', selectedOrg?.pk);

  const [nfeKeys, setNfeKeys] = useState<string[]>([]);
  const [takerRole, setTakerRole] = useState<CteTakerRole | null>(null);
  const [takerPerson, setTakerPerson] = useState<PersonItemOut | null>(null);
  const [overrides, setOverrides] = useState<CteOverrides>({});
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [failure, setFailure] = useState<EmitFailure | null>(null);

  // Derivar a cada mudança de notas, com debounce: adicionar três notas seguidas
  // faz uma chamada, não três. A resposta anterior fica na tela (esmaecida)
  // enquanto a nova chega.
  const debouncedKeys = useDebounce(nfeKeys, CTE_DERIVE_DEBOUNCE_MS);
  const deriveQuery = useQuery({
    queryKey: queryKeys.ctes.derive(selectedOrg?.pk, debouncedKeys),
    queryFn: () => apiClient.deriveCte(debouncedKeys),
    enabled: !!selectedOrg && debouncedKeys.length > 0,
    placeholderData: keepPreviousData,
    retry: false,
  });
  const derive = nfeKeys.length > 0 ? deriveQuery.data : undefined;
  const deriving = nfeKeys.length > 0 && (!sameKeys(nfeKeys, debouncedKeys) || deriveQuery.isFetching);
  const deriveError = deriveQuery.isError && !deriving
    ? (deriveQuery.error instanceof Error ? deriveQuery.error.message : 'Não foi possível ler as notas.')
    : null;

  const transport = cteTransport(derive, overrides);
  // Origem e destino decidem o escopo do CFOP (5 ou 6).
  const charge = useCteCharge(transport.origin, transport.destination);
  const freightTotal = charge.freight.total;
  const form: CteFormState = {nfeKeys, takerRole, takerPerson, overrides, ...charge.state};
  const role = effectiveTakerRole(form, derive);
  const missing = deriveError ? ['Revise as notas escolhidas'] : cteMissing(form, derive, deriving);

  const emit = useMutation({
    mutationFn: () => {
      const body = cteEmitBodySchema.parse(buildCteEmitBody(form, derive));
      return apiClient.emitCte(body);
    },
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.ctes.lists(selectedOrg?.pk)});
      toast.success('CT-e enviado, aguardando autorização da SEFAZ.');
      router.push('/cte');
    },
    onError: (err) => setFailure(cteEmitFailure(err)),
  });

  const chooseRole = (r: CteTakerRole) => {
    setTakerRole(r);
    if (r !== takerRole) setTakerPerson(null);
  };

  return (
    <div className="max-w-3xl space-y-4">
      <HomologationBanner environment={config?.environment}/>

      <CteNotesPicker keys={nfeKeys} onChange={setNfeKeys} derive={derive} deriving={deriving}
                      error={deriveError} onRetry={() => void deriveQuery.refetch()} overrides={overrides}
                      warnings={warningsFor(derive?.warnings, 'notes')}/>

      <CteTakerChoice role={role} suggestion={derive?.taker_suggestion ?? null} loaded={!!derive}
                      senderName={transport.senderName} recipientName={transport.recipientName}
                      person={takerPerson} onRole={chooseRole} onPerson={setTakerPerson}
                      warnings={warningsFor(derive?.warnings, 'taker')}/>

      <CteFreightFields {...charge.freight}/>

      <CteTransportData derive={derive} overrides={overrides} onChange={setOverrides}
                        warnings={warningsFor(derive?.warnings, 'transport')}/>

      <CteChargeExtras charge={charge} xmlErrors={failure?.issues?.filter((i) => i.target === 'aut_xml').map((i) => i.text)}/>

      <EmitError failure={failure}/>

      <div className="sticky bottom-(--bottomnav-height) -mx-4 flex items-center justify-between gap-3 border-t border-gray-200 bg-gray-50 px-4 py-3 md:-mx-8 md:px-8">
        <div className="min-w-0">
          <p className="text-xs text-gray-500">Frete</p>
          <p className="text-base font-semibold text-gray-900">{freightTotal ? formatCurrency(freightTotal) : 'R$ 0,00'}</p>
          {missing.length > 0 && (
            <p data-testid="cte-missing" className="truncate text-xs text-warning">
              {missing[0]}{missing.length > 1 ? ` (+${missing.length - 1})` : ''}
            </p>
          )}
        </div>
        <Button type="button" variant="brand" disabled={missing.length > 0 || emit.isPending}
                onClick={() => {
                  setFailure(null);
                  setConfirmOpen(true);
                }}>
          {emit.isPending ? 'Emitindo…' : 'Emitir CT-e'}
        </Button>
      </div>

      <EmitConfirmModal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={() => {
          setConfirmOpen(false);
          emit.mutate();
        }}
        docLabel="CT-e"
        summary={[
          {label: 'Notas', value: derive ? cteSummary(derive, overrides) : '-'},
          {
            label: 'Tomador',
            value: role ? `${CTE_TAKER_LABELS[role]} · ${
              role === 'remetente' ? transport.senderName
                : role === 'destinatario' ? transport.recipientName : takerPerson?.name ?? '-'}` : '-',
          },
          {label: 'Frete', value: freightTotal ? formatCurrency(freightTotal) : '-'},
        ]}
      />
    </div>
  );
}
