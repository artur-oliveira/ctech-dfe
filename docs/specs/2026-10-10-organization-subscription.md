# The DF-e subscription belongs to the organization

Status: **Design approved, not implemented** · 2026-10-10 ·
Upstream: `ctech-billing/docs/specs/2026-10-10-plans-design.md` § 8 (organization customers) ·
Supersedes: decisions D1 and D1a of [`2026-08-16-assinaturas-billing-dfe.md`](../plans/2026-08-16-assinaturas-billing-dfe.md) ·
Related: ctech-billing ADR 0021 (organization as workspace; two counters), ADR 0023 (membership in account,
authorization in the product), ADR 0025 amendment of 2026-10-07 (an organization's subscription is managed by
its owners and admins)

## Outcome

The DF-e subscription moves from the owner's user (`USER_{sub}`) to the **ctech-account organization** that holds
the companies (`ORG_{organization_id}`). Any `owner` or `admin` of that organization sees, pays and manages it.
Quotas, usage counters and the subscription snapshot follow the organization.

## Why D1a no longer holds

D1 put the subscription on the user because "per organization" would have charged Pro ten times to someone with
ten companies. In the DF-e of 2026-08, *organização* meant one **company** (one CNPJ). Since the re-key
(`2026-08-29-company-rekey.md`) every company belongs to a ctech-account organization, which is a **workspace**
that holds many companies. A subscription on that workspace charges once for all of them, and `quota_companies`
means what it says: Pro covers 10 companies inside one organization.

What D1 did that this spec does not keep: one subscription covering companies in **different** organizations of
the same owner. Each organization now pays for itself. Two organizations are two customers — which is what an
accountant firm and a personal company are.

## Decisions

| # | Decision |
|---|---|
| O1 | Customer is `ORG_{organization_id}`, the organization of the company being acted on (from reach). |
| O2 | `owner` and `admin` of the organization (ctech-account role) manage the plan; others see it. |
| O3 | Existing subscriptions migrate by a one-shot, idempotent script, dry-run by default; it **stops** on any paid subscription instead of duplicating a charge. |
| O4 | The users quota is removed: `quota_users` leaves the catalogue and the UI, `price_dfe_ondemand_user` is archived. |
| O5 | The company quota is checked where a company starts to count: when it is **enabled** (first fiscal configuration), not only on the legacy create route. |
| O6 | Over the limit, nothing is removed or blocked that already exists; only enabling a new company is refused. |

## 1. Keys and services

**Re-keyed from `USER_{sub}` to `ORG_{organization_id}`** (`repositories/account_billing.go`):

| Item | Before | After |
|---|---|---|
| Subscription snapshot | `pk=USER_{sub}` | `pk=ORG_{organization_id}` |
| Usage counters | `USAGE_{sub}#{period}` | `USAGE_{organization_id}#{period}` |
| Company quota guard | `QUOTA_GUARD_{sub}#companies` | `QUOTA_GUARD_{organization_id}#companies` |
| Reservations in the results consumer (`consumer/results.go`) | user + period | organization + period |

The 13-month TTL and the conditional `ADD` against the limit are unchanged.

**`services/billing.go`:**
- `GetOrCreateCustomer(organizationID)` sends `ExternalRef: ORG_{organization_id}` and the name and document
  of the organization's **billing company** (its first linked company: legal name and CNPJ), or the owner's name
  and CPF when it has none. Billing writes the `CUSTOMER_ORG#` pointer (its spec § 8).
- `Snapshot`, `Sync`, `Choose`, `Change`, `Cancel`, `Usage` take the organization id.
- `SnapshotForOrg(companyPK)` resolves company → organization (the reach answer, already cached per company and
  user) → snapshot. `OwnerOf` and `ownedOrganizations` are removed with the dual read (§ 4).
- `companiesUsed(organizationID)` counts the **enabled** companies whose `organization_id` is that organization.
  Local company records already carry `organization_id` (`LinkService.Link`); a GSI on it answers the list. The
  enablement source is unchanged (`FiscalConfigEnablement`), and its fallback (count all, the stricter answer)
  stays.

**Webhook:** `SyncBySubscription` accepts `ORG_` references. A `USER_` reference is logged and ignored; after
the migration the only ones expected are the cancellations of the old subscriptions.

## 2. Routes, authorization and UI

- `/billing/subscription*` requires `Dfe-Organization-Pk` (the selected company). The subscription addressed is
  that of the company's organization.
- **Managing** (choose, change, cancel, pay) requires the ctech-account role `owner` or `admin` in that
  organization, read the same way reach is. Others get **403** on the writes and can read the plan and usage.
