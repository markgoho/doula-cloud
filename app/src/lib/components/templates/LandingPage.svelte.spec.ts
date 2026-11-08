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
			title: 'Sign in',
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
			.element(page.getByRole('heading', { level: 1, name: 'Sign in' }))
			.toBeVisible();
		expect(page.getByRole('heading').all()).toHaveLength(1);
		expect(document.title).toContain('Sign in');
	});

	// ADR-0021: the h1 says what the page is for; the greeting and the lede
	// are the one line under it, so reading order is visual order.
	it('greets on the line under the title, with the content after it', async () => {
		await setup();

		const title = page.getByRole('heading', { level: 1 }).element().getBoundingClientRect();
		const greeting = page.getByText('Good morning.', { exact: true });
		await expect.element(greeting).toBeVisible();
		await expect.element(page.getByText('Welcome to DoulaCloud.', { exact: false })).toBeVisible();
		const doors = page.getByText('The doors');
		await expect.element(doors).toBeVisible();
		const line = greeting.element().getBoundingClientRect();
		expect(line.top).toBeGreaterThanOrEqual(title.bottom);
		expect(doors.element().getBoundingClientRect().top).toBeGreaterThanOrEqual(line.bottom);
	});

	// ADR-0024: one column at 320px, nothing wider than the page.
	it('keeps one column at 320px with nothing wider than the page', async () => {
		await page.viewport(320, 640);
		await setup();

		const title = page.getByRole('heading', { level: 1 }).element().getBoundingClientRect();
		const doors = page.getByText('The doors').element().getBoundingClientRect();
		expect(doors.top).toBeGreaterThan(title.bottom);
		expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(320);
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
				title: 'Sign in',
				greeting: 'Good morning.',
				lede: 'Welcome to DoulaCloud.',
				content: textSnippet('The doors')
			}
		});

		// querySelector, case 2 of svelte-tests.md: the root is a layout box
		// with no role of its own.
		const root = main.querySelector('[data-fills-main]')!.getBoundingClientRect();
		expect(Math.round(root.bottom)).toBeGreaterThanOrEqual(window.innerHeight - 1);
		main.remove();
	});
});
