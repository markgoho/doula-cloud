/**
 * What every page of a Client's details journey needs and none of them
 * owns (#1610, ADR-0017's amendment of 2026-10-02): where it starts,
 * where it goes back to, what the Client is called, and the journey the
 * shared question pages are handed.
 */

import { resolve } from '$app/paths';
import { page } from '#lib/appState.svelte.js';
import { clientDetails } from '#lib/clientDetailsFlow.svelte.js';
import { ENGAGEMENT_PARAMETER } from '#lib/clientDetailsJourney.js';
import { intakeFlow } from '#lib/intakeFlow.svelte.js';
import { DETAILS_STEPS, intakeStepList } from '#lib/intakeJourney.js';
import type { QuestionJourney } from '../../questions/questionJourney.js';

export function detailsBasePath(practiceId: string, clientId: string): string {
	return resolve('/practices/[practiceId]/clients/[clientId]/details', { practiceId, clientId });
}

/**
 * The link that opens the journey, on her record and on her
 * Engagement's page. The Engagement's page names itself, so the
 * journey's first Back and its save return there.
 */
export function detailsHref(practiceId: string, clientId: string, engagementId?: string): string {
	const base = detailsBasePath(practiceId, clientId);
	return engagementId ? `${base}?${ENGAGEMENT_PARAMETER}=${encodeURIComponent(engagementId)}` : base;
}

/** The screen that opened the journey: her Engagement's page, or her
 * record. Where the first question's Back goes, and where a save lands. */
export function returnHref(practiceId: string, clientId: string, engagementId?: string): string {
	return engagementId
		? resolve('/practices/[practiceId]/engagements/[engagementId]', { practiceId, engagementId })
		: resolve('/practices/[practiceId]/clients/[clientId]', { practiceId, clientId });
}

function currentIds(): { practiceId: string; clientId: string } {
	return { practiceId: page.params.practiceId ?? '', clientId: page.params.clientId ?? '' };
}

/**
 * What the Client is called on these pages: #463's rule with no pronoun
 * in it, read off the record on file, since the journey never asks the
 * name.
 */
export function knownAs(): string {
	const record = clientDetails.record;
	return record?.preferredName.trim() || record?.givenName.trim() || 'the Client';
}

/**
 * Her details journey, as the shared question pages see it: its own
 * draft, every step but the name, and Back to the screen that opened
 * it. No free save: the journey saves once, at its check page.
 */
export const detailsQuestions: QuestionJourney = {
	get label() {
		return `${knownAs()}'s details`;
	},
	draft: clientDetails.draft,
	get steps() {
		return intakeStepList(intakeFlow.sections, DETAILS_STEPS);
	},
	get sections() {
		return intakeFlow.sections;
	},
	get isReady() {
		return intakeFlow.status === 'ready' && clientDetails.status === 'ready';
	},
	get basePath() {
		const { practiceId, clientId } = currentIds();
		return detailsBasePath(practiceId, clientId);
	},
	get exitHref() {
		const { practiceId, clientId } = currentIds();
		return returnHref(practiceId, clientId, clientDetails.engagementId);
	},
	get knownAs() {
		return knownAs();
	}
};
