import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import RadioGroup from './RadioGroup.svelte';
/*
 * The real stylesheet, so the layout regression below measures the row a
 * person sees: without it `cluster-l` is an unstyled inline element and
 * the defect it had cannot be reproduced. Global, so it reaches every
 * test in this file, none of which asserts on what it changes.
 */
import '#lib/styles/app.css';

type Mode = 'signup' | 'login';

const options: { value: Mode; label: string }[] = [
	{ value: 'signup', label: "I'm new here -- create an account" },
	{ value: 'login', label: 'I already have an account -- log in' }
];

interface SetupOptions {
	legend?: string;
	name?: string;
	value?: Mode;
}

async function setup({ legend = 'Mode', name, value = 'signup' }: SetupOptions = {}) {
	const onChange = vi.fn();
	await render(RadioGroup<Mode>, { legend, name, options, value, onChange });
	return { onChange };
}

describe('RadioGroup.svelte', () => {
	it('renders a fieldset with the given legend', async () => {
		await setup({ legend: 'Mode' });

		await expect.element(page.getByRole('group', { name: 'Mode' })).toBeInTheDocument();
	});

	it('renders an option for each entry in options', async () => {
		await setup();

		await expect.element(page.getByLabelText("I'm new here -- create an account")).toBeInTheDocument();
		await expect.element(page.getByLabelText('I already have an account -- log in')).toBeInTheDocument();
	});

	it('reflects the given value as the checked option', async () => {
		await setup({ value: 'login' });

		await expect.element(page.getByLabelText('I already have an account -- log in')).toBeChecked();
		await expect.element(page.getByLabelText("I'm new here -- create an account")).not.toBeChecked();
	});

	it('calls onChange with the selected option value when clicked', async () => {
		const { onChange } = await setup({ value: 'signup' });

		await page.getByLabelText('I already have an account -- log in').click();

		expect(onChange).toHaveBeenCalledWith('login');
	});

	it('gives options in the same group a shared name so only one can be selected', async () => {
		await setup();

		const firstName = page.getByLabelText("I'm new here -- create an account").element().getAttribute('name');
		const secondName = page.getByLabelText('I already have an account -- log in').element().getAttribute('name');

		expect(firstName).not.toBeNull();
		expect(firstName).toBe(secondName);
	});

	it('auto-generates a distinct name per instance, so unrelated groups do not collide', async () => {
		const { unmount } = await render(RadioGroup<Mode>, {
			legend: 'Mode',
			options,
			value: 'signup',
			onChange: vi.fn()
		});
		const firstName = page.getByLabelText("I'm new here -- create an account").element().getAttribute('name');
		await unmount();

		await render(RadioGroup<Mode>, { legend: 'Mode', options, value: 'signup', onChange: vi.fn() });
		const secondName = page.getByLabelText("I'm new here -- create an account").element().getAttribute('name');

		expect(firstName).not.toBe(secondName);
	});

	it('accepts a caller-supplied name', async () => {
		await setup({ name: 'mode' });

		await expect
			.element(page.getByLabelText("I'm new here -- create an account"))
			.toHaveAttribute('name', 'mode');
	});

	/*
	 * #464: a question page makes the group's name the page's own <h1>, so
	 * the Template already owns the <fieldset> and its <legend>. A second
	 * one here would nest fieldsets and announce the question twice.
	 */
	it('renders no fieldset and no legend when the group is unnamed', async () => {
		const { container } = await render(RadioGroup<Mode>, {
			options,
			value: 'signup',
			onChange: vi.fn()
		});

		expect(container.querySelectorAll('fieldset')).toHaveLength(0);
		expect(container.querySelectorAll('legend')).toHaveLength(0);
		await expect.element(page.getByLabelText("I'm new here -- create an account")).toBeVisible();
	});

	/*
	 * The duplicate-check page (#432) offers the Practice's existing Clients
	 * as options, and a name alone cannot tell two Sarahs apart -- the
	 * history and the consequence belong to the option.
	 */
	it('describes an option from its own description', async () => {
		const { container } = await render(RadioGroup<Mode>, {
			legend: 'Mode',
			options: [
				{ value: 'signup' as Mode, label: 'Sarah Whitfield', description: 'Added 4 March 2026.' },
				{ value: 'login' as Mode, label: 'No, this is a different person' }
			],
			value: 'signup',
			onChange: vi.fn()
		});

		const described = page.getByLabelText('Sarah Whitfield').element().getAttribute('aria-describedby');
		expect(container.querySelector(`#${described}`)?.textContent).toBe('Added 4 March 2026.');
		expect(
			page.getByLabelText('No, this is a different person').element().getAttribute('aria-describedby')
		).toBeNull();
	});

	/*
	 * GOV.UK asks for a refusal twice -- in the summary at the top, and
	 * again against the control -- because a reader who has scrolled past
	 * the summary has nothing left to act on.
	 */
	it('prints a refusal against the group and joins it to the fieldset', async () => {
		const { container } = await render(RadioGroup<Mode>, {
			legend: 'Mode',
			name: 'mode',
			options: [
				{ value: 'signup' as Mode, label: 'Sign up' },
				{ value: 'login' as Mode, label: 'Log in' }
			],
			value: '' as Mode,
			onChange: vi.fn(),
			error: 'Select whether you are signing up or logging in'
		});

		await expect.element(page.getByRole('alert')).toHaveTextContent('Select whether you are signing up or logging in');
		const fieldset = container.querySelector('fieldset')!;
		expect(container.querySelector(`#${fieldset.getAttribute('aria-describedby')}`)?.textContent).toBe(
			'Select whether you are signing up or logging in'
		);
	});

	// #1590: a question can open with nothing selected, and carry a hint
	// of its own under the legend that the fieldset is described by.
	it('selects nothing when the value is undefined, and describes the group by its hint and its refusal', async () => {
		const { container } = await render(RadioGroup<Mode>, {
			legend: 'Mode',
			hint: 'You answer this one time.',
			name: 'mode',
			options,
			value: undefined,
			onChange: vi.fn(),
			error: 'Select a mode'
		});

		await expect.element(page.getByLabelText("I'm new here -- create an account")).not.toBeChecked();
		await expect.element(page.getByLabelText('I already have an account -- log in')).not.toBeChecked();
		await expect.element(page.getByText('You answer this one time.')).toBeVisible();
		expect(container.querySelector('fieldset')!.getAttribute('aria-describedby')).toBe('mode-hint mode-error');
	});

	/*
	 * A legend-less group has no <fieldset> to be described by, so the
	 * refusal has to carry itself -- role="alert" announces it either way.
	 */
	it('still announces a refusal when the Template owns the legend', async () => {
		await render(RadioGroup<Mode>, {
			options: [
				{ value: 'signup' as Mode, label: 'Sign up' },
				{ value: 'login' as Mode, label: 'Log in' }
			],
			value: '' as Mode,
			onChange: vi.fn(),
			error: 'Choose one'
		});

		await expect.element(page.getByRole('alert')).toHaveTextContent('Choose one');
	});

	/*
	 * Regression, #1596: an option was a `cluster-l`, which is flex-wrap,
	 * so a label that could not share a line with its radio dropped whole
	 * onto the next line -- a bare radio, then a name with no control
	 * beside it. Found by looking at the Start work form's fourteen-Doula
	 * roster at 320px (ADR-0024): "Marguerite Throckmorton-Balasubramanian"
	 * sat under a radio that read as belonging to nobody. The same defect
	 * `LabeledField` had in #510, and asserted the same way: on the
	 * label's own top edge, because the label staying beside its control
	 * is the thing that was broken. The continuum sweep cannot see it --
	 * a dropped label makes the row narrower, not wider.
	 */
	it('keeps a long label beside its radio instead of dropping it to its own line, at 320px', async () => {
		await page.viewport(320, 600);
		const longName = 'Marguerite Throckmorton-Balasubramanian';
		await render(RadioGroup<string>, {
			legend: 'Who is the Doula?',
			options: [
				{ value: 'staff-1', label: 'Jo Li' },
				{ value: 'staff-2', label: longName }
			],
			value: '',
			onChange: vi.fn()
		});

		const radio = page.getByLabelText(longName).element();
		const label = page.getByText(longName).element();

		expect(label.getBoundingClientRect().top).toBeLessThan(radio.getBoundingClientRect().bottom);
		// And it wraps inside its own column rather than pushing past the edge.
		expect(label.getBoundingClientRect().right).toBeLessThanOrEqual(320);
	});
});
