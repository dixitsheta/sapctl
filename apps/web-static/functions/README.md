# Cloudflare Pages Functions

This directory holds JavaScript Pages Functions auto-deployed by Cloudflare
when the static site is published. No build step.

## Routes

| Route | File | Purpose |
|---|---|---|
| `POST /api/contact` | `api/contact.js` | Contact-form submit handler with Turnstile + email forward |
| `GET  /api/releases` | `api/releases.js` | Live proxy + edge-cached mirror of GitHub Releases for changelog + trust portal |

## Configure once after first deploy

Cloudflare dashboard -> Workers & Pages -> Pages project `sapctl` ->
Settings -> Variables and Secrets -> **Production** (and **Preview**, separately):

### Required for production contact form

| Name | Type | Value | Where to get it |
|---|---|---|---|
| `TURNSTILE_SECRET` | Secret | server-side Turnstile secret key | https://dash.cloudflare.com -> Turnstile -> create site -> bind to `sapctl.dev` -> copy server key |

Also update the **site key** in `contact.html` (look for `data-sitekey="1x000..."`)
to match the public site key from the same Turnstile widget.

### Required for email delivery

| Name | Type | Value | Where to get it |
|---|---|---|---|
| `RESEND_API_KEY` | Secret | `re_...` API key | https://resend.com -> API Keys -> create. Free tier = 100 emails/day. |
| `CONTACT_TO`     | Plain text | `sales@sapctl.dev` (or whatever destination inbox) | n/a |
| `CONTACT_FROM`   | Plain text | `noreply@sapctl.dev` | must be a verified domain in Resend |

If `RESEND_API_KEY` is unset the function still returns 204 (so the form
shows "Sent") but the message is only logged to CF Workers logs. Set the key
before going live with public traffic.

### Optional

| Name | Type | Value | Effect |
|---|---|---|---|
| `GITHUB_TOKEN` | Secret | classic PAT with `public_repo` scope | bumps GitHub API rate limit on `/api/releases` from 60/hr to 5000/hr |

## Local dev

CF Pages Functions can run locally with wrangler:

```bash
npm install -g wrangler
cd apps/web-static
wrangler pages dev . --port 8788
```

Then POST to `http://localhost:8788/api/contact` etc.

## Threat model

- Contact form: Turnstile + honeypot + JSON schema validation + 4 KB body cap.
- Releases proxy: read-only, public data, edge-cached.
- No persistence layer; nothing to drain on incident.
- Secrets only via CF env. `git grep TURNSTILE_SECRET` returns zero.
