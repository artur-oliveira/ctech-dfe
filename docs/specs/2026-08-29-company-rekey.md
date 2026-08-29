# Re-keying the DF-e onto platform companies

## Outcome

The DF-e stops owning who a company *is* and stops keying its data by CNPJ. Every table it
partitions by `CNPJ_{digits}` moves to the platform `company_id` issued by `ctech-account`,
and the DF-e keeps exactly what is its own: inscrição estadual, regime, fiscal address,
série and numbering, CSC/CSRT and the A1 certificate.

This implements [ctech-billing ADR 0022](../../../ctech-billing/docs/adr/0022-company-identity-in-account.md)
on this side of the line. The other side already shipped:
[platform companies](../../../ctech-account/docs/specs/2026-08-29-platform-companies.md).

## The forcing function

This is not tidying. The current key is **wrong** as of ADR 0022, and stays wrong until it
moves.

`repositories.ParseOrgPK` accepts `CNPJ_{digits}` or `CPF_{digits}`
(`api/internal/repositories/organizations.go:16`), so the partition key *is* the tax id and
is therefore globally unique per document. ADR 0022 decided the opposite: two organizations
may each hold the same CNPJ — the accountant carrying a client's CNPJ while the client's own
staff also issues, and the client who changes accountants.

Under today's key those two are the same partition. They would share one certificate, one
fiscal config, one numbering sequence and one document list. Not a collision the product
detects and refuses: a silent merge of two customers' fiscal data.

So the choice is between the key moving and ADR 0022 not being true. Everything below is
the cost of the first.

## The seam already exists

The expensive-looking part is cheap. Every repository in this codebase takes the partition
key as a parameter — 188 references, none of which construct it. The key is resolved once,
at the edge:

- `PermChecker.parseUserOrganizationRole` reads the `Dfe-Organization-Pk` header (or the
  `:org_pk` path parameter), validates it, and publishes it to locals
  (`api/internal/middleware/rbac.go:66-111`).
- `middleware.GetOrgPK(c)` is what every handler reads.
- `subscription.go`'s `organizationOf` looks in the same two places.

Change what those resolve to and the whole write and read path follows. **The code change is
the middleware and four places that read the CNPJ back out of the key; the data change is
the whole cost.** Sections below name both.

## What the DF-e keeps, and what it reads

| | Lives in `ctech-account` | Lives here |
|---|---|---|
| Who the company is | `tax_id`, `tax_id_kind`, `legal_name`, `trade_name` | — |
| Who may act for it | membership + the `User ↔ Company` actor edge | the permission model (unchanged, phase 3) |
| How it emits | — | IE, CRT/regime, fiscal address, série, numbering, CSC/CSRT, the A1 certificate |

**The certificate never leaves.** It is a private key, and nothing about it — not even a flag
saying one exists — is mirrored into `ctech-account`.

### The local company record

`{env}_organizations` stops being an identity record and becomes the DF-e's projection of one,
re-keyed to `pk = {company_id}`:

| Field | Source |
|---|---|
| `organization_id` | accounts; the workspace this company belongs to |
| `tax_id`, `tax_id_kind`, `legal_name` | accounts; **a cache**, refreshed on read-through |
| every fiscal field | here; the DF-e is the authority |

Two ids are stored, not one, because authorization needs both: the actor edge is
`(organization_id, company_id, user_id)`, and a `company_id` alone cannot be checked without
first discovering its organization. One `GetItem` here answers both, which keeps the hot path
one read.

The cached identity is a cache and must read like one: a rename in accounts is not an error
here, and nothing in this repo may treat its copy as authoritative.

> **Amended during implementation.** This section first said the identity is refreshed on read
> once its copy passes a TTL. There is no way to do that: `ctech-account`'s company routes sit
> behind `RequireClientID(SelfClientID)`, so a dfe-issued token is refused, and no service
> credential exists for this direction. Inventing one is a cross-service auth decision and not
> a detail of this re-key.
>
> So **the identity is written and never re-read**: by the migration, and by the handoff that
> links a company. The cost is small and worth stating, because "the cache goes stale" is the
> obvious objection. `tax_id` and `tax_id_kind` never change — a company whose tax id was wrong
> is a different company, and the answer is to register that one. `legal_name` is display-only
> here; the `xNome` on a document comes from this repo's own `name` field. A refresher belongs
> in the phase that also decides the credential.

