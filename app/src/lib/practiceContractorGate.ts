/**
 * Whether the signed-in Staff member is a contractor Doula holding
 * neither the owner nor the admin role -- ADR-0017's "solo Practice"
 * owner-contractor keeps whatever this gates, the same carve-out
 * client/search.go's SearchHandler applies.
 *
 * Shared by every route `load` that needs to know before first paint
 * (billing/+page.ts's same rationale for a role-gated read) -- first
 * `clients/search/+page.ts` (#501), now also `clients/+page.ts` (#539).
 * A session-read failure falls back to `isContractor: false` rather than
 * throwing: every caller uses this for UX only -- the BFF handler it
 * mirrors is the real boundary -- so the worse case of a hiccup here is
 * the pre-gate behavior (the control renders, then a readable 403 if
 * she truly is a contractor), never a page that fails to load for
 * anyone else.
 */
import { apiFetch } from './api.js';

export interface ContractorGate {
	isContractor: boolean;
}

export async function loadContractorGate(practiceId: string): Promise<ContractorGate> {
	try {
		const response = await apiFetch(`/api/practices/${practiceId}/session`);
		if (!response.ok) return { isContractor: false };

		const body: { roles: string[]; isContractor: boolean } = await response.json();
		return {
			isContractor:
				body.isContractor && !body.roles.includes('owner') && !body.roles.includes('admin')
		};
	} catch {
		// A network failure or an unparseable body is no different from a
		// non-ok response, above -- fall back rather than fail the page.
		return { isContractor: false };
	}
}
