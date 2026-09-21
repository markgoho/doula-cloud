import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import SignContract from './SignContract.svelte';
import { SERVICE_PROBLEM } from '#lib/formErrors.js';

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

	// #1228: Sign is never disabled -- ADR-0021's pattern is a refusal
	// through the summary, not a control withheld until the answer looks
	// right (`required`'s own browser bubble did that job before
	// StackedForm's `novalidate` took it away).
	it('refuses an empty submit through the summary, naming both fields', async () => {
		const { onSign } = await setup();
		await affirmDisclosure();

		await page.getByRole('button', { name: 'Sign' }).click();

		expect(onSign).not.toHaveBeenCalled();
		await expect.element(page.getByText('There is a problem')).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Enter your full legal name' })).toBeVisible();
		await expect
			.element(
				page.getByRole('link', {
					name: 'Confirm that you have read the Contract and are signing it electronically'
				})
			)
			.toBeVisible();
	});

	it('refuses a submit with the name filled but the attestation unticked', async () => {
		const { onSign } = await setup();
		await affirmDisclosure();

		await page.getByLabelText('Full legal name').fill('Jamie Doe');
		await page.getByRole('button', { name: 'Sign' }).click();

		expect(onSign).not.toHaveBeenCalled();
		// .first() -- #1228's ErrorSummary repeats the message as a link, so
		// it now appears twice on screen (the summary and the field).
		await expect
			.element(page.getByText('Confirm that you have read the Contract and are signing it electronically').first())
			.toBeVisible();
	});

	it('treats a whitespace-only name as not enough', async () => {
		const { onSign } = await setup();
		await affirmDisclosure();

		await page.getByLabelText('Full legal name').fill(' '.repeat(3));
		await page
			.getByLabelText('I have read this Contract and I am signing it electronically')
			.click();
		await page.getByRole('button', { name: 'Sign' }).click();

		expect(onSign).not.toHaveBeenCalled();
		await expect.element(page.getByText('Enter your full legal name').first()).toBeVisible();
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

	it('falls back to the service problem message when onSign rejects with a non-Error value', async () => {
		const onSign = vi.fn().mockRejectedValue('boom');
		await render(SignContract, { onSign });
		await affirmDisclosure();

		await page.getByLabelText('Full legal name').fill('Jamie Doe');
		await page
			.getByLabelText('I have read this Contract and I am signing it electronically')
			.click();
		await page.getByRole('button', { name: 'Sign' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent(SERVICE_PROBLEM);
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