## The série rule this spec owes ADR 0022

ADR 0022 upgraded a limit from "accepted" to "enforced", and this is the repo that enforces it:

> **An NF-e is unique by (CNPJ, modelo, série, número, ambiente).** Two enabled companies
> sharing a tax id must not share a série.

`ctech-account` lets duplicate identity through on purpose — a CNPJ is public, and registering
it is a claim, not a capability. The hazard is in issuance, so the refusal belongs here.

**Where:** at the point a company is enabled for a document type, and at any change to its
série. Not at issuance time — a rejection there means somebody already believes they can emit,
and a numbering gap may already exist.

**How:** a conditional write on a claim row keyed by `(tax_id, modelo, série, ambiente)`,
scoped to nothing — deliberately global, because the SEFAZ is global. The same shape
`ctech-account` uses for its tax-id lock, and for the same reason: a read-then-write lets two
concurrent enablements both find the série free.

**What the person sees:** the série is already in use for this CNPJ, and they must choose
another. Naming *which* organization holds it would disclose that somebody else carries their
CNPJ, which is not ours to say — the person's own accountant is not our fact to reveal.

## What breaks when the key stops being a CNPJ

Four places read the tax id back out of the partition key. Each fails silently under a UUID —
no error, just wrong behaviour — so each is named here rather than left to be discovered.

**1. `cnpjRoot` and matriz/filial certificate reuse.**
`services/organizations.go:97` slices the raiz out of the PK, and `branchCertificate` uses it
to find a sibling organization sharing that root so a filial can reuse the matriz PFX. Under a
UUID, `cnpjRoot` returns `""`, `branchCertificate` returns nil, and every filial registration
starts demanding its own certificate — with the message "certificado A1 é obrigatório", which
tells the customer nothing about what actually broke.

Fix: take the root from the **cached `tax_id`**, and scope the sibling search to the same
`organization_id`. Scoping it is not incidental — under ADR 0022 two organizations may hold
the same raiz, and a search that ignored the workspace would offer one customer another
customer's certificate.

**2. `ParseOrgPK`.** The prefix check is the only place that hardcodes the key's shape. It
becomes a UUID check. Its error message is user-facing and must stop saying "deve começar com
CNPJ_ ou CPF_".

**3. The `Dfe-Organization-Pk` header.** Its comment says it must match
`ui/src/lib/api/client.ts` and never be renamed (`middleware/rbac.go:22`). The name stays; the
value becomes a `company_id`. Renaming it would be a coordinated deploy across two apps for a
word, and the comment is right.

**4. Anything reading a CNPJ off a document partition.** `documents.go:16` documents
`pk = {env}#{org_pk}`, which becomes `{env}#{company_id}`. Grep for `CNPJ_` before the flip;
the compiler catches nothing here.

## The migration

Two passes, and the second is the expensive one.

### Pass 1 — companies into `ctech-account`

The organization migration that already ran mapped dfe *organizations* onto platform
*organizations*. They were really companies, so this pass unfuses them: for each dfe
organization, register a Company in `ctech-account` under the platform organization that pass
one created, carrying `source_system` / `source_ref` for the same idempotency.

`source_ref` is the old `CNPJ_{digits}` PK, so the mapping `old PK → company_id` is
recoverable at any time, by anybody, without a file that has to survive.

Every membership becomes an actor edge on that company. The permissions list that pass one
sent to the "needs a human" bucket is still not representable, and still must not be silently
dropped — the same report, the same refusal to guess.

### Pass 2 — re-key every table

**Copy, do not move.** Every row is written under the new key and the old partition is left
exactly as it is. That is the whole rollback plan: if the flip goes wrong, the middleware
points back at the old key and every byte is still there. Deleting the old partitions is a
separate, later, deliberate step — after a full numbering cycle has passed under the new key.

Tables, by shape:

