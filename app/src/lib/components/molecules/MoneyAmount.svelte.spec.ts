import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import MoneyAmount from './MoneyAmount.svelte';

interface SetupOptions {
	amount?: string;
	state?: { word: string; variant?: 'error' };
	isTracked?: boolean;
}

async function setup({ amount = '$950.00', state, isTracked = false }: SetupOptions = {}) {
	const { container } = await render(MoneyAmount, { amount, state, isTracked });
	return { container };
}

describe('MoneyAmount.svelte', () => {
	it('says the amount, then the state, as one phrase', async () => {
		const { container } = await setup({ state: { word: 'paid' } });
		await expect.element(page.getByText('$950.00')).toBeVisible();
		await expect.element(page.getByText('paid')).toBeVisible();
		expect(container.textContent).toBe('$950.00, paid');
	});

	it('badges a state that asks something of the reader', async () => {
		await setup({ state: { word: '12 days overdue', variant: 'error' } });
		await expect.element(page.getByText('12 days overdue')).toBeVisible();
		// querySelector, case 2: a badge's color is drawn and not announced, so
		// only the drawn tree tells a badge from a plain word.
		expect(page.getByText('12 days overdue').element().closest('.error')).not.toBeNull();
	});

	it('says the amount alone with no state', async () => {
		const { container } = await setup();
		expect(container.textContent).toBe('$950.00');
	});

	it('ends every amount on one edge in its table form, whatever the state says', async () => {
		await setup({ state: { word: 'owed' }, isTracked: true });
		await render(MoneyAmount, { amount: '$12,950.00', state: { word: 'payment on its way' }, isTracked: true });
		const gaps = ['$950.00', '$12,950.00'].map((amount) => {
			const element = page.getByText(amount, { exact: true }).element();
			return element.parentElement!.getBoundingClientRect().right - element.getBoundingClientRect().right;
		});
		expect(gaps[0]).toBeCloseTo(gaps[1]!, 0);
	});
});
