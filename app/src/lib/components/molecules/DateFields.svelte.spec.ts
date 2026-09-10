import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
/*
 * The real cascade: the box widths asserted at the foot of this file are
 * `calc(3ch + var(--space-6))`, which computes to nothing without the
 * token layer, and they are measured with `border-box` from the reset.
 */
import '#lib/styles/app.css';
import DateFields from './DateFields.svelte';
import type { DateField, DateParts } from '#lib/intakeDate.js';

interface SetupOptions {
	parts?: DateParts;
	legend?: string;
	hint?: string;
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

	// #943: a hint nobody hears is not a hint. It has to reach a reader
	// who never sees the paragraph -- from the group, and from each box
	// she tabs into.
	it('announces a hint from the group and from every box', async () => {
		await setup({ legend: 'When did the pregnancy end?', hint: 'The day the pregnancy ended.' });

		await expect.element(page.getByText('The day the pregnancy ended.')).toBeVisible();
		await expect
			.element(page.getByRole('group', { name: 'When did the pregnancy end?' }))
			.toHaveAttribute('aria-describedby', 'dob-hint');
		await expect.element(page.getByLabelText('Month')).toHaveAttribute('aria-describedby', 'dob-hint');
	});

	// The hint first, then the refusal: what the box is for has to be
	// announced before what is wrong with what is in it.
	it('announces the hint before the refusal when it carries both', async () => {
		await setup({
			legend: 'When did the pregnancy end?',
			hint: 'The day the pregnancy ended.',
			error: 'Enter the date the pregnancy ended'
		});

		await expect
			.element(page.getByRole('group', { name: 'When did the pregnancy end?' }))
			.toHaveAttribute('aria-describedby', 'dob-hint dob-error');
	});

	// No legend is the question-page shape (#464): the Template owns the
	// fieldset, so the hint has nothing to hang off but the boxes.
	it('announces a hint from the boxes alone when the Template owns the group', async () => {
		await setup({ hint: 'The day the pregnancy ended.' });

		await expect.element(page.getByText('The day the pregnancy ended.')).toBeVisible();
		await expect.element(page.getByLabelText('Year')).toHaveAttribute('aria-describedby', 'dob-hint');
	});

	/*
	 * GOV.UK's Dates sizing, and what holds it now that the two rules
	 * that used to (#805's `:global(input)` and a `min-inline-size: 0` on
	 * each box) are gone. The continuum sweep cannot: three boxes that
	 * are each too wide for their content still fit, one under the other,
	 * which is how they went unnoticed in the first place. The figure to
	 * beat is the browser's default input `size` -- about 193px in this
	 * column -- so a box measured well under that is a box that was
	 * sized rather than left alone.
	 */
	describe('box widths', () => {
		it('sizes a two-digit box to two digits and a four-digit box wider, in a column that could hold a sentence', async () => {
			const { container } = await setup();
			container.style.inlineSize = '600px';

			const month = page.getByLabelText('Month').element().getBoundingClientRect().width;
			const year = page.getByLabelText('Year').element().getBoundingClientRect().width;

			expect(month).toBeLessThan(100);
			expect(year).toBeGreaterThan(month);
			expect(year).toBeLessThan(150);
		});
	});
});
