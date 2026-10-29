/*
 * One Client field on its own page (#2193), as the continuum check sees
 * it. It opens on the list fixture's choice question, so the answers box,
 * the locked kind and the place select are all drawn. A field not yet
 * saved is a second branch (no place, no Archive, a kind still open), and
 * a Doula, who may not change the fields, a third.
 */
import { jsonResponse } from '#lib/testResponse.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../../routeFixture.js';
import { template } from '../page.fixture.js';
import Page from './+page.svelte';

export const changedField = template.fields[1];

export const asNewField: RouteVariant = {
	name: 'A Client field, not yet saved',
	params: { practiceId: 'practice-1', questionId: 'new' },
	url: 'https://example.test/practices/practice-1/settings/client-fields/new?kind=multi_select',
	readyText: 'Add a question'
};

export const asDoula: RouteVariant = {
	name: 'A Client field, as a Doula',
	pageData: practiceSession(['doula'])
};

export const fixture: RouteFixture = {
	name: 'A Client field',
	component: Page,
	params: { practiceId: 'practice-1', questionId: changedField.id },
	url: `https://example.test/practices/practice-1/settings/client-fields/${changedField.id}`,
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(template),
	readyText: 'Change a question',
	variants: [asNewField, asDoula]
};
