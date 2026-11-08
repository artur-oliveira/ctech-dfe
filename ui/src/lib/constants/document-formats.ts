/**
 * Formatos impressos selecionáveis (parâmetro `format` do DANFE, DANFC-e e DAMDFE). Os valores vêm do backend
 * (`api/internal/services/documents/formats.go`); sem escolha, a UI não envia `format` e a API usa o do próprio XML.
 * O DACTE tem formato único.
 */
export interface PrintFormat {
  value: string
  label: string
}

export const PRINT_FORMATS = {
  nfe: [
    {value: 'retrato', label: 'DANFE Retrato A4'},
    {value: 'paisagem', label: 'DANFE Paisagem A4'},
    {value: 'simplificado', label: 'Simplificado (bobina)'},
    {value: 'etiqueta', label: 'Etiqueta'},
  ],
  nfce: [
    {value: 'completo', label: 'DANFC-e Completo'},
    {value: 'resumido', label: 'DANFC-e Resumido'},
    {value: 'mensagem', label: 'Mensagem eletrônica'},
  ],
  mdfe: [
    {value: 'retrato', label: 'DAMDFE Retrato'},
    {value: 'paisagem', label: 'DAMDFE Paisagem'},
  ],
} as const satisfies Record<string, readonly PrintFormat[]>;

/** Formatos que o MOC DANFE 2.11 veda na NF-e em EPEC. */
const FORMATS_BLOCKED_IN_EPEC = ['simplificado', 'etiqueta'];
const TP_EMIS_EPEC = '4';
/** O tpEmis é a posição 35 da chave de acesso. */
const TP_EMIS_KEY_INDEX = 34;

/** Opções do DANFE: sem simplificado e etiqueta quando a chave é de uma NF-e em EPEC. */
export function nfeFormats(accessKey: string): readonly PrintFormat[] {
  if (accessKey[TP_EMIS_KEY_INDEX] !== TP_EMIS_EPEC) return PRINT_FORMATS.nfe;
  return PRINT_FORMATS.nfe.filter((f) => !FORMATS_BLOCKED_IN_EPEC.includes(f.value));
}

export const PROBLEM_INVALID_DOCUMENT_FORMAT = 'invalid-document-format';
export const PROBLEM_DOCUMENT_FORMAT_NOT_ALLOWED = 'document-format-not-allowed';

export const MSG_FORMAT_INVALID = 'Este formato não existe para o documento. Escolha uma das opções da lista.';
export const MSG_FORMAT_NOT_ALLOWED = 'Este formato não pode ser impresso para este documento (o DANFE simplificado e a etiqueta não existem na emissão em EPEC). Escolha outro formato.';
export const MSG_XML_NOT_READY = 'O XML do documento ainda não está disponível. Tente novamente em instantes.';
const MSG_PRINT_FAILED = 'Erro ao gerar o PDF.';

/** Texto simples para as recusas do impresso: dois problemas por `code`, 404 sem XML e o `detail` da API. */
export function printProblemMessage(err: unknown): string {
  const e = err as {status?: number; raw?: {code?: string}; message?: string} | undefined;
  if (e?.raw?.code === PROBLEM_INVALID_DOCUMENT_FORMAT) return MSG_FORMAT_INVALID;
  if (e?.raw?.code === PROBLEM_DOCUMENT_FORMAT_NOT_ALLOWED) return MSG_FORMAT_NOT_ALLOWED;
  if (e?.status === 404) return MSG_XML_NOT_READY;
  return err instanceof Error ? err.message : MSG_PRINT_FAILED;
}
