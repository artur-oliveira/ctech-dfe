import {contextForPath} from '@/lib/navigation/nav'

export type DfeThemeKey = 'nfe' | 'nfce' | 'nfse' | 'cte' | 'mdfe'

/** Maps the current route to the DF-e document context it belongs to, so the
 *  UI can recolor via the `data-dfe-theme` attribute (see globals.css).
 *
 *  The route -> context map lives in `lib/navigation/nav` — the same config that
 *  draws the sidebar — so a registry that belongs to NFS-e is themed teal
 *  wherever it lives in the URL space. */
export function getDfeThemeFromPath(pathname: string): DfeThemeKey {
  return contextForPath(pathname)?.key ?? 'nfe'
}
