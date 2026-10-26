import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Component } from 'svelte';
import { toPageState, type RouteFixture } from '../../routeFixture.js';
import { SignupDraft, signupDraft } from './signupJourney.svelte.js';
import YouPage from './+page.svelte';
import PracticePage from './practice/+page.svelte';
import WherePage from './where/+page.svelte';
import { fixture as youFixture } from './page.fixture.js';
import { fixture as practiceFixture } from './practice/page.fixture.js';
import { fixture as whereFixture } from './where/page.fixture.js';

/*
 * Setting up a Practice as three question pages (#2049). These are the
 * pages; the check page, where the Practice is created, has its own file
 * beside its route. The 320px conformance of each page is the continuum
 * sweep's, through the same fixtures.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const globalFetch = vi.hoisted(() => vi.fn());

beforeEach(() => {
	vi.stubGlobal('fetch', globalFetch);
	goto.mockReset();
	globalFetch.mockReset();
	signupDraft.clear();
});

interface SetupOptions {
	/**
	The page reached by a Change link, or by a refusal's link.
	*/
	isFromCheck?: boolean;
}

function setupFor(fixture: RouteFixture, Page: Component<never>) {
	const RoutePage = Page as Component;
	return async ({ isFromCheck = false }: SetupOptions = {}) => {
		const state = toPageState(fixture);
		if (isFromCheck) state.url = new URL(`${state.url.href}?from=check`);
		Object.assign(pageState, state);
		await render(RoutePage);
	};
}

const setupYou = setupFor(youFixture, YouPage);
const setupPractice = setupFor(practiceFixture, PracticePage);
const setupWhere = setupFor(whereFixture, WherePage);

const continueButton = () => testPage.getByRole('button', { name: 'Continue' });

async function fillYou(password = 'correct horse battery') {
	await testPage.getByLabelText('First name').fill('Priya');
	await testPage.getByLabelText('Last name').fill('Sharma');
	await testPage.getByLabelText('Email').fill('priya@example.com');
	await testPage.getByLabelText('Password').fill(password);
}

function summaryLinks() {
	return testPage
		.getByRole('alert')
		.getByRole('link')
		.elements()
		.map((link) => link.textContent?.trim());
}

describe('the first page: about you', () => {
	it('asks the question as the page heading, inside the journey', async () => {
		await setupYou();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'About you' })).toBeVisible();
		await expect.element(testPage.getByRole('navigation', { name: 'Setting up your Practice' })).toBeVisible();
		await expect.element(testPage.getByText('Step 1 of 3 · You')).toBeVisible();
		// Back from the first page is the screen with the door on it.
		await expect.element(testPage.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/');
	});

	it('asks for a first name and a last name with GOV.UK autocomplete tokens (#1537)', async () => {
		await setupYou();

		await expect.element(testPage.getByLabelText('First name')).toHaveAttribute('autocomplete', 'given-name');
		await expect.element(testPage.getByLabelText('Last name')).toHaveAttribute('autocomplete', 'family-name');
	});

	it('states the password rule before she types (#1538), and tells Safari (#1813)', async () => {
		await setupYou();

		await expect
			.element(testPage.getByText('Must be 15 characters or more, and not a commonly used password'))
			.toBeVisible();
		await expect.element(testPage.getByLabelText('Password')).toHaveAttribute('passwordrules', 'minlength: 15;');
	});

	it('refuses an empty Continue, every reason in the order of the boxes', async () => {
		await setupYou();
		await continueButton().click();

		await expect.element(testPage.getByRole('link', { name: 'Enter your first name' })).toBeVisible();
		expect(summaryLinks()).toEqual([
			'Enter your first name',
			'Enter your last name',
			'Enter your email address',
			'Enter a password'
		]);
		expect(goto).not.toHaveBeenCalled();
	});

	it('refuses a commonly used password, and sends it nowhere (#1539)', async () => {
		await setupYou();
		await fillYou('PasswordPassword');
		await continueButton().click();

		await expect
			.element(testPage.getByRole('link', { name: 'Password is too common. Enter a different password' }))
			.toBeVisible();
		// The list is in the browser: checking a password against it is no request at all.
		expect(globalFetch).not.toHaveBeenCalled();
		expect(goto).not.toHaveBeenCalled();
	});

	it('refuses a password of 14 characters', async () => {
		await setupYou();
		await fillYou('a'.repeat(14));
		await continueButton().click();

		await expect
			.element(testPage.getByRole('link', { name: 'Password must be 15 characters or more' }))
			.toBeVisible();
	});

	it('goes on to the Practice name, and keeps the answers for a reload but never the password', async () => {
		await setupYou();
		await fillYou('correct horse battery staple');
		await continueButton().click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/signup/practice'));
		const stored = sessionStorage.getItem('signup-draft') ?? '';
		expect(JSON.parse(stored)).toMatchObject({
			answers: { firstName: 'Priya', lastName: 'Sharma', email: 'priya@example.com' },
			visited: ['you']
		});
		expect(stored).not.toContain('correct horse');
		expect(signupDraft.password).toBe('correct horse battery staple');
	});

	it('goes back to the check page when reached from it', async () => {
		await setupYou({ isFromCheck: true });
		await fillYou();
		await continueButton().click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/signup/check'));
		await expect.element(testPage.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/signup/check');
	});

	it("says what the check page's button was refused for, beside the box it is about", async () => {
		signupDraft.refusals = [
			{
				message: 'This email address already has an account. Sign in instead.',
				targetId: 'signup-email',
				href: '/signup?from=check'
			},
			{ message: 'Choose the state you work from', targetId: 'signup-work-state', href: '/signup/where?from=check' }
		];
		await setupYou({ isFromCheck: true });

		// Its own refusal, in its summary linking to its own box, and not
		// the refusal that belongs to another page.
		await expect
			.element(testPage.getByRole('link', { name: 'This email address already has an account. Sign in instead.' }))
			.toHaveAttribute('href', '#signup-email');
		await expect
			.element(testPage.getByLabelText('Email'))
			.toHaveAccessibleDescription('This email address already has an account. Sign in instead.');
		expect(testPage.getByText('Choose the state you work from').elements()).toHaveLength(0);

		await fillYou();
		await continueButton().click();
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/signup/check'));
		// Answered here, so no page shows it again; the other page's stays.
		expect(signupDraft.refusals.map((refusal) => refusal.targetId)).toEqual(['signup-work-state']);
	});
});

