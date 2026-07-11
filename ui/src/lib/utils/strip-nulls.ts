/**
 * Deep-removes `undefined` from a value always, and `null` when `dropNull` is
 * true. Used to shrink request payloads: drop nulls on create (POST), keep them
 * on update (PUT/PATCH) where an explicit null means "clear this field".
 * Preserves falsy non-null values (0, '', false) and array element order.
 */
export function stripNulls<T>(value: T, dropNull: boolean): T {
  if (Array.isArray(value)) {
    return value.map((v) => stripNulls(v, dropNull)) as unknown as T
  }
  if (value !== null && typeof value === 'object') {
    const out: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      if (v === undefined) continue
      if (v === null && dropNull) continue
      out[k] = stripNulls(v, dropNull)
    }
    return out as T
  }
  return value
}

/**
 * Whether a request body is a plain JSON value safe to run `stripNulls` over.
 * Plain objects and arrays qualify; FormData/Blob/ArrayBuffer/URLSearchParams
 * and primitives do NOT — flattening them would corrupt the request (e.g. a
 * file upload would become `{}`).
 */
export function isStrippableBody(data: unknown): boolean {
  if (Array.isArray(data)) return true
  return data != null && typeof data === 'object' && (data as object).constructor === Object
}
