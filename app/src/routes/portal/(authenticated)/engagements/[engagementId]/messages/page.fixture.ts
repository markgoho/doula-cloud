/*
 * The Client-portal message thread, as the continuum check sees it
 * (#595).
 *
 * `RecordDetail`'s title ("Messages") is static across load states (see
 * the route's own markup), so `readyText` does not gate on the fetch the
 * way the approval screen's own does -- `mountInFrame`'s webfont wait
 * (`ensureFontLoaded`, `style-guide/continuum.ts`) is what lets the
 * thread's own `onMount` load resolve before the sweep measures anyway.
 * A message body is exactly the free text #530's URL was found in: a
 * Practice paste-able referral link, this time typed by the Client
 * rather than a Staff member.
 *
 * Two Messages, not one (#720): `MessageThread` renders a whole
 * attachment block only when `attachmentFilename` is set, so a fixture
 * holding only a body-only message never shows it. The second message
 * is from the Client rather than Staff, carrying both an attachment
 * whose own filename is a Client's typed text and the body a Client
 * would type alongside it.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Message } from '#lib/components/organisms/MessageThread.svelte';
import type { RouteFixture } from '../../../../../routeFixture.js';
import Page from './+page.svelte';

const messages: Message[] = [
	{
		messageId: 'message-1',
		senderType: 'staff',
		senderId: 'staff-1',
		senderName: 'Anne-Marie Ochieng-Whitfield',
		body: 'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		createdAt: '2026-08-01T10:00:00Z'
	},
	{
		messageId: 'message-2',
		senderType: 'client',
		senderId: 'client-1',
		senderName: 'Persephone Ochieng-Whitfield',
		body: 'Here is the photo you asked for, taken at the appointment this morning.',
		attachmentFilename: 'ultrasound-scan-persephone-ochieng-whitfield-2027-09-14.png',
		attachmentContentType: 'image/png',
		createdAt: '2026-08-02T10:00:00Z'
	}
];

export const fixture: RouteFixture = {
	name: 'The Client-portal message thread',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1/messages',
	pageData: { practiceName: 'Riverside Doula Collective' },
	// The attachment's own binary fetch is answered as a refusal rather
	// than left to the default: `jsonResponse`'s fake Response has no
	// `.blob()`, and `loadAttachmentPreviews` already takes its "response
	// not ok" branch gracefully, the same state the style-guide's own
	// "Default" section demonstrates -- the inline preview is that page's
	// own further variant, not this route's.
	respond: (path) =>
		path.endsWith('/attachment')
			? jsonResponse('not found', 404)
			: jsonResponse({ items: messages, hasMore: false }),
	readyText: 'Messages'
};
