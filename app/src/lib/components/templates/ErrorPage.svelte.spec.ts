import { createRawSnippet } from 'svelte';
import type { ErrorKind } from '#lib/errorPage.js';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorPage from './ErrorPage.svelte';

interface SetupOptions {
	kind?: ErrorKind;
	wayOutHref?: string;
	wayOutLabel?: string;
}

async function setup({
	kind = 'notFound',
	wayOutHref = '/practices/practice-1',
	wayOutLabel = 'Go to your Practice overview'
}: SetupOptions = {}) {
	await render(ErrorPage, { kind, wayOutHref, wayOutLabel });
}

describe('ErrorPage.svelte', () => {
	it('renders distinct copy for a not-found page, and says trying again will not help', async () => {
		await setup({ kind: 'notFound' });

		await expect.element(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
		await expect.element(page.getByText('Trying again will not change that.')).toBeVisible();
	});

	it('renders distinct copy for a refused page, and says trying again will not help', async () => {
		await setup({ kind: 'refused' });

		await expect.element(page.getByRole('heading', { name: 'You cannot view this' })).toBeVisible();
		await expect.element(page.getByText('Trying again will not change that.')).toBeVisible();
	});

	it('renders distinct copy for a locked Practice, naming who can undo it rather than blaming a role', async () => {
		await setup({ kind: 'practiceLocked' });

		await expect.element(page.getByRole('heading', { name: 'This Practice is locked' })).toBeVisible();
		await expect.element(page.getByText('A Practice Owner can restore it')).toBeVisible();
	});

	// The one refusal state whose reader can act, so it is the one that
	// must not end by saying trying again is pointless (#918).
	it('renders distinct copy for a second-factor refusal, and names the step instead of saying trying again will not help', async () => {
		await setup({ kind: 'secondFactor' });

		await expect
			.element(page.getByRole('heading', { name: 'Set up a second sign-in factor' }))
			.toBeVisible();
		await expect.element(page.getByText('then try this page again')).toBeVisible();
		await expect
			.element(page.getByText('Trying again will not change that.'))
			.not.toBeInTheDocument();
	});

	// #1532: one act needs a second factor, not the whole Practice, so the
	// copy must not say the Practice's screens are shut.
	it('renders distinct copy for an act that needs a second factor, and names the step', async () => {
		await setup({ kind: 'actSecondFactor' });

		await expect
			.element(page.getByRole('heading', { name: 'This needs two-factor authentication' }))
			.toBeVisible();
		await expect.element(page.getByText('then try again')).toBeVisible();
		await expect.element(page.getByText('before its screens open')).not.toBeInTheDocument();
	});

	it('renders distinct copy for an unavailable page, and says trying again will help', async () => {
		await setup({ kind: 'unavailable' });

		await expect.element(page.getByRole('heading', { name: 'DoulaCloud is unavailable' })).toBeVisible();
		await expect.element(page.getByText('Try again shortly.')).toBeVisible();
	});

	it('renders distinct copy for a problem page, and says trying again will help', async () => {
		await setup({ kind: 'problem' });

		await expect.element(page.getByRole('heading', { name: 'Sorry, there is a problem' })).toBeVisible();
		await expect.element(page.getByText('Try again in a few minutes.')).toBeVisible();
	});

	// #2710: a page met in place of an error page brings its own words.
	it('renders a title and body the caller wrote, in place of a kind', async () => {
		await render(ErrorPage, {
			title: 'Your work has ended',
			body: createRawSnippet(() => ({ render: () => '<p>Your Attachment ended on Mar 1, 2027.</p>' })),
			wayOutHref: '/practices/practice-1/offers',
			wayOutLabel: 'Your offers'
		});

		await expect.element(page.getByRole('heading', { level: 1, name: 'Your work has ended' })).toBeVisible();
		await expect.element(page.getByText('Your Attachment ended on Mar 1, 2027.')).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Your offers' })).toBeVisible();
	});

	it('renders the given way-out link', async () => {
		await setup({ wayOutHref: '/portal/(signed-out)/login', wayOutLabel: 'Sign in' });

		await expect.element(page.getByRole('link', { name: 'Sign in' })).toBeVisible();
		expect(page.getByRole('link', { name: 'Sign in' }).element()).toHaveAttribute(
			'href',
			'/portal/(signed-out)/login'
		);
	});
});
