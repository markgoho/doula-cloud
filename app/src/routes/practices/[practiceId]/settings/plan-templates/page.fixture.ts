/*
 * The Plan Template editor, as the continuum check sees it (#595).
 *
 * A field's own `label` -- the question a Practice wants asked of every
 * Client's Birth Plan -- is its one free-text surface, so it carries
 * #530's own URL.
 *
 * Two Fields, not one (#720): `isSelectType()` is the only thing this
 * editor branches on -- an Options textarea renders only for
 * `single_select`/`multi_select` -- so a fixture holding only a
 * `long_text` field never shows the Options editor at all. The second
 * Field is `single_select`, with its own hostile label and three
 * options, one of them #530's own URL, since an option is a Practice's
 * free text too.
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
		},
		{
			id: 'field-2',
			type: 'single_select',
			label: 'Who is your primary support person during labor and delivery?',
			options: [
				'Anne-Marie Ochieng-Whitfield',
				'Persephone Ochieng-Whitfield',
				'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
			],
			order: 2
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
