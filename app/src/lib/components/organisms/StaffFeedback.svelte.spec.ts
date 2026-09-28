import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { RefusalError } from '#lib/formErrors.js';
import StaffFeedback from './StaffFeedback.svelte';

const CONTROL = 'Tell us what is not working or what you need.';

type SetupOptions = Partial<ComponentProps<typeof StaffFeedback>>;

function setup({
	email = 'jordan@fingerlakesbirth.example',
	onSend = vi.fn().mockResolvedValue(undefined),
	...rest
}: SetupOptions = {}) {
	return render(StaffFeedback, { email, onSend, ...rest });
}

async function openDrawer() {
	await page.getByRole('button', { name: CONTROL }).click();
}

describe('StaffFeedback', () => {
	it("shows the Staff banner's sentence and control", async () => {
		await setup();

		await expect
			.element(page.getByText('Doula Cloud is new, and you are one of the first to use it.', { exact: false }))
			.toBeVisible();
		await expect.element(page.getByRole('button', { name: CONTROL })).toBeVisible();
	});

	it('opens the drawer to its heading and intro when the banner control is clicked', async () => {
		await setup();

		await openDrawer();

		await expect.element(page.getByRole('dialog', { name: 'Send feedback to Doula Cloud' })).toBeVisible();
		await expect
			.element(
				page.getByText(
					'The Doula Cloud team reads every piece of feedback during the pilot. It is how we decide what to fix first.'
				)
			)
			.toBeVisible();
	});

	it('lists the role and Practice line once both are given', async () => {
		await setup({ practiceName: 'Finger Lakes Birth Collective', roles: ['owner'] });

		await openDrawer();
		await page.getByText('What else we send with your feedback').click();

		await expect
			.element(page.getByText('your role and Practice: Owner, Finger Lakes Birth Collective'))
			.toBeVisible();
	});

	it('omits the role line under /account, where no Practice is given', async () => {
		await setup();

		await openDrawer();
		await page.getByText('What else we send with your feedback').click();

		expect(page.getByText('your role and Practice', { exact: false }).elements()).toHaveLength(0);
	});

	it('calls onSend with the form input, closes the drawer, and shows a focused Notice naming the sender', async () => {
		const onSend = vi.fn().mockResolvedValue(undefined);
		const { container } = await setup({ email: 'jordan@fingerlakesbirth.example', onSend });

		await openDrawer();
		await page.getByLabelText('Something is not working').click();
		await page.getByRole('button', { name: 'Send feedback' }).click();

		await expect.poll(() => onSend.mock.calls.length).toBe(1);
		expect(onSend.mock.calls[0][0]).toMatchObject({ kind: 'not_working', text: '' });

		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
		const message =
			'Feedback sent. Thank you. If a reply would help, Mark Goho, who builds Doula Cloud, will email you at jordan@fingerlakesbirth.example.';
		await expect.element(page.getByText(message)).toBeVisible();
		// The focused element is the Notice's own wrapper (`.notice`,
		// tabindex="-1"), not the <p role="status"> text inside it -- no
		// accessible role names that wrapper, so this is the querySelector
		// exception for a deliberately non-accessible element, not a
		// shortcut past an accessible query.
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
		await expect.element(page.getByRole('dialog', { name: 'Send feedback to Doula Cloud' })).toBeVisible();
		await expect.element(page.getByLabelText('Tell us more')).toHaveValue('Kept on a refusal.');
	});
});
