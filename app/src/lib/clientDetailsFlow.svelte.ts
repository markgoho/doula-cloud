/**
 * A Client's details journey, as module state (#1610): the record on
 * file it starts from, the draft each question page writes, and the
 * screen that opened it.
 *
 * Separate from intake's `intakeDraft` and for the reason
 * `questions/questionJourney.ts` gives: each journey's route fixtures
 * seed their own state, and two journeys sharing one draft would measure
 * whichever fixture was imported last. The Practice's template is the
 * one thing shared, through `intakeFlow`, because it is the Practice's
 * and the same for both.
 */

import type { Fetcher } from './fetcher.js';
import { loadClientDetail, type ClientDetail } from './clientDetail.js';
import { answersOnFile, canAddDetails } from './clientDetailsJourney.js';
import { IntakeDraft } from './intakeDraft.svelte.js';
import { PageRead } from './pageRead.svelte.js';

export class ClientDetailsFlow {
	/**
	The Client whose read is in flight, so a re-run while it is is a no-op.
	*/
	#readingFor = '';
	clientId = $state('');
	/**
	 * Her record's read: loading, loaded with the record on file as it was
	 * read when the journey opened, or failed. One value, where a status
	 * and an error string beside it were two (#2258's shape).
	 */
	readonly read = new PageRead<ClientDetail>();
	/**
	 * The Engagement whose page opened the journey, which is where the
	 * first question's Back and the save go. Undefined when her record
	 * opened it, which is then where they go.
	 */
	engagementId = $state<string | undefined>();
	/**
	What has been typed, mirrored per Client under its own namespace.
	*/
	readonly draft = new IntakeDraft('details');

	/**
	 * The door (#1610): forgets any earlier visit, so the journey starts
	 * from the record on file rather than from a draft an earlier visit
	 * left behind -- the same reason intake's door clears its draft.
	 */
	open(clientId: string, engagementId: string | undefined): void {
		this.clientId = clientId;
		this.#readingFor = '';
		this.read.reset();
		this.engagementId = engagementId;
		this.draft.scope = clientId;
		this.draft.clear();
	}

	/**
	 * Reads her record and opens the draft from it. A second call for the
	 * same Client is a no-op once it is under way, so a layout effect that
	 * re-runs on each step costs one comparison rather than a round trip.
	 *
	 * A read for one Client that answers after the journey has moved on to
	 * another -- `open` or `load` for someone else -- is dropped, record
	 * and draft alike: the latest read wins (`PageRead`).
	 */
	async load(fetcher: Fetcher, practiceId: string, clientId: string): Promise<void> {
		if (this.clientId === clientId && (this.read.isLoaded || this.#readingFor === clientId)) return;
		this.clientId = clientId;
		this.#readingFor = clientId;
		const isCurrent = await this.read.load(() => loadClientDetail(fetcher, practiceId, clientId), 'Failed to load Client');
		if (this.#readingFor === clientId && this.read.state.status !== 'loading') this.#readingFor = '';
		const record = this.read.value;
		// A merged row carries only `{id, mergedInto}` (detail.go), and an
		// erased one only placeholders: neither is a record to start from.
		// The layout reads `record` and says why instead.
		if (isCurrent && record && canAddDetails(record)) this.draft.start(clientId, answersOnFile(record));
	}

	/**
	The record on file, once read.
	*/
	get record(): ClientDetail | undefined {
		return this.read.value;
	}
}

/**
The one details journey the `details` layout and its steps share.
*/
export const clientDetails = new ClientDetailsFlow();
