import type { ComponentProps } from 'svelte';
import { page as browserPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { RefusalError } from '#lib/formErrors.js';
import { browserName } from '#lib/feedback.js';
import FeedbackForm from './FeedbackForm.svelte';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/practices/practice-1/clients'),
	data: {} as Record<string, unknown>,
	route: { id: '/practices/[practiceId]/clients' } as { id: string | null }
}));
vi.mock('$app/state', () => ({ page: pageState }));

type SetupOptions = Partial<ComponentProps<typeof FeedbackForm>>;

function setup({
	legend = 'What kind of feedback is it?',
	errorKind = 'Select what kind of feedback it is',
	textLabel = 'Tell us more',
	textHint = 'What were you trying to do, and what happened?',
	destination = 'This goes to the Doula Cloud team, not to your Practice.',
	detailsSummary = 'What else we send with your feedback',
	onSend = vi.fn().mockResolvedValue(undefined),
	onSent = vi.fn(),
	...rest
}: SetupOptions = {}) {
	return render(FeedbackForm, {
		legend,
		errorKind,
		textLabel,
		textHint,
		destination,
		detailsSummary,
		onSend,
		onSent,
		...rest
	});
}

beforeEach(() => {
	pageState.url = new URL('https://example.test/practices/practice-1/clients');
	pageState.route = { id: '/practices/[practiceId]/clients' };
});

