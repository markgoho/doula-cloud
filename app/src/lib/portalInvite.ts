/**
 * The Client portal-invite vocabulary (#346, #785, #255): one word per
 * state, shared by every screen that names it -- the Clients list column
 * and the Engagement page's own readout -- so a Client's portal state
 * reads the same way wherever it appears, per ADR-0021's "reuse the
 * existing derivation, do not fork it" reading of docs/design/govuk-alignment.md.
 *
 * Structural, not tied to ClientListItem or any one DTO: any object
 * carrying `portalInviteStatus`/`emailSuppressed` -- the Clients list's
 * ClientListItem, the Engagement page's own EngagementSummary fields --
 * satisfies PortalInviteSubject.
 */
export interface PortalInviteSubject {
	portalInviteStatus?: string;
	emailSuppressed?: boolean;
}

/** #346: labels for portal_invite_outbox's states, plus "accepted" (from
 * client_portal_users.identity_uid) and the absent-key fallback
 * `portalInviteStatusText` uses for a Client never invited at all.
 * "complained" reads as informational -- the mail arrived, re-inviting
 * will not help -- unlike "bounced"/"dead_lettered", which ask for one. */
export const portalInviteStatusLabel: Record<string, string> = {
	pending: 'Invite pending',
	sent: 'Invite sent',
	bounced: 'Bounced — needs re-invite',
	dead_lettered: 'Dead-lettered — needs re-invite',
	complained: 'Marked as spam (no action needed)',
	accepted: 'Accepted'
};

/** #785: the same two failed states read differently once the address is
 * suppressed (ADR-0029). `portalInviteStatusLabel` predates #733's
 * send-time guard and was correct when it was written: a re-invite was
 * worth a try. It no longer is -- a send to a suppressed address is
 * refused before Mailgun is asked and dead-letters immediately, so the
 * only move that changes anything is lifting the block on **Blocked
 * email addresses**. Keyed on the suppression rather than folded into
 * the map above because the outbox status does not change when Staff
 * clear one: the row stays 'bounced'/'dead_lettered' afterwards, and
 * re-invite becomes the right answer again. */
export const suppressedPortalInviteStatusLabel: Record<string, string> = {
	bounced: 'Bounced — unblock the address to invite again',
	dead_lettered: 'Not sent — unblock the address to invite again'
};

/** True when subject's words are the suppressed pair above -- the invite
 * cannot be delivered rather than merely having failed once. */
export function isBlockedInvite(subject: PortalInviteSubject): boolean {
	if (!subject.emailSuppressed || !subject.portalInviteStatus) return false;
	return Object.hasOwn(suppressedPortalInviteStatusLabel, subject.portalInviteStatus);
}

/** The one word a screen shows for subject's portal-invite state --
 * "Never invited" for an absent status, the blocked pair while the
 * address is suppressed, otherwise `portalInviteStatusLabel`'s own
 * wording. */
export function portalInviteStatusText(subject: PortalInviteSubject): string {
	const status = subject.portalInviteStatus;
	if (!status) return 'Never invited';
	if (isBlockedInvite(subject)) return suppressedPortalInviteStatusLabel[status];
	return portalInviteStatusLabel[status] ?? 'Never invited';
}

/** Whether subject has never been sent a portal invite at all -- the
 * #255 precondition PostSendContractHandler enforces server-side, and
 * the Engagement page's own Contract-section block mirrors client-side. */
export function hasNeverBeenInvited(subject: PortalInviteSubject): boolean {
	return !subject.portalInviteStatus;
}

/** Whether subject has already accepted her portal invite -- #255's
 * "do not offer a portal invite to a Client who has already accepted
 * one". */
export function hasAcceptedPortalInvite(subject: PortalInviteSubject): boolean {
	return subject.portalInviteStatus === 'accepted';
}
