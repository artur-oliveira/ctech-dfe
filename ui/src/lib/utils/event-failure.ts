import {ApiError} from '@/lib/api/client';
import {CTE_EVENT_ERRORS} from '@/lib/constants/cte';

/** Falha de cancelamento ou evento (CT-e e NF-e): o `code` da api vira frase em pt-BR; o resto segue a mensagem da api. */
export function cteEventFailureMessage(err: unknown, fallback: string): string {
  const code = err instanceof ApiError ? (err.raw as {code?: unknown} | undefined)?.code : undefined;
  if (typeof code === 'string' && CTE_EVENT_ERRORS[code]) return CTE_EVENT_ERRORS[code];
  return err instanceof Error ? err.message : fallback;
}
