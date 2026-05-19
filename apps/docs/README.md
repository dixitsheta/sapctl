# apps/docs -- docs.sapctl.dev

Astro Starlight project hosting the public documentation at
`https://docs.sapctl.dev`.

## Dev

```bash
cd apps/docs
npm install
npm run dev    # http://localhost:4321
```

Hot-reload on every `.md` / `.mdx` change.

## Build + preview

```bash
npm run build
npm run preview
```

Output goes to `dist/`. Deploy `dist/` to Cloudflare Pages on subdomain
`docs.sapctl.dev` (separate Pages project from sapctl.dev marketing site).

## Layout

```
src/content/docs/
  index.mdx            splash
  install.md
  getting-started.md
  auth.md
  recipes/             10 golden recipes (autogen sidebar)
  reference/           command reference (autogen sidebar)
src/styles/theme.css   marketing-site token alignment
src/assets/logo.svg    sapctl mark
astro.config.mjs       Starlight config (sidebar, search, edit links)
content.config.ts      Starlight content collections
```

## Cloudflare Pages config

| Field | Value |
|---|---|
| Project name | `sapctl-docs` |
| Production branch | `main` |
| Framework preset | **Astro** |
| Build command | `cd apps/docs && npm install && npm run build` |
| Build output directory | `apps/docs/dist` |
| Custom domain | `docs.sapctl.dev` |
