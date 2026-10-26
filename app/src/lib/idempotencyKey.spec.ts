import { describe, it, expect, vi, beforeEach } from 'vitest';

type Module = typeof import('./idempotencyKey');

// A fresh copy of the module for each test, so a key one test leaves
// pending never reaches the next one -- and so a test can stand for a
// reload of the page, where everything the module held is gone.
async function load(): Promise<Module> {
	vi.resetModules();
	return import('./idempotencyKey');
}

function keyOf(init: RequestInit): string | null {
	return new Headers(init.headers).get('Idempotency-Key');
}

// A Message with one attachment, the multipart body a portal compose sends.
function form(text: string): FormData {
	const data = new FormData();
	data.append('body', 'See attached');
	data.append('file', new File([text], 'scan.pdf', { type: 'application/pdf', lastModified: 1 }));
	return data;
}

const json = (body: unknown): RequestInit => ({
	method: 'POST',
	headers: { 'Content-Type': 'application/json' },
	body: JSON.stringify(body)
});

describe('keyRequest', () => {
	let m: Module;
	beforeEach(async () => {
		m = await load();
	});

	it('gives a POST an Idempotency-Key and keeps the headers it already had', () => {
		const { init } = m.keyRequest('/api/portal/engagements/e-1/messages', json({ body: 'Hello' }));

		expect(keyOf(init)).toMatch(/^[0-9a-f-]{36}$/);
		expect(new Headers(init.headers).get('Content-Type')).toBe('application/json');
	});

	it('leaves a request that is not a POST as it was', () => {
		const get = m.keyRequest('/api/portal/session', {});
		const put = m.keyRequest('/api/portal/engagements/e-1/notification-preference', { method: 'PUT', body: '{}' });

		expect(get.init).toEqual({});
		expect(keyOf(put.init)).toBeNull();
	});

	it('reuses the key when the same submit is sent again after a dropped connection', () => {
		const first = m.keyRequest('/api/practices/p-1/clients', json({ givenName: 'Ana' }));
		// The connection dropped: fetch threw, so the submit never settled.
		const retry = m.keyRequest('/api/practices/p-1/clients', json({ givenName: 'Ana' }));

		expect(keyOf(retry.init)).toBe(keyOf(first.init));
	});

	it('reuses the key when the same submit is sent again after a 5xx', () => {
		const first = m.keyRequest('/api/practices/p-1/clients', json({ givenName: 'Ana' }));
		first.settle(new Response(undefined, { status: 503 }));
		const retry = m.keyRequest('/api/practices/p-1/clients', json({ givenName: 'Ana' }));

		expect(keyOf(retry.init)).toBe(keyOf(first.init));
	});

	it('gives a new submit a new key once the last one settled', () => {
		const first = m.keyRequest('/api/practices/p-1/clients', json({ givenName: 'Ana' }));
		first.settle(new Response(undefined, { status: 201 }));
		const next = m.keyRequest('/api/practices/p-1/clients', json({ givenName: 'Ana' }));

		expect(keyOf(next.init)).not.toBe(keyOf(first.init));
	});

	it('treats a 4xx as settled, so the corrected submit gets a new key', () => {
		const first = m.keyRequest('/api/practices/p-1/clients', json({ givenName: '' }));
		first.settle(new Response(undefined, { status: 400 }));
		const again = m.keyRequest('/api/practices/p-1/clients', json({ givenName: '' }));

		expect(keyOf(again.init)).not.toBe(keyOf(first.init));
	});

	it('gives a different body, or a different path, its own key', () => {
		const ana = m.keyRequest('/api/practices/p-1/clients', json({ givenName: 'Ana' }));
		const bea = m.keyRequest('/api/practices/p-1/clients', json({ givenName: 'Bea' }));
		const elsewhere = m.keyRequest('/api/practices/p-2/clients', json({ givenName: 'Ana' }));

		expect(keyOf(bea.init)).not.toBe(keyOf(ana.init));
		expect(keyOf(elsewhere.init)).not.toBe(keyOf(ana.init));
	});

	it('reuses the key for the same multipart submit, a file read by its name, size, type and date', () => {
		const path = '/api/portal/engagements/e-1/messages';
		const first = m.keyRequest(path, { method: 'POST', body: form('a') });
		const retry = m.keyRequest(path, { method: 'POST', body: form('b') });
		const other = m.keyRequest(path, { method: 'POST', body: form('longer') });

		expect(keyOf(retry.init)).toBe(keyOf(first.init));
		expect(keyOf(other.init)).not.toBe(keyOf(first.init));
	});

	it('reads a POST with no body, and a URLSearchParams body, as a submit it can match', () => {
		const empty = m.keyRequest('/api/practices/p-1/engagements/e-1/portal-invite', { method: 'post' });
		const emptyAgain = m.keyRequest('/api/practices/p-1/engagements/e-1/portal-invite', { method: 'post' });
		const form = m.keyRequest('/api/x', { method: 'POST', body: new URLSearchParams({ a: '1' }) });
		const formAgain = m.keyRequest('/api/x', { method: 'POST', body: new URLSearchParams({ a: '1' }) });

		expect(keyOf(emptyAgain.init)).toBe(keyOf(empty.init));
		expect(keyOf(formAgain.init)).toBe(keyOf(form.init));
	});

	it('gives a body it cannot read a fresh key every time, never a guess', () => {
		const first = m.keyRequest('/api/x', { method: 'POST', body: new Blob(['raw']) });
		first.settle(new Response(undefined, { status: 503 }));
		const second = m.keyRequest('/api/x', { method: 'POST', body: new Blob(['raw']) });

		expect(keyOf(second.init)).not.toBe(keyOf(first.init));
	});
});

describe('a key the caller already made (#1933)', () => {
	it('is sent as given, and never replaced or remembered by the seam', async () => {
		const m = await load();
		const key = m.newIdempotencyKey();
		const queued = { method: 'POST', headers: { [m.IDEMPOTENCY_KEY_HEADER]: key }, body: '{"body":"Hi"}' };

		const sent = m.keyRequest('/api/portal/engagements/e-1/messages', queued);
		sent.settle(new Response(undefined, { status: 201 }));

		expect(sent.init).toBe(queued);
		expect(keyOf(sent.init)).toBe(key);
	});

	it('is the same key on every replay, across a reload of the page', async () => {
		// The write is made: its key is made with it and stored on the
		// device beside it.
		const before = await load();
		const stored = JSON.stringify({
			path: '/api/portal/engagements/e-1/messages',
			body: '{"body":"Sent with no signal"}',
			key: before.newIdempotencyKey()
		});

		// The page reloads: the module starts empty, and the queue reads the
		// write back and sends it -- twice, the second a retry.
		const after = await load();
		const write = JSON.parse(stored) as { path: string; body: string; key: string };
		const replay = () =>
			after.keyRequest(write.path, {
				method: 'POST',
				headers: { [after.IDEMPOTENCY_KEY_HEADER]: write.key },
				body: write.body
			});

		expect(keyOf(replay().init)).toBe(write.key);
		expect(keyOf(replay().init)).toBe(write.key);
	});

	it('is a fresh value each time one is made', async () => {
		const m = await load();

		expect(m.newIdempotencyKey()).not.toBe(m.newIdempotencyKey());
	});
});
