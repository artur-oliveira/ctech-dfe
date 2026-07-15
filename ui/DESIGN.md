---
name: ctech-dfe UI
description: Calm, efficient fiscal-document workspace — one quiet operator, four document accents
colors:
  primary: "#218768"
  primary-strong: "#1c6c55"
  primary-soft: "#f0faf6"
  neutral-bg: "#ffffff"
  neutral-surface: "#f8fafc"
  neutral-border: "#e2e8f0"
  neutral-muted: "#64748b"
  neutral-ink: "#0f172a"
  destructive: "#dc2626"
  accent-nfce: "#3b82f6"
  accent-cte: "#8b5cf6"
  accent-mdfe: "#f59e0b"
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
    rounded: "{rounded.md}"
    padding: "0 10px"
  button-primary-hover:
    backgroundColor: "{colors.primary-strong}"
  input:
    backgroundColor: "transparent"
    textColor: "{colors.neutral-ink}"
    height: "32px"
    rounded: "{rounded.md}"
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
- One quiet green brand, four document accents — the surface stays calm; color signals *which document type* you're in.
- Compact, consistent controls (32px height) that respect expert density without clutter.
- Real-time status is always legible — you never wonder "did it go through?".
- Flat surfaces at rest; depth appears only as a response to state (hover, elevation, focus).
- Mobile-first from 375px; the same vocabulary across every screen.

## 2. Colors

A near-white canvas with a single soft-green brand accent, plus four contextual accents that recolor the whole surface per document type. Neutrals are a cool slate ramp; the brand green is the only always-on color.