describe('FeedbackForm', () => {
	it('shows the kind legend and all three radio choices', async () => {
		await setup({ legend: 'What kind of feedback is it?' });

		await expect.element(browserPage.getByText('What kind of feedback is it?')).toBeVisible();
		await expect.element(browserPage.getByLabelText('Something is not working')).toBeVisible();
		await expect.element(browserPage.getByLabelText('An idea or a request')).toBeVisible();
		await expect.element(browserPage.getByLabelText('Something else')).toBeVisible();
	});

	it('shows the text label, hint, destination line and details summary a caller passes', async () => {
		await setup({
			textLabel: 'Tell us more',
			textHint: 'What were you trying to do, and what happened?',
			destination: 'This goes to the Doula Cloud team, not to your Practice.',
			detailsSummary: 'What else we send with your feedback'
		});

		await expect.element(browserPage.getByLabelText('Tell us more')).toBeVisible();
		await expect.element(browserPage.getByText('What were you trying to do, and what happened?')).toBeVisible();
		await expect
			.element(browserPage.getByText('This goes to the Doula Cloud team, not to your Practice.'))
			.toBeVisible();
		await expect.element(browserPage.getByText('What else we send with your feedback')).toBeVisible();
	});

	it('lists the page, time, screen width and browser, and build in the disclosure, and omits the role line when none is given', async () => {
		await browserPage.viewport(1024, 800);
		await setup();

		await browserPage.getByText('What else we send with your feedback').click();

		await expect
			.element(browserPage.getByText('the page you were on: /practices/practice-1/clients'))
			.toBeVisible();
		await expect
			.element(browserPage.getByText(`your screen width and browser: 1024px, ${browserName(navigator.userAgent)}`))
			.toBeVisible();
		await expect.element(browserPage.getByText('the version of Doula Cloud: dev')).toBeVisible();
		expect(browserPage.getByText('your role and Practice', { exact: false }).elements()).toHaveLength(0);
	});

	it('lists the role and Practice line when one is given', async () => {
		await setup({ roleAndPractice: 'Owner, Finger Lakes Birth Collective' });

		await browserPage.getByText('What else we send with your feedback').click();

		await expect
			.element(browserPage.getByText('your role and Practice: Owner, Finger Lakes Birth Collective'))
			.toBeVisible();
	});

	it('refuses an empty submit with the error summary, and marks the radio group', async () => {
		const onSend = vi.fn();
		await setup({ onSend, errorKind: 'Select what kind of feedback it is' });

		await browserPage.getByRole('button', { name: 'Send feedback' }).click();

		await expect
			.element(browserPage.getByRole('heading', { level: 2, name: 'There is a problem' }))
			.toBeVisible();
		await expect(
			browserPage.getByText('Select what kind of feedback it is', { exact: false }).elements().length
		).toBeGreaterThan(0);
		expect(onSend).not.toHaveBeenCalled();
	});

	it('posts the kind, text, page, build and screen width once a kind is chosen', async () => {
		await browserPage.viewport(1280, 900);
		const onSend = vi.fn().mockResolvedValue(undefined);
		await setup({ onSend });

		await browserPage.getByLabelText('An idea or a request').click();
		await browserPage.getByLabelText('Tell us more').fill('Add a way to export invoices.');
		await browserPage.getByRole('button', { name: 'Send feedback' }).click();

		await expect.poll(() => onSend.mock.calls.length).toBe(1);
		expect(onSend).toHaveBeenCalledWith({
			kind: 'idea_or_request',
			text: 'Add a way to export invoices.',
			page: { url: '/practices/practice-1/clients', route: { id: '/practices/[practiceId]/clients' } },
			appBuild: 'dev',
			screenWidth: 1280
		});
	});

	it('posts an empty route id when $app/state matched no route', async () => {
		// eslint-disable-next-line unicorn/no-null -- $app/state's own route.id type, for the "no route matched" case
		pageState.route = { id: null };
		const onSend = vi.fn().mockResolvedValue(undefined);
		await setup({ onSend });

		await browserPage.getByLabelText('Something else').click();
		await browserPage.getByRole('button', { name: 'Send feedback' }).click();

		await expect.poll(() => onSend.mock.calls.length).toBe(1);
		expect(onSend.mock.calls[0][0].page.route).toEqual({ id: '' });
	});

	it('calls onSent and clears the draft once the send succeeds', async () => {
		const onSend = vi.fn().mockResolvedValue(undefined);
		const onSent = vi.fn();
		await setup({ onSend, onSent });

		await browserPage.getByLabelText('Something else').click();
		await browserPage.getByLabelText('Tell us more').fill('A note.');
		await browserPage.getByRole('button', { name: 'Send feedback' }).click();

		await expect.poll(() => onSent.mock.calls.length).toBe(1);
		await expect.element(browserPage.getByLabelText('Tell us more')).toHaveValue('');
	});

	it('shows an untargeted refusal in the drawer and keeps the drafted text', async () => {
		const onSend = vi.fn().mockRejectedValue(new RefusalError('There is a problem with the service.'));
		await setup({ onSend });

		await browserPage.getByLabelText('Something is not working').click();
		await browserPage.getByLabelText('Tell us more').fill('Kept on a refusal.');
		await browserPage.getByRole('button', { name: 'Send feedback' }).click();

		await expect.element(browserPage.getByText('There is a problem with the service.')).toBeVisible();
		await expect.element(browserPage.getByLabelText('Tell us more')).toHaveValue('Kept on a refusal.');
	});

	it('targets the text field when the refusal names it', async () => {
		const onSend = vi
			.fn()
			.mockRejectedValue(
				new RefusalError('Shorten the feedback to 5,000 characters or less', {
					text: 'Shorten the feedback to 5,000 characters or less'
				})
			);
		await setup({ onSend });

		await browserPage.getByLabelText('Something is not working').click();
		await browserPage.getByRole('button', { name: 'Send feedback' }).click();

		await expect
			.element(browserPage.getByText('Shorten the feedback to 5,000 characters or less').first())
			.toBeVisible();
		await expect.element(browserPage.getByLabelText('Tell us more')).toHaveAttribute('aria-invalid', 'true');
	});

	it('uses the submitLabel a caller passes', async () => {
		await setup({ submitLabel: 'Send' });

		await expect.element(browserPage.getByRole('button', { name: 'Send' })).toBeVisible();
	});
});
