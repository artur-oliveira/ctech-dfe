export const formatCpfCnpj = (pk: string): string => {
  const raw = unformatCpfCnpj(pk);
  if (raw.length === 11)
    return raw.replace(/(\d{3})(\d{3})(\d{3})(\d{2})/, '$1.$2.$3-$4')
  return raw.replace(/([A-Z0-9]{2})([A-Z0-9]{3})([A-Z0-9]{3})([A-Z0-9]{4})(\d{2})/, '$1.$2.$3/$4-$5')
}

export const unformatCpfCnpj = (pk: string): string => {
  return pk.replace(/^(CPF_|CNPJ_)/, '').replace(/[^A-Z0-9]/gi, '').toUpperCase();
}

export const docLabel = (pk: string): string => {
  return pk.startsWith('CPF_') ? 'CPF' : 'CNPJ'
}
