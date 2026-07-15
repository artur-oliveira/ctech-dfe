---
target: src/app/nfce/emit/page.tsx + src/app/mdfe/emit/page.tsx
total_score: 25
p0_count: 0
p1_count: 3
timestamp: 2026-07-15T23-38-21Z
slug: app-nfce-emit-page-tsx-src-app-mdfe-emit-page-tsx
---
Method: dual-agent (A: general-purpose · B: general-purpose)

## Design Health Score

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 3 | Skeletons + "Emitindo…" present; silent auto-add of a product on step-advance goes uncommunicated |
| 2 | Match System / Real World | 4 | Correct pt-BR fiscal vocabulary, `GlossaryTerm` on jargon, honest confirm copy |
| 3 | User Control and Freedom | 2 | No draft persistence, no unload guard across a 3-6 step wizard, breadcrumb link discards state silently |
| 4 | Consistency and Standards | 3 | Strong component reuse (Modal/EmitConfirmModal/SectionCard) undercut by nav buttons breaking the project's own mobile touch-target rule |
| 5 | Error Prevention | 2 | CFOP/payment validation is real; manual CFOP input has no invalid-state styling; disabled Emit gives zero reason |
| 6 | Recognition Rather Than Recall | 3 | Persistent totals help; disabled Emit forces recall of unstated conditions |
| 7 | Flexibility and Efficiency of Use | 1 | No shortcuts, no bulk actions, no skip-ahead for repeat users |
| 8 | Aesthetic and Minimalist Design | 4 | Faithful to the documented design system; restrained, consistent |
| 9 | Error Recovery | 2 | Raw `err.message` surfaced verbatim; error boxes not `role="alert"` |
| 10 | Help and Documentation | 1 | `GlossaryTerm` covers 2 NFC-e fields only; MDF-e's jargon (fronteira, carga lotação, modal) has none |
| **Total** | | **25/40** | **Acceptable — significant improvements needed before users are happy** |

## Anti-Patterns Verdict

**Clean of AI slop.** No side-stripes, gradient text, glassmorphism, hero-metric tiles, or decorative card grids. The step-number wizard is a real functional gate (`canNext`/`canEmit`), not scaffolding. This reads as a deliberately designed system, not template output — both assessments agree here.

**Deterministic scan**: `detect.mjs` returned a clean pass (exit 0, `[]`) across all six target files (both `page.tsx` wrappers, both emit forms, `emit-confirm-modal.tsx`, `modal.tsx`). Zero rule hits. No false positives to adjudicate since there were no findings — the detector has nothing to add or contradict here; this critique rests entirely on the manual review.

**Browser visualization**: skipped. No browser-automation tool (Playwright/Puppeteer/computer-use) is exposed in this session, so Assessment B correctly fell back to a code-only pass and did not touch the already-running dev server (localhost:3000). No user-visible overlay exists for this run.

## Overall Impression

The fiscal logic embedded in these two wizards is genuinely good — CFOP eligibility filtering, live payment-total reconciliation, border-aware MDF-e routing — and the component vocabulary (Modal, EmitConfirmModal, StepIndicator, SectionCard) is reused cleanly across two structurally different documents. The gaps are all in *how the wizard talks back to the user at moments that matter*: a silent auto-added product, a disabled Emit button that won't say why, and — the biggest miss — a legally consequential SEFAZ submission that ends in a bare redirect with no acknowledgment. The confirm modal added recently is a real improvement, but it's the only reassurance beat in the whole flow, and it's thinner (3 fields) than the 3-6 steps of data entry it's vouching for.

## What's Working

1. **Domain logic surfaced as UI, not bolted on** — CFOP restricted to valid outbound codes per product, live "Restam/Troco/confere" payment reconciliation, border-aware route suggestion. This is real fiscal correctness doing double duty as UX guidance.
2. **Disciplined reuse across divergent flows** — one `StepIndicator`, one `Modal`, one `EmitConfirmModal` serving a 3-step and a 6-step wizard without diverging or duplicating.
3. **Honest, plain-language copy** — "não pode ser desfeita diretamente. Confira os dados antes de confirmar," "a NFC-e pode ser emitida assim mesmo" — no bureaucratic hedging, matches the "Quiet Operator" tone.

## Priority Issues

**[P1] Wizard's own primary navigation violates the project's mandatory 44px mobile touch-target rule.**
- **Why it matters**: `Voltar`/`Próximo`/`Emitir NFC-e`/`Emitir MDF-e` — the highest-frequency taps in the entire flow (used at every one of 3-6 steps) — are wired to `size="sm"` (`NfceEmitForm.tsx:652-656`, `MdfeEmitForm.tsx:773-777`). Verified in `button.tsx:31`: `sm` is a fixed `h-7` (28px) at every breakpoint; only `size="default"` (`button.tsx:29`) gets `min-h-11 sm:min-h-0 sm:h-8`. CLAUDE.md reserves `sm` for "dense tables/cards on desktop" — a full-width wizard footer on mobile is neither.
- **Fix**: Switch those four buttons to `size="default"`.
- **Suggested command**: `/impeccable adapt`

**[P1] Silent auto-add of a product without user action.**
- **Why it matters**: `NfceEmitForm.tsx:485-488` — leaving the "Consumidor" step with zero products picked silently injects the first CFOP-eligible catalog product into the cart. Verified by reading `goNext`. This is an invisible mutation of a legally-binding fiscal document; a cashier who doesn't notice ships the wrong line item with no cue anything happened.
- **Fix**: Gate advancement on an explicit product pick (mirror the `produtos`-step `canNext` check onto `consumidor`), or show a visible inline toast the moment the auto-add fires.
- **Suggested command**: `/impeccable harden`

