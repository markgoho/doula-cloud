/*
 * #501 (ADR-0017): decides, before the search screen ever mounts, whether
 * this Staff member gets it at all. A contractor Doula never originates a
 * Client at a Practice she contracts for -- client/search.go's
 * SearchHandler refuses her with a 403 naming the reason -- so this load
 * intercepts ahead of that refusal rather than letting the search form
 * render and fail on submit.
 *
 * Through `load`, not the page's usual onMount fetch, so the branch is
 * settled before first paint (billing/+page.ts's same rationale for a
 * role-gated read). Unlike billing, a session-read failure here falls
 * back to `isContractor: false` rather than throwing: this check is
 * UX-only -- SearchHandler's own refusal is the real boundary -- so the
 * worse case of a hiccup here is the unchanged pre-#501 behaviour (the
 * search form, then a readable 403 if she truly is a contractor), never a
 * page that fails to load for anyone else.
 */
import { apiFetch } from '#lib/api.js';
import type { PageLoad } from './$types';

export interface ClientSearchGate {
	/** True only for a contractor Doula with neither the owner nor the
	 * admin role -- ADR-0017's "solo Practice" owner-contractor keeps the
	 * search screen, the same carve-out SearchHandler applies. */
	isContractor: boolean;
}

export const load: PageLoad = async ({ params }): Promise<ClientSearchGate> => {
	try {
		const response = await apiFetch(`/api/practices/${params.practiceId}/session`);
		if (!response.ok) return { isContractor: false };

		const body: { roles: string[]; isContractor: boolean } = await response.json();
		return { isContractor: body.isContractor && !body.roles.includes('owner') && !body.roles.includes('admin') };
	} catch {
		// A network failure or an unparseable body is no different from a
		// non-ok response, above -- fall back rather than fail the page.
		return { isContractor: false };
	}
};
