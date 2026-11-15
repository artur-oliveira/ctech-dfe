'use client';

import {useEffect} from 'react';
import {useSearchParams} from 'next/navigation';

export const EVENT_QUERY_PARAM = 'event';
/** Classe da linha de evento destacada pela notificação (`?event=<sk>`). */
export const EVENT_HIGHLIGHT_CLASS = 'bg-brand-50 ring-1 ring-inset ring-brand-200';
export const eventRowId = (sk: string) => `event-${sk}`;

/**
 * SK do evento que a notificação pediu para destacar. Quando a lista de eventos
 * termina de carregar (`ready`), rola até a linha.
 */
export function useHighlightedEvent(ready: boolean): string | null {
  const sk = useSearchParams().get(EVENT_QUERY_PARAM);
  useEffect(() => {
    if (!ready || !sk) return;
    document.getElementById(eventRowId(sk))?.scrollIntoView({block: 'center', behavior: 'smooth'});
  }, [ready, sk]);
  return sk;
}
