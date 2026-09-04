/*
 * The Client Field Template editor, as the continuum check sees it
 * (#595).
 *
 * A field's own `label` is a Practice's free text (what extra question
 * it wants asked of every Client), so it carries #530's own URL --
 * loosely realistic as a field label, but the value proven to break a
 * grid track, which is the point.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Template } from '#lib/clientFieldTemplate.js';
import type { RouteFixture } from '../../../../routeFixture.js';
import Page from './+page.svelte';

const template: Template = {
	fields: [
		{
			id: 'field-1',
			type: 'short_text',
			label: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
			order: 1,
			archived: false
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
