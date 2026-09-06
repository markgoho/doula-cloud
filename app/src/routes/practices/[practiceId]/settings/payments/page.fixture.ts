/*
 * The Stripe Connect settings screen, as the continuum check sees it
 * (#595).
 *
 * `requirementsDue` is a list of Stripe field paths, not Practice-typed
 * content, and `website` only gates a boolean here -- so this measures
 * the widest realistic status combination rather than any hostile
 * string.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ConnectStatusResult } from '#lib/payments.js';
import type { PracticeWebsite } from '#lib/website.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const status: ConnectStatusResult = {
	status: 'payouts_restricted',
	cardPaymentsStatus: 'active',
	payoutsStatus: 'restricted',
	requirementsDue: ['individual.verification.document', 'external_account', 'business_profile.url']
};

const website: PracticeWebsite = {
	mode: 'hosted',
	ownUrl: '',
	serviceDescription: 'Full-spectrum birth and postpartum doula support',
	cancellationPolicy: 'Full refund up to 30 days before the due date',
	updatedBy: 'Anne-Marie Ochieng-Whitfield',
	updatedAt: '2026-08-01T00:00:00Z',
	pageState: 'live',
	pageCheckedAt: '2026-08-01T00:00:00Z',
	pageCheckDetail: '',
	pageUrl: 'https://doula.cloud/p/riverside-doula-collective'
};

export const fixture: RouteFixture = {
	name: 'The Stripe Connect settings screen',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/payments',
	pageData: {
		session: {
			practiceId: 'practice-1',
			practiceName: 'Riverside Doula Collective',
			roles: ['owner'],
			isContractor: false
		}
	},
	respond: (path) => {
		if (path.endsWith('/website')) return jsonResponse(website);
		return jsonResponse(status);
	},
	readyText: 'Payments'
};
