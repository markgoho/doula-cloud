import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import CheckboxGroup, { checkboxFieldId } from './CheckboxGroup.svelte';
/*
 * The real stylesheet, so the 320px test below measures the row a person
 * sees, as RadioGroup.svelte.spec.ts does. Global; no other test here
 * asserts on what it changes.
 */
import '#lib/styles/app.css';

type Role = 'owner' | 'admin' | 'doula';

const options: { value: Role; label: string; description?: string }[] = [
	{ value: 'owner', label: 'Owner', description: 'Runs the Practice' },
	{ value: 'admin', label: 'Admin' },
	{ value: 'doula', label: 'Doula' }
];

interface SetupOptions {
	legend?: string;
	hint?: string;
	name?: string;
	value?: Role[];
	error?: string;
}

async function setup({ legend = 'Roles', hint, name, value = [], error }: SetupOptions = {}) {
	const onChange = vi.fn();
	await render(CheckboxGroup<Role>, {
		legend,
		hint,
		name,
		options,
		value,
		onChange,
		error
	});
	return { onChange };
}

describe('CheckboxGroup.svelte', () => {
	it('renders a fieldset named by its legend', async () => {
		await setup();

		await expect.element(page.getByRole('group', { name: 'Roles' })).toBeVisible();
	});

	it('renders a box for each entry in options, named by its label', async () => {
		await setup();

		for (const label of ['Owner', 'Admin', 'Doula']) {
			await expect.element(page.getByRole('checkbox', { name: label, exact: true })).toBeVisible();
		}
	});

	it('ticks the chosen values', async () => {
		await setup({ value: ['admin'] });

		await expect.element(page.getByRole('checkbox', { name: 'Admin' })).toBeChecked();
		await expect.element(page.getByRole('checkbox', { name: 'Owner' })).not.toBeChecked();
	});

	// The options' own order, not the click order, so one set of answers
	// always reads the same way.
	it('reports a ticked box in the order the options are listed', async () => {
		const { onChange } = await setup({ value: ['doula'] });

		await page.getByRole('checkbox', { name: 'Owner' }).click();

		expect(onChange).toHaveBeenCalledWith(['owner', 'doula']);
	});

	it('reports an unticked box taken out', async () => {
		const { onChange } = await setup({ value: ['owner', 'doula'] });

		await page.getByRole('checkbox', { name: 'Owner' }).click();

		expect(onChange).toHaveBeenCalledWith(['doula']);
	});

	it('keeps a stored value no option offers any more', async () => {
		const { onChange } = await setup({ value: ['retired' as Role] });

		await page.getByRole('checkbox', { name: 'Admin' }).click();

		expect(onChange).toHaveBeenCalledWith(['admin', 'retired']);
	});

	it('builds each box id from the name, so a summary link can reach the first', async () => {
		await setup({ name: 'invite-roles' });

		await expect
			.element(page.getByRole('checkbox', { name: 'Owner' }))
			.toHaveAttribute('id', checkboxFieldId('invite-roles', 'owner'));
		await expect.element(page.getByRole('checkbox', { name: 'Owner' })).toHaveAttribute('name', 'invite-roles');
	});

	it('describes an option by its own description, and only that option', async () => {
		await setup();

		await expect
			.element(page.getByRole('checkbox', { name: 'Owner' }))
			.toHaveAccessibleDescription('Runs the Practice');
		await expect.element(page.getByRole('checkbox', { name: 'Admin' })).not.toHaveAttribute('aria-describedby');
	});

	it('describes the group by its hint and its refusal, and marks each box refused', async () => {
		await setup({
			name: 'roles',
			hint: 'Select all that apply',
			error: 'Select at least one role'
		});

		await expect.element(page.getByText('Select all that apply')).toBeVisible();
		await expect.element(page.getByRole('alert')).toHaveTextContent('Select at least one role');
		await expect
			.element(page.getByRole('group', { name: 'Roles' }))
			.toHaveAttribute('aria-describedby', 'roles-hint roles-error');
		await expect.element(page.getByRole('checkbox', { name: 'Admin' })).toHaveAttribute('aria-invalid', 'true');
	});

	it('renders no fieldset when a Template owns the question, and still announces a refusal', async () => {
		const { container } = await render(CheckboxGroup<Role>, {
			options,
			value: [],
			onChange: vi.fn(),
			error: 'Select at least one role'
		});

		// querySelector, case 3: the absence of an element is a fact
		// about the document, and an unnamed group has no accessible name
		// to query by.
		expect(container.querySelectorAll('fieldset')).toHaveLength(0);
		await expect.element(page.getByRole('alert')).toHaveTextContent('Select at least one role');
		await expect.element(page.getByRole('checkbox', { name: 'Owner' })).toBeVisible();
	});

	/* As RadioGroup's #1596 regression: a label too long for the row wraps
	   beside its box, never under it, at 320px. */
	it('keeps a long label beside its box at 320px', async () => {
		await page.viewport(320, 600);
		const longLabel = 'My partner, for the whole labor and the first hours after the birth';
		await render(CheckboxGroup<string>, {
			legend: 'Who do you want in the room?',
			options: [{ value: 'partner', label: longLabel }],
			value: [],
			onChange: vi.fn()
		});

		const box = page.getByRole('checkbox', { name: longLabel }).element();
		const label = page.getByText(longLabel).element();

		expect(label.getBoundingClientRect().top).toBeLessThan(box.getBoundingClientRect().bottom);
		expect(label.getBoundingClientRect().right).toBeLessThanOrEqual(320);
	});
});
