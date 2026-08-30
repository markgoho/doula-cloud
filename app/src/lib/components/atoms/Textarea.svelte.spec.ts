import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Textarea from './Textarea.svelte';
import LabeledTextarea from './harness/LabeledTextarea.svelte';

type SetupOptions = Partial<Omit<ComponentProps<typeof Textarea>, 'onInput'>>;

async function setup({ value = '', ...rest }: SetupOptions = {}) {
	const onInput = vi.fn();
	await render(Textarea, { value, onInput, ...rest });
	return { onInput };
}

describe('Textarea.svelte', () => {
	it('renders a textbox with the given value', async () => {
		await setup({ value: 'Her waters broke at 4am' });

		await expect.element(page.getByRole('textbox')).toHaveValue('Her waters broke at 4am');
	});

	it('calls onInput with the new value when typed into', async () => {
		const { onInput } = await setup();

		await page.getByRole('textbox').fill('A note');

		expect(onInput).toHaveBeenCalledWith('A note');
	});

	it('auto-generates a distinct id per instance, so external labels can target each safely', async () => {
		const { unmount } = await render(Textarea, { value: '', onInput: vi.fn() });
		const firstId = page.getByRole('textbox').element().id;
		await unmount();

		await render(Textarea, { value: '', onInput: vi.fn() });
		const secondId = page.getByRole('textbox').element().id;

		expect(firstId).not.toBe('');
		expect(secondId).not.toBe(firstId);
	});

	it('accepts a caller-supplied id', async () => {
		await setup({ id: 'contract-prose' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('id', 'contract-prose');
	});

	it('reflects name, placeholder, required and disabled', async () => {
		await setup({ name: 'terms', placeholder: 'Terms', required: true, disabled: true });
		const textbox = page.getByRole('textbox');

		await expect.element(textbox).toHaveAttribute('name', 'terms');
		await expect.element(textbox).toHaveAttribute('placeholder', 'Terms');
		await expect.element(textbox).toHaveAttribute('required');
		await expect.element(textbox).toBeDisabled();
	});

	it('names itself with ariaLabel, for the rows that have no label of their own', async () => {
		await setup({ ariaLabel: 'Options, one per line' });

		await expect.element(page.getByLabelText('Options, one per line')).toBeVisible();
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
		await setup({ invalid: true, describedBy: 'prose-error' });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('aria-describedby', 'prose-error');
	});

	it('omits aria-describedby when there is neither a description nor a limit', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('aria-describedby');
	});
});

describe('Textarea.svelte starting height', () => {
	it('defaults to four rows', async () => {
		await setup();

		await expect.element(page.getByRole('textbox')).toHaveAttribute('rows', '4');
	});

	it('takes a taller starting height, for a contract body', async () => {
		await setup({ rows: 12 });

		await expect.element(page.getByRole('textbox')).toHaveAttribute('rows', '12');
	});

	/*
	 * The growth floor is the same number as `rows`, so the two agree
	 * whether or not the browser supports `field-sizing`.
	 */
	it('hands the row count to the stylesheet as well as to the attribute', async () => {
		await setup({ rows: 7 });

		expect(page.getByRole('textbox').element().getAttribute('style')).toContain('--textarea-rows: 7');
	});
});

describe('Textarea.svelte character count', () => {
	it('ships no counter at all when there is no limit', async () => {
		await setup({ value: 'Anything at all' });

		await expect.element(page.getByText(/characters remaining/)).not.toBeInTheDocument();
	});

	it('counts down from the limit', async () => {
		await setup({ value: 'four', maxLength: 40 });

		await expect.element(page.getByText('You have 36 characters remaining')).toBeVisible();
	});

	it('says character, not characters, at one', async () => {
		await setup({ value: 'a'.repeat(39), maxLength: 40 });

		await expect.element(page.getByText('You have 1 character remaining')).toBeVisible();
	});

	it('counts code points, not UTF-16 units, because the server counts runes', async () => {
		await setup({ value: '👶', maxLength: 40 });

		await expect.element(page.getByText('You have 39 characters remaining')).toBeVisible();
	});

	it('goes negative rather than truncating, and says how far over', async () => {
		await setup({ value: 'a'.repeat(42), maxLength: 40 });

		await expect.element(page.getByText('You have 2 characters too many')).toBeVisible();
	});

	it('sets no maxlength, so pasted text is never silently cut', async () => {
		await setup({ maxLength: 40 });

		await expect.element(page.getByRole('textbox')).not.toHaveAttribute('maxlength');
	});

	it('describes the control with the budget, so the limit is announced on arrival', async () => {
		await setup({ id: 'service-description', maxLength: 40 });

		await expect
			.element(page.getByRole('textbox'))
			.toHaveAttribute('aria-describedby', 'service-description-budget');
		await expect
			.element(page.getByText('You can enter up to 40 characters'))
			.toHaveAttribute('id', 'service-description-budget');
	});

	it('keeps a caller description ahead of the budget it appends', async () => {
		await setup({ id: 'service-description', describedBy: 'sd-hint', maxLength: 40 });

		await expect
			.element(page.getByRole('textbox'))
			.toHaveAttribute('aria-describedby', 'sd-hint service-description-budget');
	});

	it('hides the visible count from a screen reader, so the number is not spoken twice', async () => {
		await setup({ maxLength: 40 });

		await expect
			.element(page.getByText('You have 40 characters remaining'))
			.toHaveAttribute('aria-hidden', 'true');
	});
});

describe('Textarea.svelte count announcement', () => {
	/*
	 * The count a screen reader hears is a second node, deliberately behind
	 * the visible one: a live region on the node that changes per keystroke
	 * announces every character typed.
	 */
	it('leaves the live region silent until the typing stops', async () => {
		const { container } = await render(Textarea, {
			value: 'four',
			onInput: vi.fn(),
			maxLength: 40
		});
		const liveRegion = container.querySelector('[aria-live="polite"]')!;

		expect(liveRegion.textContent).toBe('');

		await vi.waitFor(
			() => expect(liveRegion.textContent).toBe('You have 36 characters remaining'),
			{ timeout: 3000 }
		);
	});
});

describe('Textarea.svelte inside LabeledField', () => {
	it('gets the label, the hint and the error unchanged', async () => {
		await render(LabeledTextarea, {
			label: 'What your Practice offers',
			hint: 'Say what kind of support you provide.',
			error: 'Enter a description of what your Practice offers'
		});
		const textbox = page.getByLabelText('What your Practice offers');

		await expect.element(textbox).toBeVisible();
		await expect.element(textbox).toHaveAttribute('aria-invalid', 'true');
		await expect.element(page.getByText('Say what kind of support you provide.')).toBeVisible();
		await expect
			.element(page.getByRole('alert'))
			.toHaveTextContent('Enter a description of what your Practice offers');

		const describedBy = textbox.element().getAttribute('aria-describedby') ?? '';
		const described = describedBy
			.split(' ')
			.map((id) => document.querySelector(`#${id}`)?.textContent?.trim());

		expect(described).toEqual([
			'Say what kind of support you provide.',
			'Enter a description of what your Practice offers'
		]);
	});

	it('appends its own budget to the ids LabeledField built', async () => {
		await render(LabeledTextarea, {
			label: 'What your Practice offers',
			hint: 'Say what kind of support you provide.',
			maxLength: 40
		});

		const textbox = page.getByLabelText('What your Practice offers');
		const describedBy = textbox.element().getAttribute('aria-describedby') ?? '';

		expect(describedBy.split(' ')).toHaveLength(2);
		expect(describedBy).toMatch(/-budget$/);

		// Typed through the real pair, so the count is the one a person sees.
		await textbox.fill('Postpartum support');

		await expect.element(page.getByText('You have 22 characters remaining')).toBeVisible();
	});
});