- `GET /organizations/:org_pk/plan` stays, answering for the organization of that company.
- **Subscription gate** (`middleware/subscription.go`): unchanged semantics — fail-open when billing does not
  answer, only `ACTIVE`/`TRIALING` grant — now keyed by the organization.
- **UI:**
  - `assinatura` and `onboarding/plano` act on the current company's organization and name it ("Plano da
    organização X").
  - Switching between companies of the same organization does not change the plan shown.
  - `useSubscriptionNotice` shows to `owner` and `admin`.
  - The users meter leaves `UsageList` and `lib/constants/billing.ts`.
- An organization with no subscription sends an `owner`/`admin` to plan selection, as onboarding does today; a
  `member` sees "a organização ainda não escolheu um plano".

## 3. Quotas

**Users (O4).** Removed from `ctech-billing/api/tenants/ctech.json` (`quota_users` on every DF-e price;
`price_dfe_ondemand_user` archived). People in an organization are ctech-account's business and are shared
between products; a DF-e quota on them would make an invitation to a Finanças-only organization depend on a
DF-e plan. The code already stopped enforcing it (`billing.go`, the note above `CheckMDFEScope`).

**Companies (O5).** A company counts when it is enabled (ADR 0021). Today the check sits only on the legacy
create route (`api/v1/organizations.go:111`), so a linked company that later gets a fiscal configuration is
never checked and never reported on the on-demand price.
- One function, `ReserveCompany(organizationID, companyPK)`, used where a company **becomes** enabled: the first
  fiscal configuration saved for a company with none. It checks the quota, writes the `QUOTA_GUARD_` item in the
  same transaction as the configuration, and reports `companies` usage with key `company:{companyPK}`, so a
  company disabled and re-enabled in one period is one charge.
- `Link` stays unchecked on purpose: a linked, unconfigured company cannot emit and costs nothing.
- Above the limit (after a downgrade or the migration), existing enabled companies keep emitting; only enabling
  another is refused (402 `quota_exceeded`).

**Documents.** Unchanged, re-keyed. **CT-e** has a table and a meter (`MeterCTe`, `MeterForTable["ctes"]`) but
no emission path yet. When CT-e emission is built it must call `PrepareUsageReservation(…, MeterCTe, …)` like
the other four; a Free plan (`quota_cte: 0`) then refuses it with the quota 402.

## 4. Migration

`cmd/migrate-billing-org`, idempotent, `--dry-run` by default (`--apply` to write):

1. List `USER_` snapshots with an entitled subscription.
2. For each, ask ctech-account which organizations the user owns that have companies in the DF-e.
3. If the subscription's price is not R$ 0, **stop for that user** and list it. Nothing is duplicated.
4. For each organization: `GetOrCreateCustomer(ORG_…)`, create a subscription with the **same `price_id`**, `Sync`.
5. Copy the current period's counters `USAGE_{sub}#{period}` to `USAGE_{org}#{period}` for each migrated
   organization, so nobody gets a fresh quota mid-month.
6. Cancel the `USER_` subscription.

Before running it, production is checked: at design time there were two entitled subscriptions, both on
`unlimited_internal` (R$ 0).

**Deploy order:**
1. ctech-billing: `ORG_` customers and the pointer (its spec § 10, step 1).
2. DF-e with a **dual read**: snapshot by `ORG_`, falling back to the owner's `USER_` while the migration window
   is open. Writes (usage, guard) go to `ORG_` only.
3. The script: dry-run, review, `--apply`.
4. A deploy that removes the dual read, `OwnerOf` and `ownedOrganizations`.

## 5. Tests

1. Two companies of one organization share one snapshot and one counter.
2. Two organizations of the same owner have separate quotas and subscriptions.
3. `member` → 403 on choose/change/cancel; `admin` → allowed; both read the plan.
4. Webhook with `ORG_` updates the right snapshot; `USER_` is ignored and logged.
5. Script: `--dry-run` writes nothing; `--apply` twice creates nothing the second time; a paid subscription is
   listed and skipped; counters copied.
6. Dual read: an organization with only the owner's `USER_` snapshot is served from it; with an `ORG_` one, the
   `ORG_` wins.
7. First fiscal configuration above the company limit → 402 and no configuration written; re-enabling in the
   same period reports once.
8. `Link` above the limit → succeeds (unconfigured company), as today.
9. Usage UI no longer lists users; the catalogue has no `quota_users`.

## Not in this spec

- CT-e emission itself.
- Organizations' Finanças plans (ctech-billing plans spec, out of scope there too).
- Moving a company between organizations (ctech-account has no such operation).
