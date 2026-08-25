import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import MembershipFields from './MembershipFields.svelte';

interface SetupOptions {
	roles?: string[];
	employmentType?: 'employee' | 'contractor';
}

async function setup({ roles = ['doula'], employmentType = 'employee' }: SetupOptions = {}) {
	const onRolesChange = vi.fn();
	const onEmploymentTypeChange = vi.fn();
	await render(MembershipFields, {
		roles,
		employmentType,
		onRolesChange,
		onEmploymentTypeChange
	});
	return { onRolesChange, onEmploymentTypeChange };
}

describe('MembershipFields', () => {
	it('checks the roles the membership already holds', async () => {
		await setup({ roles: ['owner', 'doula'] });

		await expect.element(page.getByRole('checkbox', { name: 'Owner' })).toBeChecked();
		await expect.element(page.getByRole('checkbox', { name: 'Doula' })).toBeChecked();
		await expect.element(page.getByRole('checkbox', { name: 'Admin' })).not.toBeChecked();
	});

	// The order is the component's own, not the click order, so one set of
	// roles always reads the same way wherever it is shown.
	it('adds a checked role in the order the fieldset lists them', async () => {
		const { onRolesChange } = await setup({ roles: ['doula'] });

		await page.getByRole('checkbox', { name: 'Owner' }).click();

		expect(onRolesChange).toHaveBeenCalledWith(['owner', 'doula']);
	});

	it('removes an unchecked role', async () => {
		const { onRolesChange } = await setup({ roles: ['owner', 'doula'] });

		await page.getByRole('checkbox', { name: 'Owner' }).click();

		expect(onRolesChange).toHaveBeenCalledWith(['doula']);
	});

	it('reports the employment type chosen', async () => {
		const { onEmploymentTypeChange } = await setup({ employmentType: 'employee' });

		await page.getByRole('radio', { name: 'Contractor' }).click();

		expect(onEmploymentTypeChange).toHaveBeenCalledWith('contractor');
	});

	it('selects the employment type the membership already carries', async () => {
		await setup({ employmentType: 'contractor' });

		await expect.element(page.getByRole('radio', { name: 'Contractor' })).toBeChecked();
	});
});
