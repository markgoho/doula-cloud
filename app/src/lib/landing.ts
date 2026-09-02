export interface Membership {
	practiceId: string;
	practiceName: string;
	roles: string[];
}

export interface SessionInfo {
	memberships: Membership[];
	// The Go BFF's staffauth session response omits this key entirely (omitempty) when unset --
	// it never sends a JSON null, so this stays `undefined`, not `null`.
	lastPracticeId: string | undefined;
	/*
	 * The person behind the session, widened onto the same response by
	 * #437 rather than given an endpoint of its own. Two screens need to
	 * read a Staff member's own record back out -- the account screen
	 * where she corrects her work state, and Invitation acceptance, which
	 * has to show her what she already asserted instead of asking again
	 * and discarding the answer -- and both already make this exact call
	 * to find out where to land her. A second round trip to learn facts
	 * the first one could have carried is a round trip nobody needs.
	 *
	 * decideLanding() ignores every one of these; they are here because
	 * this is the type of what the endpoint returns, not because landing
	 * has an opinion about them.
	 */
	staffId: string;
	name: string;
	// The Staff shell's avatar menu shows this beside the name (#452), off
	// practices/+layout.svelte's own inline session type -- /account's bar
	// (#484) reads the same `/api/staff/session` fact off this SessionInfo
	// instead, since that is the type its own `loadAccountSession()` already
	// holds.
	email: string;
	// The USPS two-letter code, e.g. "NY" -- workStateName() in
	// workStates.ts turns it into something a person recognizes.
	workState: string;
	// RFC 3339, the day she last asserted it. The only staleness signal
	// the design has, since nothing prompts a re-assertion (#415).
	workStateReportedAt: string;
}

export type Landing =
	| { type: 'redirect'; practiceId: string }
	| { type: 'picker'; memberships: Membership[] };

/**
 * Decides where a Staff member lands after login: straight to their only
 * Practice, back to their last-used Practice if it's still one they
 * belong to, or a picker otherwise (including when they have none yet).
 */
export function decideLanding(
	// Only the two fields it actually reads, not the whole SessionInfo:
	// #437 widened that response with facts about the person (her name,
	// her work state) which have nothing to do with where she lands, and
	// a function that demands them would be claiming otherwise. A caller
	// holding a full SessionInfo still satisfies this.
	session: Pick<SessionInfo, 'memberships' | 'lastPracticeId'>
): Landing {
	if (session.memberships.length === 1) {
		return { type: 'redirect', practiceId: session.memberships[0].practiceId };
	}

	const lastUsed = session.memberships.find((m) => m.practiceId === session.lastPracticeId);
	if (lastUsed) {
		return { type: 'redirect', practiceId: lastUsed.practiceId };
	}

	return { type: 'picker', memberships: session.memberships };
}
