/** Ícones usados só pela navegação (barra lateral, busca global). */

const STROKE = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 2,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
} as const

export const ClipboardIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" {...STROKE}>
    <path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>
    <rect x="8" y="2" width="8" height="4" rx="1"/>
    <line x1="9" y1="12" x2="15" y2="12"/>
    <line x1="9" y1="16" x2="15" y2="16"/>
  </svg>
)

export const BuildingIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" {...STROKE}>
    <rect x="3" y="9" width="18" height="13"/>
    <path d="M9 22V12h6v10"/>
    <path d="M3 9l9-7 9 7"/>
  </svg>
)

export const CardIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" {...STROKE}>
    <rect x="2" y="5" width="20" height="14" rx="2"/>
    <line x1="2" y1="10" x2="22" y2="10"/>
  </svg>
)

export const GridIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" {...STROKE}>
    <rect x="3" y="3" width="7" height="7"/>
    <rect x="14" y="3" width="7" height="7"/>
    <rect x="14" y="14" width="7" height="7"/>
    <rect x="3" y="14" width="7" height="7"/>
  </svg>
)
