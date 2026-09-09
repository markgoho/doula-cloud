import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { CLOUD_RUN_SERVICE_DESCRIPTION } from '#lib/costBreakdown.js';
import type { DashboardData } from '#lib/dashboard.js';
import Page from './+page.svelte';

const loadDashboard = vi.hoisted(() => vi.fn());
vi.mock('#lib/dashboard.js', () => ({ loadDashboard }));

const data: DashboardData = {
	breakdown: {
		total: 6.5,
		services: [
			{
				service: CLOUD_RUN_SERVICE_DESCRIPTION,
				cost: 6.5,
				share: 1,
				skus: [{ sku: 'CPU Allocation Time', cost: 6.5 }],
				usageDetailAvailable: true
			}
		],
		costsThrough: '2026-09-07T00:00:00.000Z'
	},
	usage: {
		since: '2026-09-01T00:00:00.000Z',
		through: '2026-09-08T04:30:00.000Z',
		cloudRun: {
			billableInstanceTime: 620_750,
			cpuAllocationTime: 172,
			memoryAllocationTime: 86,
			requestCount: 4785
		},
		cloudSql: {},
		cloudStorage: {},
		firestore: {},
		firebaseHosting: {}
	}
};

describe('the GCP spend dashboard page', () => {
	beforeEach(() => {
		loadDashboard.mockReset();
	});

	it('reads once as it mounts, without anyone pressing sync', async () => {
		loadDashboard.mockResolvedValue(data);
		await render(Page, {});

		await expect
			.element(testPage.getByRole('complementary').getByText('$6.50'))
			.toBeVisible();
		expect(loadDashboard).toHaveBeenCalledTimes(1);
	});

	it('reads again when the button is pressed, and only then', async () => {
		loadDashboard.mockResolvedValue(data);
		await render(Page, {});
		await expect
			.element(testPage.getByRole('button', { name: 'Sync again' }))
			.toBeVisible();
		expect(loadDashboard).toHaveBeenCalledTimes(1);

		await testPage.getByRole('button', { name: 'Sync again' }).click();

		await expect
			.element(testPage.getByRole('button', { name: 'Sync again' }))
			.toBeVisible();
		expect(loadDashboard).toHaveBeenCalledTimes(2);
	});

	it('shows the mount read in progress', async () => {
		const pending = Promise.withResolvers<DashboardData>();
		loadDashboard.mockReturnValue(pending.promise);
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Syncing…' })).toBeVisible();
		await expect
			.element(testPage.getByText('Reading the billing export…'))
			.toBeVisible();

		pending.resolve(data);
	});

	it('shows the breakdown and usage a successful sync produced', async () => {
		loadDashboard.mockResolvedValue(data);
		await render(Page, {});

		await expect
			.element(testPage.getByRole('complementary').getByText('$6.50'))
			.toBeVisible();
		const service = testPage.getByText(CLOUD_RUN_SERVICE_DESCRIPTION, { exact: true });
		await expect.element(service).toBeVisible();

		await service.click();
		await expect.element(testPage.getByText('CPU Allocation Time')).toBeVisible();

		await expect
			.element(testPage.getByRole('heading', { name: 'Cloud Run usage' }))
			.toBeVisible();
		await expect.element(testPage.getByText('4.79K')).toBeVisible();
	});

	it('shows why the mount read failed', async () => {
		loadDashboard.mockRejectedValue(new Error('query timed out'));
		await render(Page, {});

		const alert = testPage.getByRole('alert');
		await expect.element(alert).toBeVisible();
		await expect.element(testPage.getByText('query timed out', { exact: false })).toBeVisible();
	});
});
