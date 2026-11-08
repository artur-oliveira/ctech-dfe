/**
 * Contingência (docs/specs/2026-11-08-contingencia.md): forma de emissão corrente por documento e estado dos
 * documentos emitidos fora do ar. Tudo que a UI diz sobre isso sai daqui, para a tela de configuração, o aviso
 * do app, as listas e os detalhes falarem igual.
 */

import type {DocVariant} from '@/lib/schemas/fiscal-configs';

export type EmissionType = 'normal' | 'svc' | 'epec' | 'offline'
/** Documentos que têm forma de emissão (a NFS-e não tem). */
export type EmissionDoc = Exclude<DocVariant, 'nfse'>

export const EMISSION_DOCS: readonly EmissionDoc[] = ['nfe', 'nfce', 'cte', 'mdfe'];
export const EMISSION_NORMAL: EmissionType = 'normal';

/** Justificativa (`xJust`): 15 a 256 caracteres, só ao sair de `normal`. */
export const EMISSION_REASON_MIN = 15;
export const EMISSION_REASON_MAX = 256;

/**
 * A API guarda a flag mas ainda não troca a forma de emissão sozinha (fatia E da spec, seção 9). Enquanto for falso,
 * o interruptor aparece desligado e travado: ligar prometeria algo que não acontece. Virar `true` quando a fatia sair.
 */
export const AUTO_CONTINGENCY_AVAILABLE = false;

export const DOC_LABELS: Record<EmissionDoc, string> = {nfe: 'NF-e', nfce: 'NFC-e', cte: 'CT-e', mdfe: 'MDF-e'};

/** Campos de contingência que o GET da configuração devolve (`emission_options` é a lista válida do documento). */
export interface EmissionFields {
  emission_type?: EmissionType | null
  emission_options?: EmissionType[] | null
  auto_contingency?: boolean
  contingency_since?: string | null
  contingency_reason?: string | null
}

/** Corpo do `PUT /{doc}-emission`. */
export interface EmissionBody {
  emission_type: EmissionType
  auto_contingency: boolean
  /** Obrigatória ao sair de `normal`; não vai ao voltar a `normal`. */
  justification?: string
}

const EMISSION_LABELS: Record<EmissionType, string> = {
  normal: 'Normal',
  svc: 'Contingência SVC',
  epec: 'EPEC',
  offline: 'Contingência offline',
};

/** O SVC muda de nome conforme o documento; quem decide o ambiente é a UF do emitente. */
const SVC_DETAIL: Partial<Record<EmissionDoc, string>> = {nfe: 'SVC-AN ou SVC-RS conforme a UF', cte: 'SVC-RS ou SVC-SP'};

const EMISSION_HINTS: Record<EmissionType, string> = {
  normal: 'A nota vai direto para a SEFAZ do seu estado.',
  svc: 'Usa o ambiente de contingência da SEFAZ, que autoriza a nota na hora.',
  epec: 'Registra um evento prévio na SEFAZ Nacional e a nota é transmitida depois.',
  offline: 'A nota é assinada e impressa agora, e a transmissão fica pendente até a SEFAZ voltar.',
};

export function emissionLabel(type: EmissionType, doc: EmissionDoc): string {
  const detail = type === 'svc' ? SVC_DETAIL[doc] : undefined;
  return detail ? `${EMISSION_LABELS.svc} (${detail})` : EMISSION_LABELS[type];
}

export const emissionHint = (type: EmissionType): string => EMISSION_HINTS[type];

/** Ausente é normal. */
export const currentEmission = (cfg: EmissionFields | null | undefined): EmissionType => cfg?.emission_type ?? EMISSION_NORMAL;

export const inContingency = (cfg: EmissionFields | null | undefined): boolean => currentEmission(cfg) !== EMISSION_NORMAL;

/** Estado de um documento emitido em contingência EPEC/offline (`contingency_state`). */
export type ContingencyState = 'pending' | 'authorized_late' | 'rejected_late' | 'overdue'

export const CONTINGENCY_PENDING: ContingencyState = 'pending';

export interface ContingencyDocFields {
  contingency_state?: ContingencyState | null
  contingency_deadline?: string | null
}

