/*
 * The address question, in a Client's details journey (#1610). Seeded
 * through `detailsFixture.ts`; see that file for why.
 */
import type { RouteFixture } from '../../../../../../routeFixture.js';
import { clientId, practiceId, seedDetails } from '../detailsFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedDetails();

export const fixture: RouteFixture = {
	name: 'A Client’s address, added from the record',
	component: Page,
	params: { practiceId, clientId },
	url: 'https://example.test/practices/practice-1/clients/client-1/details/address',
	readyText: 'address?'
};
