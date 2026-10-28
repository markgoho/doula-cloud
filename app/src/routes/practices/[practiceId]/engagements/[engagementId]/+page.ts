import { streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import { engagementURL, type EngagementSummary } from '#lib/engagementDetail.js';
import type { PageLoad } from './$types';

/**
 * The Engagement itself, read through `load` (#695) so a hovered link
 * starts it, and handed over unawaited (#2249), so a click changes screen
 * at once.
 *
 * Only this one section is read here. The other six -- Visits, Messages,
 * both Plan types, the Contract, the Invoices, the Offers -- stay after
 * mount on purpose: they arrive one at a time and each renders as it
 * lands.
 *
 * A Doula who is not attached to this Engagement gets a 403 (ADR-0008,
 * #350), which the screen draws as the page `practices/+error.svelte`
 * would.
 */
export const load: PageLoad = ({ params }): { read: Promise<StreamedRead<EngagementSummary>> } => ({
	read: streamRead<EngagementSummary>(engagementURL({ practiceId: params.practiceId, engagementId: params.engagementId })
	)
});
