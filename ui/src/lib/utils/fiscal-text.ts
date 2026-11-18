/**
 * Texto livre que vai ao XML fiscal. Espelha o validador `fiscaltext` e `services.CleanText` da API: o cliente normaliza
 * e valida, o servidor repete as duas coisas (nunca confia no cliente).
 */

/** Sem espaço nas pontas, sem controle, só até U+00FF (TString do leiaute) e sem `<` nem `>`. */
export const FISCAL_TEXT = /^[!-;=?-ÿ](?:[ -;=?-ÿ]*[!-;=?-ÿ])?$/;

/** Código de referência curto (assinante, plano, contrato): letras, dígitos, ponto, barra, hífen e sublinhado. */
export const REF_CODE = /^[A-Za-z0-9._/-]{1,60}$/;

export const FISCAL_TEXT_MESSAGE = 'Use só letras, números e pontuação comum (sem < ou >), sem espaços nas pontas';
export const REF_CODE_MESSAGE = 'Use só letras, números, ponto, barra, hífen e sublinhado';

/** Tira controles, troca qualquer espaçamento por um espaço, junta repetições e apara as pontas. */
export function cleanText(s: string): string {
  return s.replace(/[\u0000-\u001f\u007f-\u009f]+/g, (m) => (/[\t\n\r\f\v]/.test(m) ? ' ' : '')).replace(/\s+/g, ' ').trim();
}

/** Texto opcional: vazio vira undefined; o resto sai limpo. */
export function cleanOptional(s: string): string | undefined {
  const v = cleanText(s);
  return v === '' ? undefined : v;
}
