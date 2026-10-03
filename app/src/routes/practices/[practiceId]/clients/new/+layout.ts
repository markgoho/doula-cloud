/*
 * #1609 (ADR-0017): decides, before any intake question mounts, whether
 * this Staff member gets intake at all. Intake used to be reached only
 * through the search, whose own load sends a contractor Doula to the
 * door that explains (#501). Now the overview and the Clients list open
 * the name question directly while a Practice holds no Client record,
 * so intake needs the same gate. `CreateHandler` and RLS still refuse
 * her save; this only puts the door in front of a 403.
 *
 * Reads the Membership `practices/[practiceId]/+layout.ts` already
 * resolved (#835) through `parent()`, the same as `clients/+page.ts`
 * and `clients/search/+page.ts`.
 */
import { isAmbientContractor } from '#lib/roles.js';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent }): Promise<{ isContractor: boolean }> => {
	const { session } = await parent();
	return { isContractor: isAmbientContractor(session) };
};
