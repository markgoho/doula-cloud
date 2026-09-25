import { practicePages } from '#lib/server/practicePages.js';
import { renderSitemap } from '#lib/server/sitemap.js';
import type { RequestHandler } from './$types';

// Set here as well as in +layout.ts: a +server.ts takes no page options
// from a layout.
// eslint-disable-next-line unicorn/consistent-boolean-name -- `prerender` is SvelteKit's mandated export name for this config
export const prerender = true;

export const GET: RequestHandler = () =>
	new Response(renderSitemap(practicePages), {
		headers: { 'content-type': 'application/xml' }
	});
