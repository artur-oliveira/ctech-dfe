import {WEBHOOK_EVENT_LABELS} from '@/lib/constants/webhooks';

const NOT_PUBLIC = 'O endereço precisa ser um site público na internet.';

/** IPv4 literals the server refuses (private, loopback, link-local, CGNAT, "this network"). */
function isPrivateIPv4(host: string): boolean {
  const m = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(host);
  if (!m) return false;
  const [a, b] = [Number(m[1]), Number(m[2])];
  return a === 10 || a === 127 || a === 0 || (a === 169 && b === 254) || (a === 172 && b >= 16 && b <= 31)
    || (a === 192 && b === 168) || (a === 100 && b >= 64 && b <= 127);
}

/**
 * Same rule as the server's webhooks.CheckURL, for an answer before the round
 * trip. The server stays the authority (it also checks where the name resolves);
 * this only saves a request on the obvious mistakes.
 */
export function validateWebhookUrl(raw: string): string | null {
  let u: URL;
  try {
    u = new URL(raw.trim());
  } catch {
    return 'Informe uma URL completa, começando com https://.';
  }
  if (u.protocol === 'http:') return 'O endereço precisa usar https.';
  if (u.protocol !== 'https:' || !u.hostname) return 'Informe uma URL completa, começando com https://.';
  if (u.username || u.password) return 'Não coloque usuário e senha na URL.';
  const host = u.hostname.replace(/^\[|\]$/g, '').replace(/\.$/, '').toLowerCase();
  const internalName = !host.includes('.') && !host.includes(':');
  if (internalName || host === 'localhost' || host.endsWith('.localhost') || host.endsWith('.local')
    || host.endsWith('.internal') || host.includes(':') || isPrivateIPv4(host)) {
    return NOT_PUBLIC;
  }
  return null;
}

/** Turns the registration challenge's reason (the server's text) into plain Portuguese. */
export function describeChallengeError(detail: string): string {
  if (/did not answer/.test(detail)) return 'O endereço não respondeu.';
  const answered = /answered HTTP (\d+)/.exec(detail);
  if (answered) {
    return `O endereço respondeu HTTP ${answered[1]}; esperávamos HTTP 200 devolvendo o código de verificação.`;
  }
  if (/hub\.challenge/.test(detail)) {
    return 'O endereço respondeu, mas não devolveu o código de verificação (hub.challenge) como corpo da resposta.';
  }
  return detail;
}

export interface FilterSummary {
  events: string
  companies: string
}

/** One line each for what an endpoint receives; empty filters mean everything. */
export function filterSummary(
  events: string[] | null | undefined,
  companyPks: string[] | null | undefined,
  companyNames: Map<string, string>,
): FilterSummary {
  const ev = events ?? [];
  const cp = companyPks ?? [];
  let companies = 'Todas as empresas';
  if (cp.length === 1) {
    companies = companyNames.get(cp[0]) ?? '1 empresa';
  } else if (cp.length > 1) {
    companies = `${cp.length} empresas`;
  }
  return {
    events: ev.length ? ev.map((e) => WEBHOOK_EVENT_LABELS[e] ?? e).join(', ') : 'Todos os eventos',
    companies,
  };
}
