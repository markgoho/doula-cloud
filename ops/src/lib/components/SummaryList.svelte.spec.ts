import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import SummaryList from './SummaryList.svelte';

describe('SummaryList', () => {
	it('states each answer, with a Change button only where it can be changed', async () => {
		const onchange = vi.fn();
		await render(SummaryList, {
			rows: [
				{ key: 'Practice', value: 'Lakeside Birth Collective' },
				{ key: 'Credits', value: '2', onchange }
			]
		});

		await expect.element(page.getByText('Lakeside Birth Collective')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Change practice' })).not.toBeInTheDocument();
		await page.getByRole('button', { name: 'Change credits' }).click();
		expect(onchange).toHaveBeenCalledOnce();
	});
});
