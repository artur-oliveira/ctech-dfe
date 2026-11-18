'use client';

import {useEffect, useMemo, useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import type {BpeLineItemOut} from '@/lib/types/api';

const PAGE_SIZE = 100;

/** Linhas do cadastro de uma variante: o seletor da venda lista só as que servem a ela. */
export function useBpeLines(variant: BpeLineItemOut['variant']) {
  const {selectedOrg} = useAuth();
  const query = useQuery({
    queryKey: queryKeys.bpeLines.list(selectedOrg?.pk),
    queryFn: () => apiClient.getBpeLines({limit: PAGE_SIZE}),
    enabled: !!selectedOrg,
  });
  const lines = useMemo(() => (query.data?.items ?? []).filter((l) => l.variant === variant), [query.data, variant]);
  const options = useMemo(() => lines.map((l) => ({value: l.sk, label: l.name})), [lines]);
  const bySk = useMemo(() => new Map(lines.map((l) => [l.sk, l])), [lines]);
  return {lines, options, bySk, isPending: query.isPending};
}

/** Nome do município pelo código IBGE; a tabela pesa centenas de kB, então carrega só quando um rótulo precisa dela. */
export function useCityName(): (code: string) => string {
  const [names, setNames] = useState<Map<string, string> | null>(null);
  useEffect(() => {
    let alive = true;
    void import('@/lib/data/cities').then((m) => {
      if (alive) setNames(new Map(m.CITIES.map((c) => [c.code, `${c.description} / ${c.uf}`])));
    });
    return () => { alive = false; };
  }, []);
  return (code) => names?.get(code) ?? code;
}
