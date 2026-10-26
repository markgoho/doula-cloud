/*
 * Any address under a Practice that no route answers (#2048, which
 * retired `clients/[clientId]/edit`): a 404 inside the Staff shell.
 *
 * With no route matched, SvelteKit renders the root `+error.svelte`
 * outside every layout, so a signed-in Staff member following an old
 * link lost her navigation and was offered "Sign in". Matching the rest
 * of the path here instead puts the refusal under
 * `practices/+error.svelte`, inside `practices/+layout.svelte`, with
 * the way out to her Practice overview. A more specific route always
 * wins over a rest parameter, so this answers only what nothing else
 * does.
 */
import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = () => {
	error(404, 'Not found');
};
