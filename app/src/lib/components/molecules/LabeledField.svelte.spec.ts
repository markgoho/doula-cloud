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

	it('renders the control before the label inside a cluster-l wrapper in inline orientation', async () => {
		const { container } = await setup({ orientation: 'inline' });

		const label = page.getByText('Client name').element();
		const control = page.getByLabelText('Client name').element();

		expect(control.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(container.querySelector('cluster-l')).toContainElement(control);
	});
});
