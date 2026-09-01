import {DOC_CONTEXTS, NAV_GROUPS, SHARED_REGISTRIES} from '@/lib/navigation/nav'

/** Sufixo do documento, igual ao `title.template` do layout raiz. */
export const TITLE_SUFFIX = 'CTech DF-e'

/**
 * Telas de app que não são item de navegação. São sempre um passo a partir de
 * uma listagem (`/persons/new`) ou de um documento (`/nfe/detail`), então viram
 * prefixo do rótulo da rota-mãe: "Novo · Pessoas".
 */
const SEGMENT_LABELS: Record<string, string> = {
  new: 'Novo',
  edit: 'Editar',
  detail: 'Detalhes',
  distributions: 'Distribuições',
  link: 'Vincular',
}

/** Rotas fora da navegação — fluxos de identidade e telas de sistema. */
const STANDALONE_TITLES: Record<string, string> = {
  '/login': 'Entrar',
  '/callback': 'Entrando…',
  '/invite': 'Convite',
  '/profile': 'Meu perfil',
  '/onboarding': 'Primeiros passos',
  '/terms-addendum': 'Termos de uso',
  '/unavailable': 'Fora do ar',
}

/** Todos os rótulos de rota conhecidos, do mais específico para o mais genérico. */
const ROUTE_LABELS: [string, string][] = [
  ...NAV_GROUPS.flatMap(g => g.items.map(i => [i.href, i.label] as [string, string])),
  ...SHARED_REGISTRIES.map(i => [i.href, i.label] as [string, string]),
  ...DOC_CONTEXTS.flatMap(ctx => [
    ...(ctx.emit ? [[ctx.emit.href, ctx.emit.label] as [string, string]] : []),
    ...ctx.items.map(i => [i.href, i.label] as [string, string]),
  ]),
  ...Object.entries(STANDALONE_TITLES),
].sort((a, b) => b[0].length - a[0].length)

/**
 * Nome da tela para a rota, sem o sufixo do produto.
 *
 * Sai da mesma configuração que desenha a navegação: rótulo da rota conhecida
 * mais específica, com o segmento extra (`new`, `edit`, `detail`) na frente.
 * Página nova registrada em `nav.tsx` ganha título junto, sem passo extra.
 */
export function pageNameForPath(pathname: string): string | null {
  const match = ROUTE_LABELS.find(
    ([href]) => pathname === href || pathname.startsWith(href + '/'),
  )
  if (!match) return null
  const [href, label] = match
  if (pathname === href) return label

  const segment = pathname.slice(href.length + 1).split('/')[0]
  const segmentLabel = SEGMENT_LABELS[segment]
  return segmentLabel ? `${segmentLabel} · ${label}` : label
}

/** Título completo do documento, no mesmo formato do `title.template` raiz. */
export function documentTitleForPath(pathname: string): string {
  const name = pageNameForPath(pathname)
  return name ? `${name} | ${TITLE_SUFFIX}` : TITLE_SUFFIX
}
