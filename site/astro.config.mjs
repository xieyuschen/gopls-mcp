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
			description: 'Semantic Go code understanding for AI assistants',
			pagination: false,
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
				{
					label: 'Benchmarks',
					autogenerate: { directory: 'benchmarks' },
				},
			],
		}),
	],
});