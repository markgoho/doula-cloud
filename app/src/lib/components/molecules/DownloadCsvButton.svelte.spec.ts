import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DownloadCsvButton from './DownloadCsvButton.svelte';

async function setup({ hasRows = true, isExporting = false }: { hasRows?: boolean; isExporting?: boolean } = {}) {
	const onDownload = vi.fn(async () => {});
	await render(DownloadCsvButton, { isExporting, hasRows, description: 'A spreadsheet of the 3 Contracts this list shows.', onDownload });
	return { onDownload };
}

describe('DownloadCsvButton', () => {
	it('is read with how many rows the file will hold, and downloads when pressed', async () => {
		const { onDownload } = await setup();

		const button = page.getByRole('button', { name: 'Download CSV' });
		await expect.element(button).toHaveAccessibleDescription('A spreadsheet of the 3 Contracts this list shows.');
		await button.click();

		expect(onDownload).toHaveBeenCalledOnce();
	});

	it('takes no margin when flush', async () => {
		await render(DownloadCsvButton, { isExporting: false, hasRows: true, description: 'x', onDownload: async () => {}, isFlush: true });

		const box = page.getByRole('button', { name: 'Download CSV' }).element().parentElement!;
		expect(getComputedStyle(box).marginBlockStart).toBe('0px');
	});

	it('stays in its place, unavailable, while there is nothing to download', async () => {
		await setup({ hasRows: false });

		await expect.element(page.getByRole('button', { name: 'Download CSV' })).toHaveAttribute('aria-disabled', 'true');
	});
});
