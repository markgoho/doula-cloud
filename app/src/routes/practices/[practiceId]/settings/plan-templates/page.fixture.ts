/*
 * The Plan Template editor, as the continuum check sees it (#595).
 *
 * A field's own `label` -- the question a Practice wants asked of every
 * Client's Birth Plan -- is its one free-text surface, so it carries
 * #530's own URL.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Template } from '#lib/planTemplate.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const template: Template = {
	planType: 'birth_plan',
	fields: [
		{
			id: 'field-1',
			type: 'long_text',
			label: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			order: 1
		}
	]
};

export const fixture: RouteFixture = {
	name: 'The Plan Template editor',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/plan-templates',
	respond: () => jsonResponse(template),
	readyText: 'Plan Templates'
};
