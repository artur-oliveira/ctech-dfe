/**
 * Conversão de `<input type="datetime-local">` para o formato de data-hora que
 * o leiaute exige (`TDateTimeUTC`: `AAAA-MM-DDTHH:mm:ssTZD`).
 *
 * O input entrega `2026-09-11T10:00` — sem segundos e sem fuso. Fixar `-03:00`
 * mente para quem emite no Acre (`-05:00`), em Manaus (`-04:00`) ou em Fernando
 * de Noronha (`-02:00`), e a SEFAZ compara a data-hora com a da emissão.
 */

/** Deslocamento de fuso do navegador no instante dado, no formato `±HH:MM`. */
export function localTimezoneOffset(at: Date = new Date()): string {
  // getTimezoneOffset devolve minutos a somar para chegar ao UTC: a oeste de
  // Greenwich é positivo, e o offset ISO é o inverso disso.
  const minutes = -at.getTimezoneOffset()
  const sign = minutes < 0 ? '-' : '+'
  const abs = Math.abs(minutes)
  const hh = String(Math.floor(abs / 60)).padStart(2, '0')
  const mm = String(abs % 60).padStart(2, '0')
  return `${sign}${hh}:${mm}`
}

/**
 * Converte o valor de um `datetime-local` para data-hora com fuso. Devolve
 * string vazia quando o campo está em branco — o chamador manda `null` e a tag
 * simplesmente não sai.
 */
export function datetimeLocalToOffset(value: string): string {
  if (!value) return ''
  // O input omite os segundos quando são zero; o leiaute os exige.
  const withSeconds = value.length === 16 ? `${value}:00` : value
  const parsed = new Date(withSeconds)
  if (Number.isNaN(parsed.getTime())) return ''
  return `${withSeconds}${localTimezoneOffset(parsed)}`
}
