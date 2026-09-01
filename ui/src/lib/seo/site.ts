/**
 * Constantes de SEO. O produto é privado: só a landing e o guia são indexáveis.
 * Por isso a regra é sempre pelo avesso — bloqueia tudo, libera as exceções — e
 * não há lista de rotas de app para manter em sincronia.
 */

/** Origem pública canônica. Sem barra final — as rotas já começam com `/`. */
export const PUBLIC_ORIGIN = 'https://dfe.aoctech.app'

/** As únicas áreas rastreáveis. `/$` casa só a raiz exata. */
export const CRAWLABLE_PATTERNS = ['/$', '/guide'] as const

export const absoluteUrl = (path: string) => `${PUBLIC_ORIGIN}${path}`
