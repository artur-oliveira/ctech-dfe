/** "14/11/2026 13:02", or "" when the value is missing or not a date. */
export function formatDateTime(iso: string | undefined | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleString('pt-BR', {dateStyle: 'short', timeStyle: 'short'});
}
