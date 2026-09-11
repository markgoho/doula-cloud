/*
 * The Delete-this-Practice settings screen, as the continuum check sees
 * it (#595, #871). An Owner viewing the screen before she has started
 * anything: nothing pending, no unsettled Invoice in the way.
 *
 * Two sessions (#928). `isOwner` is the only predicate here and it does
 * not merely withhold controls: anybody else reads a Notice of her own,
 * saying the setting is not hers, which is a sentence no Owner branch
 * draws.
 */
import { jsonResponse } from '#lib/testResponse.js';
import { practiceSession, type RouteFixture, type RouteVariant } from '../../../../routeFixture.js';
import Page from './+page.svelte';

export const status = { pending: false, hasUnsettledInvoices: false };

/*
 * Everyone who is not an Owner, Admin included -- one Notice, no Badge,
 * no button, and no fetch at all: the route asks for the deletion status
 * only when `isOwner`, so the inherited `respond` is never reached on
 * this branch.
 *
 * Its own longest line is the Notice, which is this repo's copy rather
 * than anything a Practice types. The screen's one Practice-typed value
 * is the Practice name, and it appears only inside the ConfirmDialog's
 * consequence -- a dialog that is closed at rest, on a branch that has no
 * dialog trigger to open it.
 */
export const nonOwner: RouteVariant = {
	name: 'The Delete-this-Practice settings screen, as a non-Owner',
	pageData: practiceSession(['admin'])
};

export const fixture: RouteFixture = {
	name: 'The Delete-this-Practice settings screen, as an Owner',
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/delete',
	pageData: practiceSession(['owner']),
	respond: () => jsonResponse(status),
	readyText: 'Delete this Practice',
	variants: [nonOwner]
};
