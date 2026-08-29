import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import WorkStateField from './WorkStateField.svelte';

describe('WorkStateField', () => {
	it('asks which state the person works from', async () => {
		await render(WorkStateField, { value: '' });
		await expect.element(page.getByLabelText('Which state do you work from?')).toBeVisible();
	});

	// The reason is stated because the question is personal and its purpose
	// is not obvious, and it is wired to the control with aria-describedby
	// rather than merely sitting near it (#415).
	it('states why it is asked, and describes the control', async () => {
		await render(WorkStateField, { value: '' });
		const select = page.getByLabelText('Which state do you work from?');
		await expect.element(select).toBeVisible();
		const control = await select.element();
		const describedBy = control.getAttribute('aria-describedby');
		expect(describedBy).toBeTruthy();
		const hint = document.querySelector(`#${describedBy}`);
		expect(hint?.textContent).toContain('not your address');
	});

	it('offers every state, and starts on none of them', async () => {
		await render(WorkStateField, { value: '' });
		const select = (await page
			.getByLabelText('Which state do you work from?')
			.element()) as HTMLSelectElement;
		// 51 states plus the "Choose a state" placeholder.
		expect(select.options).toHaveLength(52);
		expect(select.value).toBe('');
	});

	it('shows the state it was given', async () => {
		await render(WorkStateField, { value: 'New York' });
		const select = (await page
			.getByLabelText('Which state do you work from?')
			.element()) as HTMLSelectElement;
		expect(select.value).toBe('New York');
	});
});
