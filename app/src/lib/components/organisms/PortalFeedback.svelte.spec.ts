import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { RefusalError } from '#lib/formErrors.js';
import PortalFeedback from './PortalFeedback.svelte';

const CONTROL = 'Tell us what is not working or what you need.';

type SetupOptions = Partial<ComponentProps<typeof PortalFeedback>>;

function setup({
	email = 'alex.rivera@example.com',
	onSend = vi.fn().mockResolvedValue(undefined),
	...rest
}: SetupOptions = {}) {
	return render(PortalFeedback, { email, onSend, ...rest });
}

async function openDrawer() {
	await page.getByRole('button', { name: CONTROL }).click();
}

describe('PortalFeedback', () => {
	it("shows the Portal banner's sentence and control", async () => {
		await setup();

		await expect.element(page.getByText('This care portal is new.', { exact: false })).toBeVisible();
		await expect.element(page.getByRole('button', { name: CONTROL })).toBeVisible();
	});

	it('opens the drawer to its heading and, with a Practice known, its intro and destination', async () => {
		await setup({ practiceName: 'Finger Lakes Birth Collective' });

		await openDrawer();

		await expect.element(page.getByRole('dialog', { name: 'Send feedback about this portal' })).toBeVisible();
		await expect
			.element(
				page.getByText(
					'Finger Lakes Birth Collective uses Doula Cloud to run this portal. The Doula Cloud team reads every piece of feedback.'
				)
			)
			.toBeVisible();
		await expect
			.element(
				page.getByText(
					'This goes to the Doula Cloud team, not to Finger Lakes Birth Collective. For anything about your care, message your doula.'
				)
			)
			.toBeVisible();
	});

	it('names "your doula\'s Practice" when the screen has no single Practice to name', async () => {
		await setup();

		await openDrawer();

		await expect
			.element(
				page.getByText(
					"Your doula's Practice uses Doula Cloud to run this portal. The Doula Cloud team reads every piece of feedback."
				)
			)
			.toBeVisible();
		await expect
			.element(
				page.getByText(
					"This goes to the Doula Cloud team, not to your doula's Practice. For anything about your care, message your doula."
				)
			)
			.toBeVisible();
	});

	it('lists "Client, {Practice name}" once a Practice is known', async () => {
		await setup({ practiceName: 'Finger Lakes Birth Collective' });

		await openDrawer();
		await page.getByText('What else we send with your feedback').click();

		await expect
			.element(page.getByText('your role and Practice: Client, Finger Lakes Birth Collective'))
			.toBeVisible();
	});

	it('omits the role line when there is no single Practice', async () => {
		await setup();

		await openDrawer();
		await page.getByText('What else we send with your feedback').click();

		expect(page.getByText('your role and Practice', { exact: false }).elements()).toHaveLength(0);
	});

	it('calls onSend with the form input, closes the drawer, and shows a focused Notice naming the sender', async () => {
		const onSend = vi.fn().mockResolvedValue(undefined);
		const { container } = await setup({ email: 'alex.rivera@example.com', onSend });

		await openDrawer();
		await page.getByLabelText('Something is not working').click();
		await page.getByRole('button', { name: 'Send feedback' }).click();

		await expect.poll(() => onSend.mock.calls.length).toBe(1);
		expect(onSend.mock.calls[0][0]).toMatchObject({ kind: 'not_working', text: '' });

		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
		const message =
			'Feedback sent. Thank you. If a reply would help, the Doula Cloud team will email you at alex.rivera@example.com.';
		await expect.element(page.getByText(message)).toBeVisible();
		// The focused element is the Notice's own wrapper (`.notice`,
		// tabindex="-1"), not the <p role="status"> text inside it -- no
		// accessible role names that wrapper, so this is the querySelector
		// exception for a deliberately non-accessible element, matching
		// StaffFeedback's own spec.
		expect(document.activeElement).toBe(container.querySelector('.notice'));
	});

	it('keeps the drawer open and shows the refusal when the send fails', async () => {
		const onSend = vi
			.fn()
			.mockRejectedValue(new RefusalError('There is a problem with the service. Try again in a few minutes.'));
		await setup({ onSend });

		await openDrawer();
		await page.getByLabelText('An idea or a request').click();
		await page.getByLabelText('Tell us more').fill('Kept on a refusal.');
		await page.getByRole('button', { name: 'Send feedback' }).click();

		await expect
			.element(page.getByText('There is a problem with the service. Try again in a few minutes.'))
			.toBeVisible();
		await expect.element(page.getByRole('dialog', { name: 'Send feedback about this portal' })).toBeVisible();
		await expect.element(page.getByLabelText('Tell us more')).toHaveValue('Kept on a refusal.');
	});
});
