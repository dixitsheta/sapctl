# Manual configuration guide

One-time human-in-the-loop config to make the live site fully working.
None of this lives in Git. All secrets stay in Cloudflare dashboard envs.

---

## Section A -- Cloudflare Turnstile (contact-form bot protection)

### A1. Create Turnstile site

1. Open https://dash.cloudflare.com
2. Left nav -> **Turnstile**
3. **Add site**
   - Site name: `sapctl-contact`
   - Domain: `sapctl.dev`
   - Widget mode: **Managed** (default)
   - Click **Create**
4. Two keys appear:
   - **Site Key** (public, copy)
   - **Secret Key** (server, copy)

### A2. Wire site key into HTML

Open `apps/web-static/contact.html`. Find:

```html
<div class="cf-turnstile" data-sitekey="1x00000000000000000000AA" data-theme="dark"></div>
```

Replace `1x00000000000000000000AA` with the **Site Key** from A1.

Commit + push:

```bash
git add apps/web-static/contact.html
git commit -m "fix(web): real Turnstile site key"
git push
```

Cloudflare Pages redeploys in ~30s.

### A3. Wire secret key into CF Pages env

1. https://dash.cloudflare.com -> **Workers & Pages** -> tab **Pages** -> project `sapctl`
2. **Settings** -> **Variables and Secrets**
3. **Production** environment -> **Add**
   - Type: **Secret**
   - Name: `TURNSTILE_SECRET`
   - Value: paste **Secret Key** from A1
   - Save
4. Repeat for **Preview** environment if you use preview deploys

---

## Section B -- Resend (email delivery)

### B1. Create Resend account

1. https://resend.com -> Sign up (free tier: 100 emails/day, 3000/month)
2. Verify email

### B2. Verify your sending domain

1. Resend dashboard -> **Domains** -> **Add Domain**
2. Enter `sapctl.dev` -> **Add**
3. Resend shows DNS records (SPF, DKIM, MX for replies). Copy them.
4. Go to https://dash.cloudflare.com -> **sapctl.dev** -> **DNS** -> **Records**
5. **Add record** for each one Resend gave you. Usually:
   - TXT record: `_resend` -> `re-...` (DKIM)
   - TXT record: `@` -> `v=spf1 include:amazonses.com ~all`
   - MX record (optional, for replies)
6. Back in Resend -> click **Verify DNS Records**
7. Wait 5-15 min for verification

### B3. Create API key

1. Resend dashboard -> **API Keys** -> **Create API Key**
2. Name: `sapctl-pages-contact`
3. Permission: **Sending access** -> **Full access**
4. Domain: `sapctl.dev`
5. Click **Add** -> **copy the `re_...` key NOW**, you can't see it again

### B4. Wire into CF Pages env

CF dashboard -> Pages -> `sapctl` -> Settings -> Variables and Secrets -> **Production**:

| Name | Type | Value |
|---|---|---|
| `RESEND_API_KEY` | Secret | `re_...` from B3 |
| `CONTACT_TO` | Plain text | `your-email@example.com` (or `sales@sapctl.dev` once that inbox exists) |
| `CONTACT_FROM` | Plain text | `noreply@sapctl.dev` |

---

## Section C -- GitHub PAT for /api/releases (optional)

Boosts rate limit from 60/hr to 5000/hr. Only matters if site sees real traffic.

### C1. Create PAT

1. https://github.com/settings/tokens?type=beta -> **Generate new token**
2. Token name: `sapctl-pages-releases`
3. Expiration: 1 year (calendar reminder day 350)
4. Repository access: **Only select repositories** -> `dixitsheta/sapctl`
5. Repository permissions:
   - **Contents**: Read-only
   - **Metadata**: Read-only (auto)
6. Generate -> **copy the `github_pat_...` value NOW**

### C2. Wire into CF Pages env

CF dashboard -> Pages -> `sapctl` -> Settings -> Variables and Secrets:

| Name | Type | Value |
|---|---|---|
| `GITHUB_TOKEN` | Secret | `github_pat_...` from C1 |

---

## Section D -- SAP API Hub key refresh (when Hermes/sapctl returns 401)

Sandbox API key rotates on profile change OR after inactivity.

### D1. Get fresh key

1. https://api.sap.com -> top-right avatar (yellow `DS`) -> **Settings**
2. Scroll to **API Keys** section
3. Click **Show** or **Regenerate**
4. Copy the 32-char hex key

### D2. Save locally

```bash
KEY='PASTE-32-CHAR-KEY'

/usr/local/bin/sapctl \
  auth login --flow apikey --label sandbox --api-key "$KEY"

echo "$KEY" > ~/.config/sapctl/sap-api-hub-key
chmod 600 ~/.config/sapctl/sap-api-hub-key
```

### D3. Verify

