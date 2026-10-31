import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { CONNECTION_PROBLEM } from '#lib/formErrors.js';
import { jsonResponse } from '#lib/testResponse.js';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { fixture } from './page.fixture.js';
import Messages from './+page.svelte';
if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * #2210: what the Client's thread says after Send. The page and its
 * reads come from the route's own fixture, installed the way
 * `route-continuum.svelte.spec.ts` installs them.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const sent = {
	messageId: 'message-3',
	senderType: 'client',
	senderId: 'client-1',
	senderName: 'Persephone Ochieng-Whitfield',
	body: 'Did that go through?',
	createdAt: '2026-08-03T23:00:00Z'
};

interface SetupOptions {
	practiceName?: string;
	/**
	 * What a POST does: answers with the stored Message, or rejects as a
	 * dropped connection does.
	 */
	post?: 'stored' | 'unreachable';
}

async function setupMessages({ practiceName, post = 'stored' }: SetupOptions = {}) {
	Object.assign(pageState, toPageState(fixture));
	if (practiceName !== undefined) pageState.data = { ...pageState.data, practiceName };
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method !== 'POST') return respond(path);
		if (post === 'unreachable') return Promise.reject(new TypeError('Failed to fetch'));
		return Promise.resolve(jsonResponse(sent, 201));
	});
	await render(Messages);
	await page.getByLabelText('Message').fill(sent.body);
	await page.getByRole('button', { name: 'Send', exact: true }).click();
}

describe('The Client-portal message thread, after Send (#2210)', () => {
	it('says the message went to the Practice, where a reply shows, and that it is not for anything urgent', async () => {
		await setupMessages();

		await expect
			.element(page.getByRole('status'))
			.toHaveTextContent(
				'Message sent to Riverside Doula Collective. A reply will show here. Do not rely on messages for anything urgent.'
			);
		await expect.element(page.getByText(sent.body)).toBeVisible();
	});

	it('names "your doula" when the Practice name is not known', async () => {
		await setupMessages({ practiceName: '' });

		await expect.element(page.getByRole('status')).toHaveTextContent(/^Message sent to your doula\./);
	});

	it('says the service could not be reached, keeps the draft, and says nothing was sent, with no signal', async () => {
		await setupMessages({ post: 'unreachable' });

		await expect.element(page.getByRole('alert').getByText(CONNECTION_PROBLEM)).toBeVisible();
		await expect.element(page.getByLabelText('Message')).toHaveValue(sent.body);
		expect(page.getByRole('status').elements()).toHaveLength(0);
	});
});
