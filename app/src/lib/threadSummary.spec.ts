import { describe, expect, it } from 'vitest';
import type { Message } from './components/organisms/MessageThread.svelte';
import { latestMessageText, unansweredSince } from './threadSummary.js';

function message(senderType: Message['senderType'], createdAt: string, body = 'Hello'): Message {
	return {
		messageId: createdAt,
		senderType,
		senderId: senderType,
		senderName: senderType === 'client' ? 'Camille Boyd' : 'Priya Raman',
		body,
		createdAt
	};
}

describe('latestMessageText', () => {
	it('quotes the newest message, its sender and its day', () => {
		const thread = [message('staff', '2026-10-20T15:00:00Z'), message('client', '2026-10-24T15:00:00Z', 'Thursday works.')];
		expect(latestMessageText(thread)).toBe('“Thursday works.” — Camille Boyd, Oct 24, 2026');
	});

	it('cuts a long message at a word, with an ellipsis', () => {
		const long = `${'word '.repeat(30)}end`;
		const text = latestMessageText([message('staff', '2026-10-24T15:00:00Z', long)])!;
		expect(text).toMatch(/^“(word ){15}word…” — Priya Raman, Oct 24, 2026$/);
	});

	it('cuts a long message with no space where it must', () => {
		const text = latestMessageText([message('staff', '2026-10-24T15:00:00Z', 'x'.repeat(100))])!;
		expect(text).toBe(`“${'x'.repeat(80)}…” — Priya Raman, Oct 24, 2026`);
	});

	it('names a file sent with no words', () => {
		const file = { ...message('client', '2026-10-24T15:00:00Z', ''), attachmentFilename: 'scan.pdf' };
		expect(latestMessageText([file])).toBe('scan.pdf — Camille Boyd, Oct 24, 2026');
	});

	it('reads nothing for an empty thread', () => {
		expect(latestMessageText([])).toBeUndefined();
	});
});

describe('unansweredSince', () => {
	it('is the first of the Client messages after the last Staff one', () => {
		const thread = [
			message('client', '2026-10-18T15:00:00Z'),
			message('staff', '2026-10-20T15:00:00Z'),
			message('client', '2026-10-22T15:00:00Z'),
			message('client', '2026-10-24T15:00:00Z')
		];
		expect(unansweredSince(thread)).toBe('2026-10-22T15:00:00Z');
	});

	it('is nothing once Staff has answered, or with no messages', () => {
		const answered = [message('client', '2026-10-22T15:00:00Z'), message('staff', '2026-10-24T15:00:00Z')];
		expect(unansweredSince(answered)).toBeUndefined();
		expect(unansweredSince([])).toBeUndefined();
	});
});
