import { error } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

// Never deployed -- this route only exists for local `vite dev`. Left with
// SSR enabled (the default) so this guard runs server-side on the initial
// request too, producing a real 404 status instead of a 200 shell that only
// redirects after client-side hydration.
export const load: LayoutLoad = () => {
	if (!import.meta.env.DEV) error(404);
};