```bash
/usr/local/bin/sapctl s4 catalog discover --cred sandbox --top 3
```

Expected: 3 OData service IDs printed.

### D4. Restart Hermes

Close + reopen Hermes session so it respawns sapctl as subprocess.

---

## Section E -- SAP BTP XSUAA binding refresh (if BTP commands return 401)

### E1. Open service-key JSON

In BTP cockpit -> subaccount -> Instances and Subscriptions -> find your XSUAA
instance -> **Service Keys** -> click key -> view JSON. Or:

```bash
cat ~/.config/sapctl/xsuaa-binding.json
```

### E2. Re-save credential in sapctl

```bash
CID=$(jq -r .clientid     ~/.config/sapctl/xsuaa-binding.json)
CSECRET=$(jq -r .clientsecret ~/.config/sapctl/xsuaa-binding.json)
TURL=$(jq -r '.url + "/oauth/token"' ~/.config/sapctl/xsuaa-binding.json)

/usr/local/bin/sapctl \
  auth login --flow xsuaa --label btp-trial \
  --client-id "$CID" --client-secret "$CSECRET" --token-url "$TURL"
```

### E3. Verify

```bash
/usr/local/bin/sapctl btp subaccount list --cred btp-trial
```

If 401 even with fresh creds: trial subaccount expired (90-day max). Recreate
in cockpit, regenerate service key, repeat E1-E3.

---

## Section F -- sapctl.dev DNS polish (optional, post-launch SEO)

### F1. Confirm DNS resolves

```bash
dig +short sapctl.dev
curl -I https://sapctl.dev
```

Both should return Cloudflare IPs / `HTTP/2 200`.

### F2. Submit sitemap to Google

1. https://search.google.com/search-console
2. **Add property** -> `https://sapctl.dev`
3. Verify via DNS TXT record (CF auto-populates if logged in)
4. Once verified -> **Sitemaps** -> add `https://sapctl.dev/sitemap.xml`

### F3. Submit sitemap to Bing

1. https://www.bing.com/webmasters
2. **Import from Google Search Console** (fastest path)

### F4. Run Lighthouse

Chrome -> DevTools -> Lighthouse tab -> Mobile + Desktop -> run.
Target >= 95 across Performance, Accessibility, Best Practices, SEO.

### F5. Verify JSON-LD

https://search.google.com/test/rich-results -> enter `https://sapctl.dev`.
Expected: Organization + SoftwareApplication + FAQPage schemas valid.

---

## Section G -- One-time host setup for sapctl on macOS

### G1. Symlink binary into PATH

```bash
sudo ln -sf /usr/local/bin/sapctl /usr/local/bin/sapctl
sapctl version
```

### G2. macOS Gatekeeper notarization (only when distributing built binary)

Self-built sapctl runs locally without signing. For distributing to others
without `cosign verify`, optionally:

```bash
codesign --sign - --force apps/cli/bin/sapctl
xattr -cr apps/cli/bin/sapctl
```

Full notarization (Apple Developer account, $99/yr) is Y2 work; not needed
for Phase 1.

---

## Section H -- GitHub repo flip to public + branch protection

### H1. Visibility

1. https://github.com/dixitsheta/sapctl -> **Settings** -> scroll to **Danger Zone**
2. **Change repository visibility** -> **Make public** -> type repo name to confirm
3. Verify no secrets leaked: `git log -p --all | rg -i 'TURNSTILE_SECRET|RESEND_API_KEY|GITHUB_TOKEN|SAPCTL_S4_COMM_PASSWORD'`
   (zero hits expected; `.env`/binding JSON are gitignored)

### H2. Enable Discussions

Settings -> **Features** section -> tick **Discussions** -> **Set up discussions**.

### H3. Branch protection on `main`

Settings -> **Branches** -> **Add rule** for `main`:
- Require pull request before merging
- Require status checks: pick `build (ubuntu-latest / amd64)`, `test (ubuntu / amd64)`
- Require linear history
- Do not allow bypassing the above settings

### H4. Default branch + repo metadata

- About (top-right) -> Edit -> set description: "The unified open-source CLI for SAP. CRA-ready, MCP-emitting, audit-signed."
- Website: `https://sapctl.dev`
- Topics: `sap`, `cli`, `mcp`, `agent`, `cra`, `dora`, `compliance`, `s4hana`, `btp`, `datasphere`, `air-gap`, `cosign`, `sbom`

---

## Section I -- docs.sapctl.dev Cloudflare Pages project

Separate Pages project from the marketing `sapctl` project; same repo,
different build root + subdomain.

### I1. Connect

CF dashboard -> Workers & Pages -> Pages tab -> **Create** -> **Connect to Git** ->
select `dixitsheta/sapctl` again.

### I2. Build

