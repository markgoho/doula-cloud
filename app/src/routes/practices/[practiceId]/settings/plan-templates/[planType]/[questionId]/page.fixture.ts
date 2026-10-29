/*
 * One Plan Template question on its own page (#2193), as the continuum
 * check sees it. It opens on the list fixture's choice question, whose
 * label and answers are the hostile values (#530's URL among them), so the
 * answers box and the place select are both drawn. A question not yet
 * saved is a second branch: no place to choose, and no Remove.
 */
import { jsonResponse } from '#lib/testResponse.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../../../routeFixture.js';
import { careTemplate } from '../../page.fixture.js';
import Page from './+page.svelte';

export const changedQuestion = careTemplate.fields[1];

export const asNewQuestion: RouteVariant = {
	name: 'A Plan Template question, not yet saved',
	params: { practiceId: 'practice-1', planType: 'care_plan', questionId: 'new' },
	url: 'https://example.test/practices/practice-1/settings/plan-templates/care_plan/new?kind=single_select',
	readyText: 'Add a question'
};

export const fixture: RouteFixture = {
	name: 'A Plan Template question',
	component: Page,
	params: { practiceId: 'practice-1', planType: 'care_plan', questionId: changedQuestion.id },
	url: `https://example.test/practices/practice-1/settings/plan-templates/care_plan/${changedQuestion.id}`,
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(careTemplate),
	readyText: 'Change a question',
	variants: [asNewQuestion]
};
