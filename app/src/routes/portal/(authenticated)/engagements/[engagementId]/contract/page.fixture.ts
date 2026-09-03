/*
 * The Client-portal read-only Contract, as the continuum check sees it
 * (#595).
 *
 * The `<h1>` only renders once the Contract has loaded (see the route's
 * own `{#if contract === null}`/`{:else}` branches), so `readyText`
 * genuinely gates on the fetch. `values.client_name` carries #537's own
 * hyphenated double-barrelled name, since a merge field's value is typed
 * by Staff about the Client, the same shape the two existing fixtures
 * already measure.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Contract } from '#lib/contract.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

const contract: Contract = {
	engagementId: 'engagement-1',
	status: 'sent',
	prose: 'This Contract is between {{practice_name}} and {{client_name}} for {{scope_of_service}}.',
	mergeFields: ['practice_name', 'client_name', 'scope_of_service'],
	values: {
		practice_name: 'Riverside Doula Collective',
		client_name: 'Anne-Marie Ochieng-Whitfield',
		scope_of_service: 'Full-spectrum birth doula support'
	}
};

export const fixture: RouteFixture = {
	name: 'The Client-portal Contract',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1/contract',
	pageData: { practiceName: 'Riverside Doula Collective' },
	respond: () => jsonResponse(contract),
	readyText: 'Contract'
};
