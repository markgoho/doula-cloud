import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
import DateFields from './DateFields.svelte';
import type { DateField, DateParts } from '#lib/intakeDate.js';

interface SetupOptions {
	parts?: DateParts;
	legend?: string;
	error?: string;
	invalidField?: DateField;
}

async function setup({ parts = { month: '', day: '', year: '' }, ...rest }: SetupOptions = {}) {
	const onChange = vi.fn();
	const { container } = await render(DateFields, { name: 'dob', parts, onChange, ...rest });
	return { onChange, container };
}

describe('DateFields', () => {
	it('asks for the month, the day and the year, in that order', async () => {
		await setup();

		await expect.element(page.getByLabelText('Month')).toBeVisible();
		await expect.element(page.getByLabelText('Day')).toBeVisible();
		await expect.element(page.getByLabelText('Year')).toBeVisible();
	});

	it('shows what has already been typed', async () => {
		await setup({ parts: { month: '02', day: '09', year: '1988' } });

		await expect.element(page.getByLabelText('Year')).toHaveValue('1988');
	});

	it('reports the whole date when one box changes', async () => {
		const { onChange } = await setup({ parts: { month: '', day: '9', year: '1988' } });

		await page.getByLabelText('Month').fill('2');

		expect(onChange).toHaveBeenCalledWith({ month: '2', day: '9', year: '1988' });
	});

	// Content sizing, #466: a month box that could hold a sentence tells
	// the reader the wrong thing about what goes in it.
	it('accepts two characters for the month and the day and four for the year', async () => {
		await setup();

		await expect.element(page.getByLabelText('Month')).toHaveAttribute('maxlength', '2');
		await expect.element(page.getByLabelText('Day')).toHaveAttribute('maxlength', '2');
		await expect.element(page.getByLabelText('Year')).toHaveAttribute('maxlength', '4');
	});

	// #469: intake is a doula entering a Client's information, never the
	// Client's own, so no box carries a self-entry token.
	it('carries no autocomplete token on any box', async () => {
		await setup();

		await expect.element(page.getByLabelText('Year')).toHaveAttribute('autocomplete', 'off');
	});

	it('offers a numeric keyboard', async () => {
		await setup();

		await expect.element(page.getByLabelText('Day')).toHaveAttribute('inputmode', 'numeric');
	});

	it('announces the refusal once, for the group', async () => {
		await setup({ error: 'Date of birth must be a real date', invalidField: 'day' });

		await expect.element(page.getByRole('alert')).toHaveTextContent(
			'Date of birth must be a real date'
		);
	});

	it('marks only the box the refusal is about', async () => {
		await setup({ error: 'Date of birth must be a real date', invalidField: 'day' });

		await expect.element(page.getByLabelText('Day')).toHaveAttribute('aria-invalid', 'true');
		await expect.element(page.getByLabelText('Month')).toHaveAttribute('aria-invalid', 'false');
	});

	// On a question page the Template owns the <fieldset> and its <legend>
	// is the <h1>, so a second one here would announce the question twice.
	it('renders no group of its own when it is given no legend', async () => {
		const { container } = await setup();

		expect(container.querySelector('fieldset')).toBeNull();
	});

	it('renders its own group when it is given a legend', async () => {
		await setup({ legend: 'Date of birth' });

		await expect.element(page.getByRole('group', { name: 'Date of birth' })).toBeVisible();
	});

	it('describes its own group by the refusal', async () => {
		await setup({ legend: 'Date of birth', error: 'Date of birth must be a real date' });

		await expect
			.element(page.getByRole('group', { name: 'Date of birth' }))
			.toHaveAttribute('aria-describedby', 'dob-error');
	});
});
