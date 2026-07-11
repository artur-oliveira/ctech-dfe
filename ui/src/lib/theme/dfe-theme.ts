export type DfeThemeKey = 'nfe' | 'nfce' | 'cte' | 'mdfe'

const THEME_ROUTE_PREFIXES: Record<Exclude<DfeThemeKey, 'nfe'>, string> = {
  nfce: '/nfce',
  cte: '/cte',
  mdfe: '/mdfe',
}

/** Maps the current route to the DF-e document context it belongs to, so the
 *  UI can recolor via the `data-dfe-theme` attribute (see globals.css). */
export function getDfeThemeFromPath(pathname: string): DfeThemeKey {
  for (const [theme, prefix] of Object.entries(THEME_ROUTE_PREFIXES) as [DfeThemeKey, string][]) {
    if (pathname === prefix || pathname.startsWith(prefix + '/')) return theme
  }
  return 'nfe'
}