### Primary
- **Soft Green** (#218768): the brand accent and the fill of every primary action (buttons, active nav, key links). Hover deepens to #1c6c55.
- **Green Wash** (#f0faf6): tinted backgrounds for selected rows, soft callouts, and the login gradient base.

### Contextual accents (one per document type)
- **NF-e Green** (#218768): the default — no `data-dfe-theme` attribute needed. Carries the NF-e surface.
- **NFC-e Blue** (#3b82f6): applied via `data-dfe-theme="nfce"`; recolors sidebar, buttons, and badges for NFC-e.
- **CT-e Violet** (#8b5cf6): applied via `data-dfe-theme="cte"`.
- **MDF-e Amber** (#f59e0b): applied via `data-dfe-theme="mdfe"`.

Each accent drives the same `--brand-*` / `--primary-*` scale, so components never change — only the hue does.

### Neutral
- **Ink** (#0f172a): headings and body text. Body must hold ≥4.5:1 on white.
- **Muted Slate** (#64748b): secondary text, captions, placeholders. Use only where contrast still clears 4.5:1.
- **Hairline** (#e2e8f0): borders, dividers, input strokes.
- **Surface** (#f8fafc): section headers, sidebar fill, the calm second layer behind white cards.
- **Canvas** (#ffffff): page background and card surfaces.
- **Danger** (#dc2626, `text-danger`): destructive-action *text* ("Cancelar", "Remover", "Excluir", "×"). red-600 ≈ 4.83:1 on white — the AA floor; red-500 (#ef4444 ≈ 3.76:1) fails and must not be used for resting destructive text. All destructive text routes through the `--color-danger` token so contrast can't drift.

### Named Rules
**The One Accent Per Surface Rule.** The core UI is green. A non-green accent appears only inside a `data-dfe-theme` scope (NFC-e/CT-e/MDF-e) — never as free decoration on a green screen.

**The Calm Baseline Rule.** Page and card backgrounds stay near-white (#ffffff / #f8fafc). Depth comes from hairline borders and state shadows, never from a colored wash.

## 3. Typography

**Display Font:** Geist Sans (with system-ui fallback)
**Body Font:** Geist Sans (with system-ui fallback)
**Label/Mono Font:** Geist Sans (labels are tracked uppercase, not a separate face)

**Character:** One well-tuned sans carries the entire system — headings, body, data, and labels. Hierarchy comes from weight and size, not from a second family. This is deliberate: the tool should disappear into the task.

### Hierarchy
- **Display** (600, 1.5rem / 24px, line-height 1.2, -0.01em): page titles (`PageHeader` h1). Where the eye lands first.
- **Title** (600, 1.125rem / 18px, line-height 1.3): section headings, card titles, dialog titles.
- **Body** (400, 0.875rem / 14px, line-height 1.5): default text, table cells, descriptions. Prose caps at 65–75ch; data tables run denser.
- **Label** (600, 0.75rem / 12px, tracked 0.06em, uppercase): `SectionCard` headers and form-field labels. Small but never muted-gray-on-tint.
- **Caption / Secondary** (0.8rem / ~12.8px): the ShadCN secondary step — `sm` button text, form validation/description messages (`form.tsx`, `button.tsx`), and compact field notes. Sits between Body and Label; a deliberate, documented step, not off-ramp drift.

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
- **Shape:** gently rounded (10px radius, `rounded-lg`), 32px tall, compact horizontal padding.
- **Primary / Brand:** soft-green fill (#218768) with white text; hover to #1c6c55. This is the only filled, colored button.
- **Outline:** transparent with a hairline border; hover fills with Surface (#f8fafc).
- **Secondary / Ghost:** quiet fills for low-emphasis actions; hover to Surface.
- **Destructive:** tinted red text on a 10%-opacity red fill (never a hard red block) — error is signaled, not shouted.
- **Focus:** 3px ring in the brand green at 50% opacity; `disabled` drops to 50% opacity.
- All sizes share the same shape: `xs` (24px) → `sm` (28px) → `default` (32px) → `lg` (36px), plus square icon variants.

### Inputs / Fields
- **Style:** transparent fill, 1px hairline border (#e2e8f0), 32px tall, 10px radius. Text is Ink (#0f172a).
- **Focus:** border shifts to the brand green and gains a 3px green ring at 50% opacity.
- **Error:** border turns destructive red with a soft red ring; paired with red-600 helper text.
- **Placeholder:** Muted Slate (#64748b) — must keep ≥4.5:1; never the washed-out #94a3b8 default.
- Debounced inputs and a currency/numeric input family reuse this exact vocabulary.

### Badges / Chips
- **Style:** pill-shaped (full radius), 20px tall, 12px tracked label. Default is a solid green fill with white text.
- **State:** `secondary` = neutral fill, `outline` = hairline border, `destructive` = tinted red. Used for document status (issued / pending / error) and filters.

### Cards / Containers
- **Corner Style:** 14px radius (`rounded-xl`).
- **Background:** white on the canvas, with a hairline border (#e2e8f0).
- **Shadow Strategy:** resting `Card` shadow; lifts to `Card Hover` on interaction.
- **Signature — SectionCard:** a titled block — a soft Surface (#f8fafc) header strip with a tracked uppercase label (12px, Muted Slate), a hairline divider, then 20px-padded body. Groups related fields without nesting cards.

### Navigation
- **Shell:** a 240px left sidebar (near-black fill in dark mode, white in light) plus a 60px top bar divided by a hairline. Active item carries the brand-green accent.
- **Mobile:** the sidebar collapses; the same vocabulary and touch targets (≥44px) persist. Actions reflow to stacked, full-width controls.

### Signature Component — Contextual DF-e Theme
The single distinctive pattern: setting `data-dfe-theme="nfce | cte | mdfe"` on an ancestor recolors every `bg-brand-*` / `text-primary-*` element underneath to that document type's accent (blue / violet / amber). NF-e is the default green and needs no attribute. This lets one component library serve four fiscal products with zero per-type markup.

## 6. Do's and Don'ts

### Do:
- **Do** keep the core UI green; let NFC-e/CT-e/MDF-e accents appear only inside their `data-dfe-theme` scope.
- **Do** hold body text at ≥4.5:1 — bump Muted Slate toward Ink before you reach for lighter gray.
- **Do** reuse the 32px control height and 10px radius everywhere; consistency is the product's virtue.
- **Do** show skeleton loaders (not center spinners) for initial list/table loads, and a subtle dim for background refetches.
- **Do** debounce every input that triggers an API call (300ms default).
- **Do** treat modals as a last resort — exhaust inline and progressive disclosure first.

### Don't:
- **Don't** build a bureaucratic government portal: dated, low-contrast, no hierarchy, anxiety-inducing.
- **Don't** fall into the generic SaaS cream/beige monoculture — keep the canvas near-white and the accent intentional.
- **Don't** use playful / gamified consumer aesthetics that undercut fiscal seriousness.
- **Don't** ship a dense enterprise tool with no visual hierarchy or breathing room.
- **Don't** add a second font family for display or labels; weight and size carry hierarchy.
- **Don't** put a colored wash behind resting surfaces — depth comes from hairlines and state shadows, not fills.
- **Don't** use a border-left/right stripe wider than 1px as a colored accent on cards or list rows.
- **Don't** make inactive states full-saturation; reserve the green (and the per-type accents) for primary actions, current selection, and state indicators.

## 7. Status Badges Are Doc-Neutral (intentional)

Status badges (`Autorizada`, `Rejeitada`, `Pendente`, `Cancelada`, …) use a **fixed
semantic palette** — green / red / amber / gray — and are **not** recolored by
`data-dfe-theme`. This is a deliberate decision, not an omission:

- Status is a *universal state vocabulary*. A user must recognize "Autorizada" instantly
  across every document type. Recoloring it to blue on NFC-e and violet on CT-e would
  break that instant recognition and conflict with the "color signals state, not type"
  rule (§Named Rules).
- The per-type accents (`nfce` / `cte` / `mdfe`) stay scoped to *structural* surfaces —
  sidebar, primary CTAs, section accents — wherever the `data-dfe-theme` ancestor sits.
  Status indicators sit *inside* document rows regardless of type, so they stay neutral.

If a future need arises to tint status per type, it must be a conscious, opt-in override —
never a side effect of the theme attribute.

### Utility tokens (kept in sync with THEME.md)

- `--color-gray-400` is anchored to `#64748b` (slate-500) so secondary text holds ≥4.5:1
  contrast — the default Tailwind `gray-400` (`#9ca3af`) fails AA.
- `bg-gradient-login` (`linear-gradient(135deg,#f0faf6,#d4f1e6,#a9e3cd)`) and the
  `shadow-card` / `shadow-card-hover` / `shadow-modal` / `shadow-topbar` scale are
  defined in `tailwind.config.ts` and reused as the single source for those effects.