const field = () => testPage.getByLabelText('What is your Practice called? (optional)');

describe('the second page: the Practice name (#1536)', () => {
	it('says it is optional, and what an empty one does, before she types', async () => {
		await setupPractice();

		await expect.element(testPage.getByRole('heading', { level: 1, name: /What is your Practice called/ })).toBeVisible();
		await expect.element(field()).toHaveAttribute('autocomplete', 'organization');
		await expect.element(field()).not.toBeRequired();
		await expect
			.element(field())
			.toHaveAccessibleDescription('Clients see it on Contracts and card statements. Leave it empty to use your own name.');
	});

	it('goes on with the box empty, since nothing here is required', async () => {
		await setupPractice();
		await continueButton().click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/signup/where'));
		await expect.element(testPage.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/signup');
	});

	it('keeps what she types', async () => {
		await setupPractice();
		await field().fill('Riverside Doulas');
		await continueButton().click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalled());
		expect(signupDraft.answers.practiceName).toBe('Riverside Doulas');
	});
});

describe('the third page: where she works', () => {
	it('refuses a Continue with no state chosen, and goes on to the check page once one is', async () => {
		await setupWhere();
		await continueButton().click();

		await expect.element(testPage.getByRole('link', { name: 'Choose the state you work from' })).toBeVisible();
		expect(goto).not.toHaveBeenCalled();

		await testPage.getByRole('combobox', { name: 'Which state do you work from?' }).selectOptions('New Jersey');
		await testPage
			.getByRole('combobox', { name: 'What timezone does this Practice work in?' })
			.selectOptions('Central time (Chicago)');
		await continueButton().click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/signup/check'));
		expect(signupDraft.answers).toMatchObject({ workStateName: 'New Jersey', timezone: 'America/Chicago' });
		expect(signupDraft.visited).toEqual(['where']);
	});

	it('marks the pages she has continued from as completed in the rail', async () => {
		signupDraft.visit('you');
		signupDraft.visit('practice');
		await setupWhere();

		await expect.element(testPage.getByText('Step 3 of 3 · Where you work')).toBeVisible();
	});
});

function refuse(): never {
	throw new DOMException('denied', 'SecurityError');
}

describe('the draft through a reload (ADR-0059)', () => {
	it('is read back from the tab, every answer but the password', () => {
		sessionStorage.setItem(
			'signup-draft',
			JSON.stringify({ answers: { firstName: 'Priya', email: 'priya@example.com' }, visited: ['you'] })
		);
		const reloaded = new SignupDraft();

		expect(reloaded.answers).toMatchObject({ firstName: 'Priya', email: 'priya@example.com', lastName: '' });
		expect(reloaded.visited).toEqual(['you']);
		expect(reloaded.password).toBe('');
	});

	it('reads a stored copy with no list of pages as none visited', () => {
		sessionStorage.setItem('signup-draft', JSON.stringify({ answers: { firstName: 'Priya' } }));

		expect(new SignupDraft().visited).toEqual([]);
	});

	it('starts empty from a stored copy that is not JSON', () => {
		sessionStorage.setItem('signup-draft', '{not json');

		expect(new SignupDraft().answers.firstName).toBe('');
	});

	it('keeps working in a browser that refuses the storage', () => {
		const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(refuse);
		const removeItem = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(refuse);
		try {
			signupDraft.update({ firstName: 'Priya' });
			expect(signupDraft.answers.firstName).toBe('Priya');
			signupDraft.clear();
			expect(signupDraft.answers.firstName).toBe('');
		} finally {
			setItem.mockRestore();
			removeItem.mockRestore();
		}
	});
});
