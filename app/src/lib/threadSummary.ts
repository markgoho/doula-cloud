/*
 * The two facts the Staff Engagement page's Messages section reads ahead
 * of the thread (#2104), derived from the thread the page already holds
 * (newest last, the order MessageThread draws it in).
 */
import type { Message } from './components/organisms/MessageThread.svelte';
import { formatInstant } from './dates.js';

/**
How much of the newest message the Latest row quotes before it cuts.
*/
const EXCERPT_LENGTH = 80;

function excerpt(body: string): string {
	const text = body.trim().replaceAll(/\s+/g, ' ');
	if (text.length <= EXCERPT_LENGTH) return text;
	const cut = text.slice(0, EXCERPT_LENGTH);
	const lastSpace = cut.lastIndexOf(' ');
	return `${(lastSpace > 0 ? cut.slice(0, lastSpace) : cut).trimEnd()}…`;
}

/**
 * The Latest row: the newest message quoted, who sent it, and the day --
 * "“Thursday works.” — Camille Boyd, Oct 24, 2026". A file sent with no
 * words reads as its file name. Undefined for an empty thread.
 */
export function latestMessageText(messages: readonly Message[]): string | undefined {
	const latest = messages.at(-1);
	if (!latest) return undefined;
	// A Message carries words, a file, or both (the BFF refuses neither),
	// so one with no words has a file.
	const what = latest.body ? `“${excerpt(latest.body)}”` : latest.attachmentFilename!;
	return `${what} — ${latest.senderName}, ${formatInstant(latest.createdAt)}`;
}

/**
 * Unanswered (GLOSSARY.md): the newest message is the Client's, and no
 * Staff message follows it. Answers the instant the run of unanswered
 * Client messages began -- the first one after the last Staff message --
 * or undefined when the thread is answered or empty. Only the messages
 * the page has loaded are read, so a run longer than the first page
 * reports the oldest loaded one.
 */
export function unansweredSince(messages: readonly Message[]): string | undefined {
	let since: string | undefined;
	for (let index = messages.length - 1; index >= 0 && messages[index]!.senderType === 'client'; index -= 1) {
		since = messages[index]!.createdAt;
	}
	return since;
}
