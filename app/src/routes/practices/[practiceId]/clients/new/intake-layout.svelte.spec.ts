import { createRawSnippet } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { intakeDraft } from '#lib/intakeDraft.svelte.js';
import Layout from './+layout.svelte';
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

// The fixture's seed leaves the template already read for this Practice,
// so the layout renders its question without a fetch of its own.
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
	it('shows the question', async () => {
		await setup();

		await expect.element(testPage.getByText('the name question')).toBeVisible();
	});

	it('reads the screen that opened intake directly', async () => {
		await setup({ search: '?from=overview' });

		await expect.element(testPage.getByText('the name question')).toBeVisible();
		expect(intakeDraft.origin).toBe('overview');
	});

	// A Change round trip carries `from=check`, which names no screen: the
	// origin already read is kept, so Back still goes where it went.
	it('keeps the origin it already read when the URL names none', async () => {
		intakeDraft.origin = 'clients';

		await setup({ search: '?from=check' });

		await expect.element(testPage.getByText('the name question')).toBeVisible();
		expect(intakeDraft.origin).toBe('clients');
	});
});
