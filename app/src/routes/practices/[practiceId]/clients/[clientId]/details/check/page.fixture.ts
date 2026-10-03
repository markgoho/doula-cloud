/*
 * The end of a Client's details journey (#1610): every question with
 * what the save will send, a Practice's own sections included, which
 * takes the list past the count that widens the column. Seeded through
 * `detailsFixture.ts`; see that file for why.
 */
import type { RouteFixture } from '../../../../../../routeFixture.js';
import { clientId, practiceId, seedDetails } from '../detailsFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedDetails();

export const fixture: RouteFixture = {
	name: 'Checking the details added from a Client’s record',
	component: Page,
	params: { practiceId, clientId },
	url: 'https://example.test/practices/practice-1/clients/client-1/details/check',
	readyText: 'Check '
};
