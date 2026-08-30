export function triggerRemoteDownload(url: string): void {
  const a = document.createElement('a')
  a.href = url
  a.rel = 'noopener noreferrer'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

export function formatNsu(nsu: number): string {
  return String(nsu).padStart(15, '0')
}

export function formatDatetimeBR(dateStr: string): string {
  return new Date(dateStr).toLocaleString('pt-BR', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

export function formatISODateBR(dateStr: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateStr)
  return match ? `${match[3]}/${match[2]}/${match[1]}` : dateStr
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
    .replace(/(\d{2})(\d{2})(\d{2})(\d{14})(\d{2})(\d{3})(\d{9})(\d)(\d{8})(\d)/, '$1-$2-$3-$4-$5-$6-$7-$8-$9-$10')

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
  }
}
