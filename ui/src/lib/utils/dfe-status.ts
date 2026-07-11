import type {QueryClient} from '@tanstack/react-query'
import type {PaginatedResponse} from '@/lib/types/api'

/**
 * Optimistically patches a document's status across every cached paginated list
 * page matching the given prefix (e.g. ['nfes', orgPk] / ['nfces', orgPk] /
 * ['mdfes', orgPk]).
 *
 * Used for transitional states like `cancel_pending` / `close_pending`: the list
 * reads from an eventually-consistent DynamoDB GSI, so a fresh refetch would
 * still return the pre-transition status. Patching the cache keeps the
 * transitional state visible until the WebSocket delivers the final status from
 * the worker.
 *
 * Generic over the status string so it works for any DFe list (NF-e/NFC-e/MDF-e).
 */
export function setDocStatusOptimistic<S extends string>(
  qc: QueryClient,
  listPrefix: readonly unknown[],
  accessKey: string,
  status: S,
): void {
  qc.setQueriesData<PaginatedResponse<{sk: string; status: S}>>({queryKey: listPrefix}, (old) => {
    if (!old?.items) return old
    return {
      ...old,
      items: old.items.map((it) => (it.sk === accessKey ? {...it, status} : it)),
    }
  })
}
