import { page as testPage, userEvent } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { fillUtmFields } from '#lib/waitlist.js';
import WaitlistCard from './WaitlistCard.svelte';

const action = 'https://buttondown.example/api/emails/embed-subscribe/doulacloud';

describe('WaitlistCard, with Buttondown’s address (#358)', () => {
	it('posts to it, with Buttondown’s seven field names exactly', async () => {
		const { container } = await render(WaitlistCard, { action });
		const form = container.querySelector('form');
		expect(form?.getAttribute('method')).toBe('post');
		expect(form?.getAttribute('action')).toBe(action);
		expect([...(form?.elements ?? [])].map((element) => (element as HTMLInputElement).name).filter(Boolean)).toEqual([
			'metadata__first_name',
			'email',
			'metadata__pilot_interest',
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

	it('goes First name, Email address, the pilot box, its terms, then the button, by keyboard', async () => {
		await render(WaitlistCard, { action });
		// The first field takes focus by hand, since where the test
		// runner's own page leaves focus is not the card's to decide.
		(testPage.getByLabelText('First name').element() as HTMLElement).focus();
		await expect.element(testPage.getByLabelText('First name')).toHaveFocus();
		await userEvent.tab();
		await expect.element(testPage.getByLabelText('Email address')).toHaveFocus();
		await userEvent.tab();
		await expect.element(testPage.getByLabelText("I'd like to be in the pilot group")).toHaveFocus();
		await userEvent.tab();
		await expect.element(testPage.getByRole('link', { name: 'pilot terms' })).toHaveFocus();
		await userEvent.tab();
		await expect.element(testPage.getByRole('button', { name: 'Add me to the list' })).toHaveFocus();
	});

	it('carries the fine print and a link to the Privacy Policy beside it', async () => {
		await render(WaitlistCard, { action });
		await expect
			.element(testPage.getByText('A confirmation email now, a welcome once you confirm, and one when we open.', { exact: false }))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Privacy Policy' })).toHaveAttribute('href', '/privacy');
	});

	// A signup with an empty campaign input would clear the tag Buttondown
	// already holds for that address (#1771), so an input the script has
	// not filled is not part of the submission. A script inserted as
	// markup does not run, so a render here is a page with the script
	// blocked.
	it('submits only the name and the address while no campaign input is filled (#1771)', async () => {
		const { container } = await render(WaitlistCard, { action });
		await testPage.getByLabelText('First name').fill('Ada');
		await testPage.getByLabelText('Email address').fill('ada@example.com');
		await expect.element(testPage.getByRole('button', { name: 'Add me to the list' })).toBeVisible();
		expect([...new FormData(container.querySelector('form')!)]).toEqual([
			['metadata__first_name', 'Ada'],
			['email', 'ada@example.com']
		]);
	});

	it('submits each campaign input the script fills, with its value (#1771)', async () => {
		const { container } = await render(WaitlistCard, { action });
		await testPage.getByLabelText('Email address').fill('ada@example.com');
		fillUtmFields('?utm_source=dona-summit-2026&utm_campaign=booth');
		expect([...new FormData(container.querySelector('form')!)]).toEqual([
			['email', 'ada@example.com'],
			['metadata__utm_source', 'dona-summit-2026'],
			['metadata__utm_campaign', 'booth']
		]);
	});

	// A blank first name would clear the name Buttondown already holds for
	// that address (#1898), so the script leaves it out of the submission.
	it('submits a blank first name only while the script is blocked (#1898)', async () => {
		const { container } = await render(WaitlistCard, { action });
		await testPage.getByLabelText('Email address').fill('ada@example.com');
		await expect.element(testPage.getByLabelText('First name')).toBeVisible();
		const form = container.querySelector('form')!;
		expect([...new FormData(form)]).toEqual([
			['metadata__first_name', ''],
			['email', 'ada@example.com']
		]);
		fillUtmFields('');
		expect([...new FormData(form)]).toEqual([['email', 'ada@example.com']]);
	});

	// The pilot box (#2179): optional, its hint read with it, and the pilot
	// terms beside it, where a person asks to take part (#444).
	it('offers the pilot as an optional box, with the pilot terms in its hint', async () => {
		await render(WaitlistCard, { action });
		const box = testPage.getByRole('checkbox', { name: "I'd like to be in the pilot group" });
		await expect.element(box).not.toBeChecked();
		await expect.element(box).not.toBeRequired();
		await expect
			.element(box)
			.toHaveAccessibleDescription(
				"A small private pilot runs from now until January, and I'll write to you myself about it. What it gives and what it costs are in the pilot terms."
			);
		await expect.element(testPage.getByRole('link', { name: 'pilot terms' })).toHaveAttribute('href', '/pilot-terms');
	});

	// An unticked checkbox is never submitted, so it cannot clear a value
	// Buttondown holds (#1771); the tests above submit the form unticked.
	it('submits the pilot answer only when the box is ticked (#2179)', async () => {
		const { container } = await render(WaitlistCard, { action });
		await testPage.getByLabelText('Email address').fill('ada@example.com');
		await testPage.getByRole('checkbox', { name: "I'd like to be in the pilot group" }).click();
		await expect.element(testPage.getByRole('checkbox', { name: "I'd like to be in the pilot group" })).toBeChecked();
		fillUtmFields('');
		expect([...new FormData(container.querySelector('form')!)]).toEqual([
			['email', 'ada@example.com'],
			['metadata__pilot_interest', 'yes']
		]);
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
