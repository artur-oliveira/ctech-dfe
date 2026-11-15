export function triggerRemoteDownload(url: string): void {
  const a = document.createElement('a');
  a.href = url;
  a.rel = 'noopener noreferrer';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

// Posições do número dentro da chave: NF-e, NFC-e, CT-e e MDF-e (44 dígitos) levam
// cUF, AAMM, CNPJ, modelo e série antes dos 9 dígitos do número; a chave da NFS-e
// nacional (50) leva cLocEmi, ambiente, tipo e inscrição antes dos 13 do número.
const KEY_44_NUMBER = [25, 34] as const;
const KEY_NFSE_LENGTH = 50;
const KEY_NFSE_NUMBER = [23, 36] as const;
const KEY_44_LENGTH = 44;

/** Número do documento (nNF, nCT, nMDF, nNFSe) lido da chave de acesso; null se a chave não tem o tamanho esperado. */
export function docNumberFromKey(accessKey: string | null | undefined): number | null {
  const key = accessKey ?? '';
  const range = key.length === KEY_NFSE_LENGTH ? KEY_NFSE_NUMBER : key.length === KEY_44_LENGTH ? KEY_44_NUMBER : null;
  if (!range) return null;
  const n = Number(key.slice(range[0], range[1]));
  return Number.isInteger(n) ? n : null;
}

export function formatNsu(nsu: number): string {
  return String(nsu).padStart(15, '0');
}

export function formatDatetimeBR(dateStr: string): string {
  return new Date(dateStr).toLocaleString('pt-BR', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

export function formatISODateBR(dateStr: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateStr);
  return match ? `${match[3]}/${match[2]}/${match[1]}` : dateStr;
}

export interface AccessKeyComposition {
  uf_code: string;
  year: string;
  month: string;
  cpf_cnpj: string;
  model: string;
  serie: string;
  number: string;
  emission_type: string;
  dfe_code: string;
  verification_digit: string;
  formatted: string;
}

export const parseAccessKey = (accessKey: string): AccessKeyComposition => {
  if (!accessKey || accessKey.length !== 44) {
    throw new Error('Invalid access key');
  }
  //  22-2606-03518739000188-55-001-000178049-1-33277442-2
  const formatted = accessKey
    .replace(/(\d{2})(\d{2})(\d{2})(\d{14})(\d{2})(\d{3})(\d{9})(\d)(\d{8})(\d)/, '$1-$2-$3-$4-$5-$6-$7-$8-$9-$10');

  const parts = formatted.split('-');

  return {
    uf_code: parts[0],
    year: parts[1],
    month: parts[2],
    cpf_cnpj: parts[3],
    model: parts[4],
    serie: parts[5],
    number: parts[6],
    emission_type: parts[7],
    dfe_code: parts[8],
    verification_digit: parts[9],
    formatted: formatted,
  };
};
