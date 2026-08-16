import { describe, expect, it } from 'vitest';
import {
	asPushMessage,
	notificationFor,
	parsePushPayload,
	PUSH_MESSAGE_TYPE,
	pushMessageFor,
	threadURLFor
} from './push.js';

describe('parsePushPayload', () => {
	it('parses an Engagement-only payload (Client recipient)', () => {
		expect(parsePushPayload('{"engagementId":"e1"}')).toEqual({ engagementId: 'e1' });
	});

	it('parses an Engagement+Practice payload (Staff recipient)', () => {
		expect(parsePushPayload('{"engagementId":"e1","practiceId":"p1"}')).toEqual({
			engagementId: 'e1',
			practiceId: 'p1'
		});
	});

	it('rejects malformed JSON', () => {
		expect(parsePushPayload('not json')).toBeUndefined();
	});

	it('rejects a JSON payload that is not an object', () => {
		expect(parsePushPayload('"just a string"')).toBeUndefined();
	});

	it('rejects a payload missing engagementId', () => {
		expect(parsePushPayload('{"practiceId":"p1"}')).toBeUndefined();
	});

	it('rejects a payload with an empty engagementId', () => {
		expect(parsePushPayload('{"engagementId":""}')).toBeUndefined();
	});

	it('rejects a payload with a non-string practiceId', () => {
		expect(parsePushPayload('{"engagementId":"e1","practiceId":42}')).toBeUndefined();
	});
});

describe('notificationFor', () => {
	it('builds a content-free notification carrying only the payload as data', () => {
		const { title, options } = notificationFor({ engagementId: 'e1', practiceId: 'p1' });
		expect(title).toBe('New message');
		expect(options.body).not.toContain('e1');
		expect(options.data).toEqual({ engagementId: 'e1', practiceId: 'p1' });
	});
});

describe('threadURLFor', () => {
	it('routes to the Practice-scoped Staff thread when practiceId is present', () => {
		expect(threadURLFor({ engagementId: 'e1', practiceId: 'p1' })).toBe(
			'/practices/p1/engagements/e1'
		);
	});

	it('routes to the Client-portal thread when practiceId is absent', () => {
		expect(threadURLFor({ engagementId: 'e1' })).toBe('/portal/engagements/e1');
	});
});

describe('pushMessageFor / asPushMessage', () => {
	it('round-trips a payload through pushMessageFor and asPushMessage', () => {
		const payload = { engagementId: 'e1', practiceId: 'p1' };
		const message = pushMessageFor(payload);
		expect(message).toEqual({ type: PUSH_MESSAGE_TYPE, payload });
		expect(asPushMessage(message)).toEqual(message);
	});

	it('rejects data with the wrong type tag', () => {
		expect(asPushMessage({ type: 'something-else', payload: { engagementId: 'e1' } })).toBeUndefined();
	});

	it('rejects data that is not an object', () => {
		expect(asPushMessage('not an object')).toBeUndefined();
		// MessageEvent.data can be the DOM value null, not just this module's own undefined sentinel.
		// eslint-disable-next-line unicorn/no-null
		expect(asPushMessage(null)).toBeUndefined();
	});

	it('rejects a message whose payload is not an object', () => {
		expect(asPushMessage({ type: PUSH_MESSAGE_TYPE, payload: 'nope' })).toBeUndefined();
	});

	it('rejects a message whose payload is missing engagementId', () => {
		expect(asPushMessage({ type: PUSH_MESSAGE_TYPE, payload: { practiceId: 'p1' } })).toBeUndefined();
	});
});
