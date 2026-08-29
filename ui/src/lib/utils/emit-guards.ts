/**
 * Regras que impedem a nota de avançar com um erro que a SEFAZ rejeitaria.
 * Vivem fora do formulário porque são aritmética e leiaute, não interface — e
 * porque a NFC-e precisa das mesmas regras com uma exceção só (troco).
 */

/**
 * Tolerância de centavo em toda comparação de soma. Abaixo dela a diferença é
 * arredondamento; a partir dela é a rejeição "somatório dos valores de pagamento
 * difere do total da NF-e", a mais frequente das evitáveis.
 */
export const SUM_TOLERANCE = 0.01

/** O VIN não usa I, O e Q — confundem com 1 e 0, e a SEFAZ recusa. */
const VIN_FORBIDDEN = /[IOQ]/
const VIN_LENGTH = 17

function fmtBRL(n: number): string {
  return n.toLocaleString('pt-BR', {style: 'currency', currency: 'BRL'})
}

export interface UnitDataItem {
  prodType?: string | null
  chassi?: string | null
  nSerie?: string | null
  nMotor?: string | null
  armaCount: number
}

/**
 * Dado obrigatório por unidade que a SEFAZ só cobra na emissão. O label já marca
 * esses campos com `*`; esta função é o que faz o `*` valer antes do envio, em
 * vez de virar rejeição depois que o operador saiu da tela.
 */
export function unitDataGap(item: UnitDataItem): string | null {
  if (item.prodType === 'veiculo') {
    const chassi = (item.chassi ?? '').trim()
    if (chassi.length !== VIN_LENGTH) return `Chassi precisa ter ${VIN_LENGTH} caracteres.`
    if (VIN_FORBIDDEN.test(chassi)) return 'Chassi não pode conter as letras I, O ou Q.'
    if (!(item.nSerie ?? '').trim()) return 'Informe o número de série do veículo.'
    if (!(item.nMotor ?? '').trim()) return 'Informe o número do motor.'
  }
  if (item.prodType === 'arma' && item.armaCount === 0) {
    return 'Adicione ao menos uma arma (série e cano) a este item.'
  }
  return null
}

/**
 * Diferença entre o total da nota e o que foi pago. `allowChange` só é verdadeiro
 * na NFC-e, onde o excedente é troco legítimo (vTroco); na NF-e ele é rejeição.
 */
export function paymentBalanceGap(remaining: number, allowChange: boolean): string | null {
  if (remaining > SUM_TOLERANCE) return `Faltam ${fmtBRL(remaining)} em pagamentos.`
  if (remaining < -SUM_TOLERANCE && !allowChange) {
    return `Pagamentos excedem o total em ${fmtBRL(-remaining)} — a NF-e não admite troco.`
  }
  return null
}

/** As parcelas têm que somar exatamente a fatura, ou a duplicata é rejeitada. */
export function duplicataSumGap(faturaTotal: number, allocated: number): string | null {
  const diff = faturaTotal - allocated
  if (Math.abs(diff) < SUM_TOLERANCE) return null
  return diff > 0
    ? `As parcelas somam ${fmtBRL(allocated)} de ${fmtBRL(faturaTotal)} da fatura.`
    : `As parcelas excedem a fatura em ${fmtBRL(-diff)}.`
}
