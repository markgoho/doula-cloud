/**
 * A Practice's own timezone (#1166): the load/save half of the settings
 * screen, decoupled from SvelteKit and the DOM so it can be unit-tested
 * directly -- mirrors rates.ts.
 *
 * There is no client-side check of the zone name here beyond "she chose
 * something". The IANA database is what decides whether a name is a zone,
 * and it lives in the Go BFF (`ianazone`); duplicating a list here would
 * be a second answer that can drift from the first.
 */

import type { Fetcher } from './fetcher.js';

import { refusalError } from './formErrors.js';

export interface PracticeTimezone {
	timezone: string;
}

function timezonePath(practiceId: string): string {
	return `/api/practices/${practiceId}/timezone`;
}

/** Reads the zone the Practice keeps its calendar days in. Throws a
 * RefusalError on a non-2xx response, so the screen can read the field
 * the BFF named rather than only its caller-facing summary. */
export async function loadPracticeTimezone(
	fetcher: Fetcher,
	practiceId: string
): Promise<PracticeTimezone> {
	const response = await fetcher(timezonePath(practiceId));
	if (!response.ok) {
		throw await refusalError(response);
	}
	return response.json();
}

/** States the Practice's zone. Only an Owner's or Admin's session reaches
 * this without a 403 -- practicetimezone.PutHandler is the authority, and
 * this module makes no role check of its own, the same split rates.ts
 * draws. */
export async function savePracticeTimezone(
	fetcher: Fetcher,
	practiceId: string,
	timezone: string
): Promise<PracticeTimezone> {
	const response = await fetcher(timezonePath(practiceId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ timezone })
	});
	if (!response.ok) {
		// A RefusalError rather than a plain Error: the BFF keys its
		// refusal to the `timezone` field (docs/api-design.md section 7
		// rule 4), and a plain Error would drop that sentence -- the one
		// written for the person -- in favor of the summary written for
		// the caller.
		throw await refusalError(response);
	}
	return response.json();
}
