import { page as testPage, userEvent } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import WaitlistCard from './WaitlistCard.svelte';

const action = 'https://buttondown.example/api/emails/embed-subscribe/doulacloud';

describe('WaitlistCard, with Buttondown’s address (#358)', () => {
	it('posts to it, with Buttondown’s six field names exactly', async () => {
		const { container } = await render(WaitlistCard, { action });
		const form = container.querySelector('form');
		expect(form?.getAttribute('method')).toBe('post');
		expect(form?.getAttribute('action')).toBe(action);
		expect([...(form?.elements ?? [])].map((element) => (element as HTMLInputElement).name).filter(Boolean)).toEqual([
			'metadata__first_name',
			'email',
			'metadata__utm_source',
			'metadata__utm_medium',
			'metadata__utm_campaign',
			'metadata__utm_content'
		]);
	});

	it('refuses only an empty or malformed address, in the browser', async () => {
		await render(WaitlistCard, { action });
		const email = testPage.getByLabelText('Email address');
		await expect.element(email).toHaveAttribute('type', 'email');
		await expect.element(email).toBeRequired();
		await expect.element(testPage.getByLabelText('First name')).not.toBeRequired();
	});

	it('goes First name, Email address, then the button, by keyboard', async () => {
		await render(WaitlistCard, { action });
		// The first field takes focus by hand, since where the test
		// runner's own page leaves focus is not the card's to decide.
		(testPage.getByLabelText('First name').element() as HTMLElement).focus();
		await expect.element(testPage.getByLabelText('First name')).toHaveFocus();
		await userEvent.tab();
		await expect.element(testPage.getByLabelText('Email address')).toHaveFocus();
		await userEvent.tab();
		await expect.element(testPage.getByRole('button', { name: 'Add me to the list' })).toHaveFocus();
	});

	it('carries the fine print and a link to the Privacy Policy beside it', async () => {
		await render(WaitlistCard, { action });
		await expect
			.element(testPage.getByText('One confirmation email now, one when we open.', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Privacy Policy' })).toHaveAttribute('href', '/privacy');
	});

	it('carries the script that fills the hidden inputs, after them', async () => {
		const { container } = await render(WaitlistCard, { action });
		const script = container.querySelector('script');
		expect(script?.textContent).toContain('(location.search);');
		expect(container.querySelector('form')?.compareDocumentPosition(script!)).toBe(
			Node.DOCUMENT_POSITION_FOLLOWING
		);
	});
});

describe('WaitlistCard, before the list has an address', () => {
	it('shows no form, and says the list is not open yet', async () => {
		const { container } = await render(WaitlistCard, {});
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Join the waitlist' })).toBeVisible();
		await expect.element(testPage.getByText("The list isn't open yet.", { exact: false })).toBeVisible();
		expect(container.querySelector('form, script')).toBeNull();
	});
});
