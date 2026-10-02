import '#lib/styles/app.css';
import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { beforeAll, describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import LandingPage from './LandingPage.svelte';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

type SetupOptions = Partial<ComponentProps<typeof LandingPage>>;

async function setup(overrides: SetupOptions = {}) {
	return render(LandingPage, {
		props: {
			title: 'Sign in or set up a Practice',
			greeting: 'Good morning.',
			lede: 'Welcome to DoulaCloud.',
			content: textSnippet('The doors'),
			...overrides
		}
	});
}

describe('LandingPage.svelte', () => {
	it('names the page with its title, as the one h1 and in the tab', async () => {
		await setup();

		await expect
			.element(page.getByRole('heading', { level: 1, name: 'Sign in or set up a Practice' }))
			.toBeVisible();
		expect(page.getByRole('heading').all()).toHaveLength(1);
		expect(document.title).toContain('Sign in or set up a Practice');
	});

	// ADR-0021: the h1 says what the page is for; the greeting is a
	// paragraph, so it adds nothing to the heading outline.
	it('greets as a paragraph, with the line under it, and renders the content', async () => {
		await setup();

		const greeting = page.getByText('Good morning.');
		await expect.element(greeting).toBeVisible();
		expect(greeting.element().tagName).toBe('P');
		await expect.element(page.getByText('Welcome to DoulaCloud.')).toBeVisible();
		await expect.element(page.getByText('The doors')).toBeVisible();
	});

	it('draws the mark as decoration, since the bar above already names the product', async () => {
		const { container } = await setup();

		// querySelector, case 2 of svelte-tests.md: the mark is aria-hidden.
		const mark = container.querySelector('svg')!;
		expect(mark).toHaveAttribute('aria-hidden', 'true');
		expect(mark).toHaveAttribute('width', '200');
	});

	it('sets the two panels side by side where there is room', async () => {
		await page.viewport(1280, 800);
		await setup();

		const greeting = page.getByText('Good morning.').element().getBoundingClientRect();
		const title = page.getByRole('heading', { level: 1 }).element().getBoundingClientRect();
		expect(title.left).toBeGreaterThan(greeting.right);
	});

	// ADR-0024: at 320px the panels stack, nothing is wider than the page,
	// and the mark is drawn smaller than 200px rather than overflowing.
	it('stacks the panels at 320px with nothing wider than the page', async () => {
		await page.viewport(320, 640);
		const { container } = await setup();

		const greeting = page.getByText('Good morning.').element().getBoundingClientRect();
		const title = page.getByRole('heading', { level: 1 }).element().getBoundingClientRect();
		expect(title.top).toBeGreaterThan(greeting.bottom);
		expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(320);
		expect(container.querySelector('svg')!.getBoundingClientRect().width).toBeLessThan(200);
	});
});

/*
 * #1653: the shell owns the window's height and this Template fills it.
 * Mounted inside a real `<main>` in `<body>`, because the height comes
 * from base.css's rules on those two elements and from nowhere else.
 */
async function setupInShell() {
	const main = document.createElement('main');
	document.body.append(main);
	// `target`, so the Template's root is `main`'s own child, as it is in
	// the app: base.css reads `main:has(> [data-fills-main])`.
	await render(LandingPage, {
		target: main,
		props: {
			title: 'Sign in or set up a Practice',
			greeting: 'Good morning.',
			lede: 'Welcome to DoulaCloud.',
			content: textSnippet('The doors')
		}
	});
	// querySelector, case 2 of svelte-tests.md: a panel is a layout box
	// with no role of its own.
	const welcome = main.querySelector('.welcome')!.getBoundingClientRect();
	const greeting = page.getByText('Good morning.').element().getBoundingClientRect();
	const lede = page.getByText('Welcome to DoulaCloud.').element().getBoundingClientRect();
	return { main, welcome, greeting, lede };
}

describe('LandingPage.svelte in the shell', () => {
	// The root layout registers these in the app. Without them `grid-l`
	// ignores its `min`, and the panels split at a width the app never uses.
	beforeAll(() => {
		registerLayoutPrimitives();
	});

	it('reaches the bottom of a window taller than its content, with the content centered', async () => {
		await page.viewport(1280, 1600);
		const { main, welcome, greeting, lede } = await setupInShell();

		expect(Math.round(welcome.bottom)).toBeGreaterThanOrEqual(window.innerHeight - 1);
		// Centered: free space both under the last line and over the
		// greeting. Top-aligned content would leave almost none over it.
		const spaceUnder = welcome.bottom - lede.bottom;
		const spaceOver = greeting.top - welcome.top;
		expect(spaceUnder).toBeGreaterThan(welcome.height / 4);
		expect(spaceOver).toBeGreaterThan(welcome.height / 4);
		expect(document.documentElement.scrollHeight).toBeLessThanOrEqual(window.innerHeight);
		main.remove();
	});

	it('centers the welcome block side to side, with the mark at its full size', async () => {
		await page.viewport(1600, 900);
		const { main, welcome, greeting } = await setupInShell();

		// querySelector, case 2 of svelte-tests.md: the block and the mark
		// are layout boxes with no role (the mark is aria-hidden).
		const block = main.querySelector(':scope .welcome > stack-l')!.getBoundingClientRect();
		const mark = main.querySelector('svg')!.getBoundingClientRect();
		const spaceBefore = block.left - welcome.left;
		const spaceAfter = welcome.right - block.right;
		expect(Math.abs(spaceBefore - spaceAfter)).toBeLessThanOrEqual(1);
		expect(spaceBefore).toBeGreaterThan(100);
		// One shared left edge inside the block.
		expect(Math.round(greeting.left)).toBe(Math.round(mark.left));
		expect(Math.round(mark.width)).toBe(200);
		main.remove();
	});

	it('starts the welcome at the gutter where the panels stack, level with the doors', async () => {
		await page.viewport(600, 900);
		const { main, greeting } = await setupInShell();

		const title = page.getByRole('heading', { level: 1 }).element().getBoundingClientRect();
		expect(title.top).toBeGreaterThan(greeting.bottom);
		expect(Math.round(greeting.left)).toBe(Math.round(title.left));
		main.remove();
	});

	it('scrolls in a window shorter than its content, with nothing cut off', async () => {
		await page.viewport(320, 300);
		const { main, welcome, lede } = await setupInShell();

		expect(document.documentElement.scrollHeight).toBeGreaterThan(window.innerHeight);
		expect(welcome.bottom).toBeGreaterThan(lede.bottom);
		expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(320);
		main.remove();
	});
});
