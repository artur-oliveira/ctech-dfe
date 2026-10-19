# Tema Visual - ctech-dfe

## Paleta de Cores Primária

Escala `--brand-*` definida em `ui/src/app/globals.css` (fonte de verdade; `primary-*` e `brand-*` do Tailwind
resolvem para ela). Contexto de produto e papéis em `ui/DESIGN.md`.

### Cores Principais

```
brand-50:  #f0faf6
brand-100: #d4f1e6
brand-200: #a9e3cd
brand-300: #74cfb0
brand-400: #44b896
brand-500: #2ea87f  (marca)
brand-600: #1c6c55  (botões, acentos)
brand-700: #195644
brand-800: #195644
brand-900: #164738
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

### Marca do produto

- `ui/public/app.svg` é a fonte canônica da marca DF-e: o nó central ligado aos quatro pontos sobre o quadrado verde.
- A landing page, seu estado inicial de carregamento e o cabeçalho da sidebar usam esse mesmo ativo. Ícones de arquivo/documento continuam disponíveis para ações e tipos de conteúdo, mas não representam a identidade do produto.

### Uso dos tokens

Botões primários usam `brand-600` (hover `brand-700`), foco usa `brand-500`, bordas e fundos claros usam
`brand-100`/`brand-200`, erros usam `text-danger`. Componentes devem usar o token, nunca hex fixo.
Os hex antigos desta página (`#50ba95`, `#409d80`, `#30806a`, gradiente de login) não existem mais no código.

## Contextual DF-e Theme

The `primary-*`/`brand-*` Tailwind scale resolves from CSS custom properties (`--brand-50`…`--brand-900`, defined in
`ui/src/app/globals.css`) rather than fixed hex values. Setting `data-dfe-theme="nfce" | "cte" | "mdfe" | "nfse"` on any ancestor
element overrides that scale for everything underneath it — no component changes needed, since every `bg-primary-600`,
`text-brand-700`, etc. picks it up automatically via inheritance. `"nfe"` (or no attribute) is the default green scale.

- `ui/src/lib/theme/dfe-theme.ts` — `getDfeThemeFromPath()` maps a route (`/nfce/...`, `/cte/...`, `/mdfe/...`, `/nfse/...`) to its
  theme key.
- `RootLayout` applies it app-wide based on the current route, so the whole authenticated UI (sidebar, buttons, badges)
  recolors to match whichever document type the user is working in.
- The landing page (`ui/src/app/page.tsx`) applies it from the
  `AuthorizationCard` carousel's current document instead of the route.

Accent colors per document type (also used for icon backgrounds in
`lib/constants/dfe-documents.tsx`): NF-e `#2ea87f` (green/default), NFC-e
`#3b82f6` (blue), CT-e `#8b5cf6` (violet), MDF-e `#f59e0b` (amber), NFS-e
`#0f766e` (teal). Contextual document actions such as **Substituir** use the inherited brand token,
never another document's raw accent.
