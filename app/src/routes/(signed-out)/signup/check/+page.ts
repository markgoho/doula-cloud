import { error } from '@sveltejs/kit';
import { apiFetch } from '#lib/api.js';
import { comingVersionsPath, type ComingVersion } from '#lib/legal.js';
import type { PageLoad } from './$types';

export interface SignupData {
	/**
	Each version of the Terms of Service or the Privacy Policy that has not started, oldest first within each document. Empty outside a notice period.
	*/
	coming: ComingVersion[];
}

const unread = 'the coming versions of the Terms could not be read';

/**
 * #1824 (ADR-0053): a Practice that signs up during a notice period
 * agrees to the coming versions too, and the form names each one. They
 * come from the BFF, which writes the same versions into the
 * `terms_accepted` row, read at its own instant -- never from a copy in
 * the app, whose build would go stale and whose reader's clock can be
 * wrong.
 *
 * A failed read refuses the page rather than drawing the form without
 * the line: the form must not leave out a version the press records, and
 * a BFF that cannot answer this read cannot take the signup either.
 * `apiFetch`, not `apiFetchWithSession`: there is no session here.
 */
export const load: PageLoad = async (): Promise<SignupData> => {
	let response: Response;
	try {
		response = await apiFetch(comingVersionsPath);
	} catch {
		error(503, unread);
	}
	if (!response.ok) error(503, unread);
	const { coming } = (await response.json()) as SignupData;
	return { coming };
};
