# apps/web-static — sapctl.dev v0

Static-site copy of `ref/sapctl/`, the production-grade design source. This is the **v0** marketing site, deployed as-is to Cloudflare Pages at https://sapctl.dev.

v1 will port this to Next.js 15 at `apps/web/` per `/plans/03-ui-integration.md` Phase 4.

## Deploy

Cloudflare Pages settings:
- Production branch: `main`
- Build command: (none)
- Build output: `apps/web-static`
- Environment variables: none

## Verify locally

```bash
cd apps/web-static
python3 -m http.server 8080
# open http://localhost:8080
```

## Pending v0 polish (master plan Phase 0 tasks 0.6-0.13)

- [ ] Swap Tailwind CDN runtime config for compiled `tailwind.min.css`
- [ ] Install Plausible or Cloudflare Web Analytics
- [ ] Verify JSON-LD via Google Rich Results Test
- [ ] Submit `sitemap.xml` to Google Search Console + Bing
- [ ] Update `robots.txt` Sitemap entry to `https://sapctl.dev/sitemap.xml`
- [ ] Lighthouse audit: target >= 95 across Perf / A11y / Best Practices / SEO

## License & disclaimer

Independent open-source project. Not affiliated with SAP SE.
