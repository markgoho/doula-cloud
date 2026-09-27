import { createRawSnippet } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Details from './Details.svelte';

interface SetupOptions {
	summary?: string;
	hiddenSummary?: string;
	open?: boolean;
	onToggle?: (isOpen: boolean) => void;
}

/*
 * `getByRole('button', ...)` does not resolve a native <summary> in this
 * stack -- Chromium exposes it under an internal disclosure role that
 * Playwright's accessibility-tree lookup does not map to "button", the
 * same reason HistoryDisclosure's own spec drives its <details> through
 * `getByText` rather than a role query. `open` on the element (case 3,
 * `.claude/rules/svelte-tests.md`) is the fact under test: it is not in
 * the accessible tree at all, it is the platform state the tree is
 * computed FROM.
 */
async function setup({ summary = 'Show more', hiddenSummary, open, onToggle }: SetupOptions = {}) {
	const { container } = await render(Details, {
		summary,
		hiddenSummary,
		open,
		onToggle,
		children: createRawSnippet(() => ({ render: () => '<p>The disclosed content</p>' }))
	});
	const fullSummaryText = hiddenSummary ? `${summary} ${hiddenSummary}` : summary;
	return {
		container,
		details: container.querySelector('details')!,
		summaryText: page.getByText(fullSummaryText)
	};
}

describe('Details', () => {
	it('starts closed unless the caller opens it', async () => {
		const { details } = await setup();

		await expect.element(page.getByText('The disclosed content')).not.toBeVisible();
		expect(details.open).toBe(false);
	});

	it('renders open when the caller sets open', async () => {
		const { details } = await setup({ open: true });

		await expect.element(page.getByText('The disclosed content')).toBeVisible();
		expect(details.open).toBe(true);
	});

	it('opens on a click of the summary', async () => {
		const { summaryText, details } = await setup();

		await summaryText.click();

		await expect.element(page.getByText('The disclosed content')).toBeVisible();
		expect(details.open).toBe(true);
	});

	it('opens on the keyboard, with no mouse involved', async () => {
		const { container, details } = await setup();
		container.querySelector('summary')!.focus();

		await userEvent.keyboard('{Enter}');

		await expect.element(page.getByText('The disclosed content')).toBeVisible();
		expect(details.open).toBe(true);
	});

	it('appends the hidden text to the summary’s accessible name only', async () => {
		const { container } = await setup({ summary: 'Membership history', hiddenSummary: 'for Renata Alvarez' });

		await expect.element(page.getByText('Membership history for Renata Alvarez')).toBeVisible();
		const hidden = container.querySelector('.visually-hidden');
		expect(hidden).toHaveTextContent('for Renata Alvarez');
	});

	it('renders no hidden span when the caller passes no hiddenSummary', async () => {
		const { container } = await setup();

		expect(container.querySelector('.visually-hidden')).toBeNull();
	});

	it('calls onToggle with the new open state', async () => {
		const onToggle = vi.fn();
		const { summaryText } = await setup({ onToggle });

		await summaryText.click();
		await expect.element(page.getByText('The disclosed content')).toBeVisible();

		expect(onToggle).toHaveBeenCalledWith(true);
	});

	it('calls onToggle again when closed', async () => {
		const onToggle = vi.fn();
		const { summaryText } = await setup({ open: true, onToggle });

		await summaryText.click();
		await expect.element(page.getByText('The disclosed content')).not.toBeVisible();

		expect(onToggle).toHaveBeenCalledWith(false);
	});

	it('adds no ARIA over the native element', async () => {
		const { details } = await setup();
		const summary = details.querySelector('summary')!;

		expect([...details.attributes].some((attribute) => attribute.name.startsWith('aria-'))).toBe(false);
		expect([...summary.attributes].some((attribute) => attribute.name.startsWith('aria-'))).toBe(false);
	});
});
