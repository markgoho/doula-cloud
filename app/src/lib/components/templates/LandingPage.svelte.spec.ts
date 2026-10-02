import '#lib/styles/app.css';
import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
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
 * That is a contract between two files with nothing else to hold it: this
 * Template marks its root `data-fills-main`, and base.css answers
 * `main:has(> [data-fills-main])`. Rename either side and no type, lint
 * rule or other spec notices; the panels just stop short of the window
 * again. So the Template is mounted in a real `<main>` in `<body>`, as its
 * own child, which is where the app puts it.
 *
 * What the rules then do with that height is CSS's business and is not
 * asserted here.
 */
describe('LandingPage.svelte in the shell', () => {
	it('is given the height of a window taller than its content', async () => {
		await page.viewport(1280, 1600);
		const main = document.createElement('main');
		document.body.append(main);
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
		expect(Math.round(welcome.bottom)).toBeGreaterThanOrEqual(window.innerHeight - 1);
		main.remove();
	});
});
