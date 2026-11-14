import { cannotBeReachedDraft } from '#lib/cannotBeReachedDraft.svelte.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import { ownerSession, practiceId, respond, seedDraft } from '../journeyFixture.js';
import Page from './+page.svelte';

// eslint-disable-next-line unicorn/no-top-level-side-effects -- installing state IS what a fixture does: the sweep mounts this route without the layout that fills it, and the module has no other moment to do it in.
seedDraft();
// eslint-disable-next-line unicorn/no-top-level-side-effects -- as above.
cannotBeReachedDraft.setCover('engagement-1', 'staff-3');

export const fixture: RouteFixture = {
	name: 'Recording a time a Doula cannot be reached: check',
	component: Page,
	params: { practiceId },
	url: 'https://example.test/practices/practice-1/on-call/cannot-be-reached/check',
	pageData: ownerSession,
	respond,
	readyText: 'Check the time before recording it'
};
