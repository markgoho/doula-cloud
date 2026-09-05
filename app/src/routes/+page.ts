import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch, probeSession } from '#lib/api.js';
import { decideLanding, type Membership, type SessionInfo } from '#lib/landing.js';
import { decidePortalLanding, type Engagement, type PortalSessionInfo } from '#lib/portalLanding.js';
import type { PageLoad } from './$types';

export type RootLanding =
	| { type: 'staff-picker'; memberships: Membership[] }
	| { type: 'portal-picker'; engagements: Engagement[] }
	| { type: 'signed-out' };

/**
 * / has no session of its own to protect, so it probes for one of each
 * kind and reuses whichever population's own login screen already decides
 * where she lands (#357) -- decideLanding for a Staff member, decidePortalLanding
 * for a Client-portal user -- rather than a third copy of either decision.
 * `probeSession` (#lib/api.js) is the same helper both login screens now
 * use for their own on-load check (#283) -- see its own doc comment for
 * why `apiFetchWithSession` is the wrong tool here.
 */
export const load: PageLoad = async (): Promise<RootLanding> => {
	/*
	 * Read as a status rather than through `probeSession` (#745). That
	 * helper answers "is there a Staff session" with one `undefined` for
	 * two different people: nobody signed in, and somebody signed in whose
	 * identity resolves to no staff row -- a signup whose second half
	 * failed. The first belongs on the signed-out home below; the second
	 * belongs on `/no-practice`, and telling her to sign up or log in is
	 * telling her to do the two things that have already stopped working
	 * for her.
	 */
	const staffResponse = await readSession('/api/staff/session');
	if (staffResponse?.ok) {
		const landing = decideLanding((await staffResponse.json()) as SessionInfo);
		if (landing.type === 'redirect') {
			redirect(303, resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
		}
		// No Membership at all is a state, not a picker with nothing in it:
		// she was removed from her last Practice. Same screen, same words.
		else if (landing.type === 'no-practice') {
			redirect(303, resolve('/(signed-out)/no-practice'));
		}
		return { type: 'staff-picker', memberships: landing.memberships };
	}

	const portalSession = await probeSession<PortalSessionInfo>('/api/portal/session');
	if (portalSession) {
		const landing = decidePortalLanding(portalSession);
		if (landing.type === 'redirect') {
			redirect(
				303,
				resolve('/portal/(authenticated)/engagements/[engagementId]', {
					engagementId: landing.engagementId
				})
			);
		}
		return { type: 'portal-picker', engagements: landing.engagements };
	}

	// A live credential that resolves to neither population: the 404 is
	// the BFF saying "your session is fine, and it is nobody's" (#745).
	if (staffResponse?.status === 404) {
		redirect(303, resolve('/(signed-out)/no-practice'));
	}

	return { type: 'signed-out' };
};

/**
 * The session read `probeSession` cannot do: the status survives, so
 * "no session" and "a session belonging to nobody" stay apart. A thrown
 * fetch -- offline, a rewrite miss -- is still `undefined`, which reads
 * as "can't tell, proceed as signed out" exactly as it does there.
 */
async function readSession(path: string): Promise<Response | undefined> {
	try {
		return await apiFetch(path);
	} catch {
		return undefined;
	}
}
