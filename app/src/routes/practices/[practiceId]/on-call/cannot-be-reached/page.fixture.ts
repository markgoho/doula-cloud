import type { RouteFixture } from '../../../../routeFixture.js';
import { ownerSession, practiceId, respond } from './journeyFixture.js';
import Page from './+page.svelte';

export const fixture: RouteFixture = {
	name: 'Recording a time a Doula cannot be reached: which Doula',
	component: Page,
	params: { practiceId },
	url: 'https://example.test/practices/practice-1/on-call/cannot-be-reached',
	pageData: ownerSession,
	respond,
	readyText: 'Which Doula cannot be reached?'
};