| Field | Value |
|---|---|
| Project name | `sapctl-docs` |
| Production branch | `main` |
| Framework preset | **Astro** |
| Build command | `cd apps/docs && npm install && npm run build` |
| Build output directory | `apps/docs/dist` |
| Root directory | *(empty -- leave repo root)* |

### I3. Custom domain

Project -> **Custom domains** -> **Set up a custom domain** -> `docs.sapctl.dev`.
CF auto-creates CNAME (apex flattening if needed). SSL via Universal SSL.

### I4. Env vars (none required)

Astro Starlight build does not need API keys.

### I5. Verify

```bash
curl -I https://docs.sapctl.dev
```

Expect `HTTP/2 200`. Test deep links: `/install/`, `/recipes/sox-journal/`,
`/reference/cli/`.

---

## Section J -- Google Search Console verification

`GSC/google41f66520f7aade66.html` is the Google site-verification token.
It must be served from `https://sapctl.dev/google41f66520f7aade66.html`.

### J1. Move into web root

The file lives under `apps/web-static/` so Cloudflare Pages serves it at
the apex.

### J2. Verify in Search Console

1. https://search.google.com/search-console -> **Add property** -> `https://sapctl.dev`
2. Choose **HTML file** verification method
3. Search Console confirms the filename matches what was downloaded
4. Click **Verify**
5. Once verified -> **Sitemaps** -> add `https://sapctl.dev/sitemap.xml`

If verification fails: confirm `curl https://sapctl.dev/google41f66520f7aade66.html`
returns the single-line `google-site-verification: ...` content.

---

## Section K. Central `~/.config/sapctl/.env` migration

Now that sapctl auto-loads `~/.config/sapctl/.env` on every invocation, consolidate all credentials here instead of exporting them from `~/.zshrc` / `~/.bashrc`.

### K1. Create the file from the template

```bash
mkdir -p ~/.config/sapctl
cp .env.example ~/.config/sapctl/.env
chmod 600 ~/.config/sapctl/.env
```

### K2. Fill in your values

Open `~/.config/sapctl/.env` and uncomment + fill the relevant lines:

```bash
# SAP API Hub sandbox key (from Section D)
SAPCTL_SANDBOX_API_KEY=your-32-char-hex-key

# S/4HANA Cloud Public Edition trial
SAPCTL_S4_HOST=myXXXXXX-api.s4hana.cloud.sap
SAPCTL_S4_UI_HOST=myXXXXXX.s4hana.cloud.sap
SAPCTL_S4_TENANT_ID=myXXXXXX
SAPCTL_S4_COMM_USER=your-comm-user
SAPCTL_S4_COMM_PASSWORD=your-comm-password

# BTP (xsuaa flow -- fill after Section E)
SAPCTL_BTP_REGION=eu10
SAPCTL_BTP_SUBDOMAIN=your-subdomain
```

The CF Pages secrets (`TURNSTILE_SECRET`, `RESEND_API_KEY`, etc.) listed at the bottom of `.env.example` are reference-only -- set those in the Cloudflare dashboard per Sections A-C, never here.

### K3. Remove redundant shell exports

If you previously exported these in `~/.zshrc` or `~/.bashrc`, remove them. The file takes precedence only when the shell var is absent, so duplicate exports are harmless but noisy.

```bash
# lines like these can go:
# export SAPCTL_SANDBOX_API_KEY=...
# export SAPCTL_S4_HOST=...
```

### K4. Confirm gitignore covers it

```bash
grep '\.env' .gitignore
```

Expected: at minimum `.env` and `.env.*` are listed. The repo-root `.env.example` is committed; `~/.config/sapctl/.env` lives only on your machine and is never tracked.

### K5. Verify auto-load

```bash
SAPCTL_DEBUG_ENV=1 sapctl version
```

Expected: one-line summary on stderr listing which vars were sourced from the file vs the shell, then the version string.

---

## Configuration ledger (track when each is done)

Tick when actually wired in production:

- [ ] A. Turnstile site key in `contact.html` + secret in CF Pages env
- [ ] B. Resend domain verified + API key in CF Pages env + CONTACT_TO/FROM
- [ ] C. GitHub PAT in CF Pages env (optional)
- [ ] D. Fresh SAP API Hub key in `sapctl auth login`
- [ ] E. Fresh BTP XSUAA binding in `sapctl auth login`
- [ ] F. sapctl.dev indexed in Google + Bing, Lighthouse >= 95
- [ ] G. sapctl symlink in PATH
- [ ] H. GitHub repo public + Discussions on + branch protection on main
- [ ] I. docs.sapctl.dev Pages project live
- [ ] J. Google Search Console verified + sitemap submitted
- [ ] K. Central `~/.config/sapctl/.env` created, chmod 600, values filled

Untouched checkboxes after a month -> revisit.
