import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import LabeledField from './LabeledField.svelte';

interface ControlProperties {
	id: string;
	describedBy: string | undefined;
	invalid: boolean;
}

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
	await render(LabeledField, { label, children: inputSnippet(), ...rest });
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

	it('defaults to stacked orientation, rendering the label before the control', async () => {
		await setup();

		const label = page.getByText('Client name').element();
		const control = page.getByLabelText('Client name').element();

		expect(label.compareDocumentPosition(control) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('renders the control before the label in inline orientation', async () => {
		await setup({ orientation: 'inline' });

		const label = page.getByText('Client name').element();
		const control = page.getByLabelText('Client name').element();

		expect(control.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});
});
