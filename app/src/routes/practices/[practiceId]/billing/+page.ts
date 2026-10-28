import { streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import { billingPath, type Balance } from '#lib/billing.js';
import type { PageLoad } from './$types';

/**
 * Loads through SvelteKit's `load`, so a hovered link starts the read
 * (#471), and hands the read over unawaited, so a click changes screen at
 * once (#2249). Billing reads are Owner/Admin-only (ADR-0008): a Doula who
 * types this URL gets a 403, which the screen draws as the page
 * `practices/+error.svelte` would.
 */
export const load: PageLoad = ({ params }): { read: Promise<StreamedRead<Balance>> } => ({
	read: streamRead<Balance>(billingPath(params.practiceId))
});
