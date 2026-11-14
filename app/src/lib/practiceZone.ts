import { page } from './appState.svelte.js';
import { readerTimeZone, zoneNote } from './zonedTime.js';

/**
 * The zone a Staff screen reads every time in (ADR-0036's 2026-11-13
 * amendment): the Practice's, which the Practice session carries.
 */
export function practiceTimeZone(): string {
	return (page.data as { session: { timezone: string } }).session.timezone;
}

/**
 * The line that says so, for a reader whose own zone is another, and nothing
 * for one in the Practice's.
 */
export function practiceZoneNote(now: Date = new Date()): string | undefined {
	return zoneNote(practiceTimeZone(), readerTimeZone(), now);
}
