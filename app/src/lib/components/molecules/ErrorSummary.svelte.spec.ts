import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorSummary from './ErrorSummary.svelte';

type SetupOptions = Partial<ComponentProps<typeof ErrorSummary>>;

async function setup(overrides: SetupOptions = {}) {
	return render(ErrorSummary, {
		props: {
			errors: [
				{ message: 'Enter your email address', targetId: 'email' },
				{ message: 'Enter your password', targetId: 'password' }
			],
			...overrides
		}
	});
}

describe('ErrorSummary.svelte', () => {
	// The heading is the pattern. It is fixed so that a reader who has met
	// it once knows what the box is before reading a word of it.
	it('says "There is a problem", whatever the errors are', async () => {
		await setup({ errors: [{ message: 'Select at least one role', targetId: 'roles' }] });

		await expect
			.element(page.getByRole('heading', { level: 2, name: 'There is a problem' }))
			.toBeVisible();
	});

	it('lists one entry per error, each linking to its control', async () => {
		await setup();

		const first = page.getByRole('link', { name: 'Enter your email address' });
		const second = page.getByRole('link', { name: 'Enter your password' });

		await expect.element(first).toBeVisible();
		await expect.element(first).toHaveAttribute('href', '#email');
		await expect.element(second).toBeVisible();
		await expect.element(second).toHaveAttribute('href', '#password');
	});

	/*
	 * A refusal that belongs to the submission rather than to any field --
	 * the service was unreachable, the server refused for a reason nothing
	 * on the page caused. GOV.UK renders such an entry as plain text, and a
	 * link that goes nowhere useful is worse than no link at all.
	 */
	it('renders an entry with no target as text rather than as a link', async () => {
		const { container } = await setup({
			errors: [{ message: 'There is a problem with the service. Try again in a few minutes.' }]
		});

		await expect
			.element(page.getByText('There is a problem with the service. Try again in a few minutes.'))
			.toBeVisible();
		expect(container.querySelectorAll(':scope li a')).toHaveLength(0);
	});

	/*
	 * The half no markup can do: a person who has just pressed a button is
	 * looking at the button, and the refusal is at the other end of the
	 * page. Taking focus is what announces it and puts the first fix one
	 * Tab away.
	 */
	it('takes focus when it appears', async () => {
		const { container } = await setup();

		expect(document.activeElement).toBe(container.querySelector('.summary'));
	});

	it('never joins the tab order', async () => {
		const { container } = await setup();

		expect(container.querySelector('.summary')).toHaveAttribute('tabindex', '-1');
	});

	/*
	 * A clean form must not have its focus stolen, and the guard is that
	 * there is nothing here to focus: an empty array renders no element at
	 * all, not an empty box and not a hidden live region.
	 */
	it('renders nothing at all when there is no error', async () => {
		const { container } = await setup({ errors: [] });

		expect(container.querySelector('.summary')).toBeNull();
		expect(container.textContent).not.toContain('There is a problem');
	});

	/*
	 * Announcing the list rather than the wrapper -- GOV.UK's own
	 * arrangement, and the reason the alert is an inner element instead of
	 * being put on the focusable container.
	 */
	it('announces the list to assistive technology', async () => {
		const { container } = await setup();

		const alert = container.querySelector('[role="alert"]');
		expect(alert).not.toBeNull();
		expect(alert?.textContent).toContain('Enter your email address');
	});

	// Two fields can be refused for the same reason, so a message is not an
	// identity: the list is keyed on index (#425, #464).
	it('lists two entries that share a message', async () => {
		const { container } = await setup({
			errors: [
				{ message: 'Enter a date', targetId: 'from' },
				{ message: 'Enter a date', targetId: 'to' }
			]
		});

		expect(container.querySelectorAll(':scope li')).toHaveLength(2);
	});
});
