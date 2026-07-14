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

### Task 9 — polish (final pass)
After #4b/#6/#8 land: re-run `/impeccable critique all` and resolve any new flags.
Then refresh the design sidecar: `/impeccable document` (DESIGN.md is newer than
`.impeccable/design.json` — non-blocking hook reminder).

---

## Files touched this session
`components/ui/modal.tsx`, `components/ui/status-badge.tsx` (new), `components/ui/empty-state.tsx`,
`components/ui/button.tsx` (read only), `app/globals.css`, `app/page.tsx`,
`app/not-found.tsx`, `app/nfe/page.tsx`, `app/nfce/page.tsx`, `app/mdfe/page.tsx`,
`components/layout/Topbar.tsx`, `components/nfe/NfeEmitForm.tsx`,
`components/nfe/NfeStatusBadge.tsx`, `components/mdfe/MdfeStatusBadge.tsx`,
`components/dfe/DfeDetail.tsx`, `DESIGN.md`.

Nothing committed. Delete this file when the plan is complete.
