// Cloudflare Pages Function: POST /api/interest
//
// Collects Team-package interest (seats + willingness to pay) and emails a
// structured lead record to the operator's inbox via Resend. This is the
// "collectible document" — every submission lands as a formatted email you
// can forward to a spreadsheet or CRM.
//
// Env (set in CF Pages -> sapctl -> Settings -> env):
//   TURNSTILE_SECRET   server-side Turnstile secret key
//   RESEND_API_KEY     re_... API key from resend.com
//   CONTACT_TO         destination inbox (e.g. sales@sapctl.dev)
//   CONTACT_FROM       sender (must be a verified domain in Resend)

export async function onRequestPost({ request, env }) {
  let body;
  try {
    body = await request.json();
  } catch (_) {
    return text(400, 'invalid JSON');
  }

  // honeypot
  if (body.website) {
    return text(204, '');
  }

  const required = ['name', 'email', 'seats', 'budget'];
  for (const k of required) {
    if (!body[k] || String(body[k]).trim().length === 0) {
      return text(400, `missing field: ${k}`);
    }
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(body.email)) {
    return text(400, 'invalid email');
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
  const subject = `[sapctl/interest] ${body.name} (${body.company || 'no company'}) — ${body.seats} seats`;
  const text_ = [
    '=== sapctl Team-package interest ===',
    '',
    `Name:     ${body.name}`,
    `Email:    ${body.email}`,
    `Company:  ${body.company || '(not provided)'}`,
    `Seats:    ${body.seats}`,
    `Budget:   ${body.budget}`,
    `Notes:    ${body.notes || '(none)'}`,
    `IP:       ${request.headers.get('CF-Connecting-IP') || ''}`,
    `UA:       ${request.headers.get('User-Agent') || ''}`,
    '',
    '---',
    'Forward this to your lead tracker / spreadsheet.',
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
