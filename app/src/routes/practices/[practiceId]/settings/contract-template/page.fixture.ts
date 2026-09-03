/*
 * The Contract Template editor, as the continuum check sees it (#595).
 *
 * `prose` is the one Practice-typed field this screen has -- long-form,
 * so its own realistic length matters more than #537's URL, which is
 * still included as one line since a Practice could paste a referral
 * link into its own contract terms.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { ContractTemplate } from '#lib/contractTemplate.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const template: ContractTemplate = {
	prose:
		'This Contract is between {{practice_name}} and {{client_name}} for {{scope_of_service}}, ' +
		'beginning {{engagement_start_date}} and ending {{engagement_end_date}}. ' +
		'See https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake for the full engagement letter.'
};

export const fixture: RouteFixture = {
	name: 'The Contract Template editor',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/contract-template',
	respond: () => jsonResponse(template),
	readyText: 'Contract Template'
};
