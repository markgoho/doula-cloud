import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { expectRefusalKeepsDrawerOpen, sendNotWorkingFeedback } from '#testing/feedbackDrawer.js';
import StaffFeedback from './StaffFeedback.svelte';

const CONTROL = 'Tell us what you think.';

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
	it("shows the Staff banner's tag and control", async () => {
		await setup();

		await expect
			.element(page.getByText('Pilot', { exact: true }))
			.toBeVisible();
		await expect.element(page.getByRole('button', { name: CONTROL })).toBeVisible();
	});

	it('opens the drawer to its heading and intro when the banner control is clicked', async () => {
		await setup();

		await openDrawer();

		await expect.element(page.getByRole('dialog', { name: 'Send feedback to DoulaCloud' })).toBeVisible();
		await expect
			.element(
				page.getByText(
					'The DoulaCloud team reads every piece of feedback during the pilot. It is how we decide what to fix first.'
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

		await sendNotWorkingFeedback(openDrawer);

		await expect.poll(() => onSend.mock.calls.length).toBe(1);
		expect(onSend.mock.calls[0][0]).toMatchObject({ kind: 'not_working', text: '' });

		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
		const message =
			'Feedback sent. Thank you. If a reply would help, Mark Goho, who builds DoulaCloud, will email you at jordan@fingerlakesbirth.example.';
		await expect.element(page.getByText(message)).toBeVisible();
		// The focused element is the Notice's own wrapper (`.notice`,
		// tabindex="-1"), not the <p role="status"> text inside it -- no
		// accessible role names that wrapper, so this is the querySelector
		// exception for a deliberately non-accessible element, not a
		// shortcut past an accessible query.
		expect(document.activeElement).toBe(container.querySelector('.notice'));
	});

	// #1528's own finding, shared here since feedbackSentNotice (#lib/
	// feedback.js) is now the one place both StaffFeedback and
	// PortalFeedback build this Notice: `email` comes off an async,
	// best-effort session read, so it can genuinely be empty.
	it('drops the "will email you at" clause when no email is known', async () => {
		const onSend = vi.fn().mockResolvedValue(undefined);
		await setup({ email: '', onSend });

		await sendNotWorkingFeedback(openDrawer);

		await expect.element(page.getByText('Feedback sent. Thank you.', { exact: true })).toBeVisible();
	});

	it('keeps the drawer open and shows the refusal when the send fails', async () => {
		await expectRefusalKeepsDrawerOpen(setup, openDrawer, 'Send feedback to DoulaCloud');
	});
});
