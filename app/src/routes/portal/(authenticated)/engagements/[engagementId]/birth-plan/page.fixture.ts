/*
 * The Client-portal read-only Birth Plan, as the continuum check sees it
 * (#595).
 *
 * The `<h1>` only renders once the Instance has loaded (see the route's
 * own `{#if instance === null}`/`{:else}` branches), so `readyText`
 * genuinely gates on the fetch rather than racing it. The answer to the
 * long_text field carries #530's own URL -- a Practice-defined field
 * label is this screen's free text too, but the answer a Client pastes
 * in is the value that actually broke a grid track.
 *
 * Two Fields, not one (#720): `BirthPlanView`'s own render adds a whole
 * `dd` shaped differently per `field.type` -- `multi_select` joins every
 * chosen option into one line rather than showing the single answer a
 * `long_text` field does -- so a fixture holding only a `long_text`
 * field never shows that joined line at all. `checkbox` renders to a
 * bare "Yes"/"No" and is confined the way a Badge's own label is, so it
 * earns no row of its own here.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Instance } from '#lib/planInstance.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

const instance: Instance = {
	engagementId: 'engagement-1',
	planType: 'birth_plan',
	fields: [
		{ id: 'support-people', type: 'long_text', label: 'Who do you want with you, and what should we know about them?', order: 1 },
		{ id: 'people-present', type: 'multi_select', label: 'Who do you want in the room with you?', order: 2 }
	],
	answers: {
		'support-people':
			'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		'people-present': [
			'My partner, for the whole labour',
			'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
		]
	}
};

export const fixture: RouteFixture = {
	name: 'The Client-portal Birth Plan',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1/birth-plan',
	pageData: { practiceName: 'Riverside Doula Collective' },
	respond: () => jsonResponse(instance),
	readyText: 'Birth Plan'
};
