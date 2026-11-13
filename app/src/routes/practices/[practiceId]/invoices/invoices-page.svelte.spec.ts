import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { StreamedRead } from '#lib/streamedRead.svelte.js';
import type { PracticeInvoiceListData } from '#lib/invoice.js';
import Page from './+page.svelte';

const pageState = vi.hoisted(() => ({
	params: { practiceId: 'practice-1' } as Record<string, string>,
	url: new URL('https://example.test/practices/practice-1/invoices'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const invalidate = vi.hoisted(() => vi.fn(async () => {}));
vi.mock('$app/navigation', () => ({ invalidate, goto: vi.fn() }));

vi.mock('#lib/api.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.js')>()),
	didFollowSessionRefusal: vi.fn(async () => false)
}));
vi.mock('./InvoicesScreen.svelte', async () => import('#lib/TestRetry.svelte'));

function failedWith(status: number): Promise<StreamedRead<PracticeInvoiceListData>> {
	return Promise.resolve({ isLoaded: false, response: new Response('', { status }) });
}

async function setupPage(status: number) {
	return render(Page, { params: { practiceId: 'practice-1' }, data: { read: failedWith(status) } } as never);
}

// T8 (#2746): a failure trying again might cure stays inside the screen.
describe('the Invoices page when its read fails', () => {
	it.each([500, 503])('hands a %i to the screen, whose Try again runs the read again', async (status) => {
		await setupPage(status);

		const retry = testPage.getByRole('button', { name: 'Retry' });
		await expect.element(testPage.getByText('Read failed')).toHaveAttribute('title', 'true');
		await retry.click();
		await vi.waitFor(() => expect(invalidate).toHaveBeenCalledWith('app:invoices'));
	});

	it('draws the refusal page for a 403, which trying again cannot cure', async () => {
		await setupPage(403);

		await expect.element(testPage.getByRole('heading', { name: 'You cannot view this' })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
	});
});
