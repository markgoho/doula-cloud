import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { probeSession } from '#lib/api.js';
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
