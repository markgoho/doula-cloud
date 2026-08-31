import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import SignContract from './SignContract.svelte';

async function setup() {
	const onSign = vi.fn().mockResolvedValue(undefined);
	await render(SignContract, { onSign });
	return { onSign };
}

async function affirmDisclosure() {
	await page.getByRole('button', { name: 'I agree to sign electronically, continue' }).click();
}

describe('SignContract.svelte', () => {
	it('renders the ESIGN disclosure screen first, with no signing UI or path to onSign', async () => {
		const { onSign } = await setup();

		await expect
			.element(page.getByRole('heading', { name: 'Electronic signature disclosure' }))
			.toBeInTheDocument();
		await expect.element(page.getByLabelText('Full legal name')).not.toBeInTheDocument();
		await expect.element(page.getByRole('checkbox')).not.toBeInTheDocument();
		await expect
			.element(page.getByRole('button', { name: 'Sign', exact: true }))
			.not.toBeInTheDocument();
		expect(onSign).not.toHaveBeenCalled();
	});

	it('reveals the signature form once the disclosure is affirmed', async () => {
		await setup();

		await affirmDisclosure();

		await expect.element(page.getByLabelText('Full legal name')).toBeInTheDocument();
		await expect
			.element(page.getByLabelText('I have read this Contract and I am signing it electronically'))
			.toBeInTheDocument();
	});

	it('disables Sign until both a full legal name and the attestation are given', async () => {
		await setup();
		await affirmDisclosure();

		const submit = page.getByRole('button', { name: 'Sign' });
		await expect.element(submit).toBeDisabled();

		await page.getByLabelText('Full legal name').fill('Jamie Doe');
		await expect.element(submit).toBeDisabled();

		await page
			.getByLabelText('I have read this Contract and I am signing it electronically')
			.click();
		await expect.element(submit).toBeEnabled();
	});

	it('treats a whitespace-only name as not enough to enable Sign', async () => {
		await setup();
		await affirmDisclosure();

		await page.getByLabelText('Full legal name').fill(' '.repeat(3));
		await page
			.getByLabelText('I have read this Contract and I am signing it electronically')
			.click();

		await expect.element(page.getByRole('button', { name: 'Sign' })).toBeDisabled();
	});

	it('calls onSign with the trimmed name and attestation state on submit', async () => {
		const { onSign } = await setup();
		await affirmDisclosure();

		await page.getByLabelText('Full legal name').fill('  Jamie Doe  ');
		await page
			.getByLabelText('I have read this Contract and I am signing it electronically')
			.click();
		await page.getByRole('button', { name: 'Sign' }).click();

		expect(onSign).toHaveBeenCalledWith('Jamie Doe', true);
	});

	it('shows the error message when onSign rejects', async () => {
		const onSign = vi.fn().mockRejectedValue(new Error('contract is not awaiting signature'));
		await render(SignContract, { onSign });
		await affirmDisclosure();

		await page.getByLabelText('Full legal name').fill('Jamie Doe');
		await page
			.getByLabelText('I have read this Contract and I am signing it electronically')
			.click();
		await page.getByRole('button', { name: 'Sign' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('contract is not awaiting signature');
	});

	it('shows a generic error message when onSign rejects with a non-Error value', async () => {
		const onSign = vi.fn().mockRejectedValue('boom');
		await render(SignContract, { onSign });
		await affirmDisclosure();

		await page.getByLabelText('Full legal name').fill('Jamie Doe');
		await page
			.getByLabelText('I have read this Contract and I am signing it electronically')
			.click();
		await page.getByRole('button', { name: 'Sign' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Failed to sign');
	});

	/*
	 * Regression, #510: the consent checkbox's label is the longest an
	 * inline field carries anywhere in the app, and it orphaned onto its own
	 * line at 320px (ADR-0024). The viewport has to be narrowed after
	 * `affirmDisclosure`, not before -- the disclosure screen is what a
	 * static sweep of the style guide's own fixture sees, and it has no
	 * checkbox in it at all, which is why the earlier defect went unnoticed.
	 */
	it('checks the consent checkbox with its label attached, at 320px', async () => {
		await setup();
		await affirmDisclosure();
		await page.viewport(320, 700);

		const labelText = 'I have read this Contract and I am signing it electronically';
		const control = page.getByLabelText(labelText);
		await control.click();

		await expect.element(control).toBeChecked();
		const label = page.getByText(labelText).element();
		expect(label.getBoundingClientRect().top).toBeLessThan(control.element().getBoundingClientRect().bottom);
	});
});
