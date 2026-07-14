# Product

## Register

product

## Platform

web

## Users

Mixed audience, served through one portal plus a developer API:

- **Accountants / escritórios** issuing and managing documents on behalf of many client companies — high volume, multi-tenant, the heaviest daily users.
- **Business owners / internal finance teams** issuing their own company's documents and tracking status.
- **Developers / integrators** driving the issuance pipeline programmatically via the API (secondary; reached through docs and API surfaces, not the portal).

The portal UI is optimized for the human operators (accountants and business owners); API developers are a secondary audience who experience the brand through docs and integration flows.

## Product Purpose

A SaaS for issuing and managing Brazilian Electronic Tax Documents — NF-e, NFC-e, CT-e, MDF-e — through direct SEFAZ communication (SOAP + mTLS), with real-time document status over WebSocket. It exists so that a user can issue, consult, and track a fiscal document without being crushed by the underlying bureaucratic complexity. Success looks like: a document issued and its status known, in the fewest calm, clear steps possible.

## Positioning

Issuing Brazilian tax documents doesn't have to feel bureaucratic — the fastest, clearest flow in the category. Every screen reinforces "simplest in category": less jargon, fewer clicks, real-time certainty.

## Brand Personality

Modern & efficient. Calm confidence, fast, sharp, no-nonsense. The voice is direct and plain-language — it translates fiscal jargon into what the user actually needs to do, and stays reassuring about compliance without performing bureaucracy.

## Anti-references

- Bureaucratic government portals: dated, low-contrast, no hierarchy, anxiety-inducing, "you are here to fill a form the state demands."
- Generic SaaS cream/beige monoculture dashboards that feel like every other tool.
- Playful / gamified consumer aesthetics that undercut fiscal seriousness.
- Dense enterprise tools with no visual hierarchy or breathing room.

## Design Principles

- **Clarity over compliance theater.** Surface only what the user needs to act; hide fiscal complexity behind sensible defaults and progressive disclosure.
- **One fast path.** Every primary task is reachable in minimal steps, and real-time status means users never wonder "did it go through?".
- **Calm by default, loud only when it matters.** Quiet baseline UI; errors and deadlines earn high-signal treatment.
- **Familiar per doc-type identity.** The contextual DF-e theme (NF-e green / NFC-e blue / CT-e violet / MDF-e amber) gives each document type its own recognizable color without rebuilding layouts — one system, four accents.
- **Accessibility as baseline, not afterthought.** WCAG 2.1 AA and Brazilian eMAG expectations; mobile-first from 375px; reduced-motion respected.

## Accessibility & Inclusion

WCAG 2.1 AA, with attention to Brazilian eMAG (government-adjacent fiscal context). Mobile-first (375px baseline). Reduced-motion alternatives required for all animation. Body text must hold ≥4.5:1 contrast; high-signal states (errors, deadlines) must not rely on color alone.
