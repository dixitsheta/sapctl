// Cloudflare Pages Function: POST /api/contact
//
// Verifies a Cloudflare Turnstile token, runs basic validation + honeypot
// check, then forwards the submission via Resend (preferred) or logs the
// payload so the operator can wire a mail provider later.
//
// Configure in Cloudflare dashboard -> Pages -> sapctl -> Settings ->
// Environment variables:
//
//   TURNSTILE_SECRET   server-side Turnstile secret key (required for prod)
//   RESEND_API_KEY     re_... API key from resend.com (optional; if absent
//                      the function logs the message body to CF logs)
//   CONTACT_TO         destination address (default: sales@sapctl.dev)
//   CONTACT_FROM       sender for Resend (must be a verified domain)
//
// Security: no secrets in code; all from env. Honeypot + Turnstile + JSON
// validation per CLAUDE.md security checklist.

export async function onRequestPost({ request, env }) {
  let body;
  try {
    body = await request.json();
  } catch (_) {
    return text(400, 'invalid JSON');
  }

  if (body.website) {
    return text(204, '');
  }

  const required = ['name', 'email', 'topic', 'message'];
  for (const k of required) {
    if (!body[k] || String(body[k]).trim().length === 0) {
      return text(400, `missing field: ${k}`);
    }
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(body.email)) {
    return text(400, 'invalid email');
  }
  if (String(body.message).length > 4000) {
    return text(400, 'message too long');
  }

  if (env.TURNSTILE_SECRET) {
    const token = body['cf-turnstile-response'];
    if (!token) return text(400, 'missing turnstile token');
    const verify = await fetch(
      'https://challenges.cloudflare.com/turnstile/v0/siteverify',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({
          secret: env.TURNSTILE_SECRET,
          response: token,
          remoteip: request.headers.get('CF-Connecting-IP') || '',
        }),
      },
    );
    const vr = await verify.json();
    if (!vr.success) {
      return text(403, 'turnstile verification failed');
    }
  }

  const to = env.CONTACT_TO || 'sales@sapctl.dev';
  const from = env.CONTACT_FROM || 'noreply@sapctl.dev';
  const subject = `[sapctl/${body.topic}] ${truncate(body.name, 40)}: ${truncate(body.message, 60)}`;
  const text_ = [
    `Name:    ${body.name}`,
    `Email:   ${body.email}`,
    `Company: ${body.company || '(not provided)'}`,
    `Topic:   ${body.topic}`,
    `IP:      ${request.headers.get('CF-Connecting-IP') || ''}`,
    `UA:      ${request.headers.get('User-Agent') || ''}`,
    '',
    body.message,
  ].join('\n');

  if (env.RESEND_API_KEY) {
    const r = await fetch('https://api.resend.com/emails', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${env.RESEND_API_KEY}`,
      },
      body: JSON.stringify({
        from,
        to: [to],
        reply_to: body.email,
        subject,
        text: text_,
      }),
    });
    if (!r.ok) {
      const detail = await r.text();
      return text(502, `resend error: ${detail.slice(0, 200)}`);
    }
    return text(204, '');
  }

  console.log(JSON.stringify({ subject, to, body: text_ }));
  return text(204, '');
}

export async function onRequestOptions() {
  return new Response(null, {
    status: 204,
    headers: {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'POST, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type',
      'Access-Control-Max-Age': '86400',
    },
  });
}

function text(status, body) {
  return new Response(body, {
    status,
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
      'Cache-Control': 'no-store',
    },
  });
}

function truncate(s, n) {
  s = String(s);
  return s.length <= n ? s : s.slice(0, n - 1) + '...';
}
