import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it } from 'vitest';
import TimezoneField from './TimezoneField.svelte';

const QUESTION = 'What timezone does this Practice work in?';

interface SetupOptions {
	value?: string;
	error?: string;
}

async function setup({ value = '', error }: SetupOptions = {}) {
	await render(TimezoneField, { value, hint: 'We use it to work out which day a Visit falls on.', error });
}

describe('TimezoneField', () => {
	it('asks the question with its hint beneath, and a required select', async () => {
		await setup();

		const field = page.getByLabelText(QUESTION);
		await expect.element(field).toBeVisible();
		await expect.element(field).toBeRequired();
		await expect
			.element(field)
			.toHaveAccessibleDescription('We use it to work out which day a Visit falls on.');
	});

	it('offers a placeholder and the seven US zones when nothing is chosen', async () => {
		await setup();

		expect(page.getByRole('option', { name: 'Choose a timezone' }).elements()).toHaveLength(1);
		expect(page.getByRole('option').elements()).toHaveLength(8);
	});

	it('shows the chosen zone by its label', async () => {
		await setup({ value: 'America/Phoenix' });

		await expect.element(page.getByLabelText(QUESTION)).toHaveValue('America/Phoenix');
		expect(page.getByRole('option', { name: 'Mountain time, no daylight saving (Phoenix)' }).elements()).toHaveLength(1);
		expect(page.getByRole('option').elements()).toHaveLength(8);
	});

	it('adds a stored zone outside the seven, labeled for a person', async () => {
		await setup({ value: 'America/Indiana/Indianapolis' });

		await expect.element(page.getByLabelText(QUESTION)).toHaveValue('America/Indiana/Indianapolis');
		expect(page.getByRole('option').elements()).toHaveLength(9);
	});

	it('changes the value to the zone chosen', async () => {
		await setup();

		const field = page.getByLabelText(QUESTION);
		await field.selectOptions('America/Chicago');

		await expect.element(field).toHaveValue('America/Chicago');
	});

	it('says what is wrong beside the select', async () => {
		await setup({ error: 'Choose a timezone' });

		const field = page.getByLabelText(QUESTION);
		await expect.element(field).toBeInvalid();
		await expect.element(field).toHaveAccessibleDescription(/Choose a timezone/);
	});
});
