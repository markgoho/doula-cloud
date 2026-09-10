import type { ComponentProps } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
/*
 * The real cascade, because the width assertions below are measurements
 * rather than attribute checks: without the reset this atom's padding
 * would sit outside its 100% (`box-sizing` is `border-box` there, not by
 * default), and without the tokens the padding and border would compute
 * to zero. Imported the way `styles/zoom.svelte.spec.ts` imports the
 * tokens -- one entry point, so the layer order is the app's own.
 */
import '#lib/styles/app.css';
import TextInput from './TextInput.svelte';

type SetupOptions = Partial<Omit<ComponentProps<typeof TextInput>, 'onInput'>>;

async function setup({ value = '', ...rest }: SetupOptions = {}) {
	const onInput = vi.fn();
	await render(TextInput, { value, onInput, ...rest });
	return { onInput };
}

/*
 * The setup for the width cases at the foot of this file: render the atom
 * into a wrapper styled the way a caller would style one, and report both
 * measured widths. At module scope because
 * `unicorn/consistent-function-scoping` refuses a helper that closes over
 * nothing its block owns (`.claude/rules/svelte-tests.md`).
 */
async function measureInWrapper(wrapperStyle: Partial<CSSStyleDeclaration>) {
	const { container } = await render(TextInput, { value: '', onInput: vi.fn() });
	Object.assign(container.style, wrapperStyle);
	const input = page.getByRole('textbox').element();
	return {
		input: input.getBoundingClientRect().width,
		wrapper: container.getBoundingClientRect().width
	};
}

