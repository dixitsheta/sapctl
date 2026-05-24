# ADR 0006 — Status page: static page + JSON source of truth over hosted Statuspage

**Status:** Accepted (Phase 7 -- 2026-05-24)
**Date:** 2026-05-24
**Deciders:** CTO / Architect
**Supersedes:** none
**Superseded by:** none

## Context

Phase 7 task 7.3 requires a public status page at `status.sapctl.dev`. When a paying
customer's CI breaks because the release pipeline hiccupped, they need a source of
truth that is not a tweet. Constraints:

- **No new monthly cost.** Y1 budget is tight; the SOC 2 vendor is already the
  biggest single line item.
- **Same toolchain as the rest of the site.** `sapctl.dev` is static HTML on
  Cloudflare Pages with a `/api/releases` edge function. A status page should not
  introduce a new platform to operate.
- **Self-serviceable by automation.** Release and incident state should be
  updatable by a GitHub Action, not a human logging into a SaaS dashboard.
- **Degrades gracefully.** If the status feed is unavailable, the page must say so
  rather than render a misleading "all green".

## Decision

Status page is **a static HTML page backed by a committed `status.json`**, not a
hosted service (Statuspage.io / Better Uptime / Instatus).

1. **Source of truth:** `apps/web-static/status.json` — committed to the repo.
   Shape: `updated_at` (ISO 8601), `overall` (enum: operational | degraded |
   maintenance | down), `components[]` (id, name, status, description),
   `incidents[]` (title, date, status). A GitHub Action updates it on release or
   incident.
2. **Page:** `apps/web-static/status.html` renders `status.json` client-side and
   additionally calls the existing `/api/releases` edge function for a live
   "current signed release" line. No new backend.
3. **Hosting:** Cloudflare Pages project on subdomain `status.sapctl.dev`. The
   subdomain isolates a status outage from a marketing-site outage (separate Pages
   project, separate deploy).
4. **Incidents feed:** `incidents.rss` (RSS 2.0) for subscribers; the same GH
   Action appends `<item>` entries.
5. **Failure mode:** if `status.json` fails to load, the banner shows "Status feed
   unavailable" instead of a false green.

## Consequences

- **+** Zero recurring cost; one platform; automatable via GH Actions.
- **+** Auditable history in git — every status change is a commit.
- **−** No automated uptime probing out of the box; component status is set by the
  release/incident Action, not by synthetic monitors. If real synthetic monitoring
  is needed later, a probe Action can write `status.json` on a schedule.
- **−** Subdomain needs its own CF Pages project + DNS (one-time manual setup;
  tracked in manual-setup.md).

## Alternatives considered

- **Hosted Statuspage.io / Instatus** — clean UX but $20–100/mo and a second
  dashboard to operate. Rejected on cost + tooling-sprawl grounds.
- **Same-domain `/status.html`** — simpler, but a marketing-site deploy failure
  would also take down the status page. Rejected; subdomain isolation is the point.
