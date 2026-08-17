import { error } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

// Never deployed -- this route only exists for local `vite dev`. The app
// is a static SPA (root +layout.ts sets ssr = false) with no server to
// produce a real 404 from, so this guard only runs client-side: a direct
// production request would 200 the SPA shell and then hit this error()
// after hydration, not before.
export const load: LayoutLoad = () => {
	if (!import.meta.env.DEV) error(404);
};
