import type { ComponentProps } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Button from './Button.svelte';

type SetupOptions = Partial<ComponentProps<typeof Button>>;

async function setup({ label = 'Save', ...rest }: SetupOptions = {}) {
	const onClick = vi.fn();
	const { container, rerender } = await render(Button, { label, onClick, ...rest });
	return { onClick, container, rerender };
}

/*
 * A Button inside a real form, so a test can tell whether its press sent
 * the form. The form cancels its own submit, so the page never navigates.
 */
async function setupInForm(options: SetupOptions) {
	const form = document.createElement('form');
	const onSubmit = vi.fn((event: SubmitEvent) => event.preventDefault());
	form.addEventListener('submit', onSubmit);
	document.body.append(form);
	await render(Button, { target: form, props: { label: 'Send', type: 'submit', ...options } });
	(page.getByRole('button').element() as HTMLButtonElement).focus();
	return { onSubmit, form };
}

function icon(container: HTMLElement) {
	return container.querySelector('svg');
}

function labelSpan(container: HTMLElement) {
	return container.querySelector(':scope button > span:not(.spinner)');
}

describe('Button.svelte', () => {
	it('renders the label as visible, accessible text by default', async () => {
		await setup({ label: 'Save' });

		await expect.element(page.getByRole('button', { name: 'Save' })).toBeVisible();
	});

	it('calls onClick when clicked', async () => {
		const { onClick } = await setup();

		await page.getByRole('button').click();

		expect(onClick).toHaveBeenCalledOnce();
	});

	it('defaults to type=button', async () => {
		await setup();

		await expect.element(page.getByRole('button')).toHaveAttribute('type', 'button');
	});

	it('reflects a non-default type', async () => {
		await setup({ type: 'submit' });

		await expect.element(page.getByRole('button')).toHaveAttribute('type', 'submit');
	});

	// #2103: an opener drawn again after its form's Cancel takes focus.
	it('takes focus the moment it appears if focusOnAppearing', async () => {
		await setup({ focusOnAppearing: true });

		await expect.element(page.getByRole('button')).toHaveFocus();
	});

	it('does not take focus when it appears by default', async () => {
		await setup();

		await expect.element(page.getByRole('button')).not.toHaveFocus();
	});

	// A row's Change stays on screen while its form is open, so it takes
	// focus when focusOnAppearing turns true, not only when it renders.
	it('takes focus when focusOnAppearing turns true', async () => {
		const { rerender } = await render(Button, { label: 'Change' });
		await expect.element(page.getByRole('button')).not.toHaveFocus();

		await rerender({ focusOnAppearing: true });

		await expect.element(page.getByRole('button')).toHaveFocus();
	});

	it('reflects disabled', async () => {
		await setup({ disabled: true });

		await expect.element(page.getByRole('button')).toBeDisabled();
	});

	// A field editor's Move down that has just moved its row to the end
	// (#1920): unavailable, but it keeps focus, so the page is not where
	// focus lands.
	it('stays focusable and does nothing when pressed if ariaDisabled', async () => {
		const { onClick } = await setup({ ariaDisabled: true });

		const button = page.getByRole('button');
		await expect.element(button).toHaveAttribute('aria-disabled', 'true');
		(button.element() as HTMLButtonElement).focus();
		await expect.element(button).toHaveFocus();
		await userEvent.keyboard('{Enter}');
		await expect.element(button).toHaveFocus();
		expect(onClick).not.toHaveBeenCalled();
	});

	it('carries no aria-disabled when ariaDisabled is not given', async () => {
		await setup();

		await expect.element(page.getByRole('button')).not.toHaveAttribute('aria-disabled');
	});

	it('renders an icon alongside the label when icon is given', async () => {
		const { container } = await setup({ icon: 'check' });

		await expect.element(page.getByRole('button', { name: 'Save' })).toBeVisible();
		expect(icon(container)).toBeVisible();
	});

	it('renders no icon when icon is omitted', async () => {
		const { container } = await setup();

		expect(icon(container)).not.toBeInTheDocument();
	});

	it('sizes the icon down for a small button', async () => {
		const { container } = await setup({ size: 'sm', icon: 'check' });

		expect(icon(container)).toHaveAttribute('width', '16');
	});

	it('sizes the icon up for a large button', async () => {
		const { container } = await setup({ size: 'lg', icon: 'check' });

		expect(icon(container)).toHaveAttribute('width', '24');
	});

	it('renders the light weight even at a size that would otherwise default to duotone', async () => {
		const { container } = await setup({ size: 'lg', icon: 'check' });

		expect(container.querySelectorAll(':scope svg path')).toHaveLength(1);
	});

	it('keeps the label in the accessibility tree but visually hides it when iconOnly, instead of relying on aria-label', async () => {
		const { container } = await setup({ icon: 'check', iconOnly: true, label: 'Confirm' });

		await expect.element(page.getByRole('button', { name: 'Confirm' })).toBeVisible();
		await expect.element(page.getByRole('button')).not.toHaveAttribute('aria-label');
		expect(labelSpan(container)).toHaveClass('visually-hidden');
	});

	it('keeps the label visible (not visually-hidden) when not iconOnly', async () => {
		const { container } = await setup();

		expect(labelSpan(container)).not.toHaveClass('visually-hidden');
	});

	it('is marked busy and unavailable when loading, but not disabled', async () => {
		await setup({ loading: true });

		const button = page.getByRole('button');
		await expect.element(button).toHaveAttribute('aria-busy', 'true');
		await expect.element(button).toHaveAttribute('aria-disabled', 'true');
		// `toBeDisabled` counts `aria-disabled` too, so the native property
		// is what says the browser will not drop focus from it.
		expect((button.element() as HTMLButtonElement).disabled).toBe(false);
	});

	// #2119: a keyboard user who presses a button that then loads keeps
	// focus on it, so a failed action does not send the next Tab to the
	// top of the page; and a press while it loads does nothing.
	it('keeps focus while its action runs, and does nothing when pressed', async () => {
		const { onClick, rerender } = await setup({ label: 'Load more' });
		const button = page.getByRole('button');
		(button.element() as HTMLButtonElement).focus();
		await userEvent.keyboard('{Enter}');
		expect(onClick).toHaveBeenCalledOnce();

		await rerender({ loading: true });
		await expect.element(button).toHaveFocus();
		await userEvent.keyboard('{Enter}');
		await userEvent.keyboard(' ');
		expect(onClick).toHaveBeenCalledOnce();

		await rerender({ loading: false });
		await expect.element(button).toHaveFocus();
	});

	it('sends its form when pressed', async () => {
		const { onSubmit, form } = await setupInForm({});

		await userEvent.keyboard('{Enter}');

		expect(onSubmit).toHaveBeenCalledOnce();
		form.remove();
	});

	it('does not send its form when pressed while loading', async () => {
		const { onSubmit, form } = await setupInForm({ loading: true });

		await userEvent.keyboard('{Enter}');

		expect(onSubmit).not.toHaveBeenCalled();
		form.remove();
	});

	// Enter in a field sends the form through its submit button's click,
	// which `disabled` used to block and the canceled click now does.
	it.each([
		{ loading: false, sends: 1 },
		{ loading: true, sends: 0 }
	])('Enter in a field sends the form $sends time(s) when loading is $loading', async ({ loading, sends }) => {
		const { onSubmit, form } = await setupInForm({ loading });
		const field = document.createElement('input');
		field.setAttribute('aria-label', 'Message');
		form.prepend(field);

		await page.getByRole('textbox', { name: 'Message' }).fill('Hello');
		await userEvent.keyboard('{Enter}');

		expect(onSubmit).toHaveBeenCalledTimes(sends);
		form.remove();
	});

	it('sets aria-busy=false when not loading', async () => {
		await setup();

		await expect.element(page.getByRole('button')).toHaveAttribute('aria-busy', 'false');
	});

	it('renders a spinner instead of the icon when loading', async () => {
		const { container } = await setup({ icon: 'check', loading: true });

		expect(container.querySelector('.spinner')).toBeVisible();
		expect(icon(container)).not.toBeInTheDocument();
	});

	// A repeated per-row Button (DataTable's rowActions, the Client Fields'
	// archived list's "Collect again") joins itself to a visually-hidden sibling naming
	// the row, the same way Link's describedBy does (#515).
	it('joins itself to qualifying text elsewhere on the page', async () => {
		await setup({ describedBy: 'row-2-name' });

		await expect
			.element(page.getByRole('button'))
			.toHaveAttribute('aria-describedby', 'row-2-name');
	});

	it('carries no aria-describedby when none is given', async () => {
		await setup();

		await expect.element(page.getByRole('button')).not.toHaveAttribute('aria-describedby');
	});

	// PilotBanner's control (#1522): inline prose that reads as a link, not
	// a control with its own box, so it skips the size-* padding scale the
	// other variants share.
	it('renders the link variant with no size class', async () => {
		const { container } = await setup({ variant: 'link' });

		const button = container.querySelector('button');
		expect(button).toHaveClass('link');
		expect(button?.className).not.toMatch(/size-/);
	});

	it('names the element it discloses via ariaControls', async () => {
		await setup({ ariaControls: 'feedback-drawer' });

		await expect
			.element(page.getByRole('button'))
			.toHaveAttribute('aria-controls', 'feedback-drawer');
	});

	it('carries no aria-controls when none is given', async () => {
		await setup();

		await expect.element(page.getByRole('button')).not.toHaveAttribute('aria-controls');
	});
});
