/*
 * The Settings hub, as the continuum check sees it (#595).
 *
 * A fixed nav (see the route's own header comment: "a way in, not a
 * settings design") -- no Practice-typed content, every label and
 * description is this repo's own copy. It reads the caller's role
 * (#606) to decide whether to include the Owner-only MFA entry, so this
 * fixture answers as an Owner -- the widest of the two lists the hub
 * can show.
 */
import type { RouteFixture } from '../../../routeFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'The Settings hub',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings',
	pageData: {
		session: {
			practiceId: 'practice-1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner'],
			isContractor: false
		}
	},
	readyText: 'Settings'
};
