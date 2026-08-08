# Tema Visual - py-dfe

## Paleta de Cores Primária

Soft Green Theme (#50ba95) - Documento Fiscal Style

### Cores Principais

```
primary-50:  #f0faf8  (background claro)
primary-100: #d5f1eb  (backgrounds)
primary-200: #a8e3d8  (borders, light elements)
primary-300: #7dd4c4  (hover states)
primary-400: #5fc9b6  (secondary buttons)
primary-500: #50ba95  (brand color)
primary-600: #409d80  (buttons, accents) ← BUTTON COLOR
primary-700: #30806a  (hover, darker accents)
primary-800: #256354  (text, dark elements)
primary-900: #1a463e  (very dark, backgrounds)
```

### Cores de status (texto)

Ancoradas em `src/app/globals.css` para que o piso AA não volte a escorregar. Os defaults do
Tailwind falham nos tamanhos em que estes estados são usados (12–14 px), então cada token fica um
passo mais escuro:

```
--color-danger:  #dc2626  (red-600,   ≈4.83:1)  text-danger  — ações destrutivas
--color-warning: #b45309  (amber-700, ≈4.68:1)  text-warning — pendências, saldo faltante
--color-success: #15803d  (green-700, ≈4.54:1)  text-success — saldo conferido
--color-gray-400: #64748b (slate-500, ≈4.76:1)  texto secundário
```

Nunca use `text-amber-600` (≈3.19:1) ou `text-green-600` (≈3.35:1) em texto em repouso. Estados de
saldo também carregam glifo (`✓` / `⌛` / `↩`) — cor nunca é o único sinal.

### Badges de status de documento

Paleta própria, fixa, **não** recolorida por `data-dfe-theme` — "Autorizada" precisa ser reconhecível
igual em qualquer documento. Um tom por significado (`success` / `danger` / `warning` / `info` /
`neutral`), definido uma vez em `ui/src/lib/data/dfe_status.ts`; status do mesmo tom se distinguem
pelo rótulo e pelo pulso, nunca por um segundo tom de vermelho. Tabela completa em DOCS.md §5.

## Componentes

### Login Page

- **Background**: Gradient soft green (`#f0faf8` → `#d5f1eb` → `#a8e3d8`)
- **Card**: White with primary-100 border
- **Button**: primary-600 (#409d80) with hover to primary-700 (#30806a)
- **Icon**: primary-600 background with emoji

### Dashboard

- **Header**: White with subtle primary-50 background
- **Cards**: White with primary-100 border
- **Buttons**: primary-600 for primary actions
- **Links**: primary-600 text

### Forms

- **Labels**: gray-700
- **Inputs**: border-gray-300 with focus ring on primary-500
- **Errors**: red-600 text with red-500 border
- **Sections**: primary-200 bottom border

## CSS Classes Used

### Gradients

```css
.bg-gradient-login {
    background-image: linear-gradient(to bottom right, #f0faf8 0%, #d5f1eb 50%, #a8e3d8 100%);
}
```

### Buttons

```css
.btn-primary {
    background-color: #409d80;
    color: white;
    border-radius: 0.5rem;
    padding: 0.75rem 1rem;
    font-weight: 600;
    transition: opacity 200ms;
}

.btn-primary:hover {
    background-color: #30806a;
}
```

## Implementation Notes

- Inline styles used for button to ensure visibility
- Gradient applied via tailwind backgroundImage customization
- All primary colors extend from tailwind.config theme
- Focus rings use primary-500 for consistency
- Error states use red-600 for contrast

## Contextual DF-e Theme

The `primary-*`/`brand-*` Tailwind scale resolves from CSS custom properties (`--brand-50`…`--brand-900`, defined in
`ui/src/app/globals.css`) rather than fixed hex values. Setting `data-dfe-theme="nfce" | "cte" | "mdfe"` on any ancestor
element overrides that scale for everything underneath it — no component changes needed, since every `bg-primary-600`,
`text-brand-700`, etc. picks it up automatically via inheritance. `"nfe"` (or no attribute) is the default green scale.

- `ui/src/lib/theme/dfe-theme.ts` — `getDfeThemeFromPath()` maps a route (`/nfce/...`, `/cte/...`, `/mdfe/...`) to its
  theme key.
- `RootLayout` applies it app-wide based on the current route, so the whole authenticated UI (sidebar, buttons, badges)
  recolors to match whichever document type the user is working in.
- The landing page (`ui/src/app/page.tsx`) applies it from the
  `AuthorizationCard` carousel's current document instead of the route.

Accent colors per document type (also used for icon backgrounds in
`lib/constants/dfe-documents.tsx`): NF-e `#2ea87f` (green/default), NFC-e
`#3b82f6` (blue), CT-e `#8b5cf6` (violet), MDF-e `#f59e0b` (amber).
