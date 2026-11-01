import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { readSession, type Operator } from '#lib/session.js';
import type { LayoutLoad } from './$types';

// eslint-disable-next-line unicorn/consistent-boolean-name -- `ssr` is SvelteKit's mandated export name
export const ssr = false;

const SIGNED_OUT_PAGES = new Set(['/sign-in', '/signed-out', '/enroll']);

export const load: LayoutLoad = async ({ url }): Promise<{ operator: Operator | undefined }> => {
	if (SIGNED_OUT_PAGES.has(url.pathname)) return { operator: undefined };
	const operator = await readSession();
	if (!operator) redirect(307, resolve('/sign-in'));
	return { operator };
};
