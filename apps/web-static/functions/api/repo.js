// Cloudflare Pages Function: GET /api/repo
//
// Proxies https://api.github.com/repos/dixitsheta/sapctl through the CF edge
// cache so the landing page can render live star count without exposing the
// user's IP to GitHub on every page load.
//
// Cache TTL 5 min (300s). GitHub API rate-limit for unauthenticated callers
// is 60/hr per IP; CF coalesces across visitors so we never hit it.
//
// Optional env:
//   GITHUB_TOKEN  bumps rate limit to 5000/hr (recommended once repo is public)

const REPO = 'dixitsheta/sapctl';
const TTL_SECONDS = 300;

export async function onRequestGet({ request, env, waitUntil }) {
  const cacheKey = new Request(new URL(request.url).toString(), request);
  const cache = caches.default;

  const cached = await cache.match(cacheKey);
  if (cached) return cached;

  const headers = {
    'User-Agent': 'sapctl.dev (+https://sapctl.dev)',
    Accept: 'application/vnd.github+json',
    'X-GitHub-Api-Version': '2022-11-28',
  };
  if (env && env.GITHUB_TOKEN) {
    headers.Authorization = `Bearer ${env.GITHUB_TOKEN}`;
  }

  const upstream = await fetch(
    `https://api.github.com/repos/${REPO}`,
    { headers },
  );

  if (!upstream.ok) {
    const detail = await upstream.text();
    return new Response(detail.slice(0, 500), {
      status: upstream.status,
      headers: { 'Content-Type': 'text/plain' },
    });
  }

  const data = await upstream.json();
  const slim = {
    full_name: data.full_name,
    html_url: data.html_url,
    stargazers_count: data.stargazers_count || 0,
    forks_count: data.forks_count || 0,
    open_issues_count: data.open_issues_count || 0,
    description: data.description || '',
    default_branch: data.default_branch || 'main',
  };

  const res = new Response(JSON.stringify(slim), {
    status: 200,
    headers: {
      'Content-Type': 'application/json; charset=utf-8',
      'Cache-Control': `public, max-age=${TTL_SECONDS}, s-maxage=${TTL_SECONDS}`,
      'Access-Control-Allow-Origin': '*',
    },
  });

  if (typeof waitUntil === 'function') {
    waitUntil(cache.put(cacheKey, res.clone()));
  } else {
    await cache.put(cacheKey, res.clone());
  }
  return res;
}
