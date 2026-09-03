/*
 * The Client-portal message thread, as the continuum check sees it
 * (#595).
 *
 * `RecordDetail`'s title ("Messages") is static across load states (see
 * the route's own markup), so this is measured the moment it mounts --
 * same as `messages`'s Staff-side sibling would be. A message body is
 * exactly the free text #530's URL was found in: a Practice paste-able
 * referral link, this time typed by the Client rather than a Staff
 * member.
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
	}
];

export const fixture: RouteFixture = {
	name: 'The Client-portal message thread',
	component: Page,
	params: { engagementId: 'engagement-1' },
	url: 'https://example.test/portal/engagements/engagement-1/messages',
	pageData: { practiceName: 'Riverside Doula Collective' },
	respond: () => jsonResponse({ items: messages, hasMore: false }),
	readyText: 'Messages'
};