- **Singleton fiscal configs** — `organization_{nfe,nfce,cte,mdfe,nfse}_configs`. One row per
  organization becomes one row per company. `preserve` fields (NSU cursors) copy across, or
  the distribution sweep re-reads years of documents on its next run.
- **Registry entities** — the fourteen `organization_*` tables behind `getOrgEntityTable`
  (tax profiles, operations, payment terms, vehicle sets, terminals, toll providers, cargo
  units, import declarations, insurance policies, product lots, fuel pumps), plus
  certificates, products, vehicles, persons and services. Same `pk/sk` shape; only the pk
  changes.
- **Documents** — `nfes`, `nfces`, `ctes`, `mdfes`, `nfses` and their events and
  distributions. The largest by far, and the only ones where the copy is not instant.

### Why documents are copied and not aliased

The obvious cheaper design is to leave documents under the old key forever and read both
partitions, merging. It was rejected.

Merging two partitions under a `LastEvaluatedKey` is a page-boundary problem, and getting it
subtly wrong duplicates or skips rows in a list of fiscal documents — a defect that looks like
a missing note and is discovered by an accountant, not by a test. It would also be permanent:
a one-time cost traded for complexity in the most-used read path, forever.

The copy is a script that runs once. It is the larger cost this week and the smaller one in
every week after.

**The threshold this reasoning depends on:** the copy is viable because the document count is
small enough to fit a maintenance window. It is small today. If it ever is not, the answer is
not the alias — it is a longer window, or a per-company cutover, both of which keep the read
path simple.

### Ordering and the write window

1. Deploy the schema and the new code paths, still resolving the old key. Nothing changes.
2. Run pass 1. `ctech-account` gains the companies; the DF-e is untouched.
3. **Freeze writes** — a maintenance window outside business hours; a fiscal emitter has no
   overnight traffic worth protecting, and the alternative is copying under concurrent writes.
4. Run pass 2. Idempotent and resumable, like pass 1: a run that dies mid-table is completed
   by the next, not restarted.
5. **Verify before flipping.** Per table, per company: row counts match, and a sampled
   comparison of item bodies matches. A flip onto an unverified copy is how a partial migration
   becomes a silent data-loss incident.
6. Flip the middleware to resolve `company_id`. Lift the freeze.
7. Watch one numbering cycle. Then, and only then, plan the deletion of the old partitions.

The verification at step 5 is the step most likely to be skipped under time pressure and the
one that must not be. It is what makes step 4's failure recoverable instead of invisible.

## Quota

Unchanged, and restated because the re-key is where it would drift: **the product counts its
own quota.** Billing carries `quota_companies` as opaque metadata (ADR 0008) and the DF-e
decides what a company is.

Two counters, never one (ADR 0021): companies that *exist* in an organization — which is
`ctech-account`'s number now — and companies *enabled for the DF-e*, which is this repo's.
The quota applies to the second. A company registered in accounts and never enabled here costs
nothing, which is what lets one organization hold forty CNPJs and emit for one.

Downgrading below the enabled count disables nothing by itself. It refuses new enablements and
asks a person to choose. A system that picks which company stops emitting picks the wrong one.

## Out of scope, deliberately

- **Membership and RBAC unification.** `ctech-account` owns membership, and this repo keeps
  its `action.resource` permission model and its own `organization_users` rows. Collapsing them
  is phase 3 and needs the answer to the permissions grants pass 1 could not represent. Doing
  both at once means a data migration and an authorization rewrite in one window.
- **Deleting the old partitions.** Named in the ordering above as a later, separate decision.
- **The handoff screen's DF-e side** — the landing route that reads `organization_id` and
  `company_id` and links them. Its contract is in the
  [handoff spec](../../../ctech-account/docs/specs/2026-08-29-organization-handoff.md),
  including the two rules it must honour: the landing route is idempotent under a refresh, and
  it must accept a `company_id` it has never seen, because a person can register in accounts
  first and arrive here later.
- **Anything fiscal moving to `ctech-account`.** Stated in ADR 0022, repeated here because the
  re-key is exactly when somebody proposes "the IE could live there too, it's small".
