import { createRawSnippet } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import Layout from './+layout.svelte';
import NamePage from './name/+page.svelte';
import { practiceId, seedIntake } from './intakeFixture.js';

/*
 * The layout every intake question sits in (#1609): it puts the
 * contractor Doula's door in front of intake, and it reads which screen
 * opened intake so the name question's Back can go there. What the
 * questions themselves do is `intake-sequence.svelte.spec.ts`'s.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

interface SetupOptions {
	isContractor?: boolean;
	search?: string;
}

async function setup({ isContractor = false, search = '' }: SetupOptions = {}) {
	Object.assign(pageState, {
		params: { practiceId },
		url: new URL(`https://example.test/practices/${practiceId}/clients/new/name${search}`)
	});
	await render(Layout, {
		// The generated `data` merges the ancestor layout's `session` (#835)
		// with this layout's own gate, the way SvelteKit does at runtime.
		data: {
			isContractor,
			session: { practiceId, staffId: 'staff-1', practiceName: 'Test Practice', roles: [], isContractor }
		},
		children: createRawSnippet(() => ({ render: () => '<p>the name question</p>' }))
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	seedIntake();
});

describe('intake for a contractor Doula (#1609)', () => {
	it('shows the door that explains, not a question that would end in a 403', async () => {
		await setup({ isContractor: true });

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Add a Client' })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Set up a Practice' })).toBeVisible();
		await expect.element(testPage.getByText('the name question')).not.toBeInTheDocument();
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});
});

describe('intake for everyone else', () => {
	// #1611: the name question is the same at every Practice, so nothing
	// is read before it is asked.
	it('shows the question without a read of its own', async () => {
		await setup();

		await expect.element(testPage.getByText('the name question')).toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('seeds the draft with what the search carried', async () => {
		// The door (`+page.ts`) clears the draft before the question opens.
		intakeDraft.clear();

		await setup({ search: '?name=Ana&email=ana%40example.com&dateOfBirth=1990-01-02&phone=' });

		await expect.element(testPage.getByText('the name question')).toBeVisible();
		expect(intakeDraft.answers).toMatchObject({
			givenName: 'Ana',
			email: 'ana@example.com',
			dateOfBirth: '1990-01-02',
			phone: ''
		});
	});

	/*
	 * #1716: a name is never split on a space, since GOV.UK (ADR-0021)
	 * says not to guess a name's structure. One word carries into Given
	 * name; two or more carry into neither field. The name question is
	 * mounted after the layout, so these read the fields she sees.
	 */
	it('carries a one-word name into Given name', async () => {
		intakeDraft.clear();

		await setup({ search: '?name=Yar' });
		await render(NamePage);

		await expect.element(testPage.getByLabelText('Given name')).toHaveValue('Yar');
		await expect.element(testPage.getByLabelText('Family name (optional)')).toHaveValue('');
	});

	it('carries a two-word name into neither name field, and still carries the rest', async () => {
		intakeDraft.clear();

		await setup({ search: '?name=Yar%20Pell&email=yar%40example.com' });
		await render(NamePage);

		await expect.element(testPage.getByLabelText('Given name')).toHaveValue('');
		await expect.element(testPage.getByLabelText('Family name (optional)')).toHaveValue('');
		expect(intakeDraft.answers).toMatchObject({ givenName: '', familyName: '', email: 'yar@example.com' });
	});

	it('reads the screen that opened intake directly', async () => {
		await setup({ search: '?from=overview' });

		await expect.element(testPage.getByText('the name question')).toBeVisible();
		expect(intakeDraft.origin).toBe('overview');
	});

	// `from` with a value that names no screen is not read: the origin
	// already read is kept, so Back still goes where it went.
	it('keeps the origin it already read when the URL names none', async () => {
		intakeDraft.origin = 'clients';

		await setup({ search: '?from=check' });

		await expect.element(testPage.getByText('the name question')).toBeVisible();
		expect(intakeDraft.origin).toBe('clients');
	});
});
