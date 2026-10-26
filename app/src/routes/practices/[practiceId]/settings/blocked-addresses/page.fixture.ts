/*
 * The blocked-addresses screen, as the continuum check sees it (#595,
 * #744).
 *
 * Two rows, not one (#596): the two causes are the whole screen, and a
 * list holding only one of them leaves the other's cell -- the sentence
 * that stands where a button would be -- unmeasured at every width.
 *
 * The content is hostile, not polite (ADR-0025, #537). An address is a
 * Client's or a Staff member's own typing, and the first row carries the
 * longest real one this repo has: a hyphenated double-barreled local
 * part with a plus-tag on a long subdomain. That is the widest a cell
 * here ever gets, and it sits beside the row's own Unblock button.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { EmailSuppression } from '#lib/emailSuppression.js';
import type { RouteFixture, RouteVariant } from '../../../../routeFixture.js';
import { nth, TYPICAL_CLIENTS } from '../../../../typicalPilot.js';
import Page from './+page.svelte';

export const suppressions: EmailSuppression[] = [
	{
		address: 'anne-marie.ochieng-whitfield+portal-intake@mail.highland-midwifery-group.example.org',
		cause: 'bounce',
		createdAt: '2027-03-14T09:30:00Z',
		clearable: true
	},
	{
		// Never lifted, by design -- the row exists so Staff can see why
		// the mail stopped, and it carries a sentence instead of a button.
		address: 'persephone@example.test',
		cause: 'complaint',
		createdAt: '2027-02-01T08:00:00Z',
		clearable: false
	}
];

/*
 * Typical pilot data (#1981), the screen this route's table floor is
 * measured on (`typicalPilot.ts` says where each value comes from): the
 * same two causes, each on an ordinary address -- a Client's mistyped
 * mailbox that bounced, and one that marked an email as spam. The long
 * plus-tagged address on a long subdomain stays on the hostile screen.
 */
export const typical: RouteVariant = {
	name: 'The blocked email addresses screen, with typical pilot data',
	typical: true,
	respond: () =>
		jsonResponse({
			suppressions: [
				{ ...suppressions[0]!, address: nth(TYPICAL_CLIENTS, 4).email },
				{ ...suppressions[1]!, address: nth(TYPICAL_CLIENTS, 5).email }
			]
		})
};

export const fixture: RouteFixture = {
	name: 'The blocked email addresses screen',
	variants: [typical],
	component: Page,
	params: { practiceId: 'practice-1' },
	url: 'https://example.test/practices/practice-1/settings/blocked-addresses',
	respond: () => jsonResponse({ suppressions }),
	readyText: 'Blocked email addresses'
};
