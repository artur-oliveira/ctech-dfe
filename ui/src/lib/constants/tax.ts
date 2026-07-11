/** CRT 1 = Simples Nacional, 2 = Simples Nacional (excesso), 4 = MEI */
export const CRT_REGIMES_SIMPLES = [1, 2, 4] as const

/** CRT 3 = Regime Normal (Lucro Real / Presumido) */
export const CRT_REGIME_NORMAL = 3 as const

export const isRegimeSimples = (crt: number | string): boolean =>
  (CRT_REGIMES_SIMPLES as readonly number[]).includes(parseInt(crt.toString()))
