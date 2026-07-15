---
target: src/app/ src/components
total_score: 29
p0_count: 0
p1_count: 3
timestamp: 2026-07-15T22-23-44Z
slug: src-app-src-components
---
Method: dual-agent (A: a01b761729a99e256 · B: a7d0afd99c22152ba)

## Design Health Score

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 3 | Broken dynamic skeleton width class `` `w-${w}` `` in `src/app/fiscal-config/page.tsx:138` — Tailwind JIT can't generate it, so the staggered skeleton never renders as designed |
| 2 | Match System / Real World | 3 | Plain PT-BR + inline `GlossaryTerm` for SEFAZ jargon; rejection reasons shown verbatim, not translated away |
| 3 | User Control and Freedom | 3 | Wizard steps strictly gated forward-only (`NfeEmitForm.tsx:1005-1015`), no jump-ahead |
| 4 | Consistency and Standards | 2 | 6 raw `<textarea>` bypass shared `Textarea` on every cancellation flow; Topbar dropdowns use `shadow-modal` instead of documented `shadow-popover`; `LoadingSkeleton` adopted by only ~4 of 12+ candidate pages |
| 5 | Error Prevention | 3 | Live char-count + min-length on cancel justification, CFOP-mix validation pre-submit, 300ms debounce respected |
| 6 | Recognition Rather Than Recall | 3 | Sidebar always icon+label; but 6+ icon-only add/remove buttons in `EntityForm.tsx` carry no label at all |
| 7 | Flexibility and Efficiency | 3 | Real shortcuts (`n`, `?`) + saved filter views + bulk actions exist, but only 3 shortcuts total and the most-used interaction (org switcher) has no keyboard nav |
| 8 | Aesthetic and Minimalist Design | 3 | Flat, hairline, `SectionCard`-disciplined — but `NfeEmitForm.tsx` at 1741 lines / ~30 `useState` is a maintainability risk |
| 9 | Error Recovery | 3 | SEFAZ rejection motive shown verbatim via modal; but several `catch` blocks collapse real errors into one generic toast (`nfe/page.tsx:103-105`) |
| 10 | Help and Documentation | 3 | Contextual glossary + shortcuts-help dialog is genuinely good; no global help center (may be intentional minimalism — n/a without product input) |
| **Total** | | **29/40** | **Good** |

## Anti-Patterns Verdict

**LLM assessment**: The authenticated app (dashboard, forms, tables) is register-appropriate — no gradient text, no side-stripe borders, no glassmorphism, status badges correctly immune to the per-doc-type theme. The one real AI-slop pocket is the public landing page, `src/app/page.tsx`: numbered `01/02/03` step markers built from `padStart(2,'0')`, four identical `BENEFITS` cards, four identical `DFE_DOCUMENTS` cards, a 3-tier pricing grid with a "Mais popular" badge, and eyebrow-style doc-code labels above sections — a generic-SaaS-template silhouette sitting right next to a brief that explicitly rejects that exact monoculture.

**Deterministic scan**: `detect.mjs` returned exit 2, 11 findings (4 `gray-on-color`, 7 `design-system-font-size`). Cross-checked against Assessment A and manual read: **all 11 are false positives**. The 7 font-size hits (`text-[0.8rem]` in `CertificateFields.tsx`, `ProductForm.tsx`, `button.tsx`, `form.tsx`) are the documented "Caption/Secondary" type step at `DESIGN.md:128` — the detector only reads the frontmatter type scale and misses that prose-documented exception. The 4 `gray-on-color` hits (`EntityForm.tsx:316,471`, `LocationPicker.tsx:59`, `AuthorizedViewersSection.tsx:82`) pair a resting `text-gray-400` with a `hover:bg-red-50` that only exists at the same moment `hover:text-red-500` also fires — the two colors never actually co-occur, the detector doesn't reason about state-scoped Tailwind variants. Net: the deterministic pass caught zero real issues here, but its false-positive pattern is worth remembering for future runs on this codebase (its type-scale check needs to read prose exceptions, and its contrast pairing needs hover/focus-state awareness).

