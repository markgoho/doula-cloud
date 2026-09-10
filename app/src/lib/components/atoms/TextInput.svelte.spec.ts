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
/*
 * The other two controls a form puts in the same column. Imported here
 * rather than asserted from each atom's own file because the claim is
 * about the three of them together: one form, one column, one width
 * (#805).
 */
import Select from './Select.svelte';
import Textarea from './Textarea.svelte';

type SetupOptions = Partial<Omit<ComponentProps<typeof TextInput>, 'onInput'>>;

async function setup({ value = '', ...rest }: SetupOptions = {}) {
	const onInput = vi.fn();
	const rendered = await render(TextInput, { value, onInput, ...rest });
	return { onInput, ...rendered };
}

/*
 * The form column the width cases below put a control in: wide enough
 * that a control which sizes itself to its own content stops well short
 * of the edge, so "fills what it is given" and "takes the browser's
 * default" cannot both pass.
 */
const column: Partial<CSSStyleDeclaration> = { display: 'block', inlineSize: '600px' };

/*
 * The setup for the width cases at the foot of this file: style the
 * wrapper the atom was rendered into the way a caller would style one,
 * and report both measured widths. At module scope because
 * `unicorn/consistent-function-scoping` refuses a helper that closes over
 * nothing its block owns (`.claude/rules/svelte-tests.md`).
 */
async function measureInWrapper(wrapperStyle: Partial<CSSStyleDeclaration>) {
	const { container } = await setup();
	Object.assign(container.style, wrapperStyle);
	return {
		inputWidth: page.getByRole('textbox').element().getBoundingClientRect().width,
		wrapperWidth: container.getBoundingClientRect().width
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
	 * The width this atom chose (#805 -- the argument is in the
	 * component's own module comment). The continuum sweep cannot hold
	 * any of it: it measures whether a subject needs more room than it is
	 * given, and a control that is too WIDE for what it holds still fits.
	 * So it is measured here, off the rendered box.
	 *
	 * Every assertion is relative -- the control against the wrapper it
	 * was put in, one control against another -- never against a stated
	 * number of pixels, per ADR-0025: nothing in this repo's verification
	 * names a width except 320.
	 */
	describe('inline size', () => {
		it('fills the space it is given', async () => {
			const { inputWidth, wrapperWidth } = await measureInWrapper(column);

			expect(inputWidth).toBeCloseTo(wrapperWidth, 1);
		});

		it('narrows to a wrapper that caps it, with no :global selector reaching past this atom', async () => {
			const { inputWidth, wrapperWidth } = await measureInWrapper({
				...column,
				maxInlineSize: '120px'
			});

			expect(inputWidth).toBeCloseTo(wrapperWidth, 1);
		});

		it('takes a flex wrapper at its stated width, with no min-inline-size override to make it apply', async () => {
			const { inputWidth, wrapperWidth } = await measureInWrapper({
				display: 'flex',
				inlineSize: '80px'
			});

			expect(inputWidth).toBeCloseTo(wrapperWidth, 1);
		});

		/*
		 * The defect #805 is named for: a form holding all three controls
		 * did not agree with itself, because only this one had no width.
		 * Each is mounted into a wrapper of the same size in turn rather
		 * than into one shared form, because `render` mounts one component
		 * -- what is being asserted is that the three answer the same
		 * column identically, which is the same claim either way.
		 */
		it('renders to the same width as Select and Textarea in a column of the same size', async () => {
			const input = await setup();
			Object.assign(input.container.style, column);
			const inputWidth = page.getByRole('textbox').element().getBoundingClientRect().width;
			await input.unmount();

			const select = await render(Select, { options: ['Home', 'Hospital'] });
			Object.assign(select.container.style, column);
			const selectWidth = page.getByRole('combobox').element().getBoundingClientRect().width;
			await select.unmount();

			const textarea = await render(Textarea, { value: '', onInput: vi.fn() });
			Object.assign(textarea.container.style, column);
			const textareaWidth = page.getByRole('textbox').element().getBoundingClientRect().width;

			expect(selectWidth).toBeCloseTo(inputWidth, 1);
			expect(textareaWidth).toBeCloseTo(inputWidth, 1);
		});
	});
});
