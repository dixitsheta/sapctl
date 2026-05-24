/* sapctl — shared site chrome (theme toggle, nav scroll, nav + footer injection)
   Loaded by every sub-page. Keep dependency-free. */

(function () {
  'use strict';

  /* ---------------- theme ---------------- */
  // Pre-paint theme application
  try {
    var stored = localStorage.getItem('sapctl-theme');
    if (stored === 'light') document.documentElement.classList.remove('dark');
    else document.documentElement.classList.add('dark');
  } catch (e) {
    document.documentElement.classList.add('dark');
  }

  function bindThemeToggle(btn) {
    if (!btn) return;
    btn.addEventListener('click', function () {
      var isDark = document.documentElement.classList.toggle('dark');
      try { localStorage.setItem('sapctl-theme', isDark ? 'dark' : 'light'); } catch (e) {}
    });
  }

  /* ---------------- nav + footer markup ---------------- */
  function navHTML(activeHref) {
    var links = [
      { href: '/index.html#product',    label: 'Product' },
      { href: '/index.html#use-cases',  label: 'Use cases' },
      { href: '/index.html#arch',       label: 'Architecture' },
      { href: '/index.html#pricing',    label: 'Pricing' },
      { href: '/docs.html',             label: 'Docs' },
      { href: '/index.html#github',     label: 'GitHub' },
    ];
    return ''
      + '<div class="container-x site-nav-inner">'
      +   '<a class="brand" href="index.html">'
      +     '<span class="brand-mark">sapctl</span>'
      +     '<span class="brand-ver">v0.1.0</span>'
      +   '</a>'
      +   '<nav class="nav-links" aria-label="Primary">'
      +     links.map(function (l) {
              var current = (activeHref && l.href.indexOf(activeHref) === 0) ? ' aria-current="page"' : '';
              return '<a href="' + l.href + '"' + current + '>' + l.label + '</a>';
            }).join('')
      +   '</nav>'
      +   '<div class="nav-right">'
      +     '<button id="theme-toggle" class="icon-btn" type="button" aria-label="Toggle colour theme">'
      +       '<i class="ti ti-moon" style="font-size:18px" data-theme-icon="dark"></i>'
      +       '<i class="ti ti-sun"  style="font-size:18px; display:none" data-theme-icon="light"></i>'
      +     '</button>'
      +     '<a class="btn-primary" href="index.html#cta">Get started</a>'
      +   '</div>'
      + '</div>';
  }

  function syncThemeIcon() {
    var dark = document.documentElement.classList.contains('dark');
    document.querySelectorAll('[data-theme-icon]').forEach(function (el) {
      var which = el.getAttribute('data-theme-icon');
      el.style.display = ((dark && which === 'dark') || (!dark && which === 'light')) ? '' : 'none';
    });
  }

  function footerHTML() {
    return ''
      + '<div class="container-x">'
      +   '<div class="foot-grid">'
      +     col('Product', [
              ['/index.html#use-cases', 'Use cases'],
              ['/index.html#arch',      'Architecture'],
              ['/index.html#pricing',   'Pricing'],
              ['/roadmap.html',         'Roadmap'],
              ['/changelog.html',       'Changelog'],
            ])
      +     col('Developers', [
              ['/docs.html',            'Docs'],
              ['/index.html#github',    'GitHub'],
              ['/index.html#docs',      'Discord'],
              ['/docs.html#mcp',        'MCP catalog'],
              ['/docs.html#examples',   'Examples'],
            ])
      +     col('Compliance', [
              ['/trust.html',           'Trust portal'],
              ['/trust.html#sbom',      'SBOM'],
              ['/security.html',        'Security policy'],
              ['/security.html#cvd',    'CVD policy'],
              ['/trust.html#annex-iv',  'Annex IV'],
            ])
      +     col('Company', [
              ['/about.html',           'About'],
              ['/blog.html',            'Blog'],
              ['/partners.html',        'Partners'],
              ['/careers.html',         'Careers'],
              ['/contact.html',         'Contact'],
            ])
      +   '</div>'
      +   '<div class="foot-grid" style="margin-top:32px">'
      +     col('Legal', [
              ['/privacy.html',         'Privacy policy'],
              ['/terms.html',           'Terms of service'],
              ['/cookies.html',         'Cookie policy'],
              ['/dpa.html',             'DPA'],
              ['/sub-processors.html',  'Sub-processors'],
            ])
      +     col('Policies', [
              ['/aup.html',             'Acceptable use'],
              ['/security.html#cvd',    'Vulnerability disclosure'],
              ['/security.html',        'Security advisories'],
              ['/trust.html',           'Trust portal'],
              ['/sitemap.html',         'Sitemap'],
            ])
      +     col('Resources', [
              ['/blog.html',            'Blog'],
              ['/changelog.html',       'Changelog'],
              ['https://status.sapctl.dev/', 'Status'],
              ['/roadmap.html',         'Roadmap'],
              ['/sitemap.xml',          'XML sitemap'],
              ['/humans.txt',           'humans.txt'],
            ])
      +     col('Connect', [
              ['https://github.com/sapctl/sapctl',    'GitHub'],
              ['https://discord.gg/sapctl',           'Discord'],
              ['https://x.com/sapctl',                'X'],
              ['https://www.linkedin.com/company/sapctl', 'LinkedIn'],
              ['mailto:hello@sapctl.dev',             'hello@sapctl.dev'],
            ])
      +   '</div>'
      +   '<div class="foot-bottom">'
      +     '<div>© 2026 sapctl contributors. Apache 2.0. sapctl is an independent open-source project and is not affiliated with or endorsed by SAP SE.</div>'
      +     '<div class="foot-social">'
      +       '<a href="https://github.com/sapctl/sapctl" aria-label="GitHub"><i class="ti ti-brand-github"  style="font-size:18px"></i></a>'
      +       '<a href="https://x.com/sapctl" aria-label="X"><i class="ti ti-brand-x" style="font-size:18px"></i></a>'
      +       '<a href="https://www.linkedin.com/company/sapctl" aria-label="LinkedIn"><i class="ti ti-brand-linkedin" style="font-size:18px"></i></a>'
      +     '</div>'
      +   '</div>'
      + '</div>';
  }

  function col(label, items) {
    return ''
      + '<div class="foot-col">'
      +   '<div class="foot-col-label">' + label + '</div>'
      +   '<ul>'
      +     items.map(function (it) { return '<li><a href="' + it[0] + '">' + it[1] + '</a></li>'; }).join('')
      +   '</ul>'
      + '</div>';
  }

  /* ---------------- mount ---------------- */
  document.addEventListener('DOMContentLoaded', function () {
    var nav = document.querySelector('[data-site-nav]');
    if (nav) {
      nav.classList.add('site-nav');
      nav.setAttribute('role', 'banner');
      nav.innerHTML = navHTML(window.location.pathname);
      bindThemeToggle(nav.querySelector('#theme-toggle'));
      syncThemeIcon();
      var onScroll = function () {
        if (window.scrollY > 4) nav.classList.add('scrolled');
        else nav.classList.remove('scrolled');
      };
      window.addEventListener('scroll', onScroll, { passive: true });
      onScroll();
    }
    var foot = document.querySelector('[data-site-footer]');
    if (foot) {
      foot.classList.add('site-footer');
      foot.setAttribute('role', 'contentinfo');
      foot.innerHTML = footerHTML();
    }
  });
})();
