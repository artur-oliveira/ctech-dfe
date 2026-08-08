---
name: ctech-dfe UI
description: Calm, efficient fiscal-document workspace — one quiet operator, five document accents
colors:
  primary: "#1c6c55"
  primary-strong: "#195644"
  primary-soft: "#f0faf6"
  neutral-bg: "#ffffff"
  neutral-surface: "#f8fafc"
  neutral-border: "#e2e8f0"
  neutral-muted: "#64748b"
  neutral-ink: "#0f172a"
  destructive: "#dc2626"
  warning: "#b45309"
  success: "#15803d"
  accent-nfce: "#3b82f6"
  accent-cte: "#8b5cf6"
  accent-mdfe: "#f59e0b"
  accent-nfse: "#0f766e"
typography:
  display:
    fontFamily: "Geist Sans, system-ui, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.01em"
  title:
    fontFamily: "Geist Sans, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.3
  body:
    fontFamily: "Geist Sans, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "Geist Sans, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: "0.06em"
rounded:
  sm: "0.5rem"
  md: "0.625rem"
  lg: "0.875rem"
  pill: "9999px"
spacing:
  sm: "0.5rem"
  md: "1rem"
  lg: "1.5rem"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "#ffffff"
    height: "32px"
    rounded: "{rounded.lg}"
    padding: "0 10px"
  button-primary-hover:
    backgroundColor: "{colors.primary-strong}"
  input:
    backgroundColor: "transparent"
    textColor: "{colors.neutral-ink}"
    height: "32px"
    rounded: "{rounded.lg}"
    padding: "0 10px"
  badge-default:
    backgroundColor: "{colors.primary}"
    textColor: "#ffffff"
    rounded: "{rounded.pill}"
    padding: "2px 8px"
---

# Design System: ctech-dfe UI

## 1. Overview

**Creative North Star: "The Quiet Operator"**

The interface is the competent colleague who has already done the bureaucratic work for you. It is calm, fast, and invisible — it disappears into the task of issuing and tracking a tax document. The voice is *sharp efficiency*: plain-language, no fiscal jargon performance, zero wasted motion. Every screen earns its density; nothing is decorative. This system explicitly rejects the **bureaucratic government portal** (dated, low-contrast, no hierarchy, anxiety-inducing), the **generic SaaS cream/beige monoculture**, **playful / gamified consumer aesthetics**, and **dense enterprise tools with no visual hierarchy**.

**Key Characteristics:**
- One quiet green brand, five document accents — the surface stays calm; color signals *which document type* you're in.
- Compact, consistent controls (32px desktop / 44px touch on mobile) that respect expert density without clutter.
- Real-time status is always legible — you never wonder "did it go through?".
- Flat surfaces at rest; depth appears only as a response to state (hover, elevation, focus).
- Mobile-first from 375px; the same vocabulary across every screen and breakpoint.

## 2. Colors

A near-white canvas with a single soft-green brand accent, plus five contextual accents that recolor the whole surface per document type. Neutrals are a cool slate ramp; the brand green is the only always-on color.