describe('TextInput.svelte', () => {
	it('renders a textbox with the given value', async () => {
		await setup({ value: 'Alex' });

		await expect.element(page.getByRole('textbox')).toHaveValue('Alex');
	});

	it('calls onInput with the new value when typed into', async () => {
		const { onInput } = await setup();

		await page.getByRole('textbox').fill('Jordan');

		expect(onInput).toHaveBeenCalledWith('Jordan');
	});

	it('defaults to type=text', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'text');
	});

	it('reflects a non-default type', async () => {
		await setup({ type: 'email' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'email');
	});

	it('accepts type=number', async () => {
		await setup({ type: 'number', value: '3' });

		await expect.element(page.getByRole('spinbutton')).toHaveAttribute('type', 'number');
	});

	it('accepts type=date', async () => {
		await setup({ type: 'date', value: '2026-01-15' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'date');
		await expect.element(page.getByRole('textbox')).toHaveValue('2026-01-15');
	});

	it('auto-generates a distinct id per instance, so external labels can target each safely', async () => {
		const { unmount } = await render(TextInput, { value: '', onInput: vi.fn() });
		const firstId = page.getByRole('textbox').element().id;
		await unmount();

		await render(TextInput, { value: '', onInput: vi.fn() });
		const secondId = page.getByRole('textbox').element().id;

		expect(firstId).not.toBe('');
		expect(secondId).not.toBe(firstId);
	});

	it('accepts a caller-supplied id', async () => {
		await setup({ id: 'client-first-name' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('id', 'client-first-name');
	});

	it('reflects disabled', async () => {
		await setup({ disabled: true });

		await expect.element(page.getByRole('textbox')).toBeDisabled();
	});

	it('reflects required', async () => {
		await setup({ required: true });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('required');
	});

	it('reflects minlength', async () => {
		await setup({ minlength: 6 });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('minlength', '6');
	});

	it('omits minlength when not provided', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('minlength');
	});

	it('reflects maxlength', async () => {
		await setup({ maxlength: 6 });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('maxlength', '6');
	});

	it('omits maxlength when not provided', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('maxlength');
	});

	it('reflects step', async () => {
		await setup({ type: 'number', step: 0.01 });

		await expect.element(page.getByRole('spinbutton')).toHaveAttribute('step', '0.01');
	});

	it('omits step when not provided', async () => {
		await setup({ type: 'number' });

		await expect.element(page.getByRole('spinbutton')).not.toHaveAttribute('step');
	});

	it('reflects ariaLabel', async () => {
		await setup({ ariaLabel: 'Field label' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('aria-label', 'Field label');
	});

	it('omits aria-label when ariaLabel is not provided', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('aria-label');
	});

	it('reflects min', async () => {
		await setup({ type: 'number', min: 1 });

		await expect.element(page.getByRole('spinbutton')).toHaveAttribute('min', '1');
	});

	it('omits min when not provided', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('min');
	});

	it('sets aria-invalid=false by default', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).toHaveAttribute('aria-invalid', 'false');
	});

	it('sets aria-invalid=true when invalid', async () => {
		await setup({ invalid: true });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true');
	});

	it('wires aria-describedby to an external error message for accessible announcement', async () => {
		await setup({ invalid: true, describedBy: 'first-name-error' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('aria-describedby', 'first-name-error');
	});

	it('omits aria-describedby when not provided', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('aria-describedby');
	});

	it('reflects autocomplete', async () => {
		await setup({ autocomplete: 'email' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('autocomplete', 'email');
	});

	it('reflects autocomplete=off, for a field about someone else', async () => {
		await setup({ autocomplete: 'off' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('autocomplete', 'off');
	});

	it('omits autocomplete when not provided', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('autocomplete');
	});

	describe('type=password', () => {
		it('defaults to hidden (#470)', async () => {
			await setup({ type: 'password', value: 'hunter2' });

			await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'password');
		});

		it('renders no reveal toggle for a non-password type', async () => {
			await setup({ type: 'email' });

			await expect.element(page.getByRole('button')).not.toBeInTheDocument();
		});

		it('names the toggle "Show password" while hidden', async () => {
			await setup({ type: 'password' });

			await expect.element(page.getByRole('button', { name: 'Show password' })).toBeVisible();
		});

		it('reveals the value and renames the toggle when clicked', async () => {
			await setup({ type: 'password', value: 'hunter2' });

			await page.getByRole('button', { name: 'Show password' }).click();

			await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'text');
			await expect.element(page.getByRole('button', { name: 'Hide password' })).toBeVisible();
		});

		it('hides the value again on a second click', async () => {
			await setup({ type: 'password', value: 'hunter2' });

			await page.getByRole('button', { name: 'Show password' }).click();
			await page.getByRole('button', { name: 'Hide password' }).click();

			await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'password');
			await expect.element(page.getByRole('button', { name: 'Show password' })).toBeVisible();
		});

		it('announces its state with aria-pressed', async () => {
			await setup({ type: 'password' });

			await expect.element(page.getByRole('button')).toHaveAttribute('aria-pressed', 'false');

			await page.getByRole('button').click();

			await expect.element(page.getByRole('button')).toHaveAttribute('aria-pressed', 'true');
		});

		it('is operable by keyboard alone', async () => {
			await setup({ type: 'password' });

			page.getByRole('button', { name: 'Show password' }).element().focus();
			await userEvent.keyboard('{Enter}');

			await expect.element(page.getByRole('textbox')).toHaveAttribute('type', 'text');
		});

		it("names which password it reveals, where a page holds more than one", async () => {
			await setup({ type: 'password', passwordLabel: 'new password' });

			await expect.element(page.getByRole('button', { name: 'Show new password' })).toBeVisible();
		});

		it('never blocks paste', async () => {
			const { onInput } = await setup({ type: 'password' });

			const source = document.createElement('input');
			source.value = 'pasted-secret';
			document.body.append(source);
			source.focus();
			source.select();
			await userEvent.copy();
			source.remove();

			await page.getByRole('textbox').click();
			await userEvent.paste();

			expect(onInput).toHaveBeenCalledWith('pasted-secret');
		});

		it('disables the toggle along with the field', async () => {
			await setup({ type: 'password', disabled: true });

			await expect.element(page.getByRole('button')).toBeDisabled();
		});
	});

	/*
	 * #805. The continuum sweep cannot hold any of this: it measures
	 * whether a subject needs more room than it is given, and a control
	 * that is too WIDE for what it holds still fits. So the width is
	 * asserted here, by measuring the rendered box.
	 *
	 * The browser default this replaces is about 208px, which is why the
	 * cases below use containers on either side of that figure -- a wider
	 * one the control has to grow into, and a narrower one it has to
	 * shrink to.
	 */
	describe('inline size', () => {
		it('fills the space it is given, so it agrees with Select and Textarea in one column', async () => {
			const { input, wrapper } = await measureInWrapper({ display: 'block', inlineSize: '600px' });

			expect(input).toBeCloseTo(wrapper, 1);
		});

		it('narrows to a wrapper that caps it, with no :global selector reaching past this atom', async () => {
			const { input, wrapper } = await measureInWrapper({
				display: 'block',
				inlineSize: '600px',
				maxInlineSize: '120px'
			});

			expect(input).toBeCloseTo(wrapper, 1);
			expect(input).toBeCloseTo(120, 1);
		});

		it('takes a flex wrapper at its stated width, with no min-inline-size override to make it apply', async () => {
			const { input, wrapper } = await measureInWrapper({
				display: 'flex',
				inlineSize: '80px'
			});

			expect(input).toBeCloseTo(wrapper, 1);
			expect(input).toBeCloseTo(80, 1);
		});
	});
});
