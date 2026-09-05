/**
 * A Practice's Contract Template: the legal prose, carrying merge-field
 * placeholders (client name, price, engagement dates, scope of service),
 * that a later ticket fills in per Engagement. This module holds the
 * load/save orchestration and validation for the practice-settings
 * screen, decoupled from SvelteKit and the DOM so it can be unit-tested
 * directly -- mirrors planTemplate.ts.
 */

import { apiErrorMessage } from './apiErrorMessage.js';

/** The merge-field placeholders a Contract Template's prose may contain --
 * exported so the settings screen can show them as a reference for the
 * Practice Owner while editing. Values match the Go BFF's default seeded
 * template (api/internal/staffauth/signup.go) exactly; there is no
 * separate structured list on the server, only prose text. */
export const MERGE_FIELDS = [
	{ token: '{{practice_name}}', label: 'Practice name' },
	{ token: '{{client_name}}', label: 'Client name' },
	{ token: '{{price}}', label: 'Price' },
	{ token: '{{engagement_start_date}}', label: 'Engagement start date' },
	{ token: '{{engagement_end_date}}', label: 'Engagement end date' },
	{ token: '{{scope_of_service}}', label: 'Scope of service' }
] as const;

export interface ContractTemplate {
	prose: string;
}

/** A minimal fetch-shaped function, injected rather than imported, so
 * load/save can be unit-tested without mocking the global fetch or
 * SvelteKit's `$app` modules -- mirrors planTemplate.ts's Fetcher. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

function templatePath(practiceId: string): string {
	return `/api/practices/${practiceId}/contract-template`;
}

/** Loads a Practice's Contract Template. Throws with the response body
 * text on a non-2xx response, mirroring planTemplate.ts's loadTemplate. */
export async function loadContractTemplate(
	fetcher: Fetcher,
	practiceId: string
): Promise<ContractTemplate> {
	const response = await fetcher(templatePath(practiceId));
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/**
Replaces a Practice's Contract Template with prose.
*/
export async function saveContractTemplate(
	fetcher: Fetcher,
	practiceId: string,
	prose: string
): Promise<ContractTemplate> {
	const response = await fetcher(templatePath(practiceId), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ prose })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Validates prose against the same rule the Go BFF enforces server-side
 * (api/internal/contracts/template.go's PutTemplateHandler) -- mirrored
 * here purely for early UX feedback before a round trip; the server
 * remains the authority. Returns an error message, or undefined if prose
 * is valid. */
export function validateProse(prose: string): string | undefined {
	if (prose.trim() === '') {
		return 'prose is required';
	}
	return undefined;
}
