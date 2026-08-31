import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import LabeledField, { type ControlProperties } from './LabeledField.svelte';

function inputSnippet() {
	return createRawSnippet<[ControlProperties]>((control) => ({
		render: () => `<input type="text" />`,
		setup: (element) => {
			const { id, describedBy, invalid } = control();
			const input = element as HTMLInputElement;
			input.id = id;
			input.setAttribute('aria-invalid', String(invalid));
			if (describedBy) input.setAttribute('aria-describedby', describedBy);
		}
	}));
}

type SetupOptions = Partial<Omit<ComponentProps<typeof LabeledField>, 'children'>>;

async function setup({ label = 'Client name', ...rest }: SetupOptions = {}) {
	return render(LabeledField, { label, children: inputSnippet(), ...rest });
}

describe('LabeledField.svelte', () => {
	it('associates the label with the control via a shared id', async () => {
		await setup({ label: 'Client name' });

		await expect.element(page.getByLabelText('Client name')).toBeInTheDocument();
	});

	it('auto-generates a distinct id per instance', async () => {
		const { unmount } = await render(LabeledField, { label: 'A', children: inputSnippet() });
		const firstId = page.getByLabelText('A').element().id;
		await unmount();

		await render(LabeledField, { label: 'A', children: inputSnippet() });
		const secondId = page.getByLabelText('A').element().id;

		expect(firstId).not.toBe('');
		expect(secondId).not.toBe(firstId);
	});

	it('accepts a caller-supplied id', async () => {
		await setup({ id: 'client-first-name' });

		await expect.element(page.getByLabelText('Client name')).toHaveAttribute('id', 'client-first-name');
	});

	it('omits the error message and aria-describedby when there is no error', async () => {
		await setup();

		await expect.element(page.getByLabelText('Client name')).not.toHaveAttribute('aria-describedby');
		await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
	});

	it('renders the error message and wires it via aria-describedby when present', async () => {
		await setup({ error: 'Name is required' });

		const control = page.getByLabelText('Client name');
		const alert = page.getByRole('alert');
		await expect.element(alert).toHaveTextContent('Name is required');

		const describedBy = control.element().getAttribute('aria-describedby');
		expect(describedBy).not.toBeNull();
		expect(alert.element().id).toBe(describedBy);
	});

	/*
	 * Position, not just presence: GOV.UK's Error message sits between the
	 * hint and the control it refuses, and this rendered below the control
	 * until #475 opened the pages in a browser. Document order is what the
	 * eye reads down the page, so it is what is asserted.
	 */
	it('renders the error message above the control it refuses', async () => {
		await setup({ error: 'Name is required' });

		const alert = page.getByRole('alert').element();
		const control = page.getByLabelText('Client name').element();

		expect(alert.compareDocumentPosition(control) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('renders the hint above the error, and both above the control', async () => {
		await setup({ hint: 'The name she goes by', error: 'Name is required' });

		const hint = page.getByText('The name she goes by').element();
		const alert = page.getByRole('alert').element();

		expect(hint.compareDocumentPosition(alert) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('marks the control invalid when there is an error', async () => {
		await setup({ error: 'Name is required' });

		await expect.element(page.getByLabelText('Client name')).toHaveAttribute('aria-invalid', 'true');
	});

	it('marks the control valid when there is no error', async () => {
		await setup();

		await expect.element(page.getByLabelText('Client name')).toHaveAttribute('aria-invalid', 'false');
	});

	it('defaults to stacked orientation, rendering the label before the control with no cluster wrapper', async () => {
		const { container } = await setup();

		const label = page.getByText('Client name').element();
		const control = page.getByLabelText('Client name').element();

		expect(label.compareDocumentPosition(control) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(container.querySelector('cluster-l')).toBeNull();
	});

	/*
	 * Regression, #425: "stacked" did not stack. A <label> is inline by
	 * default and a text input is inline-block, so the pair shared a line
	 * and every stacked field in the app read as label-beside-control.
	 * Asserted on computed style rather than markup, because the markup
	 * was already right and the rendering was not.
	 */
	it('puts a stacked label on its own line, above the control', async () => {
		const { container } = await setup();

		const label = container.querySelector('label') as HTMLLabelElement;
		const input = container.querySelector('input') as HTMLInputElement;

		expect(getComputedStyle(label).display).toBe('block');
		expect(input.getBoundingClientRect().top).toBeGreaterThanOrEqual(
			label.getBoundingClientRect().bottom
		);
	});

	it('renders the control before the label in inline orientation', async () => {
		await setup({ orientation: 'inline' });

		const label = page.getByText('Client name').element();
		const control = page.getByLabelText('Client name').element();

		expect(control.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	/*
	 * Regression, #510: an inline row that could not hold both items dropped
	 * the whole label onto its own line below the control -- an unlabelled
	 * control followed by a stray sentence. Reproduced at 320px (ADR-0024)
	 * with a label long enough that it cannot share a line with the control,
	 * the same shape as SignContract's consent checkbox. Asserted on the
	 * label's own top edge rather than markup, because the label staying
	 * beside the control -- not merely inside some wrapper -- is the thing
	 * that was broken.
	 */
	it('keeps a long inline label beside its control instead of dropping it to its own line, at 320px', async () => {
		await page.viewport(320, 400);
		const longLabel = 'I have read this Contract and I am signing it electronically';
		const { container } = await setup({ orientation: 'inline', label: longLabel });

		const control = page.getByLabelText(longLabel).element();
		const label = container.querySelector('label') as HTMLLabelElement;

		expect(label.getBoundingClientRect().top).toBeLessThan(control.getBoundingClientRect().bottom);
	});
});
