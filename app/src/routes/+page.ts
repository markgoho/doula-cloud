import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { apiFetch } from '#lib/api.js';
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
 *
 * `probeSession`, not `apiFetchWithSession`: that helper treats every 401 as
 * an expired session and sends the browser to a login screen with
 * `sessionEnded=true`. Here a 401 is the ordinary case -- an anonymous
 * first-time visitor -- not a failure, so it reads as "no session of this
 * kind" and this falls through to the next check instead. A thrown fetch
 * (offline, the wrong host) reads the same way: toward the next check, and
 * ultimately toward the signed-out landing, never toward a broken page.
 *
 * Every `redirect()` call sits outside `probeSession`'s `try` -- it throws
 * SvelteKit's own control-flow object, which a surrounding `catch` would
 * otherwise swallow as if the probe itself had failed.
 */
export const load: PageLoad = async (): Promise<RootLanding> => {
	const staffSession = await probeSession<SessionInfo>('/api/staff/session');
	if (staffSession) {
		const landing = decideLanding(staffSession);
		if (landing.type === 'redirect') {
			redirect(303, resolve('/practices/[practiceId]', { practiceId: landing.practiceId }));
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

	return { type: 'signed-out' };
};

async function probeSession<Session>(path: string): Promise<Session | undefined> {
	try {
		const response = await apiFetch(path);
		if (!response.ok) return undefined;
		return (await response.json()) as Session;
	} catch {
		// Offline, a misrouted host, or a non-JSON body from a rewrite miss --
		// every one of these means "can't tell", and this route treats that
		// the same as "no session of this kind" rather than surfacing it.
		return undefined;
	}
}
