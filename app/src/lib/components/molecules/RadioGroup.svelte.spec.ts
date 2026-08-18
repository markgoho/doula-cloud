import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import RadioGroup from './RadioGroup.svelte';

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
});
