import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Link from './Link.svelte';
import LinkInSentence from './harness/LinkInSentence.svelte';
/*
 * The app's real stylesheet (#1856). Without it there is no `reset`
 * layer, so a global rule that undoes this atom's inline sentence styles
 * -- `reset.css`'s `svg { display: block }` was the one -- cannot be
 * seen here. It comes after the component imports on purpose: the layer
 * order is declared before any spec's imports run (`src/vitest.browser.setup.ts`),
 * so where this line sits no longer changes the cascade.
 */
import '#lib/styles/app.css';

type SetupOptions = Partial<ComponentProps<typeof Link>>;

async function setup({ label = 'Docs', href = '/docs', ...rest }: SetupOptions = {}) {
	const { container } = await render(Link, { label, href, ...rest });
	return { container };
}

describe('Link.svelte', () => {
	it('renders the label as a visible link', async () => {
		await setup({ label: 'Docs' });

		await expect.element(page.getByRole('link', { name: 'Docs' })).toBeVisible();
	});

	it('renders the given href', async () => {
		await setup({ href: '/practices/1' });

		await expect.element(page.getByRole('link')).toHaveAttribute('href', '/practices/1');
	});

	// #1829: the Client search hands intake what was typed as its link is
	// followed. The href in this spec goes nowhere, so the click is
	// stopped to keep the test page where it is.
	it('calls onClick as the link is followed', async () => {
		const onClick = vi.fn((event: MouseEvent) => event.preventDefault());
		await setup({ href: '#nowhere', onClick });

		await page.getByRole('link').click();

		expect(onClick).toHaveBeenCalledOnce();
	});

	it('defaults to the primary variant class', async () => {
		await setup();

		await expect.element(page.getByRole('link')).toHaveClass('primary');
	});

	it('reflects a non-default variant', async () => {
		await setup({ variant: 'secondary' });

		await expect.element(page.getByRole('link')).toHaveClass('secondary');
	});

	it('does not set target or rel for an internal (relative) href', async () => {
		await setup({ href: '/practices/1' });

		await expect.element(page.getByRole('link')).not.toHaveAttribute('target');
		await expect.element(page.getByRole('link')).not.toHaveAttribute('rel');
	});

	it('opens an absolute http(s) href in a new tab with a safe rel', async () => {
		await setup({ href: 'https://example.com' });

		await expect.element(page.getByRole('link')).toHaveAttribute('target', '_blank');
		await expect.element(page.getByRole('link')).toHaveAttribute('rel', 'noopener noreferrer');
	});

	it('treats a protocol-relative href as external', async () => {
		await setup({ href: '//example.com' });

		await expect.element(page.getByRole('link')).toHaveAttribute('target', '_blank');
	});

	it('shows an external-link icon and includes "(opens in new tab)" in the accessible name', async () => {
		const { container } = await setup({ href: 'https://example.com', label: 'Visit site' });

		expect(container.querySelector('svg')).toBeVisible();
		await expect
			.element(page.getByRole('link', { name: 'Visit site (opens in new tab)' }))
			.toBeVisible();
	});

	// GOV.UK's Links style (#1557): a sighted reader is told too, not only
	// a screen reader, so the words are drawn in the link's own text.
	it('draws "(opens in new tab)" as visible text of an external link', async () => {
		await setup({ href: 'https://example.com', label: 'Visit site' });

		await expect.element(page.getByText('Visit site (opens in new tab)')).toBeVisible();
	});

	// #1808: a sentence that ends with an external link reads its full
	// stop straight after the link, not as an item of its own after a
	// space.
	it('reads a sentence that ends with an external link with no space before its full stop', async () => {
		await render(LinkInSentence, { href: 'https://example.com', label: 'Privacy Policy' });

		await expect
			.element(page.getByRole('button', { name: 'Continue' }))
			.toHaveAccessibleDescription('Read the Privacy Policy (opens in new tab).');
	});

	it('renders no icon and no "opens in new tab" text for an internal link', async () => {
		const { container } = await setup({ href: '/docs' });

		expect(container.querySelector('svg')).not.toBeInTheDocument();
		await expect
			.element(page.getByRole('link', { name: 'Docs (opens in new tab)' }))
			.not.toBeInTheDocument();
	});

	it('renders a leading icon and drops the underline when icon is given', async () => {
		const { container } = await setup({ icon: 'users' });

		expect(container.querySelector('svg')).toBeVisible();
		await expect.element(page.getByRole('link')).toHaveClass('has-icon');
	});

	it('has no icon and keeps the underline class off when icon is omitted', async () => {
		await setup();

		await expect.element(page.getByRole('link')).not.toHaveClass('has-icon');
	});

	it('marks the link as the current page when current is true', async () => {
		await setup({ current: true });

		await expect.element(page.getByRole('link')).toHaveAttribute('aria-current', 'page');
	});

	it('has no aria-current when current is false', async () => {
		await setup();

		await expect.element(page.getByRole('link')).not.toHaveAttribute('aria-current');
	});

	it('reflects the card variant class', async () => {
		await setup({ variant: 'card', icon: 'users' });

		await expect.element(page.getByRole('link')).toHaveClass('card');
	});

	it('reflects the rail variant class, for a contents entry', async () => {
		await setup({ variant: 'rail' });

		await expect.element(page.getByRole('link')).toHaveClass('rail');
	});

	it('reflects the chip variant class, for a jump-to target', async () => {
		await setup({ variant: 'chip' });

		await expect.element(page.getByRole('link')).toHaveClass('chip');
	});

	// #2019: the applied filter chip carries the state its own rule draws.
	it('marks a current chip as the current page', async () => {
		await setup({ variant: 'chip', current: true });

		await expect.element(page.getByRole('link')).toHaveAttribute('aria-current', 'page');
	});

	// #2019: GOV.UK's start button, a link drawn as the primary button with
	// an arrow after its words; its name is still only the label. The arrow
	// is aria-hidden, so `querySelector` is the deliberately non-accessible
	// element case of `.claude/rules/svelte-tests.md`.
	it('renders the start variant with a trailing arrow and the label as its name', async () => {
		const { container } = await setup({ variant: 'start', label: 'Invite a Staff member', href: '/invite' });

		await expect.element(page.getByRole('link', { name: 'Invite a Staff member', exact: true })).toHaveClass('start');
		expect(container.querySelector(':scope a.start > .label + svg')).not.toBeNull();
	});

	it('draws no arrow on a link that is not a start button', async () => {
		const { container } = await setup({ variant: 'primary' });

		expect(container.querySelector(':scope a svg')).toBeNull();
	});

	// #464: a step in a journey rail takes the shell's nav treatment rather
	// than the link treatment, so the accent is spent once per screen.
	it('renders the step variant class', async () => {
		await setup({ variant: 'step' });

		await expect.element(page.getByRole('link')).toHaveClass('step');
	});

	// StepRail puts a step's status in a sibling element, so without this a
	// keyboard user tabbing the rail never hears it.
	it('joins itself to qualifying text elsewhere on the page', async () => {
		await setup({ describedBy: 'step-2-status' });

		await expect.element(page.getByRole('link')).toHaveAttribute('aria-describedby', 'step-2-status');
	});

	it('carries no aria-describedby when none is given', async () => {
		await setup();

		await expect.element(page.getByRole('link')).not.toHaveAttribute('aria-describedby');
	});
});
