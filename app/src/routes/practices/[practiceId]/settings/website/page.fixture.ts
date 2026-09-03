/*
 * The Website settings screen, as the continuum check sees it (#595).
 *
 * A `mode: 'hosted'` Practice that has already answered lands on the
 * "saved" step, whose `DescriptionList` shows exactly the free text this
 * screen collects: what she offers and her cancellation policy each
 * carry #530's own URL, since a Practice could paste a referral link
 * into either.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeWebsite } from '#lib/website.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const website: PracticeWebsite = {
	mode: 'hosted',
	ownUrl: '',
	serviceDescription: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
	cancellationPolicy: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
	updatedBy: 'Anne-Marie Ochieng-Whitfield',
	updatedAt: '2026-08-01T00:00:00Z',
	pageState: 'live',
	pageCheckedAt: '2026-08-01T00:00:00Z',
	pageCheckDetail: '',
	pageUrl: 'https://doula.cloud/p/riverside-doula-collective'
};

export const fixture: RouteFixture = {
	name: 'The Website settings screen',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/website',
	respond: (path) => {
		if (path.endsWith('/session')) return jsonResponse({ practiceName: 'Riverside Doula Collective', roles: ['owner'] });
		return jsonResponse(website);
	},
	readyText: 'Your website'
};
