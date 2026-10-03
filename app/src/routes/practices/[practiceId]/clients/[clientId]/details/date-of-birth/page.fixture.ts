/*
 * The date of birth question, in a Client's details journey (#1610).
 *
 * Seeded through `detailsFixture.ts` rather than through `respond()`:
 * the journey's record and template are read once by the layout the
 * sweep does not mount. See that file for why the content is what it is.
 */
import type { RouteFixture } from '../../../../../../routeFixture.js';
import { clientId, practiceId, seedDetails } from '../detailsFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedDetails();

export const fixture: RouteFixture = {
	name: 'A Client’s date of birth, added from the record',
	component: Page,
	params: { practiceId, clientId },
	url: 'https://example.test/practices/practice-1/clients/client-1/details/date-of-birth',
	readyText: 'date of birth?'
};
