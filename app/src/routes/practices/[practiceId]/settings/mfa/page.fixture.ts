/*
 * The MFA settings screen, as the continuum check sees it (#595, #606).
 *
 * An Owner viewing the switch before she has thrown it: `required` is
 * false and a realistic slice of a 14-doula Practice's roster --
 * `withoutSecondFactor` -- has not enrolled a second factor yet, which is
 * the number this screen exists to show her before she can bar them.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const impact = { required: false, withoutSecondFactor: 6 };

export const fixture: RouteFixture = {
	name: 'The MFA settings screen',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/mfa',
	pageData: {
		session: {
			practiceId: 'practice-1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner'],
			isContractor: false
		}
	},
	respond: () => jsonResponse(impact),
	readyText: 'Multi-factor authentication'
};
