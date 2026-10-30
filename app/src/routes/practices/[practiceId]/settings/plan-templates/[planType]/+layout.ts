/*
 * #2244: "What kind of answer" and a question's own page both end in a
 * save that plans/mount.go refuses to anyone but an Owner (#1407,
 * `MAY_CHANGE.planTemplates`). The list draws no way in for anyone else;
 * someone who arrives here by address anyway is sent to the list, which
 * every Staff member may read (ADR-0008), rather than given an editor
 * whose save would be refused. The BFF still refuses the write.
 *
 * Reads the Membership `practices/[practiceId]/+layout.ts` already
 * resolved (#835) through `parent()`.
 */
import { redirect } from '@sveltejs/kit';
import { MAY_CHANGE } from '#lib/settingsSummary.js';
import { isPlanType, planTemplatesHref } from '../planTemplatePages.js';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent, params }) => {
	const { session } = await parent();
	if (!MAY_CHANGE.planTemplates(session)) {
		redirect(307, planTemplatesHref(params.practiceId, isPlanType(params.planType) ? params.planType : 'care_plan'));
	}
	return {};
};
