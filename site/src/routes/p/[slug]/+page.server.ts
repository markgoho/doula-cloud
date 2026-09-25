import { error } from '@sveltejs/kit';
import { practicePages } from '#lib/server/practicePages.js';
import type { EntryGenerator, PageServerLoad } from './$types';

// One prerendered page per published Practice, and no others. There is
// deliberately no /p/ index page: nobody decided to publish a directory
// of every Practice on the platform, and that is a product decision
// rather than a side effect of how the pages are stored.
export const entries: EntryGenerator = () => practicePages.map(({ slug }) => ({ slug }));

export const load: PageServerLoad = ({ params }) => {
	const page = practicePages.find(({ slug }) => slug === params.slug);
	// The prerenderer only asks for the slugs entries() named, so this is
	// reached only by a dev server asked for a page that was never synced.
	if (!page) error(404);
	return { page };
};
