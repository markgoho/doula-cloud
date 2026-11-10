/* eslint-disable unicorn/no-null -- doula-ops answers a null openedAt for a gate still shut */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { SERVICE_PROBLEM } from '#lib/session.js';
import Page from './+page.svelte';

/*
 * The pilot gate's read and act, each ending somewhere on screen. Before
 * CheckRead and CheckAct, a read that threw or was refused drew no heading,
 * and an act whose request threw stopped its spinner and said nothing.
 */
const appPage = vi.hoisted(() => ({ data: { operator: { name: 'Mark Goho', email: 'mark@doula.cloud' } } }));
vi.mock('$app/state', () => ({ page: appPage }));
vi.mock('$app/paths', () => ({ resolve: (route: string) => route }));
const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));
vi.mock('#lib/firebase.js', () => ({ freshIdToken: vi.fn(async () => 'fresh-token') }));

const fetcher = vi.fn<typeof fetch>();

beforeEach(() => {
	fetcher.mockReset();
	vi.stubGlobal('fetch', fetcher);
});

describe('the pilot gate', () => {
	it('draws its heading and the problem when the read throws', async () => {
		fetcher.mockRejectedValueOnce(new TypeError('Failed to fetch'));

		await render(Page);

		await expect.element(page.getByRole('heading', { level: 1, name: 'Pilot gate' })).toBeVisible();
		await expect.element(page.getByText(SERVICE_PROBLEM)).toBeVisible();
	});

	it('draws its heading and the refusal when the read is refused', async () => {
		fetcher.mockResolvedValueOnce(Response.json({ code: 'INTERNAL', message: 'The gate could not be read.' }, { status: 409 }));

		await render(Page);

		await expect.element(page.getByRole('heading', { level: 1, name: 'Pilot gate' })).toBeVisible();
		await expect.element(page.getByText('The gate could not be read.')).toBeVisible();
	});

	it('says the service has a problem when the opening throws', async () => {
		fetcher
			.mockResolvedValueOnce(Response.json({ openedAt: null }))
			.mockResolvedValueOnce(new Response(undefined, { status: 204 }))
			.mockRejectedValueOnce(new TypeError('Failed to fetch'));
		await render(Page);
		await page.getByLabelText('Password').fill('pw');
		await page.getByLabelText('Authenticator app code').fill('123456');

		await page.getByRole('button', { name: 'Open the pilot gate' }).click();

		await expect.element(page.getByText(SERVICE_PROBLEM)).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Check before you open the pilot gate' })).toBeVisible();
	});

	it('asks for the code again when doula-ops does', async () => {
		fetcher
			.mockResolvedValueOnce(Response.json({ openedAt: null }))
			.mockResolvedValueOnce(new Response(undefined, { status: 204 }))
			.mockResolvedValueOnce(Response.json({ code: 'SECOND_FACTOR_REQUIRED', message: 'Enter a new code.' }, { status: 403 }));
		await render(Page);
		await page.getByLabelText('Password').fill('pw');
		await page.getByLabelText('Authenticator app code').fill('123456');

		await page.getByRole('button', { name: 'Open the pilot gate' }).click();

		await expect
			.element(page.getByRole('link', { name: 'Enter the code your authenticator app shows now to do this.' }))
			.toBeVisible();
	});

	it('says the gate is open once it opens', async () => {
		fetcher
			.mockResolvedValueOnce(Response.json({ openedAt: null }))
			.mockResolvedValueOnce(new Response(undefined, { status: 204 }))
			.mockResolvedValueOnce(Response.json({ openedAt: '2027-01-04T14:00:00Z' }));
		await render(Page);
		await page.getByLabelText('Password').fill('pw');
		await page.getByLabelText('Authenticator app code').fill('123456');

		await page.getByRole('button', { name: 'Open the pilot gate' }).click();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Pilot gate open' })).toBeVisible();
	});
});
