/**
 * The Client portal-invite vocabulary (#346, #785, #255): one word per
 * state, shared by every screen that names it -- the Clients list column
 * and the Engagement page's own readout -- so a Client's portal state
 * reads the same way wherever it appears, per ADR-0021's "reuse the
 * existing derivation, do not fork it" reading of docs/design/govuk-alignment.md.
 *
 * Structural, not tied to ClientListItem or any one DTO: any object
 * carrying `portalInviteStatus`/`portalInviteDelivery`/`emailSuppressed`
 * -- the Clients list's ClientListItem, the Engagement page's own
 * EngagementSummary fields -- satisfies PortalInviteSubject.
 */

/** A Client's portal access, as portalinvite.Derive names it
 * (api/internal/portalinvite/lifecycle.go). `expired` is never stored: it
 * is an invitation past its expiry, worked out on the server from the
 * clock. Absent means she has never been invited. */
export type PortalAccessState = 'invited' | 'refused' | 'accepted' | 'expired' | 'revoked' | 'unlinked';

/** What the latest send of a live invitation did -- portal_invite_outbox's
 * own status, present only while the state is `invited`. A separate
 * field because it is a separate table's vocabulary. */
export type PortalInviteDelivery = 'pending' | 'sent' | 'bounced' | 'dead_lettered' | 'complained';

export interface PortalInviteSubject {
	portalInviteStatus?: PortalAccessState;
	portalInviteDelivery?: PortalInviteDelivery;
	emailSuppressed?: boolean;
}

/** #346: labels for a live invitation, by its delivery. "complained"
 * reads as informational -- the mail arrived, re-inviting will not
 * help -- unlike "bounced"/"dead_lettered", which ask for one. */
export const portalInviteDeliveryLabel: Record<PortalInviteDelivery, string> = {
	pending: 'Invite pending',
	sent: 'Invite sent',
	bounced: 'Bounced — needs re-invite',
	dead_lettered: 'Dead-lettered — needs re-invite',
	complained: 'Marked as spam (no action needed)'
};

/** Labels for every state but a live invitation, which reads by its
 * delivery above. An invitation with nothing queued reads as sent: the
 * link was handed back to the person who sent it. */
export const portalInviteStatusLabel: Record<Exclude<PortalAccessState, 'invited'>, string> = {
	accepted: 'Accepted',
	refused: 'Invite refused: signs in through another record',
	expired: 'Invite expired — needs re-invite',
	revoked: 'Invite withdrawn — needs re-invite',
	unlinked: 'Portal access removed'
};

/** #2522: the way on from a refused invite. It names no other record and
 * links to none: which record her login reaches is legible only from a
 * portal session (GLOSSARY.md, Portal Account). Combining starts from a
 * details save (ADR-0040), and the record kept is decided there. */
export const refusedInviteWayOn =
	"Combine the two records: save this Client's details, and you will be asked whether the other record is the same person.";

/** Whether subject's accept was refused because her login already
 * reaches another Client at this Practice (#2522). */
export function isRefusedInvite(subject: PortalInviteSubject): boolean {
	return subject.portalInviteStatus === 'refused';
}

/** #785: the same two failed deliveries read differently once the
 * address is suppressed (ADR-0029). `portalInviteDeliveryLabel` predates
 * #733's send-time guard and was correct when it was written: a
 * re-invite was worth a try. It no longer is -- a send to a suppressed
 * address is refused before Mailgun is asked and dead-letters
 * immediately, so the only move that changes anything is lifting the
 * block on **Blocked email addresses**. Keyed on the suppression rather
 * than folded into the map above because the delivery does not change
 * when Staff clear one: the row stays 'bounced'/'dead_lettered'
 * afterwards, and re-invite becomes the right answer again. */
export const suppressedPortalInviteStatusLabel: Partial<Record<PortalInviteDelivery, string>> = {
	bounced: 'Bounced — unblock the address to invite again',
	dead_lettered: 'Not sent — unblock the address to invite again'
};

/** Whether a live invitation's send failed -- the two deliveries a
 * re-invite (or an unblock) is the answer to. */
function hasFailedDelivery(subject: PortalInviteSubject): boolean {
	return (
		subject.portalInviteStatus === 'invited' &&
		(subject.portalInviteDelivery === 'bounced' || subject.portalInviteDelivery === 'dead_lettered')
	);
}

/** True when subject's words are the suppressed pair above -- the invite
 * cannot be delivered rather than merely having failed once. */
export function isBlockedInvite(subject: PortalInviteSubject): boolean {
	return Boolean(subject.emailSuppressed) && hasFailedDelivery(subject);
}

/** The one word a screen shows for subject's portal-invite state --
 * "Never invited" for an absent status, the blocked pair while the
 * address is suppressed, a live invitation by its delivery, otherwise
 * `portalInviteStatusLabel`'s own wording. */
export function portalInviteStatusText(subject: PortalInviteSubject): string {
	const status = subject.portalInviteStatus;
	if (!status) return 'Never invited';
	if (status === 'invited') {
		const delivery = subject.portalInviteDelivery ?? 'sent';
		if (isBlockedInvite(subject)) return suppressedPortalInviteStatusLabel[delivery] ?? portalInviteDeliveryLabel[delivery];
		return portalInviteDeliveryLabel[delivery] ?? portalInviteDeliveryLabel.sent;
	}
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

/** The question a Contract send asks, mirroring
 * portalinvite.Access.CanReachPortal: she signs in already, or holds an
 * invitation she can still accept whose send did not fail. */
export function canReachPortal(subject: PortalInviteSubject): boolean {
	if (subject.portalInviteStatus === 'accepted') return true;
	return subject.portalInviteStatus === 'invited' && !hasFailedDelivery(subject);
}
