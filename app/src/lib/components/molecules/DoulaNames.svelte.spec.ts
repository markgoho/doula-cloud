import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it } from 'vitest';
import DoulaNames from './DoulaNames.svelte';

const THREE = ['Joss Adeyemi', 'Lena Vasquez', 'Priya Raman'];
const IDS = ['a', 'b', 'c'];

describe('DoulaNames', () => {
	it('draws the first name and a count in a table cell, and every name to a screen reader', async () => {
		await render(DoulaNames, { names: THREE, ids: IDS, isTable: true });

		await expect.element(page.getByText('Joss Adeyemi +2')).toBeVisible();
		expect(page.getByText('Joss Adeyemi, Lena Vasquez, Priya Raman').elements()).toHaveLength(1);
	});

	it('calls the reader You, first', async () => {
		await render(DoulaNames, { names: THREE, ids: IDS, readerId: 'b', isTable: true });

		await expect.element(page.getByText('You +2')).toBeVisible();
	});

	it('says every name in a card', async () => {
		await render(DoulaNames, { names: THREE, ids: IDS, readerId: 'c' });

		await expect.element(page.getByText('You, Joss Adeyemi, Lena Vasquez')).toBeVisible();
	});

	it('draws both names of two that fit', async () => {
		await render(DoulaNames, { names: ['Jo Ames', 'Li Cho'], isTable: true, prefix: 'Birth: ' });

		await expect.element(page.getByText('Birth: Jo Ames, Li Cho')).toBeVisible();
	});
});