**Grep evidence** (Assessment B, supplementary): 0 side-stripe borders, 0 gradient-text, 0 raw `<img>` without alt, 0 `console.log` — clean on all the hard bans. 2 stray inline hex colors outside the token file (`no-org-banner.tsx:7`, `authorization-card.tsx:24`), 1 `shadow-lg` outside the documented shadow scale (`Header.tsx:7`), and 2 unbracketed `z-9999` utilities in near-duplicate combobox components (`ncm-combobox.tsx:254`, `combobox.tsx:153`) — worth confirming these resolve under Tailwind v4's arbitrary-value handling rather than silently no-op'ing. Assessment B independently found 6 icon-only buttons without `aria-label`, all in `EntityForm.tsx` — this corroborates Assessment A's Priority Issue #2 exactly, found by two different methods (manual read vs. grep), which is a strong signal it's real.

**Visual overlays**: Not available this run — no dev server was reachable on ports 3000-3003, and starting one was judged non-trivial (recent commit adds a mocking client with its own boot requirements) within Assessment B's time budget, so it was skipped rather than faked. No live/browser evidence is claimed anywhere in this report.

## Overall Impression

This is a well-disciplined "product register" codebase that mostly lives up to its own DESIGN.md — flat surfaces, one accent color, status badges correctly firewalled from the per-document-type theme, real contextual help instead of a bolted-on help center. The gap isn't taste, it's **enforcement drift**: several shared primitives exist (`Textarea`, `LoadingSkeleton`, `shadow-popover`) but aren't consistently reached for, so the same problem gets solved 4-6 times with slightly different, sometimes worse, code each time. The single biggest opportunity: the two most emotionally loaded moments in the product — an irreversible SEFAZ cancellation, and the org-switcher an accountant touches dozens of times a day — are exactly the two places where ad-hoc code (raw `<textarea>`, hand-rolled unlabeled menu) replaces the system's own better primitives.

## What's Working

- **`src/components/ui/table-shell.tsx`** unifies ~14 previously hand-rolled tables into one primitive that auto-collapses to stacked cards under `sm` (`globals.css:273-284`), carries `aria-label`, and has a real dimmed background-refetch state — exactly the mobile-first + loading-state discipline CLAUDE.md mandates.
- **`glossary-term.tsx` + `KeyboardShortcuts.tsx`**: fiscal jargon gets an inline, correctly-ARIA'd popover explanation instead of a wall of text, and there's a real, discoverable (`?`) power-user shortcut. This is the "quiet operator, not bureaucratic portal" brief actually showing up in code.
- **Status badges are structurally immune to accent contamination**: `NFE_STATUS_CLASSES`/`MDFE_STATUS_CLASSES` use literal semantic Tailwind colors, not `bg-brand-*`/`text-primary-*` tokens, and `globals.css:146-172` confirms `[data-dfe-theme]` only ever recolors the brand scale — the "status is universal vocabulary" rule from DESIGN.md is enforced by construction, not just convention.

## Priority Issues

**[P1] Raw `<textarea>` on every cancellation flow triggers iOS zoom at the highest-stakes moment in the app**
Why it matters: All 6 destructive-justification textareas (`src/app/nfe/page.tsx:601-608`, `src/app/nfce/page.tsx:295-298`, `src/components/mdfe/MdfeActions.tsx:112-115`, `src/components/dfe/DfeDetail.tsx:395`, `src/app/nfe/detail/page.tsx:85`, `src/components/nfce/SubstituteModal.tsx:118`) bypass the shared `Textarea`, hardcoding `text-sm` instead of the shared component's `text-base md:text-sm` iOS-zoom guard. A mobile user confirming an irreversible SEFAZ cancellation gets a viewport jump exactly when they need to read carefully.
Fix: Replace all 6 with `<Textarea>`; given 4+ call sites share near-identical label+counter+min-length markup, extract a shared `JustificationField`.
Suggested command: /impeccable harden

**[P1] Icon-only add/remove buttons with no label — confirmed independently by both assessments**
Why it matters: `src/components/EntityForm.tsx:315,470,501,513,537,549` and `src/components/mdfe/MdfeEmitForm.tsx:189,191` render only an icon with no `aria-label`, on the form every persons/vehicles/products record and every MDF-e route depends on. A screen-reader user hears "button" with no purpose. The correct pattern already exists elsewhere in the same codebase (`Sidebar.tsx:191`, `modal.tsx:114`).
Fix: Add `aria-label` to each (e.g. `aria-label="Remover e-mail"`), matching the existing convention.
Suggested command: /impeccable audit

**[P1] Topbar org/user menus reinvent an unlabeled, keyboard-inaccessible popover for the single most-used interaction**
Why it matters: `src/components/layout/Topbar.tsx:136-155` (org switcher) and `:178-217` (user menu) are hand-rolled without `role="menu"`/`role="menuitem"` and without arrow-key navigation, and use `shadow-modal` instead of the documented `shadow-popover` tier that `combobox.tsx:153` already gets right. For the heaviest-use persona — accountants switching between many client orgs — this is the least accessible, least standard interaction in the app.
Fix: Route through the existing `Combobox`/menu primitive already standardized elsewhere.
Suggested command: /impeccable audit

