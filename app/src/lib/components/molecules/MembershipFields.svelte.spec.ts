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

	/*
	 * #510 non-regression: LabeledField's inline row moved from cluster-l to
	 * a grid, and these three short-label checkboxes were the other
	 * inline-orientation consumer that grid had to keep working for. None of
	 * these labels are long enough to wrap, but the row still has to hold
	 * together at 320px (ADR-0024) rather than only at whatever width a
	 * default viewport happens to run tests at.
	 */
	it('keeps every role checkbox visible and clickable at 320px', async () => {
		await page.viewport(320, 600);
		const { onRolesChange } = await setup({ roles: ['doula'] });

		for (const role of ['Owner', 'Admin', 'Doula']) {
			await expect.element(page.getByRole('checkbox', { name: role })).toBeVisible();
		}

		await page.getByRole('checkbox', { name: 'Owner' }).click();
		expect(onRolesChange).toHaveBeenCalledWith(['owner', 'doula']);
	});
});
