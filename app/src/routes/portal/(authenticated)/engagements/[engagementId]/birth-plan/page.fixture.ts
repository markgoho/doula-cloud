/*
 * The Client-portal read-only Birth Plan, as the continuum check sees it
 * (#595).
 *
 * The `<h1>` only renders once the Instance has loaded (see the route's
 * own `{#if instance === null}`/`{:else}` branches), so `readyText`
 * genuinely gates on the fetch rather than racing it. The answer to the
 * one long_text field carries #530's own URL -- a Practice-defined field
 * label is this screen's free text too, but the answer a Client pastes
 * in is the value that actually broke a grid track.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Instance } from '#lib/planInstance.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

const instance: Instance = {
	engagementId: 'engagement-1',
	planType: 'birth_plan',
	fields: [
		{ id: 'support-people', type: 'long_text', label: 'Who do you want with you, and what should we know about them?', order: 1 }
	],
	answers: {
		'support-people':
			'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
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
