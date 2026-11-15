import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DownloadCsvNotices from './DownloadCsvNotices.svelte';

describe('DownloadCsvNotices', () => {
	it('says a refusal as an error and a finished download as a status', async () => {
		await render(DownloadCsvNotices, { refusal: 'Your Contracts did not download.', downloaded: 'Downloaded contracts.csv, 2 Contracts.' });

		await expect.element(page.getByText('Your Contracts did not download.')).toBeVisible();
		await expect.element(page.getByText('Downloaded contracts.csv, 2 Contracts.')).toBeVisible();
	});

	it('draws nothing when there is nothing to say', async () => {
		await render(DownloadCsvNotices, { refusal: '', downloaded: '' });

		await expect.element(page.getByRole('status')).not.toBeInTheDocument();
		await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
	});
});
