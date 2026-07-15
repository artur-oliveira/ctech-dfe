# Critique Action Plan — ctech-dfe/ui

Source: `/impeccable critique all` (36 routes). Decision: tackle all P1 clusters
(a11y/contrast, consistency/craft, reassurance/efficiency), all findings, doc-neutral
status decision made intentionally (see `DESIGN.md §7`).

**Verify anytime:**
```bash
npx eslint src --ext .ts,.tsx   # must be 0 errors / 0 warnings
npm test                         # ui test suite
```

> Handoff note: a session hook (`Fact-Forcing Gate`) denies the *first* write/edit to
> any file until you state its importers/callers + affected API/schemas in one line, then
> retry. Just re-issue the same edit after the one-line note.

---

## Done (ESLint green, 0/0)

| # | Action | What changed |
|---|--------|--------------|
| 1 | **harden** a11y + keyboard | `components/ui/modal.tsx` rewritten: `role="dialog"`, `aria-modal`, `aria-labelledby`, focus trap (Tab/Shift+Tab cycle), Esc→`onClose`, initial focus, body scroll-lock + restore, focus restore. Caller API unchanged (`NfeStatusCell`, `NfeEmitForm`, `PersonForm`). Added `focus-visible` outline for native checkbox/radio/date/file/select in `globals.css`. `Topbar.tsx`: Esc closes org/user dropdowns; added `aria-haspopup`/`aria-expanded`. |
| 2 | **contrast** | `globals.css`: `--color-gray-400: #64748b` (re-bases ~45 `text-gray-400` usages to 4.5:1 in one edit); placeholder `#94a3b8`→`#64748b`; scrollbar `border-radius: 3px`→`var(--radius-sm)`. `empty-state.tsx`: icon `stroke #94a3b8`→`currentColor text-gray-500`. |
| 3 | **delight** | `NfeEmitForm.handleSubmit`: was silent `router.push('/nfe')`. Now captures `NfeDetailOut`, fires `toast.success('NF-e enviada para a SEFAZ', {description: protocolo + chave})`, routes to `/nfe/detail?key=<access_key>` (live SEFAZ status). Added `import {toast} from 'sonner'`. |
| 4 | **distill** (part 1) | 4 hardcoded `var(--brand-600)` CTAs → `<Button variant="brand">`: `nfe/page.tsx`, `nfce/page.tsx`, `mdfe/page.tsx`, `not-found.tsx`. Dropped manual `onMouseEnter/Leave` hover hack + `<a href>` anti-pattern; enforces 32px control height. Logo/avatar brand circles (profile, Topbar, Sidebar) intentionally left. |
| 4 | **distill** (part 2 — badges) | New `components/ui/status-badge.tsx` = single `StatusBadge` + `StatusCell` primitive (doc-neutral fixed palette, recorded in docstring). `NfeStatusBadge.tsx` + `MdfeStatusBadge.tsx` delegate to it (all exports kept → importers untouched). `DfeDetail.tsx` inline span → `<StatusBadge size="md">`. ~60 dup lines removed. `cnpj-lookup-badge.tsx` left alone (it's inline status *text*, not a pill badge). |
| 5 | **typeset** | `app/page.tsx` landing badges `text-[0.65rem]`→`text-xs` (12px floor). |
| 7 | **document** | `DESIGN.md §7` "Status Badges Are Doc-Neutral (intentional)" + utility tokens (`--color-gray-400` anchored, gradient/shadow tokens). |
| 4b | **dead tokens** | Removed 8 unused `--gray-{50,100,200,500,600,700,800,900}` from `globals.css :root` (grep-confirmed 0 `var()` refs). Kept `--gray-300/400` (scrollbar). |
| 6 | **clarify (glossary)** | New `components/ui/glossary-term.tsx` (`GlossaryTerm`) + `lib/constants/glossary.ts` (copy once). Native Popover API (top-layer → no `overflow` clipping, touch-friendly; `.gt-pop` positions via implicit anchor in `globals.css`). Wired CFOP/ind_pag/mod_frete into `NfeEmitForm`, CFOP/ind_pag into `NfceEmitForm`. Component test added. **Skipped:** `nat_op` (would nest popover in a `<p>` → invalid HTML; field already shows full value + editar/automático), `NSU` (lives in distribution table headers, not the wizard). |
| 6 | **clarify (PT-BR)** | No change needed — every *displayed* string already accented (`Importação/Distribuição`, `Homologação`, `Cancelamento`). Only unaccented hits are internal TS union keys (`'distribuicao'`), non-user-facing. |

---

## Done this session (part 2) — ESLint 0/0, 160 tests green

| # | Action | What changed |
|---|--------|--------------|
| 4b | **TableShell** (full standardize) | New `components/ui/table-shell.tsx` (`TableShell` + `TABLE_ROW`/`TABLE_CELL` + `RowCheckbox`). Migrated ~17 tables across products, vehicles, persons, OrganizationsTable, nfe/nfce/mdfe/cte lists, cte+mdfe distributions, mdfe/nfe details, DfeDetail, certificates, audit-logs. Unified header (gray-50, uppercase 12px), row hover, `aria-label`, `dimmed` refetch state. Tab strips left untouched. Test: `TableShell.test.tsx`. |
| 4b | **dead tokens** | Removed 8 unused `--gray-*` from `globals.css`. |
| 6 | **clarify** | `GlossaryTerm` (native popover) + `lib/constants/glossary.ts`; wired CFOP/ind_pag/mod_frete into emit forms. PT-BR already accented. Test: `GlossaryTerm.test.tsx`. |
| 8 | **keyboard shortcuts** | `useKeyboardShortcuts` hook + `KeyboardShortcuts` provider in `RootLayout`: `n`=nova emissão (doc-type aware), `?`=help dialog, `Esc`=close. `/`=search dropped (no search UI exists yet). Test: `KeyboardShortcuts.test.tsx`. |
| 8 | **bulk row-select** | `useRowSelection` hook + `BulkActionBar` + `RowCheckbox`. Wired into products/vehicles/persons (select-all + bulk delete via existing undo-delete). Tests: `useRowSelection`, `BulkActionBar`. |
| 8 | **saved filter views** | `useSavedFilterViews` (localStorage) + `SavedFilterViews` popover on cte/mdfe distribution pages. Test: `useSavedFilterViews`. |
| 8 | **NSU filter** | Debounced NSU filter on cte/mdfe distributions. **Server-side** per user: `listDistributions` gains `nsu?` query param (backend will support it); filter drives the query key + resets cursor on change. |
| 8 | **expert mode** | `CollapsibleSection` (collapsed by default); wraps Transport + Informações adicionais in NfeEmitForm, Informações adicionais in NfceEmitForm. Test: `CollapsibleSection`. |

**Pre-existing typecheck error (NOT mine, out of scope):** `src/__tests__/lib/auth-name-merge.test.tsx:29` — `MeResponse.terms_addendum_accepted` required vs optional. Exists on HEAD; left untouched.

---

## Remaining

### Task 4b — TableShell extraction (consistency)
~14 hand-rolled `<table>` blocks repeat the same shell (wrapper `overflow-x-auto`,
`rounded-xl border`, header `bg-gray-50 text-xs uppercase`, row hover, etc.).
Confirm the set first:
```bash
rg -n "overflow-x-auto|<table" src/app src/components --glob '*.tsx'
```
Plan: extract `components/ui/table-shell.tsx` (`<TableShell>`: scroll wrapper +
header/body slots) and a `TableRow` hover helper; migrate pages one by one,
keeping column markup per page. Mobile-first: keep card/list fallback where a
table already does one. No API/schema change. Keep `TableShell` a11y: `role`,
`aria-label` on the table.

### Task 4b — dead `--gray-*` tokens
In `globals.css :root`, `gray-50/100/200/500/600/700/800/900` are unused
(`gray-300`/`gray-400` used by scrollbar). Confirm before removing:
```bash
rg -n "var\(--gray-(50|100|200|500|600|700|800|900)\)" src
```
If zero hits, delete those 8 tokens from `:root`.

### Task 6 — clarify (glossary + PT-BR accents)
- Add inline glossary tooltips on the emit wizard for fiscal acronyms:
  `CFOP`, `ind_pag`, `mod_frete`, `nat_op`, `NSU`. Use a small `<InfoTip>` /
  `<GlossaryTerm>` (title + popover or `title` attr) in `NfeEmitForm.tsx`
  and `NfceEmitForm.tsx`. Define the term copy once (constants).
- Normalize PT-BR accentuation across UI copy: "Distribuição" (not "Distribuicao"),
  "Homologação" (not "Homologacao"), "Autorização", "Cancelamento", etc.
  Grep for unaccented variants:
  ```bash
  rg -ni "distribuicao|homologacao|autorizacao|cancelamento|emitir" src --glob '*.tsx'
  ```

### Task 8 — adapt (power-user accelerants) — NEW FEATURES, needs scope sign-off
This is feature work, not polish. Decide before building:
- Keyboard shortcuts (e.g. `n` = nova emissão, `/` = focus busca, `Esc` close).
- Bulk / row-select on list pages (checkboxes + bulk action bar).
- Saved filter views on distribution/list pages.
- Collapsible "expert mode" section in the emit form (advanced fiscal fields).

### Task 9 — polish (final pass) — DONE (re-critique + fixes)
Re-ran `/impeccable critique all` (dual-agent). **Score 36/40** ("Strong"), 0 P0, 1 P1.
Snapshot: `.impeccable/critique/2026-07-14T23-02-00Z__src.md`. Not AI slop; all absolute
bans clear. Detector: 4 `gray-on-color` warnings = **false positives** (ghost-button
`hover:` states, gray+red never co-occur); 19 `design-system-font-size` advisories (8× `0.8rem`
= intentional ShadCN secondary step, 11× micro-typography — design-owner call, left).

**Fixed (ESLint 0/0, 161 tests, tsc clean except pre-existing auth-name-merge):**
- **P1 contrast** — resting `text-red-500`→`text-red-600` (≈3.76→4.8:1, AA) across 11 sites
  (DfeDetail, nfe/nfce/mdfe/cte pages + distributions, NfeEmitForm ×2, products Excluir);
  status-badge rejection icon `text-red-400`→`red-600`. Left `hover:text-red-500` ghost buttons.
- **P2 reduced-motion** — global `@media (prefers-reduced-motion: reduce)` guard in `globals.css`
  (zeroes animation/transition durations; catches the 40 skeleton `animate-pulse`).
- **P2 44px targets** — `Button` `default` size `h-8`→`min-h-11 sm:min-h-0 sm:h-8` (44px mobile,
  32px desktop). `sm`/`xs` unchanged (dense tables).
- **P3 modal token** — `modal.tsx` `shadow-xl`→`shadow-modal` (purpose-built token).

**P2 mobile card tables — DONE (layout pass).** Instead of hand-writing card renderers for
every table, added ONE responsive layer to the shared `TableShell`: a `.ts-mobile` block in
`globals.css` that, below the `sm` breakpoint, neutralizes the inline `min-width` (kills the
horizontal scroll — the actual defect) and collapses each row into a stacked card with the
column name (`data-label`) shown above each value. Migrated all 14 TableShell tables (products,
cte/mdfe distributions, nfe/nfce/cte/mdfe lists, persons, vehicles, certificates, audit-logs,
DfeDetail ×2, mdfe/detail, OrganizationsTable) by adding `data-label` to each data `<td>`;
checkbox/action/colspan cells omit it. Label-above-value chosen over a flex 2-col so multi-child
cells (emitente name + CNPJ) stack correctly. Test added to `TableShell.test.tsx`. ESLint 0/0,
162 tests, tsc clean (pre-existing auth error only). Not visually verified at 375px — no browser
tool this session; CSS logic reviewed by hand.

Sidecar refresh `/impeccable document` still pending (non-blocking).

---

## Done this session (part 3) — re-critique round 2, all P1–P3 fixed

Re-ran `/impeccable critique all` (dual-agent). **Score 36/40** (held). Snapshot:
`.impeccable/critique/2026-07-14T23-48-02Z__src.md`. Trend `src`: 36 → 36. Detector 23→18
(all remaining = intentional `0.8rem` ShadCN step now documented + landing facsimile + 4
gray-on-color false positives). User chose **Everything (P1–P3)** + **text-danger token** +
landing **distill in place, keep all content**.

**Fixed (ESLint 0/0, tsc clean except pre-existing auth-name-merge; login-page.test.tsx ×2 fail
is PRE-EXISTING — reproduced on clean HEAD with my page.tsx stashed):**
- **P1 destructive contrast (regression from part 2)** — new `--color-danger` (#dc2626 ≈ 4.83:1)
  token in `globals.css` @theme. Swapped **21× resting `text-red-500` + 1× `text-red-400`** →
  `text-danger` across nfe/nfce/mdfe pages + emit forms, ProductForm, persons/vehicles/
  certificates/members pages. The 6 `hover:text-red-500` ghost buttons left (hover-state FPs,
  not resting). Documented in DESIGN.md §2.
- **P2 mobile 44px targets** — `min-h-11 sm:min-h-0` (+`min-w-11` on icons) on Topbar
  hamburger/org-switcher/avatar, Sidebar close + nav items, products row actions; qty steppers
  `h-8 w-7`→`h-11 w-11 sm:h-8 sm:w-7`.
- **P2 token drift** — new `shadow-popover` token (tailwind.config + DESIGN.md §4); routed all
  floating overlays (dropdowns/popovers/suggestion lists/select) off raw `shadow-lg`/`shadow-md`;
  dialogs→`shadow-modal`; segmented thumbs + landing cards→`shadow-card`. Snapped off-ramp fonts:
  Sidebar px-literals (15/11/13px→text-base/xs/sm), MdfeEmitForm 10px→text-xs. Documented the
  `0.8rem` ShadCN step in DESIGN.md §3 (was recurring as "off-ramp"; it's intentional).
- **P3 landing distill** — folded *Como funciona* + *Benefits* into one trust block (numbered flow
  + borderless benefit strip, killed the 2nd white-card grid); removed the roadmap section +
  `ROADMAP` const; kept pricing below the fold. No new route.

**Not-fixed (verified false / intentional):**
- **P3 silent delete-undo** — FALSE finding: `useEntityDelete` already fires a `toast(..., {action:
  {label:'Desfazer'}})`; all four list pages use it. No change.
- **4× `gray-on-color`** detector warnings — false positives (`text-gray-400 hover:text-red-500
  hover:bg-red-50`; gray + red-50 never co-occur).
- **`authorization-card.tsx`** tiny fonts (landing hero) — intentional fiscal-document facsimile.
- **`Header.tsx`** `shadow-lg` — dead code (unimported), left per scope rules.

**Pre-existing, out of scope:** `login-page.test.tsx` ×2 (useAuth-without-AuthProvider, fails on
HEAD); `auth-name-merge.test.tsx` tsc error (`terms_addendum_accepted`).

---

## Files touched this session
`components/ui/modal.tsx`, `components/ui/status-badge.tsx` (new), `components/ui/empty-state.tsx`,
`components/ui/button.tsx` (read only), `app/globals.css`, `app/page.tsx`,
`app/not-found.tsx`, `app/nfe/page.tsx`, `app/nfce/page.tsx`, `app/mdfe/page.tsx`,
`components/layout/Topbar.tsx`, `components/nfe/NfeEmitForm.tsx`,
`components/nfe/NfeStatusBadge.tsx`, `components/mdfe/MdfeStatusBadge.tsx`,
`components/dfe/DfeDetail.tsx`, `DESIGN.md`.

Nothing committed. Delete this file when the plan is complete.