**[P1] Disabled "Emitir" gives no reason — and the app already wrote the explanation, it's just unreachable.**
- **Why it matters**: `MdfeEmitForm.tsx:496` gates `canEmit` on 5 unstated conditions (docs/vehicle/drivers/weights/bulk-cargo CEPs). Verified: `handleSubmit` (line 501-504) *does* set a specific message — `'Preencha carga, trajeto, veículo cadastrado e ao menos um condutor.'` — but the button is `disabled={!canEmit}`, so that code path can never execute; the one sentence that would answer "why can't I submit?" is dead code. Same shape in NFC-e (`canEmit` at line 532, message logic in `handleSubmit` 495-508, same disabled-button unreachability). Error boxes also lack `role="alert"`/`aria-live`, so screen-reader users get total silence when validation does surface.
- **Fix**: Surface the existing message text near the disabled Emit button instead of only inside the never-reached `handleSubmit` guard; add `role="alert"` to both `submitError` blocks.
- **Suggested command**: `/impeccable clarify`

**[P2] Step indicator is inaccessible on mobile, not just color-coded.**
- **Why it matters**: `StepIndicator` (`NfceEmitForm.tsx:365-389`, mirrored in `MdfeEmitForm.tsx`) renders step labels in `<span className="text-xs hidden sm:block">`. Verified: `hidden` is `display:none`, which removes the label from the accessibility tree entirely on mobile — a screen-reader user on a phone (Sam using VoiceOver, on the exact device class CLAUDE.md's mobile-first mandate targets) hears only a bare number or "✓" per step, no label, on every screen of this flow. No `aria-current="step"` either.
- **Fix**: Use `sr-only` instead of `hidden` for the label span so it stays in the accessibility tree at all breakpoints; add `aria-current="step"` to the active step.
- **Suggested command**: `/impeccable adapt`

**[P2] Silent success redirect at the highest-stakes moment in the flow.**
- **Why it matters**: `handleSubmit` calls `apiClient.emitNfce`/`emitMdfe` then immediately `router.push` (`NfceEmitForm.tsx:523-524`, `MdfeEmitForm.tsx:526-527`) — no toast, no acknowledgment. Given the app's own "Pendente" pulsing badge shows SEFAZ authorization is asynchronous, the moment right after submission ("did the tax authority actually get this?") is the single most anxious point in the task, and it gets zero reassurance — undercutting the confirm modal's promise one step earlier.
- **Fix**: A brief success toast ("Enviado, aguardando autorização da SEFAZ") before/at redirect.
- **Suggested command**: `/impeccable onboard`

## Persona Red Flags

**Jordan (first-timer)**: Gets a product silently added to their cart they never picked, with no visible cue (P1 above). Hits a disabled "Emitir MDF-e" with no stated reason even though the app already wrote one internally. MDF-e's jargon (fronteira, carga lotação, modal) has zero `GlossaryTerm` coverage, unlike NFC-e's two covered fields.

**Sam (accessibility-dependent)**: On mobile, `StepIndicator` labels are `display:none` (not just visually de-emphasized) — VoiceOver announces bare numbers with no context at every step. Row-selection checkmarks in `DocumentPicker` (`MdfeEmitForm.tsx:145-150`) are decorative spans with no `aria-pressed`/`aria-checked`. `submitError` boxes have no `role="alert"` — validation failures after clicking Emit are silent to a screen reader.

**Casey (distracted mobile)**: The always-visible sticky nav is 28px tall on a 375px screen — the most-tapped control in the whole flow is also the easiest to mis-tap (ties to the P1 touch-target finding). `MunReorderList`'s ↑/↓ reorder buttons are `icon-sm` (28px) with only `gap-2` between them — a fat-finger mis-tap silently reorders MDF-e trajectory data with no confirmation.

## Minor Observations

- `err instanceof Error ? err.message : '...'` surfaces raw thrown-error text verbatim on a compliance-facing screen (`NfceEmitForm.tsx:526`, `MdfeEmitForm.tsx:529`) — risk of leaking a technical/Axios string to the user.
- Manual CFOP fallback `<Input>` (when a product has no configured CFOP list) never shows the design system's invalid/red-border state even when it's failing `canNext`.
- `MODAIS` icons wrapped in `text-2xl` (`MdfeEmitForm.tsx:553`) — font-size has no effect on an SVG sized via explicit width/height props; likely a no-op class.
- `PAYMENT_OPTIONS` renders every SEFAZ payment-type code flat in one `<select>` with no grouping (Dinheiro/Cartão/PIX/Outros), past the cognitive-load guide's ≤4-choices-per-decision target.
- The card/PIX detail checkbox (`h-3.5 w-3.5`, `NfceEmitForm.tsx:619-624`) is a 14px box in a tight row, well under the 44px rule.

## Questions to Consider

- The app already shows a "Pendente" pulsing badge for async SEFAZ authorization elsewhere — why does the emit flow itself end in a bare redirect instead of closing that same loop?
- The confirm modal shows 3 fields for a document assembled over 3-6 steps and several minutes of entry — is that minimalism deliberate, or should the one screen carrying an explicit "cannot be undone" warning show more of what's actually about to be filed?
- `canEmit`/`canNext` already compute exactly which condition is unmet — what's stopping that same boolean logic from also driving the message shown to the user, instead of duplicating (and burying) it inside an unreachable `handleSubmit` branch?
