import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import StackedForm from './StackedForm.svelte';

// `createRawSnippet` renders one root element, which is why this is a lone
// button rather than the field-field-button run a real form holds. What the
// run needs from this component is the `space` on the stack, asserted below;
// the spacing that `space` buys is `stack-l`'s own contract, covered in
// `primitives/index.spec.ts`.
function submitButton() {
	return createRawSnippet(() => ({ render: () => '<button type="submit">Continue</button>' }));
}

interface SetupOptions {
	onSubmit?: (event: SubmitEvent) => void;
}

async function setup({ onSubmit = vi.fn() }: SetupOptions = {}) {
	const { container } = await render(StackedForm, { onSubmit, children: submitButton() });
	return { container, onSubmit };
}

describe('StackedForm', () => {
	it('renders what it is given, inside a form', async () => {
		const { container } = await setup();

		await expect.element(page.getByRole('button', { name: 'Continue' })).toBeVisible();
		expect(container.querySelector('form')).not.toBeNull();
	});

	/*
	 * `container.querySelector` rather than an accessible query: the stack is
	 * a layout primitive with no role and no accessible name, and the `space`
	 * it carries is the whole point of this component. There is nothing for
	 * `getByRole` to find, because nothing about it is announced.
	 */
	it('stacks its children at the token FormPage spends on a fieldset', async () => {
		const { container } = await setup();

		const stack = container.querySelector(':scope form > stack-l');
		expect(stack?.getAttribute('space')).toBe('var(--space-5)');
		expect(stack?.querySelector(':scope button')).not.toBeNull();
	});

	// ADR-0021: the page refuses the submit and says so once, at the top --
	// never the browser's own bubble.
	it('lets the page do the refusing, not the browser', async () => {
		const { container } = await setup();

		expect(container.querySelector('form')?.hasAttribute('novalidate')).toBe(true);
	});

	it('hands the submit to its caller', async () => {
		const onSubmit = vi.fn((event: SubmitEvent) => event.preventDefault());
		await setup({ onSubmit });

		await page.getByRole('button', { name: 'Continue' }).click();

		expect(onSubmit).toHaveBeenCalledOnce();
	});
});
