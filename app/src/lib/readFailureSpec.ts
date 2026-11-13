import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Component } from 'svelte';
import type { StreamedRead } from '#lib/streamedRead.svelte.js';

const pageState = {
	params: { practiceId: 'practice-1' } as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
};
const invalidate = vi.fn(async () => {});

export const stateModule = { page: pageState };
export const navigationModule = { invalidate, goto: vi.fn() };
export async function apiModule(importOriginal: () => Promise<typeof import('#lib/api.js')>) {
	return { ...(await importOriginal()), didFollowSessionRefusal: vi.fn(async () => false) };
}

interface ReadFailureSpec {
	title: string;
	ticket: string;
	path: string;
	loadPage: () => Promise<{ default: Component<{ params: { practiceId: string }; data: never }> }>;
	invalidates: string;
}

function failedWith(status: number): Promise<StreamedRead<unknown>> {
	return Promise.resolve({ isLoaded: false, response: new Response('', { status }) });
}

// For a route page whose screen the spec mocks to TestRetry: a failure trying
// again might cure stays inside the screen, and a refusal draws the refusal page.
export function describeReadFailure({ title, ticket, path, loadPage, invalidates }: ReadFailureSpec): void {
	async function setupPage(status: number) {
		pageState.url = new URL(`https://example.test/practices/practice-1/${path}`);
		const { default: Page } = await loadPage();
		return render(Page, { params: { practiceId: 'practice-1' }, data: { read: failedWith(status) } } as never);
	}

	describe(`${title} when its read fails`, () => {
		it.each([500, 503])(`hands a %i to the screen, whose Try again runs the read again (T8, ${ticket})`, async (status) => {
			await setupPage(status);

			const retry = testPage.getByRole('button', { name: 'Retry' });
			await expect.element(testPage.getByText('Read failed')).toHaveAttribute('title', 'true');
			await retry.click();
			await vi.waitFor(() => expect(invalidate).toHaveBeenCalledWith(invalidates));
		});

		it('draws the refusal page for a 403, which trying again cannot cure', async () => {
			await setupPage(403);

			await expect.element(testPage.getByRole('heading', { name: 'You cannot view this' })).toBeVisible();
			await expect.element(testPage.getByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
		});
	});
}
