# Personal workspaces — what the DF-e does about them

Status: **Design approved, not implemented** · 2026-10-09 · Upstream:
`ctech-account/docs/specs/2026-10-09-personal-workspaces.md` · Origin:
`ctech-billing/docs/specs/2026-10-09-shared-spaces-design.md` (ADR 0027 there)

## Context

ctech-account is gaining workspaces of `kind: personal`: lightweight shared spaces with no company,
used by ctech-billing to share finance between people (a couple, a household). Workspaces that already
exist are `kind: organization`, and an absent kind reads as that.

When this was designed, the account and billing specs said the DF-e would have to **filter personal
workspaces out of its organization list**. Reading the DF-e's source shows that is not so, and this
spec records why, and what little the DF-e does instead.

## Why the DF-e is not exposed

1. **The DF-e never lists ctech-account workspaces.** Its "organizações" screen
   (`GET /v1.0/organizations`, `ui/src/app/organizations/page.tsx`) lists the DF-e's own local company
   records, one per `company_id`. Nothing in `api/` or `ui/src` calls account's
   `/internal/users/:user_id/organizations` or `/v1.0/organizations`.
2. **Everything the DF-e does is keyed on a company**, and access to a company is decided by
   ctech-account's company-actor edge (`services.ReachService.MayAct`, ADR 0023).
3. **A personal workspace cannot hold a company.** ctech-account refuses to link one (its spec § 1),
   so there is no company id for the DF-e to reach.
4. **The handoff creates organizations only.** `startCompanyHandoff` goes to
   `/account/organizations/new`, which never creates a `personal` workspace (account spec § 5).
5. **`LinkService.Link` already verifies** that the company's organization is the one on the return
   URL, from ctech-account and not from the URL (`checkReach`).

So no screen and no route here can show, select or act on a personal workspace. **No UI filter is
needed.**

## What the DF-e adds: a kind check, in depth

Points 3 and 4 are ctech-account's rules, enforced in another repository. If either one ever
regressed, a company could exist under a personal workspace, and the DF-e would issue fiscal documents
for it as if it belonged to an organization. The DF-e checks the kind itself:

1. **ctech-account adds `organization_kind`** to the answers of
   `GET /internal/companies/:company_id/actors/:user_id` (`{may_act, organization_id,
   organization_kind}`) and `GET /internal/organizations/:organization_id/companies/:company_id`
   (`Identity`). This is the upstream change; see the amendment in ctech-account's spec § 4.
2. **`reachAnswer` stores the kind** beside `organization_id`, under the same key and TTL. A cached
   entry without it reads as `organization`, which is what every workspace was before this change.
3. **`checkReach` and the reach middleware refuse** with the existing 403 ("você não tem acesso a esta
   empresa") when the kind is anything other than `organization`. Failing closed matches the rest of
   the reach path: an unknown kind is not consent.
4. **`LinkService.Link` refuses** an `Identity` whose kind is not `organization`, before writing any
   local row.

## Tests

- reach answering `organization_kind: personal` → 403 on the reach middleware and on link, with no
  local row written;
- an unknown kind → 403;
- an absent kind (the route before the upstream change, or a cache entry from before this deploy) →
  behaves exactly as today;
- the cache key is unchanged (`dfe:reach:{company}:{user}`), and a cached personal answer is refused
  on read, not only on fetch.

## Deploy order

ctech-account first. Then the DF-e. Until the upstream route sends `organization_kind`, the DF-e reads
every workspace as `organization`, which is today's behaviour.

## Not in this spec

- Moving the DF-e subscription from the owner's user to the organization (ctech-billing PLAN.md,
  decided 2026-10-07). That is its own spec, with its own decisions: migrating live subscriptions,
  what the per-user quota counts, and what happens to companies above the quota.
- Any use of personal workspaces by the DF-e. A household has nothing to issue.
