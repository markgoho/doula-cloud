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
 *
 * A third state (#866): which plan type is showing is a field this
 * screen renders differently too, now that the switcher fetches and
 * shows a different Template per tab. `careTemplate` is the pair above,
 * fetched first since `+page.svelte` defaults to `care_plan`;
 * `birthTemplate` is a second, differently-shaped Template so the
 * switcher's own spec can assert the panel actually changed on
 * activation against this fixture's content, rather than declaring a
 * private copy of it (`.claude/rules/svelte-tests.md`).
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Template } from '#lib/planTemplate.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const careTemplate: Template = {
	planType: 'care_plan',
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

export const birthTemplate: Template = {
	planType: 'birth_plan',
	fields: [
		{
			id: 'field-3',
			type: 'long_text',
			label:
				'What does a calm room look like for Persephone Ochieng-Whitfield, and who is asked to leave if it stops looking that way?',
			order: 1
		}
	]
};

function respond(path: string) {
	return jsonResponse(path.endsWith('/plan-templates/birth_plan') ? birthTemplate : careTemplate);
}

export const fixture: RouteFixture = {
	name: 'The Plan Template editor',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/plan-templates',
	respond,
	readyText: 'Plan Templates'
};
