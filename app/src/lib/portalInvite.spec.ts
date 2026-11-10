import { describe, expect, it } from 'vitest';
import {
	canReachPortal,
	hasAcceptedPortalInvite,
	hasNeverBeenInvited,
	isBlockedInvite,
	isRefusedInvite,
	portalInviteStatusText,
	type PortalAccessState
} from './portalInvite.js';

describe('portalInviteStatusText', () => {
	it('reads an absent status as "Never invited"', () => {
		expect(portalInviteStatusText({})).toBe('Never invited');
	});

	it('reads a live invitation by its delivery', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'invited', portalInviteDelivery: 'pending' })).toBe(
			'Invite pending'
		);
		expect(portalInviteStatusText({ portalInviteStatus: 'invited', portalInviteDelivery: 'sent' })).toBe(
			'Invite sent'
		);
		expect(portalInviteStatusText({ portalInviteStatus: 'invited', portalInviteDelivery: 'skipped' })).toBe(
			'Not sent — already accepted'
		);
	});

	it('reads a live invitation with nothing queued as sent', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'invited' })).toBe('Invite sent');
	});

	it('reads every other state through the label map', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'accepted' })).toBe('Accepted');
		expect(portalInviteStatusText({ portalInviteStatus: 'expired' })).toBe('Invite expired — needs re-invite');
		expect(portalInviteStatusText({ portalInviteStatus: 'revoked' })).toBe('Invite withdrawn — needs re-invite');
		expect(portalInviteStatusText({ portalInviteStatus: 'unlinked' })).toBe('Portal access removed');
	});

	it('never reads the last delivery once the invitation is no longer live', () => {
		// The bug 00168 closed: an expired or withdrawn invite fell through
		// to the outbox's "sent".
		expect(portalInviteStatusText({ portalInviteStatus: 'expired', portalInviteDelivery: 'sent' })).toBe(
			'Invite expired — needs re-invite'
		);
	});

	it('falls back to "Never invited" for an unrecognized status', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'made-up' as PortalAccessState })).toBe('Never invited');
	});

	it('reads the blocked words once a bounced/dead-lettered delivery is suppressed', () => {
		expect(
			portalInviteStatusText({ portalInviteStatus: 'invited', portalInviteDelivery: 'bounced', emailSuppressed: true })
		).toBe('Bounced — unblock the address to invite again');
		expect(
			portalInviteStatusText({
				portalInviteStatus: 'invited',
				portalInviteDelivery: 'dead_lettered',
				emailSuppressed: true
			})
		).toBe('Not sent — unblock the address to invite again');
	});

	it('reads the ordinary words for bounced/dead-lettered while not suppressed', () => {
		expect(
			portalInviteStatusText({ portalInviteStatus: 'invited', portalInviteDelivery: 'bounced', emailSuppressed: false })
		).toBe('Bounced — needs re-invite');
	});
});

describe('isBlockedInvite', () => {
	it('is false with no status at all', () => {
		expect(isBlockedInvite({ emailSuppressed: true })).toBe(false);
	});

	it('is false when suppressed but the delivery is not one of the blocked pair', () => {
		expect(isBlockedInvite({ portalInviteStatus: 'invited', portalInviteDelivery: 'pending', emailSuppressed: true })).toBe(
			false
		);
	});

	it('is false when the delivery is blocked-shaped but not suppressed', () => {
		expect(isBlockedInvite({ portalInviteStatus: 'invited', portalInviteDelivery: 'bounced', emailSuppressed: false })).toBe(
			false
		);
	});

	it('is true for bounced/dead_lettered while suppressed', () => {
		expect(isBlockedInvite({ portalInviteStatus: 'invited', portalInviteDelivery: 'bounced', emailSuppressed: true })).toBe(
			true
		);
		expect(
			isBlockedInvite({ portalInviteStatus: 'invited', portalInviteDelivery: 'dead_lettered', emailSuppressed: true })
		).toBe(true);
	});
});

describe('hasNeverBeenInvited', () => {
	it('is true with no status', () => {
		expect(hasNeverBeenInvited({})).toBe(true);
	});

	it('is false with any status', () => {
		expect(hasNeverBeenInvited({ portalInviteStatus: 'invited' })).toBe(false);
		expect(hasNeverBeenInvited({ portalInviteStatus: 'revoked' })).toBe(false);
	});
});

describe('hasAcceptedPortalInvite', () => {
	it('is true only for "accepted"', () => {
		expect(hasAcceptedPortalInvite({ portalInviteStatus: 'accepted' })).toBe(true);
		expect(hasAcceptedPortalInvite({ portalInviteStatus: 'invited' })).toBe(false);
		expect(hasAcceptedPortalInvite({})).toBe(false);
	});
});

describe('canReachPortal', () => {
	it('is true for an accepted login or an invitation that can still be accepted', () => {
		expect(canReachPortal({ portalInviteStatus: 'accepted' })).toBe(true);
		expect(canReachPortal({ portalInviteStatus: 'invited', portalInviteDelivery: 'pending' })).toBe(true);
		expect(canReachPortal({ portalInviteStatus: 'invited', portalInviteDelivery: 'complained' })).toBe(true);
	});

	it('is false for every invitation that cannot bring her to a Contract', () => {
		expect(canReachPortal({})).toBe(false);
		expect(canReachPortal({ portalInviteStatus: 'invited', portalInviteDelivery: 'bounced' })).toBe(false);
		expect(canReachPortal({ portalInviteStatus: 'invited', portalInviteDelivery: 'dead_lettered' })).toBe(false);
		for (const state of ['refused', 'expired', 'revoked', 'unlinked'] as const) {
			expect(canReachPortal({ portalInviteStatus: state })).toBe(false);
		}
	});
});

describe('a refused invite (#2522)', () => {
	it('reads its own status, never "Invite pending"', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'refused' })).toBe(
			'Invite refused: signs in through another record'
		);
	});

	it('is told apart from every other status', () => {
		expect(isRefusedInvite({ portalInviteStatus: 'refused' })).toBe(true);
		expect(isRefusedInvite({ portalInviteStatus: 'invited' })).toBe(false);
		expect(isRefusedInvite({})).toBe(false);
	});
});
