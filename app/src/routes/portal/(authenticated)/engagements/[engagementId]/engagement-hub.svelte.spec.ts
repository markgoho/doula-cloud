import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Hub from './+page.svelte';

// The hub reads `page.params.engagementId` for the fetch and its two Link
// hrefs, and `page.data.practiceName` for the bar -- `engagements/
// [engagementId]/+layout.ts`'s load result, mirrored the same way the
// authenticated layout's own spec stands it in.
const pageState = vi.hoisted(() => ({
	params: { engagementId: 'engagement-1' },
	data: { practiceName: 'Riverside Doula Collective' }
}));
vi.mock('$app/state', () => ({ page: pageState }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

function jsonResponse(body: unknown) {
	return { ok: true, json: () => Promise.resolve(body) } as Response;
}

describe('Client portal Engagement hub', () => {
	it("shows the due date under its own label, not 'Created' (#505)", async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				engagementId: 'engagement-1',
				practiceName: 'Riverside Doula Collective',
				clientName: 'Tasha Bell',
				status: 'active',
				dueDate: '2027-06-15'
			})
		);

		await render(Hub);

		await expect.element(page.getByText('Due date')).toBeVisible();
		await expect.element(page.getByText('Jun 15, 2027')).toBeVisible();
		await expect.element(page.getByText('Created')).not.toBeInTheDocument();
	});

	// ADR-0017: a postpartum-only Engagement has no due date. #505 asks for
	// nothing to show, not a blank row and not a placeholder -- the row is
	// left out of the DescriptionList's own items entirely, so there is no
	// "Due date" label sitting over an empty value.
	it('shows nothing for a null due date -- no blank label, no placeholder', async () => {
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				engagementId: 'engagement-1',
				practiceName: 'Riverside Doula Collective',
				clientName: 'Tasha Bell',
				status: 'active'
			})
		);

		await render(Hub);

		await expect.element(page.getByText('active')).toBeVisible();
		await expect.element(page.getByText(/due date/i)).not.toBeInTheDocument();
	});
});
