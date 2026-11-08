'use client';

import {useState} from 'react';
import {useAuth} from '@/lib/hooks/useAuth';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import type {CiotEnvironment} from '@/lib/types/api';

const MDFE_ENVIRONMENT_HOMOLOGATION = 2;

/**
 * Ambiente da tela de CIOT. Começa pelo da configuração fiscal de MDF-e (a mesma viagem), sem exigi-la,
 * e a pessoa pode trocar na própria tela.
 */
export function useCiotEnvironment(): [CiotEnvironment, (env: CiotEnvironment) => void] {
  const {selectedOrg} = useAuth();
  const {config} = useFiscalConfig('mdfe', selectedOrg?.pk);
  const [chosen, setChosen] = useState<CiotEnvironment | null>(null);
  const fromConfig: CiotEnvironment = config?.environment === MDFE_ENVIRONMENT_HOMOLOGATION ? 'hom' : 'prod';
  return [chosen ?? fromConfig, setChosen];
}
