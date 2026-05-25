import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import sitemap from '@astrojs/sitemap';
import { pluginCollapsibleSections } from '@expressive-code/plugin-collapsible-sections';

export default defineConfig({
	site: 'https://gopls-mcp.org',
	integrations: [
		sitemap(),
		starlight({
			title: 'gopls-mcp',
			description: 'Semantic Go code understanding for AI Code Agents',
			favicon: '/logo.png',
			pagination: false,
			logo: {
				src: './public/logo.png',
				alt: 'gopls-mcp',
			},
			head: [
				{
					tag: 'meta',
					attributes: {
						property: 'og:image',
						content: '/logo.png',
					},
				},
				{
					tag: 'meta',
					attributes: {
						name: 'twitter:image',
						content: '/logo.png',
					},
				},
				{
					tag: 'meta',
					attributes: {
						name: 'twitter:card',
						content: 'summary_large_image',
					},
				},
			],
			social: [
				{ label: 'GitHub', href: 'https://github.com/xieyuschen/gopls-mcp', icon: 'github' }
			],

            expressiveCode: {
                themes: ['github-dark', 'github-light'],
                 plugins: [
                pluginCollapsibleSections(), 
            ],
            },
			sidebar: [
				{
					label: 'Getting Started',
					autogenerate: { directory: 'quick-start' },
				},
				{
					label: 'Case Studies',
					autogenerate: { directory: 'case-studies' },
				},
				{
					label: 'Configuration',
					items: [
						{ label: 'Configuration', link: 'config' },
					],
				},
				{
					label: 'Reference',
					autogenerate: { directory: 'reference' },
				},
			],
		}),
	],
});
