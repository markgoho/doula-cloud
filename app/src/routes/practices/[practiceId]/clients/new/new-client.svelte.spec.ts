import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({ page: { params: { practiceId: 'practice-1' } } }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api.js', () => ({ apiFetchWithSession: vi.fn() }));
vi.mock('#lib/client.js', () => ({ createClient: vi.fn() }));

describe('practices/[practiceId]/clients/new -- whose data this is (#469)', () => {
	it('never offers the signed-in doula her own stored details for a Client', async () => {
		await render(Page, {});

		await expect
			.element(testPage.getByLabelText('Their name'))
			.toHaveAttribute('autocomplete', 'off');
		await expect
			.element(testPage.getByLabelText('Their email'))
			.toHaveAttribute('autocomplete', 'off');
	});
});
