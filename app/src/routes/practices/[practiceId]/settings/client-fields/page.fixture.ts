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
 * `single_select`, with #537's hyphenated double-barreled name as one
 * of its options, since an option is a Practice's free text too. A third
 * Field is archived: `ClientFieldTemplateEditor` renders a whole
 * "Archived fields" section only once one exists (line 172's own
 * `{#if archivedFields.length > 0}`), which a fixture with no archived
 * Field never shows at all.
 *
 * Two sessions (#928), and they are two different components rather than
 * one component with controls withheld: `isOwnerOrAdmin` chooses between
 * `ClientFieldTemplateEditor` and a plain read-only `<ul>` of the same
 * Fields, each label followed by "(archived)" where it applies, with a
 * sentence under it naming who to ask. Neither tree is a subset of the
 * other.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Template } from '#lib/clientFieldTemplate.js';
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
import { practiceSession } from '../../../../routeFixture.js';
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

/*
 * The Doula's reading of the same Template. She is asked for it and
 * shown it -- the read is not gated, only the editing is -- so this
 * inherits `respond` deliberately: the three Fields above are exactly
 * what her list has to lay out, and the first of them is the pasted URL
 * that broke a grid track. What she reads instead of the editor is a
 * bare `<ul>` whose one Practice-typed value per row is the same label,
 * plus a sentence of this repo's own copy.
 */
export const asDoula: RouteVariant = {
	name: 'The Client Field Template editor, as a Doula',
	pageData: practiceSession(['doula'])
};

export const fixture: RouteFixture = {
	name: 'The Client Field Template editor, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/client-fields',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(template),
	readyText: 'Client Fields',
	variants: [asDoula]
};