### Primary
- **Soft Green** (#1c6c55): the brand accent and the fill of every primary action (buttons, active nav, key links). Hover deepens to #195644. (Darkened from the original #218768/#1c6c55 pair — the resting shade measured 4.4:1 against white, under the 4.5:1 AA floor for normal text.)
- **Green Wash** (#f0faf6): tinted backgrounds for selected rows, soft callouts, and the login gradient base.

### Contextual accents (one per document type)
- **NF-e Green** (#1c6c55): the default — no `data-dfe-theme` attribute needed. Carries the NF-e surface.
- **NFC-e Blue** (#3b82f6): applied via `data-dfe-theme="nfce"`; recolors sidebar, buttons, and badges for NFC-e.
- **CT-e Violet** (#8b5cf6): applied via `data-dfe-theme="cte"`.
- **MDF-e Amber** (#f59e0b): applied via `data-dfe-theme="mdfe"`.
- **NFS-e Teal** (#0f766e): applied via `data-dfe-theme="nfse"`.

Each accent drives the same `--brand-*` / `--primary-*` scale, so components never change — only the hue does.

### Neutral
- **Ink** (#0f172a): headings and body text. Body must hold ≥4.5:1 on white.
- **Muted Slate** (#64748b, `--color-gray-400`): secondary text, captions, placeholders. The default Tailwind `gray-400` (#9ca3af) fails AA, so globals.css anchors `--color-gray-400` to slate-500 (#64748b) instead. Use only where contrast still clears 4.5:1.
- **Hairline** (#e2e8f0): borders, dividers, input strokes.
- **Surface** (#f8fafc): section headers, sidebar fill, the calm second layer behind white cards.
- **Canvas** (#ffffff): page background and card surfaces.
- **Danger** (#dc2626, `--color-danger`): destructive-action *text* ("Cancelar", "Remover", "Excluir", "×"). red-600 ≈ 4.83:1 on white — the AA floor; red-500 (#ef4444 ≈ 3.76:1) fails and must not be used for resting destructive text. All destructive text routes through the `--color-danger` token so contrast can't drift.
- **Warning** (#b45309, `--color-warning`) and **Success** (#15803d, `--color-success`): non-destructive status *text* — pending balance, "total confere", optional-field notices. Anchored one step darker than Tailwind's defaults, which fail AA at the 12–14px sizes these states use (amber-600 ≈ 3.19:1, green-600 ≈ 3.35:1). Balance states also carry a glyph (`✓` / `⌛` / `↩`) so colour is never the only signal.

### Named Rules
**The One Accent Per Surface Rule.** The core UI is green. A non-green accent appears only inside a `data-dfe-theme` scope (NFC-e/CT-e/MDF-e/NFS-e) — never as free decoration on a green screen.

**The Calm Baseline Rule.** Page and card backgrounds stay near-white (#ffffff / #f8fafc). Depth comes from hairline borders and state shadows, never from a colored wash.

## 3. Typography

**Display Font:** Geist Sans (with system-ui fallback) — loaded as `--font-geist-sans`.
**Body Font:** Geist Sans (with system-ui fallback).
**Label/Mono Font:** Geist Sans — labels are tracked uppercase, not a separate face.

**Character:** One well-tuned sans carries the entire system — headings, body, data, and labels. Hierarchy comes from weight and size, not from a second family. This is deliberate: the tool should disappear into the task.

### Hierarchy
- **Display** (600, 1.5rem / 24px, line-height 1.2, -0.01em): page titles (`PageHeader` h1, `text-2xl`). Where the eye lands first.
- **Title** (600, 1.125rem / 18px, line-height 1.3): section headings, card titles, dialog titles (`text-lg`).
- **Body** (400, 0.875rem / 14px, line-height 1.5): default text, table cells, descriptions. Prose caps at 65–75ch; data tables run denser. Mobile inputs step up to `text-base` (16px) to prevent iOS focus-zoom, then `md:text-sm`.
- **Label** (600, 0.75rem / 12px, tracked 0.06em, uppercase): `SectionCard` headers and form-field labels. Small but never muted-gray-on-tint.
- **Caption / Secondary** (0.8rem / ~12.8px, `text-[0.8rem]`): the ShadCN secondary step — `sm` button text, form validation/description messages, and compact field notes. Sits between Body and Label; a deliberate, documented step, not off-ramp drift.

### Named Rules
**The One Family Rule.** No display or serif face in UI labels, buttons, or data. Weight and size carry hierarchy; a second font is noise.

## 4. Elevation

Flat by default. Surfaces sit on the canvas with hairline borders; shadow appears only as a response to state — hover, elevation (modal, slide-over), or focus ring. The dark sidebar is the one structural exception: it reads as a persistent second layer via a near-black fill plus a 1px top border, not via shadow.

### Shadow Vocabulary
- **Card** (`0 1px 3px 0 rgb(0 0 0 / 0.07), 0 1px 2px -1px rgb(0 0 0 / 0.07)`): resting cards — barely there, just enough to lift white off the canvas.
- **Card Hover** (`0 4px 12px 0 rgb(0 0 0 / 0.10), 0 2px 4px -1px rgb(0 0 0 / 0.06)`): on row/card hover, the surface lifts.
- **Modal** (`0 20px 60px -10px rgb(0 0 0 / 0.25)`): dialogs and slide-overs, the top of the stack.
- **Topbar** (`0 1px 0 0 #e2e8f0`): a hairline, not a shadow, separating the top bar from content.
- **Popover** (`0 10px 24px -8px rgb(0 0 0 / 0.15), 0 2px 6px -2px rgb(0 0 0 / 0.08)`): floating overlays — dropdowns, popovers, suggestion lists, the org/keyboard-shortcuts surfaces. The tier between a resting card and a modal; all floating overlays route through this, never raw `shadow-lg`.

### Named Rules
**The Flat-By-Default Rule.** Surfaces are flat at rest. Shadows appear only on hover, elevation, or focus — never as ambient decoration.

## 5. Components

### Buttons
- **Shape:** gently rounded (14px radius, `rounded-lg`). Primary is a filled soft-green (`bg-brand-600`) with white text; hover to `bg-brand-700`. This is the only filled, colored button.
- **Height (mobile-first):** default is `min-h-11` (44px) on mobile — the CLAUDE.md ≥44px touch-target rule — collapsing to `h-8` (32px) at `sm:` and up. Sizes share one shape: `xs` (24px) → `sm` (28px) → `default` (32px desktop / 44px mobile) → `lg` (36px), plus square `icon` variants.
- **Variants:** `default`/`brand` (green fill), `outline` (hairline border, hover fills Surface), `secondary`/`ghost` (quiet fills, hover to Surface), `destructive` (tinted red text on 10%-opacity red fill — `bg-destructive/10 text-destructive`), `link` (underlined brand text), `danger` (solid red fill for irreversible confirms).
- **Focus:** 3px ring in the brand green at 50% opacity (`ring-3 ring-ring/50`); `disabled` drops to 50% opacity. Invalid state (`aria-invalid`) shifts border to destructive red with a red ring.
- All buttons use the Base UI `Button` primitive so native-button semantics and focus are consistent everywhere.

### Inputs / Fields
- **Style:** transparent fill, 1px hairline border (`border-input`), 32px tall, 14px radius (`rounded-lg`). Text is Ink (#0f172a). Mobile steps the font to `text-base` (16px) to stop iOS zoom, then `md:text-sm`.
- **Focus:** border shifts to the brand green and gains a 3px green ring at 50% opacity (`focus-visible:ring-ring/50`).
- **Error:** border turns destructive red with a soft red ring; paired with red-600 helper text.
- **Placeholder:** Muted Slate (#64748b) — must keep ≥4.5:1; never the washed-out #94a3b8 default.
- Debounced inputs, `CurrencyInput`, `NumericInput`, and the select/combobox family reuse this exact vocabulary.

### Badges / Chips
- **Style:** pill-shaped (`rounded-4xl`, full radius), 20px tall (`h-5`), 12px tracked label (`text-xs`). Default is a solid green fill (`bg-primary`) with white text.
- **Variants:** `secondary` = neutral fill, `outline` = hairline border, `destructive` = tinted red, `ghost` = hover Surface.

### Status Badges (doc-neutral, intentional)
- **One vocabulary, one module.** Every status label, tone, pulse and motive title lives in `lib/data/dfe_status.ts`, mirroring `worker/internal/service/helpers.go`. `DfeStatusBadge` / `DfeStatusCell` (`components/dfe/`) are the only renderers — documents *and* SEFAZ events, every doc type. Per-doc badge modules are a drift trap: they went out of sync with the backend and shipped an untyped status.
- **Fixed semantic palette — never recolored by `data-dfe-theme`.** Status is a *universal state vocabulary*; a user must recognize "Autorizada" instantly across every document type. The per-type accents stay scoped to *structural* surfaces (sidebar, primary CTAs) and never touch status indicators. **One tone per meaning** — statuses sharing a tone are told apart by their label and pulse, never by a second shade of the same colour:
  - `success` → `bg-green-100 text-green-700` — Autorizada, Registrado (evento)
  - `danger` → `bg-red-100 text-red-700` — Rejeitada, Falha, Erro
  - `warning` → `bg-amber-50 text-amber-700` — Pendente, Tentando novamente, Cancelando
  - `info` → `bg-blue-50 text-blue-700` — Processando, Encerrando, Encerrado
  - `neutral` → `bg-gray-100 text-gray-500` — Cancelada
  - unknown status → `bg-gray-100 text-gray-600` with the **raw value** as label; never "Desconhecido", which hides what a debugger needs.
- **Gender agrees with the noun.** Labels carry an `@` placeholder (`Autorizad@`) expanded by the `gender` prop: nota is feminine (default), manifesto / conhecimento / evento masculine. Toasts resolve it from `DOC_GENDER[table_name]`.
- **Transitional (in-flight) states** — everything the worker still owes an answer for (`Pendente`, `Processando`, `Tentando novamente`, `Cancelando`, `Encerrando`) — get a subtle `animate-pulse` dot + pulse, suppressed under `prefers-reduced-motion`.
- **`retryable_failed` warns, it does not alarm.** It is a transport failure the worker retries by itself; red + "Falha" would send the user chasing a problem they cannot fix. It reads amber, pulsing, "Tentando novamente".
- **Motive:** a status whose cause is explained by a motive becomes a button opening a portal `Modal` — the modal (not a tooltip) keeps it working on mobile and unclipped inside scrollable tables. The title names the actual cause (`Motivo da rejeição` / `Motivo da falha` / `Motivo da retentativa`), and the icon follows the badge's tone.

### Cards / Containers
- **Corner Style:** 14px radius (`rounded-xl`).
- **Background:** white on the canvas, with a hairline border (#e2e8f0).
- **Shadow Strategy:** resting `Card` shadow; lifts to `Card Hover` on interaction.
- **Signature — SectionCard:** a titled block — a soft Surface (#f8fafc, `bg-gray-50/60`) header strip with a tracked uppercase label (12px, Muted Slate, `tracking-wider`), a hairline divider, then 20px-padded body (`p-5`). Groups related fields without nesting cards.

### Modal / Dialog
- **Shell:** `bg-white rounded-xl shadow-modal`, rendered in a `fixed inset-0 bg-black/50 z-50` portal. Max-height 90vh, scrolls internally.
- **Sizes:** `md` = `max-w-lg` (512px, default), `lg` = `max-w-2xl` (672px), `xl` = `max-w-4xl` (896px). Always `w-full` + `mx-4` so it never overflows at 375px.
- **Chrome:** sticky header (`border-b`, title `text-lg font-semibold text-gray-900`, ghost close icon) and sticky footer (`border-t bg-gray-50`) holding Cancel + primary Submit. Escape and focus-trap are handled; focus returns to the trigger on close.

### Navigation
- **Shell:** a 240px left sidebar plus a 60px top bar divided by a hairline. Active item carries the brand-green accent.
- **Mobile:** the sidebar collapses; the same vocabulary and touch targets (≥44px) persist. Actions reflow to stacked, full-width controls.

### Empty & Loading States
- **EmptyState:** centered, an `bg-gray-100` 48px icon tile, a `text-gray-900` title, optional `text-gray-500` description, and a single `brand` CTA. Teaches the interface; never a bare "nothing here".
- **Loading:** skeleton loaders (`animate-pulse`, suppressed under reduced-motion) for initial list/table loads — never a center spinner in content. Background refetches dim subtly.

### Signature Component — Contextual DF-e Theme
The single distinctive pattern: setting `data-dfe-theme="nfce | cte | mdfe | nfse"` on an ancestor recolors every `bg-brand-*` / `text-primary-*` element underneath to that document type's accent (blue / violet / amber / teal). NF-e is the default green and needs no attribute. This lets one component library serve five fiscal products with zero per-type markup.

### Shared Emission Vocabulary
NF-e and NFC-e share components, not flows. NF-e is a considered document and keeps its 4-step wizard, ending in a real document preview with per-block **Editar** jumps. NFC-e is a counter sale and lives on one screen: an always-focused scan/search field that adds on Enter, a running item list, a total pinned in the action bar, and "CPF na nota?" asked once, optionally, next to the payment. The pieces both consume — `ProductSearch`, `ProductLineItem`, `EmitError`, `EmitConfirmModal`, `DraftRecoveryBanner`, `useEmitDraft`, `lib/data/payment-options` — are the consistency layer; the flow shape is where the two documents are allowed to differ.

## 6. Do's and Don'ts

### Do:
- **Do** keep the core UI green; let NFC-e/CT-e/MDF-e/NFS-e accents appear only inside their `data-dfe-theme` scope.
- **Do** hold body text at ≥4.5:1 — bump Muted Slate toward Ink before you reach for lighter gray. Globals anchor `--color-gray-400` to slate-500 (#64748b) so it can't fall back to the failing Tailwind default.
- **Do** reuse the control vocabulary everywhere (32px / 44px height, 14px radius); consistency is the product's virtue.
- **Do** route all destructive text through `--color-danger` (#dc2626, red-600) so the AA floor can't drift to red-500.
- **Do** route status text through `text-warning` / `text-success`, and pair it with a glyph so colour isn't the only signal.
- **Do** give section labels `text-sm font-medium text-gray-600`.
- **Do** show skeleton loaders (not center spinners) for initial list/table loads, and a subtle dim for background refetches.
- **Do** debounce every input that triggers an API call (300ms default).
- **Do** treat modals as a last resort — exhaust inline and progressive disclosure first. When a modal is right, use the shared `Modal` (sticky chrome, focus-trap, mobile-safe).

### Don't:
- **Don't** build a bureaucratic government portal: dated, low-contrast, no hierarchy, anxiety-inducing.
- **Don't** fall into the generic SaaS cream/beige monoculture — keep the canvas near-white and the accent intentional.
- **Don't** use playful / gamified consumer aesthetics that undercut fiscal seriousness.
- **Don't** ship a dense enterprise tool with no visual hierarchy or breathing room.
- **Don't** add a second font family for display or labels; weight and size carry hierarchy.
- **Don't** put a colored wash behind resting surfaces — depth comes from hairlines and state shadows, not fills.
- **Don't** use a border-left/right stripe wider than 1px as a colored accent on cards or list rows.
- **Don't** make inactive states full-saturation; reserve the green (and the per-type accents) for primary actions, current selection, and state indicators.
- **Don't** recolor status badges by `data-dfe-theme` — status is a fixed, universal vocabulary (green/red/amber/gray/blue), not a per-type accent.
- **Don't** use red-500 (#ef4444) for resting destructive text; it fails AA. Use `text-danger` (red-600).
- **Don't** use `text-amber-600` / `text-green-600` for resting text; both fail AA at 12–14px. Use `text-warning` / `text-success`.
- **Don't** label sections with a `text-xs uppercase tracking-wider` eyebrow. Repeated on every section it is scaffolding, not hierarchy.
- **Don't** put a primary action on `size="sm"` (28px). The default size already handles the 44px mobile floor.

### Utility tokens (kept in sync with THEME.md)

- `bg-gradient-login` (`linear-gradient(135deg,#f0faf6,#d4f1e6,#a9e3cd)`) and the
  `shadow-card` / `shadow-card-hover` / `shadow-modal` / `shadow-topbar` / `shadow-popover`
  scale are defined in `tailwind.config.ts` and reused as the single source for those effects.
- `--color-gray-400` anchors to `#64748b` (slate-500); `--color-danger` to `#dc2626` (red-600);
  `--color-warning` to `#b45309` (amber-700); `--color-success` to `#15803d` (green-700). All live
  in `globals.css` so the AA floor can't drift.
