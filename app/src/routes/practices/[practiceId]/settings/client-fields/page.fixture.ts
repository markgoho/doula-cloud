/*
 * The Client Field Template editor, as the continuum check sees it
 * (#595).
 *
 * A field's own `label` is a Practice's free text (what extra question
 * it wants asked of every Client), so it carries #530's own URL --
 * loosely realistic as a field label, but the value proven to break a
 * grid track, which is the point.
 *
 * Three Fields, not one (#720), the same shape as `plan-templates`'s own
 * widening: `isSelectType()` is the only thing an active row itself
 * branches on -- an Options textarea renders only for
 * `single_select`/`multi_select` -- so the second Field is
 * `single_select`, with #537's hyphenated double-barrelled name as one
 * of its options, since an option is a Practice's free text too. A third
 * Field is archived: `ClientFieldTemplateEditor` renders a whole
 * "Archived fields" section only once one exists (line 172's own
 * `{#if archivedFields.length > 0}`), which a fixture with no archived
 * Field never shows at all.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Template } from '#lib/clientFieldTemplate.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const template: Template = {
	fields: [
		{
			id: 'field-1',
			type: 'short_text',
			label: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			order: 1,
			archived: false
		},
		{
			id: 'field-2',
			type: 'single_select',
			label: 'How did this Client hear about the Practice?',
			options: ['Referred by another Client', 'Anne-Marie Ochieng-Whitfield'],
			order: 2,
			archived: false
		},
		{
			id: 'field-3',
			type: 'short_text',
			label: 'Emergency contact, and their phone number',
			order: 3,
			archived: true
		}
	]
};

export const fixture: RouteFixture = {
	name: 'The Client Field Template editor',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/client-fields',
	respond: (path) => {
		if (path.endsWith('/session')) return jsonResponse({ roles: ['owner'] });
		return jsonResponse(template);
	},
	readyText: 'Client Fields'
};