**[P2] Shared `LoadingSkeleton` under-adopted, and one instance is silently broken**
Why it matters: Only ~4 pages use `src/components/ui/loading-skeleton.tsx`; 8+ others hand-roll the identical pulse-block pattern. Worse, `fiscal-config/page.tsx:138` builds a Tailwind class dynamically (`` `w-${w}` ``) that the JIT compiler cannot generate, so the intended staggered-width skeleton silently fails to vary.
Fix: Converge all instances on `LoadingSkeleton`; fix the dynamic-width bug with `style={{width}}` or literal classes.
Suggested command: /impeccable polish

**[P3] Dead `DashboardCard` component is a landmine that violates the design system if ever revived**
Why it matters: `src/components/dashboard/DashboardCard.tsx` has zero call sites but is exported. It's a permanent-tinted-background hero-metric-tile template with a raw emoji-string icon prop — directly against the "no colored wash on resting surfaces" rule and the one-icon-vocabulary convention. Nobody's using it today, but the next person who greps "DashboardCard" will wire in a violation.
Fix: Delete it.
Suggested command: /impeccable distill

## Persona Red Flags

**Sam (Accessibility-Dependent)**: The fiscal-config "has config" state is a bare 6px colored dot (`bg-green-500` vs `bg-gray-300`, `fiscal-config/page.tsx:110-114`) with zero text alternative — a color-only signal, the exact WCAG failure DESIGN.md warns against for status badges but misses here. Combined with the unlabeled `EntityForm.tsx` icon buttons and the un-roled Topbar menus, a screen-reader-dependent accountant cannot reliably manage records or switch organizations — two core daily tasks.

**Alex (Power User)**: Only 3 keyboard shortcuts exist app-wide (`n`, `?`, `Esc`) for a tool whose heaviest users process many documents daily. The org switcher — hit dozens of times a day — has zero arrow-key navigation, forcing mouse use or slow Tab-cycling through every org.

**Casey (Mobile)**: The cancellation-justification textarea (the single highest-anxiety screen in the app) triggers iOS auto-zoom on focus because it hardcodes `text-sm` instead of the shared component's mobile-zoom-prevention pattern — the viewport jumps at exactly the moment a stressed mobile user needs to read carefully and can least afford a layout surprise.

## Minor Observations

- 7 `text-[0.8rem]` detector hits are false positives — documented "Caption/Secondary" step, but only in prose (`DESIGN.md:128`), not in the frontmatter type scale the detector actually parses.
- 4 `gray-on-color` detector hits are false positives — resting-state color paired against a hover-only background that never co-occurs with it in any real render state; still, the combined className is confusing to read and worth tightening for clarity.
- Two unbracketed `z-9999` utilities in near-duplicate combobox components (`ncm-combobox.tsx:254`, `combobox.tsx:153`) — confirm this resolves under Tailwind v4 rather than no-op'ing, and note the duplication itself is a DRY concern.
- Stray inline hex colors outside the token file: `no-org-banner.tsx:7` (`stroke="#f59e0b"`), `authorization-card.tsx:24` (hardcoded `#2ea87f` fallback duplicating `--brand-500`).
- `Sidebar.tsx:175` / `Topbar.tsx:168` use inline `style={{backgroundColor: 'var(--brand-600)'}}` where `className="bg-brand-600"` would match how every other component applies the brand color.
- `EntityForm.tsx` caps emails/phones at 5 each with no "n/5" counter until the cap is actually hit — small discoverability gap.

## Questions to Consider

- Is the landing page (`src/app/page.tsx`) meant to follow "The Quiet Operator" principles too, or is it deliberately a separate marketing register the design system doesn't cover? Right now it reads like a different product sitting in front of this one.
- `EntityForm.tsx` is the one major form that isn't a step-wizard, unlike NF-e/NFC-e/MDF-e emission — was that a deliberate "simple enough" call, and does it still hold now that it has 4 repeatable field groups plus a PJ/PF toggle all visible at once?
- The org switcher is the single most-used interaction for the heaviest persona, yet it's the one hand-rolled, least-accessible popover in the app instead of the standardized primitive used everywhere else — was that a temporary shortcut, and is it tracked to fix?