/** `@` vira a vogal de gênero, como em `dfe_status`: Emitid@ → Emitida (nota) / Emitido (manifesto, conhecimento). */
export const CONTINGENCY_STATE_META: Record<ContingencyState, { label: string; tone: 'warning' | 'success' | 'danger'; notice: string }> = {
  pending: {
    label: 'Contingência pendente',
    tone: 'warning',
    notice: 'Emitid@ em contingência, pendente de autorização',
  },
  authorized_late: {
    label: 'Autorizad@ após contingência',
    tone: 'success',
    notice: 'Emitid@ em contingência e autorizad@ depois pela SEFAZ. O documento é válido.',
  },
  rejected_late: {
    label: 'Rejeitad@ após contingência',
    tone: 'danger',
    notice: 'Emitid@ em contingência e rejeitad@ pela SEFAZ na transmissão. O documento não tem validade; emita outro corrigindo o motivo da rejeição.',
  },
  overdue: {
    label: 'Prazo da contingência vencido',
    tone: 'danger',
    notice: 'O prazo para transmitir o documento venceu sem autorização. Confira a situação na SEFAZ antes de emitir outro.',
  },
};

export const contingencyMeta = (state: string | null | undefined) =>
  state ? CONTINGENCY_STATE_META[state as ContingencyState] : undefined;

const genderize = (text: string, male: boolean) => text.replace(/@/g, male ? 'o' : 'a');
export const contingencyLabel = (state: ContingencyState, male = false): string =>
  genderize(CONTINGENCY_STATE_META[state].label, male);

/** Frase do aviso do detalhe; no `pending` acrescenta o prazo ("prazo até ..."), já formatado por quem chama. */
export function contingencyNotice(state: ContingencyState, male: boolean, deadline?: string): string {
  const text = genderize(CONTINGENCY_STATE_META[state].notice, male);
  return state === 'pending' && deadline ? `${text}; prazo até ${deadline}.` : text.endsWith('.') ? text : `${text}.`;
}

/** Códigos de problema (RFC 7807) da troca da forma de emissão. */
export const PROBLEM_EMISSION_TYPE_INVALID = 'emission_type_invalid';
export const PROBLEM_EMISSION_REASON_REQUIRED = 'emission_reason_required';

const QR_VERSION_FIELD = 'qr_code_version';
const SVC_WITHOUT_UF = 'sem SVC designada';

export const fiscalConfigHref = (doc: EmissionDoc): string => `/fiscal-config?tab=${doc}`;

export interface ContingencyFailureView {
  message: string
  action: { label: string; href: string }
}

/**
 * Recusas da API que a contingência explica: o 400 da NFC-e de emitente CPF com QR Code v2 (rejeição 444), o 400 de
 * SVC sem UF designada e os dois 422 da troca da forma de emissão. Nenhum dos 400 traz `code`, então casam pelo texto
 * da API (`nfce_qrcode.go`, `emission_mode.go`). `doc` é o documento da emissão que falhou.
 */
export function contingencyFailure(
  raw: unknown, detail: string, doc: EmissionDoc = 'nfe',
): ContingencyFailureView | null {
  const code = (raw as { code?: unknown } | undefined)?.code;
  if (code === PROBLEM_EMISSION_TYPE_INVALID) {
    return {
      message: 'Esta forma de emissão não existe para este documento. Escolha uma das opções da lista.',
      action: {label: 'Abrir configuração', href: fiscalConfigHref(doc)},
    };
  }
  if (code === PROBLEM_EMISSION_REASON_REQUIRED) {
    return {
      message: `Explique o motivo da contingência, com ${EMISSION_REASON_MIN} a ${EMISSION_REASON_MAX} caracteres.`,
      action: {label: 'Abrir configuração', href: fiscalConfigHref(doc)},
    };
  }
  if (detail.includes(QR_VERSION_FIELD)) {
    return {
      message: 'Emitente pessoa física (CPF) só pode emitir NFC-e com o QR Code versão 3 (rejeição 444). Mude a versão do QR Code na configuração da NFC-e e emita de novo.',
      action: {label: 'Abrir configuração da NFC-e', href: fiscalConfigHref('nfce')},
    };
  }
  if (detail.includes(SVC_WITHOUT_UF)) {
    return {
      message: 'A UF da sua empresa não tem SVC designada para este documento. Escolha outra forma de emissão ou volte para Normal.',
      action: {label: `Abrir configuração da ${DOC_LABELS[doc]}`, href: fiscalConfigHref(doc)},
    };
  }
  return null;
}
