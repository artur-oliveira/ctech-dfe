// Brazilian state border adjacency graph.
// Used to generate BFS order for SEFAZ ConsultaCadastro fallback.
const ADJACENCY: Record<string, string[]> = {
  AC: ['AM', 'RO'],
  AL: ['BA', 'PE', 'SE'],
  AM: ['AC', 'AP', 'MT', 'PA', 'RO', 'RR'],
  AP: ['AM', 'PA'],
  BA: ['AL', 'GO', 'MG', 'MS', 'PE', 'PI', 'SE', 'TO'],
  CE: ['MA', 'PB', 'PE', 'PI', 'RN'],
  DF: ['GO', 'MG'],
  ES: ['BA', 'MG', 'RJ'],
  GO: ['BA', 'DF', 'MG', 'MS', 'MT', 'PA', 'TO'],
  MA: ['CE', 'PA', 'PI', 'TO'],
  MG: ['BA', 'DF', 'ES', 'GO', 'MS', 'RJ', 'SP'],
  MS: ['GO', 'MG', 'MT', 'PR', 'SP'],
  MT: ['AM', 'GO', 'MS', 'PA', 'RO', 'RR', 'TO'],
  PA: ['AM', 'AP', 'MA', 'MT', 'RO', 'RR', 'TO'],
  PB: ['CE', 'PE', 'RN'],
  PE: ['AL', 'BA', 'CE', 'PB', 'PI'],
  PI: ['BA', 'CE', 'MA', 'PE', 'TO'],
  PR: ['MS', 'RS', 'SC', 'SP'],
  RJ: ['ES', 'MG', 'SP'],
  RN: ['CE', 'PB'],
  RO: ['AC', 'AM', 'MT', 'PA'],
  RR: ['AM', 'MT', 'PA'],
  RS: ['PR', 'SC'],
  SC: ['PR', 'RS'],
  SE: ['AL', 'BA'],
  SP: ['MG', 'MS', 'PR', 'RJ'],
  TO: ['BA', 'GO', 'MA', 'MT', 'PA', 'PI'],
}

const ALL_UFS = Object.keys(ADJACENCY)

/**
 * Returns all Brazilian UFs in BFS order starting from `startUf`.
 * Unreachable UFs (shouldn't happen in a connected graph) are appended at the end.
 */
export function bfsUfOrder(startUf: string): string[] {
  const uf = startUf.toUpperCase()
  if (!ADJACENCY[uf]) return ALL_UFS

  const visited = new Set<string>([uf])
  const queue: string[][] = [[uf]]
  const result: string[] = [uf]

  while (queue.length > 0) {
    const level = queue.shift()!
    const nextLevel: string[] = []
    for (const node of level) {
      for (const neighbor of ADJACENCY[node] ?? []) {
        if (!visited.has(neighbor)) {
          visited.add(neighbor)
          result.push(neighbor)
          nextLevel.push(neighbor)
        }
      }
    }
    if (nextLevel.length > 0) queue.push(nextLevel)
  }

  // Append any UF not reachable (safety net)
  for (const u of ALL_UFS) {
    if (!visited.has(u)) result.push(u)
  }

  return result
}

/** True when the two UFs share a land border (or are the same UF). */
export function ufsBorder(a: string, b: string): boolean {
  const ua = a.toUpperCase()
  const ub = b.toUpperCase()
  if (ua === ub) return true
  return (ADJACENCY[ua] ?? []).includes(ub)
}

/**
 * Returns the intermediate UFs (excluding origin and destination) of the
 * shortest border path between `from` and `to`. Empty when they border, or when
 * either UF is unknown. Used to suggest `route` (percurso) on the MDF-e form.
 */
export function suggestRoute(from: string, to: string): string[] {
  const start = from.toUpperCase()
  const goal = to.toUpperCase()
  if (!ADJACENCY[start] || !ADJACENCY[goal] || start === goal) return []
  if (ufsBorder(start, goal)) return []

  const prev: Record<string, string | null> = {[start]: null}
  const queue = [start]
  while (queue.length > 0) {
    const node = queue.shift()!
    if (node === goal) break
    for (const neighbor of ADJACENCY[node] ?? []) {
      if (!(neighbor in prev)) {
        prev[neighbor] = node
        queue.push(neighbor)
      }
    }
  }
  if (!(goal in prev)) return []

  const path: string[] = []
  let cur: string | null = goal
  while (cur) {
    path.unshift(cur)
    cur = prev[cur]
  }
  // Drop origin + destination → only the intermediate states.
  return path.slice(1, -1)
}
