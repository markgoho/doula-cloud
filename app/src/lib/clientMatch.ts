/**
 * The one line that tells two Clients with one name apart (#2143): when
 * each record was added, how many Engagements it has, and when the Client
 * was last cared for. Find a Client prints it under each match, and "Is
 * this the same person?" under each existing record, so both screens say
 * it in the same words.
 *
 * A date of birth, an email and a phone were what those screens printed
 * before, and on a record that holds none of them they printed three
 * dashes. When the record was added and when the Practice last cared for
 * the Client are on every record, so the line never goes blank.
 *
 * Dates read through `dates.ts` like every other date the app prints
 * (ADR-0022's month-day order). The old drawing's "June 2026" had no
 * formatter behind it, and a day is no harder to read than a month.
 */
import type { ClientMatch } from './client.js';
import { formatInstant } from './dates.js';

const COMPLETED = 'completed';

export function matchHistory(match: Pick<ClientMatch, 'createdAt' | 'engagements'>): string {
	const added = `Added ${formatInstant(match.createdAt)}`;
	const { engagements } = match;
	if (engagements.length === 0) return `${added} · No Engagements yet`;

	const count = engagements.length === 1 ? '1 Engagement' : `${engagements.length} Engagements`;
	const openCount = engagements.filter((engagement) => engagement.status !== COMPLETED).length;
	if (openCount > 0) {
		// "1 Engagement · open now" rather than "1 open now" of one.
		const open = engagements.length === 1 ? 'open now' : `${openCount} open now`;
		return `${added} · ${count} · ${open}`;
	}

	// Every one has ended. The latest end is when the Client was last
	// cared for; a completion the ledger holds no date for (only a seeded
	// row) is left out rather than guessed.
	// Compared as instants, not strings: two RFC 3339 values in different
	// offsets do not sort by their text.
	let lastEnded: string | undefined;
	for (const { endedAt } of engagements) {
		if (endedAt !== undefined && (lastEnded === undefined || Date.parse(endedAt) > Date.parse(lastEnded))) {
			lastEnded = endedAt;
		}
	}
	const isOne = engagements.length === 1;
	if (lastEnded === undefined) return `${added} · ${count} · ${isOne ? 'ended' : 'all ended'}`;
	return `${added} · ${count} · ${isOne ? 'ended' : 'most recent ended'} ${formatInstant(lastEnded)}`;
}
