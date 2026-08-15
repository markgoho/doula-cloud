export interface Engagement {
	engagementId: string;
	practiceName: string;
	status: string;
}

export interface PortalSessionInfo {
	engagements: Engagement[];
}

export type PortalLanding =
	| { type: 'redirect'; engagementId: string }
	| { type: 'picker'; engagements: Engagement[] };

/**
 * Decides where a Client-portal user lands after login: straight to their
 * only Engagement, or a picker otherwise (a Client can, in principle,
 * have Engagements at more than one Practice over time, or none yet).
 * Unlike decideLanding for Staff, there is no last-used tracking here --
 * nothing persists it, and v1 doesn't ask for it.
 */
export function decidePortalLanding(session: PortalSessionInfo): PortalLanding {
	if (session.engagements.length === 1) {
		return { type: 'redirect', engagementId: session.engagements[0].engagementId };
	}

	return { type: 'picker', engagements: session.engagements };
}
