/**
 * UFs cuja SEFAZ desativou o serviço de inutilização (NT 2023.002 v1.02, Ajustes SINIEF
 * 38/2026 e 40/2026), com o primeiro dia em que ele responde "998 Serviço Desativado".
 * Espelha `inutilizationDisabledFrom` em `api/internal/services/nfes/inutilization.go`.
 */
const INUTILIZATION_DISABLED_FROM: Readonly<Record<string, string>> = {
  AC: '2026-11-11',
  BA: '2026-11-11',
  SP: '2026-11-11',
};

export function isInutilizationDisabled(uf: string | null | undefined, now: Date = new Date()): boolean {
  const from = INUTILIZATION_DISABLED_FROM[(uf ?? '').trim().toUpperCase()];
  return from !== undefined && now.getTime() >= Date.parse(`${from}T00:00:00Z`);
}
