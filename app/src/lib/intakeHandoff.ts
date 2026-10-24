/**
 * What the search that fronts intake hands to intake on a miss (#498,
 * #1829): the four values a Staff member typed, so none is typed twice.
 *
 * ## Why module memory, and not the URL
 *
 * These are a Client's name, date of birth, email and phone. In a query
 * string they were sent to the server on a reload and kept in browser
 * history and in any link that was shared; a fragment is not sent, but is
 * still kept in history and in a shared link. Module memory is in none of
 * those. SvelteKit's `page.state` is not used either: the intake door
 * (`clients/new/+page.ts`) redirects to the first question, and a
 * redirect does not carry it.
 *
 * The search sets the hand-off as its "Add a new Client" link is
 * followed, and the door takes it once -- the same take-once shape as
 * `#lib/outcome.ts`. A link opened in a new tab gets an empty module, so
 * intake there starts blank. A reload of the name question keeps the
 * values because the door wrote them into the draft's own sessionStorage
 * mirror (`intakeDraft.carry`), not because this module kept them.
 */
import type { ClientSearchFields } from './client.js';

interface Pending {
	practiceId: string;
	fields: ClientSearchFields;
}

const carried: { pending?: Pending } = {};

/**
Gives `fields` to the next intake door for `practiceId` to open.
*/
export function handOffToIntake(practiceId: string, fields: ClientSearchFields): void {
	carried.pending = { practiceId, fields };
}

/**
 * What the search handed over, read once. Undefined when nothing was
 * handed over, or when it was for another Practice.
 */
export function takeIntakeHandoff(practiceId: string): ClientSearchFields | undefined {
	const { pending } = carried;
	carried.pending = undefined;
	if (pending?.practiceId !== practiceId) return undefined;
	return pending.fields;
}
