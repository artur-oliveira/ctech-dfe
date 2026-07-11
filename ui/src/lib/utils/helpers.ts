// ─── Helpers ─────────────────────────────────────────────────────────────────

export const formatDate = (y: number, m: number, d: number): string  => {
  return `${String(d).padStart(2, '0')}/${String(m).padStart(2, '0')}/${y}`
}

export const formatCurrency = (value: string): string  =>{
  const n = parseFloat(value)
  return isNaN(n) ? value : n.toLocaleString('pt-BR', {style: 'currency', currency: 'BRL'})
}