/**
 * Runtime switch for the mock API. Lets the dev panel flip between success and
 * simulated-error flows without a rebuild. Auth endpoints (see SAFE_PATHS) are
 * excluded from forced errors so the session still establishes.
 */

import type {BillingScenario} from './fixtures';

export type MockMode = 'ok' | 'error'

export interface MockState {
  mode: MockMode
  status: number
  message: string
  /** If set, only paths containing one of these substrings error. null = all. */
  endpoints: string[] | null
  /**
   * Which subscription the account has. Separate from `mode` because these are
   * not failures — a past-due account is a working backend answering honestly,
   * and every one of these screens has to be reviewable without one.
   */
  billing: BillingScenario
  /** Emissão de NF-e e de MDF-e recusada localmente (422 `fiscal_prevalidation`), para revisar a tela de erro. */
  emitPrevalidation: boolean
  /** A organização do mock passa a CRT 3 (regime normal), que libera o IBS/CBS do CT-e. */
  regimeNormal: boolean
  /** NF-e em SVC e NFC-e offline nas configurações, para rever o aviso do app e o banner. */
  contingency: boolean
  /** Recusa da emissão ou da troca da forma de emissão: `invalid`, `reason`, `qr444` ou `svcUf`. */
  emissionProblem: 'invalid' | 'reason' | 'qr444' | 'svcUf' | null
  /** Recusa do impresso (DANFE, DANFC-e, DAMDFE, DACTE): formato inválido, não permitido ou XML ainda indisponível. */
  printProblem: 'invalidFormat' | 'notAllowed' | 'noXml' | null
  /** Recusa dos eventos F9 (reforma, entrega, ator interessado): layout, evento não autorizado, evento inexistente ou sem certificado. */
  eventProblem: 'invalid' | 'notAuthorized' | 'notFound' | 'noCertificate' | null
  /** CIOT: a declaração é recusada com 422 `ciot_validation` (`ciot-invalid`) ou o piso mínimo de frete vem calculado (`floor`). */
  ciotProblem: 'invalid' | null
  freightFloor: boolean
  /** A ANTT fora do ar (503) nas consultas de transportador (`antt-down`). */
  anttDown: boolean
}

const SAFE_PATHS = [
  '/v1.0/auth/me',
  '/v1.0/auth/roles',
  '/v1.0/organizations',
];

const DEFAULT: MockState = {
  mode: 'ok',
  status: 500,
  message: 'Erro simulado pelo mock API.',
  endpoints: null,
  billing: 'pro_active',
  emitPrevalidation: false,
  regimeNormal: false,
  contingency: false,
  emissionProblem: null,
  printProblem: null,
  eventProblem: null,
  ciotProblem: null,
  freightFloor: false,
  anttDown: false,
};

let state: MockState = {...DEFAULT};

export function getMockState(): MockState {
  return state;
}

export function setMockState(partial: Partial<MockState>): void {
  state = {...state, ...partial};
}

/** Reads `?mock=error[:status]` (e.g. `?mock=error:422`) or `?mock=prevalidation` from the URL on boot. */
export function initMockStateFromUrl(search: string): void {
  const params = new URLSearchParams(search);
  const raw = params.get('mock');
  if (!raw) return;
  if (raw === 'error' || raw.startsWith('error')) {
    const status = Number(raw.split(':')[1]) || 500;
    state = {...state, mode: 'error', status};
  } else if (raw === 'prevalidation') {
    state = {...state, emitPrevalidation: true};
  } else if (raw === 'regime-normal') {
    state = {...state, regimeNormal: true};
  } else if (raw === 'contingency') {
    state = {...state, contingency: true};
  } else if (raw === 'emission-422') {
    state = {...state, emissionProblem: 'invalid'};
  } else if (raw === 'emission-reason') {
    state = {...state, emissionProblem: 'reason'};
  } else if (raw === 'qr-444') {
    state = {...state, emissionProblem: 'qr444'};
  } else if (raw === 'svc-uf') {
    state = {...state, emissionProblem: 'svcUf'};
  } else if (raw === 'format-invalid') {
    state = {...state, printProblem: 'invalidFormat'};
  } else if (raw === 'format-not-allowed') {
    state = {...state, printProblem: 'notAllowed'};
  } else if (raw === 'xml-not-ready') {
    state = {...state, printProblem: 'noXml'};
  } else if (raw === 'event-invalid') {
    state = {...state, eventProblem: 'invalid'};
  } else if (raw === 'event-not-authorized') {
    state = {...state, eventProblem: 'notAuthorized'};
  } else if (raw === 'event-not-found') {
    state = {...state, eventProblem: 'notFound'};
  } else if (raw === 'no-certificate') {
    state = {...state, eventProblem: 'noCertificate'};
  } else if (raw === 'ciot-invalid') {
    state = {...state, ciotProblem: 'invalid'};
  } else if (raw === 'floor') {
    state = {...state, freightFloor: true};
  } else if (raw === 'antt-down') {
    state = {...state, anttDown: true};
  } else if (raw === 'ok') {
    state = {...state, mode: 'ok'};
  }
}

export function shouldError(path: string): boolean {
  if (state.mode !== 'error') return false;
  if (SAFE_PATHS.some((p) => path.startsWith(p))) return false;
  if (state.endpoints && !state.endpoints.some((e) => path.includes(e))) return false;
  return true;
}
