export interface Membership {
	practiceId: string;
	practiceName: string;
	roles: string[];
}

export interface SessionInfo {
	memberships: Membership[];
	lastPracticeId: string | null;
}

export type Landing =
	| { type: 'redirect'; practiceId: string }
	| { type: 'picker'; memberships: Membership[] };

/**
 * Decides where a Staff member lands after login: straight to their only
 * Practice, back to their last-used Practice if it's still one they
 * belong to, or a picker otherwise (including when they have none yet).
 */
export function decideLanding(session: SessionInfo): Landing {
	if (session.memberships.length === 1) {
		return { type: 'redirect', practiceId: session.memberships[0].practiceId };
	}

	const lastUsed = session.memberships.find((m) => m.practiceId === session.lastPracticeId);
	if (lastUsed) {
		return { type: 'redirect', practiceId: lastUsed.practiceId };
	}

	return { type: 'picker', memberships: session.memberships };
}
