'use client';

import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {CTE_IBSCBS_CRT, CTE_ICMS_SN, CTE_SN_CRTS} from '@/lib/constants/cte';
import {cteCfopOptions, type CteChargeState, type CteFreightLine, type CteIbsCbsState, type CteIcmsState} from '@/lib/utils/cte';
import type {CtePlace} from '@/lib/types/api';
import {AuthorizedXmlFields, type AuthorizedXmlEntry} from '@/components/mdfe/AuthorizedXmlFields';
import {CteIbsCbsFields} from './CteIbsCbsFields';

/**
 * Estado do valor da prestação (total, componentes, CFOP, ICMS, IBS/CBS, autXML) que a emissão a partir de NF-e,
 * o complemento, o substituto e o CT-e OS compartilham. `origin` e `destination` decidem o escopo do CFOP (5 ou 6).
 */
export function useCteCharge(origin: CtePlace | null, destination: CtePlace | null) {
  const {selectedOrg} = useAuth();
  const [freightTotal, setFreightTotal] = useState('');
  const [components, setComponents] = useState<CteFreightLine[]>([]);
  // `null` até o usuário mexer: vale o padrão da organização (SN para CRT 1, 2 e 4).
  const [icmsEdit, setIcmsEdit] = useState<CteIcmsState | null>(null);
  const [cfopChoice, setCfopChoice] = useState('');
  const [ibsCbs, setIbsCbs] = useState<CteIbsCbsState | null>(null);
  const [authorizedXml, setAuthorizedXml] = useState<AuthorizedXmlEntry[]>([]);

  const {data: org} = useQuery({
    queryKey: queryKeys.organizations.detail(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getOrganization(selectedOrg!.pk),
    enabled: !!selectedOrg,
  });
  const crt = String(org?.person?.crt ?? '');
  // Só o regime normal emite IBS/CBS; Simples e MEI nem veem o bloco (422 ibscbs_regime_unsupported).
  const showIbsCbs = crt === CTE_IBSCBS_CRT;
  const icms: CteIcmsState = icmsEdit ?? {cst: CTE_SN_CRTS.has(crt) ? CTE_ICMS_SN : '', p_icms: '', p_red_bc: ''};

  // Um CFOP que saiu do escopo (origem ou destino mudou) deixa de valer.
  const cfopOptions = cteCfopOptions(origin, destination);
  const cfop = cfopOptions.some((o) => o.value === cfopChoice) ? cfopChoice : '';

  const state: CteChargeState = {
    freightTotal, components, icms, cfop, ibsCbs: showIbsCbs ? ibsCbs : null, authorizedXml,
  };
  return {
    state, showIbsCbs, ibsCbs, setIbsCbs, authorizedXml, setAuthorizedXml,
    freight: {
      total: freightTotal, onTotal: setFreightTotal, components, onComponents: setComponents,
      icms, onIcms: setIcmsEdit, cfop, onCfop: setCfopChoice, cfopOptions,
    },
  };
}

export type CteCharge = ReturnType<typeof useCteCharge>

/** IBS/CBS (só CRT 3) e quem mais pode baixar o XML, logo abaixo do valor. */
export function CteChargeExtras({charge, xmlErrors}: { charge: CteCharge; xmlErrors?: string[] }) {
  return (
    <section className="space-y-4" aria-label="IBS e CBS e acesso ao XML">
      {charge.showIbsCbs && (
        <CteIbsCbsFields value={charge.ibsCbs} onChange={charge.setIbsCbs} freightTotal={charge.freight.total}/>
      )}
      <AuthorizedXmlFields entries={charge.authorizedXml} onChange={charge.setAuthorizedXml} errors={xmlErrors}/>
    </section>
  );
}
