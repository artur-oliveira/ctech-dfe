export const validateCPF = (cpf: string): boolean => {
  const clean = cpf.replace(/\D/g, '');
  if (clean.length !== 11) return false;
  if (/^(\d)\1{10}$/.test(clean)) return false;
  let sum = 0;
  for (let i = 1; i <= 9; i++) sum += parseInt(clean[i - 1]) * (11 - i);
  let rem = (sum * 10) % 11;
  if (rem === 10 || rem === 11) rem = 0;
  if (rem !== parseInt(clean[9])) return false;
  sum = 0;
  for (let i = 1; i <= 10; i++) sum += parseInt(clean[i - 1]) * (12 - i);
  rem = (sum * 10) % 11;
  if (rem === 10 || rem === 11) rem = 0;
  return rem === parseInt(clean[10]);
};

/**
 * Validates a CNPJ, alphanumeric format included (IN RFB 2229/2024).
 * Each character is worth its ASCII code minus 48 (0–9 keep their value, A=17 … Z=42),
 * the same value the access key's cDV uses; the official example 12.ABC.345/01DE-35 validates.
 * Mirrors api/internal/validation ValidCNPJ. Check digits (positions 13–14) must be numeric.
 */
/** Normaliza CPF ou CNPJ mantendo as letras do CNPJ alfanumérico (NT 2025.001); `\D` apagaria as letras. */
export const cleanDocument = (doc: string): string => doc.replace(/[^A-Z0-9]/gi, '').toUpperCase();

export const validateCNPJ = (cnpj: string): boolean => {
  const clean = cnpj.replace(/[^A-Z0-9]/gi, '').toUpperCase();
  if (clean.length !== 14) return false;
  if (new Set(clean).size === 1) return false;

  // Check digits must be numeric
  if (!/^\d$/.test(clean[12]) || !/^\d$/.test(clean[13])) return false;

  const val = (c: string): number => c.charCodeAt(0) - 48;

  const w1 = [5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2];
  const s1 = clean.slice(0, 12).split('').reduce((acc, c, i) => acc + val(c) * w1[i], 0);
  const r1 = s1 % 11;
  const d1 = r1 < 2 ? 0 : 11 - r1;
  if (val(clean[12]) !== d1) return false;

  const w2 = [6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2];
  const s2 = clean.slice(0, 13).split('').reduce((acc, c, i) => acc + val(c) * w2[i], 0);
  const r2 = s2 % 11;
  const d2 = r2 < 2 ? 0 : 11 - r2;
  return val(clean[13]) === d2;
};
