import { describe, expect, it } from 'vitest';
import {
	hasAcceptedPortalInvite,
	hasNeverBeenInvited,
	isBlockedInvite,
	portalInviteStatusText
} from './portalInvite.js';

describe('portalInviteStatusText', () => {
	it('reads an absent status as "Never invited"', () => {
		expect(portalInviteStatusText({})).toBe('Never invited');
	});

	it('reads a known status through the label map', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'pending' })).toBe('Invite pending');
		expect(portalInviteStatusText({ portalInviteStatus: 'accepted' })).toBe('Accepted');
	});

	it('falls back to "Never invited" for an unrecognized status', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'made-up' })).toBe('Never invited');
	});

	it('reads the blocked words once a bounced/dead-lettered status is suppressed', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'bounced', emailSuppressed: true })).toBe(
			'Bounced — unblock the address to invite again'
		);
		expect(portalInviteStatusText({ portalInviteStatus: 'dead_lettered', emailSuppressed: true })).toBe(
			'Not sent — unblock the address to invite again'
		);
	});

	it('reads the ordinary words for bounced/dead-lettered while not suppressed', () => {
		expect(portalInviteStatusText({ portalInviteStatus: 'bounced', emailSuppressed: false })).toBe(
			'Bounced — needs re-invite'
		);
	});
});

describe('isBlockedInvite', () => {
	it('is false with no status at all', () => {
		expect(isBlockedInvite({ emailSuppressed: true })).toBe(false);
	});

	it('is false when suppressed but the status is not one of the blocked pair', () => {
		expect(isBlockedInvite({ portalInviteStatus: 'pending', emailSuppressed: true })).toBe(false);
	});

	it('is false when the status is blocked-shaped but not suppressed', () => {
		expect(isBlockedInvite({ portalInviteStatus: 'bounced', emailSuppressed: false })).toBe(false);
	});

	it('is true for bounced/dead_lettered while suppressed', () => {
		expect(isBlockedInvite({ portalInviteStatus: 'bounced', emailSuppressed: true })).toBe(true);
		expect(isBlockedInvite({ portalInviteStatus: 'dead_lettered', emailSuppressed: true })).toBe(true);
	});
});

describe('hasNeverBeenInvited', () => {
	it('is true with no status', () => {
		expect(hasNeverBeenInvited({})).toBe(true);
	});

	it('is false with any status', () => {
		expect(hasNeverBeenInvited({ portalInviteStatus: 'pending' })).toBe(false);
	});
});

describe('hasAcceptedPortalInvite', () => {
	it('is true only for "accepted"', () => {
		expect(hasAcceptedPortalInvite({ portalInviteStatus: 'accepted' })).toBe(true);
		expect(hasAcceptedPortalInvite({ portalInviteStatus: 'pending' })).toBe(false);
		expect(hasAcceptedPortalInvite({})).toBe(false);
	});
});
