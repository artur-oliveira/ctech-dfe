---
target: src/app/page.tsx
total_score: 29
p0_count: 0
p1_count: 1
timestamp: 2026-07-15T01-55-10Z
slug: src-app-page-tsx
---

# Critique — src/app/page.tsx (ctech-dfe landing page)

Method: dual-agent (A: a7d6b069803f01a52 · B: a5176bef4b1b73fc9)

## Design Health Score (Nielsen /40 = 29, Good)

| #         | Heuristic                   | Score     | Key Issue                                                        |
|-----------|-----------------------------|-----------|------------------------------------------------------------------|
| 1         | Visibility of System Status | 3         | "Beta / cobrança não habilitada" buried as small gray disclaimer |
| 2         | Match System ↔ Real World   | 4         | Accurate PT-BR + correct fiscal model numbers                    |
| 3         | User Control & Freedom      | 3         | Plan cards have no CTA — browse-only                             |
| 4         | Consistency & Standards     | 4         | Tokens/buttons consistent; two heading systems coexist           |
| 5         | Error Prevention            | 3         | "Sob demanda" per-doc pricing beside subscriptions               |
| 6         | Recognition Not Recall      | 3         | CT-e → /cte vs siblings → /emit                                  |
| 7         | Flexibility & Efficiency    | 2         | No section nav / sticky header                                   |
| 8         | Aesthetic & Minimalist      | 2         | 7 sections + 5 tiers violate calm-by-default                     |
| 9         | Error Recovery              | 3         | No forms/error states modeled                                    |
| 10        | Help & Documentation        | 2         | Footer only; no FAQ                                              |
| **Total** |                             | **29/40** | Good                                                             |

## Anti-Patterns Verdict

LLM (A): WEAK, not fail. No absolute bans. `bg-gradient-login` is restrained brand wash. Tells: uppercase tracked mono
eyebrows (Documentos suportados, Como funciona) + hero kicker; 01/02/03 numbered flow; four near-identical card grids.

Deterministic (B): page.tsx exit 0 / []. authorization-card.tsx: 7 advisory design-system-font-size hits (0.65rem x2,
0.8rem, 0.6rem, 0.7rem x3). False positive: 0.8rem documented in DESIGN.md prose but absent from parsed YAML ramp.
Genuine drift: 0.6/0.65/0.7rem (6/7) outside ramp + documented Caption step. Browser overlay unavailable (no dev
server / no browser automation).

## Overall Impression

Competent, honest, on-brand. Best asset (self-playing AuthorizationCard SEFAZ round-trip) is also loudest element. Reads
as "template that got a good idea." Biggest opportunity: collapse choice surface (5 tiers, 7 sections) so "simples"
becomes the experience.

## What's Working

1. AuthorizationCard live simulation — demonstrates competence, distinctive.
2. Restrained on-brand palette — single green, per-doc accents, no gradient text/glass.
3. Honest PT-BR copy + upfront "cobrança não habilitada" disclaimer.

## Priority Issues

- [P1] clarify — Inconsistent doc-card destinations (CT-e /cte vs others /emit) + ungated app entry for logged-out
  users. Fix: one consistent route or login-gated intent.
- [P2] distill — 5 pricing tiers analysis paralysis; hide direct-SEFAZ differentiator. Fix: collapse to 3 or fold
  behind "ver todos".
- [P2] bolder — No closing CTA; weak peak-end. Fix: closing band → /login.
- [P2] adapt — AuthorizationCard overflows at 375px (chave de acesso ~54 chars in px-5 box, no break). Fix: break-all /
  overflow-wrap.
- [P3] typeset — Two competing heading systems (mono eyebrow vs h2 vs hero kicker). Fix: one pattern.
- [P3] audit — Font-size drift in card (0.6/0.65/0.7rem) + accent micro-text contrast (MDF-e #f59e0b ~2.1:1, primary-500
  numerals); gray disclaimers at AA threshold on tints. Fix: reserve accents for non-text, gray-600 disclaimers.

## Persona Red Flags

- Jordan: only "Entrar" CTA, no visible "Criar conta" path.
- Riley: no trust signal near CTA (no SEFAZ cert / homologação proof; unbacked 99,9% uptime).
- Sam: gray-500 at AA threshold; card accent micro-labels fail; AuthorizationCard autoplays with no pause control.
- Casey: AuthorizationCard chave de acesso overflow at 375px.

## Minor Observations

- Footer "A O CARVALHO TECH" odd casing.
- Hero eyebrow duplicates Documents section.
- Roadmap double-labeled (badge + heading).
- Max "ilimitadas" overlaps Sob demanda unlimited.
- lg:grid-cols-5 plan cards too narrow; long feature strings wrap awkwardly.

## Questions

1. Lead with "calm/simple" or with the SEFAZ-rejection fear then resolve it?
2. Does AuthorizationCard autoplay contradict calm-by-default? Play button better?
3. Does 5-tier grid hide the direct-SEFAZ differentiator?
