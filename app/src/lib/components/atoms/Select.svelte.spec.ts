import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Select from './Select.svelte';

interface SetupOptions {
	options?: string[];
	value?: string;
	placeholder?: string;
	disabled?: boolean;
	required?: boolean;
	invalid?: boolean;
	describedBy?: string;
	name?: string;
	id?: string;
}

async function setup({
	options = ['Home', 'Hospital', 'Birth center'],
	value,
	placeholder,
	disabled,
	required,
	invalid,
	describedBy,
	name,
	id
}: SetupOptions = {}) {
	await render(Select, { options, value, placeholder, disabled, required, invalid, describedBy, name, id });
}

describe('Select.svelte', () => {
	it('renders an option for each entry in options', async () => {
		await setup();

		await expect.element(page.getByRole('combobox').getByRole('option', { name: 'Home' })).toBeInTheDocument();
		await expect.element(page.getByRole('combobox').getByRole('option', { name: 'Hospital' })).toBeInTheDocument();
		await expect
			.element(page.getByRole('combobox').getByRole('option', { name: 'Birth center' }))
			.toBeInTheDocument();
	});

	it('selects the option matching the bound value', async () => {
		await setup({ value: 'Hospital' });

		await expect.element(page.getByRole('combobox')).toHaveValue('Hospital');
	});

	it('updates the bound value when the user picks an option', async () => {
		await setup();

		await page.getByRole('combobox').selectOptions('Hospital');

		await expect.element(page.getByRole('combobox')).toHaveValue('Hospital');
	});

	it('renders a disabled placeholder option when placeholder is set', async () => {
		await setup({ placeholder: 'Choose a location' });

		const placeholderOption = page.getByRole('option', { name: 'Choose a location' });
		await expect.element(placeholderOption).toBeInTheDocument();
		await expect.element(placeholderOption).toBeDisabled();
	});

	it('omits the placeholder option when placeholder is not set', async () => {
		await setup();

		await expect.element(page.getByRole('option', { name: 'Choose a location' })).not.toBeInTheDocument();
	});

	it('disables the control when disabled is true', async () => {
		await setup({ disabled: true });

		await expect.element(page.getByRole('combobox')).toBeDisabled();
	});

	it('marks the control invalid when invalid is true', async () => {
		await setup({ invalid: true });

		await expect.element(page.getByRole('combobox')).toHaveAttribute('aria-invalid', 'true');
	});

	it('marks the control valid when invalid is false', async () => {
		await setup({ invalid: false });

		await expect.element(page.getByRole('combobox')).toHaveAttribute('aria-invalid', 'false');
	});

	it('associates an external error message via describedBy', async () => {
		await setup({ describedBy: 'location-error' });

		await expect.element(page.getByRole('combobox')).toHaveAttribute('aria-describedby', 'location-error');
	});

	it('generates an id when none is supplied', async () => {
		await setup({ options: ['Home'] });

		await expect.element(page.getByRole('combobox')).toHaveAttribute('id');
	});

	it('uses a supplied id instead of generating one', async () => {
		await setup({ id: 'location' });

		await expect.element(page.getByRole('combobox')).toHaveAttribute('id', 'location');
	});

	it('sets the name attribute when supplied', async () => {
		await setup({ name: 'location' });

		await expect.element(page.getByRole('combobox')).toHaveAttribute('name', 'location');
	});

	it('marks the control required when required is true', async () => {
		await setup({ required: true });

		await expect.element(page.getByRole('combobox')).toHaveAttribute('required');
	});
});
