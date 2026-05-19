// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://docs.sapctl.dev',
  integrations: [
    starlight({
      title: 'sapctl docs',
      description:
        'The unified open-source CLI for SAP. CRA-ready, MCP-emitting, audit-signed.',
      logo: { src: './src/assets/logo.svg' },
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/dixitsheta/sapctl' },
        { icon: 'discord', label: 'Discord', href: 'https://discord.gg/sapctl' },
      ],
      customCss: ['./src/styles/theme.css'],
      sidebar: [
        {
          label: 'Getting started',
          items: [
            { label: 'Introduction', link: '/' },
            { label: 'Install', link: '/install' },
            { label: 'Quickstart', link: '/getting-started' },
            { label: 'Authentication', link: '/auth' },
          ],
        },
        {
          label: 'Recipes',
          collapsed: false,
          autogenerate: { directory: 'recipes' },
        },
        {
          label: 'Command reference',
          collapsed: true,
          autogenerate: { directory: 'reference' },
        },
        {
          label: 'Trust & compliance',
          items: [
            { label: 'Trust portal', link: 'https://sapctl.dev/trust' },
            { label: 'Security policy', link: 'https://sapctl.dev/security' },
          ],
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/dixitsheta/sapctl/edit/main/apps/docs/',
      },
      lastUpdated: true,
      pagination: true,
    }),
  ],
});
