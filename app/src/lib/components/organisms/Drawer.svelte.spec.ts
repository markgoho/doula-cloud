import { createRawSnippet } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Drawer from './Drawer.svelte';

const CONTENT = createRawSnippet(() => ({ render: () => '<p>drawer content</p>' }));
const HEADING = 'Send feedback to Doula Cloud';

/*
 * Past 28rem (448px) the panel renders at its own fixed 28rem and opens
 * non-modal (`show()`); under it, `min(28rem, 100%)` resolves to 100% and
 * it opens as a modal (`showModal()`) -- Drawer.svelte's own
 * `FULL_WIDTH_QUERY`. The viewport is pinned in every test rather than
 * left to the runner's default, the same reason `StaffTopBar.svelte.spec.ts`
 * pins one: a default under 28rem would make every "wide" assertion below
 * fail for a reason that has nothing to do with Drawer.
 */
const WIDE = [800, 900] as const;
const NARROW = [320, 700] as const;

async function setup({ open = false, heading = HEADING } = {}) {
	return render(Drawer, { open, heading, children: CONTENT });
}

describe('Drawer', () => {
	it('renders nothing visible when closed', async () => {
		await page.viewport(...WIDE);
		await setup();

		await expect.element(page.getByText('drawer content')).not.toBeVisible();
	});

	it('has a heading and a visible Close button once open', async () => {
		await page.viewport(...WIDE);
		await setup({ open: true });

		await expect
			.element(page.getByRole('heading', { level: 2, name: HEADING }))
			.toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Close' })).toBeVisible();
	});

	it('carries the heading as its accessible name', async () => {
		await page.viewport(...WIDE);
		await setup({ open: true });

		await expect.element(page.getByRole('dialog', { name: HEADING })).toBeVisible();
	});

	/*
	 * The whole point of never wrapping the dialog in `{#if open}`: a
	 * caller's own state inside `children` -- here a plain <input>, since
	 * the Feedback form's own unsent draft is exactly this shape -- is the
	 * same DOM node before and after a close, never torn down and rebuilt.
	 */
	it('keeps its contents mounted while closed, so a draft survives close and reopen', async () => {
		await page.viewport(...WIDE);
		const draft = createRawSnippet(() => ({
			render: () => '<label>Draft <input aria-label="Draft" /></label>'
		}));
		const { rerender } = await render(Drawer, { open: true, heading: HEADING, children: draft });
		await page.getByLabelText('Draft').fill('unsent text');

		await rerender({ open: false, heading: HEADING, children: draft });
		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
		await rerender({ open: true, heading: HEADING, children: draft });

		await expect.element(page.getByLabelText('Draft')).toHaveValue('unsent text');
	});
});

/*
 * A real button in the document, focused before the drawer opens and
 * handed back so a test can assert focus returned to it -- the same shape
 * `Dialog.svelte.spec.ts`'s own "gives focus back" test uses, `rerender`
 * rather than a click so the trigger is a plain DOM element the drawer
 * never has to know about (Drawer renders no trigger of its own).
 */
async function setupOpenFromTrigger() {
	const trigger = document.createElement('button');
	trigger.textContent = 'Open';
	document.body.append(trigger);
	trigger.focus();

	const rendered = await render(Drawer, { open: false, heading: HEADING, children: CONTENT });
	await rendered.rerender({ open: true, heading: HEADING, children: CONTENT });
	await expect.element(page.getByRole('dialog')).toBeVisible();

	return { ...rendered, trigger };
}

describe('when it takes the full width (a narrow screen)', () => {
	it('is modal: the background is inert and focus stays in it', async () => {
		await page.viewport(...NARROW);
		await setupOpenFromTrigger();

		expect(page.getByRole('dialog').element().contains(document.activeElement)).toBe(true);
	});

	it('closes on Escape and returns focus to the opener', async () => {
		await page.viewport(...NARROW);
		const { trigger } = await setupOpenFromTrigger();

		await userEvent.keyboard('{Escape}');

		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
		expect(document.activeElement).toBe(trigger);
		trigger.remove();
	});

	it('returns focus to the opener on Close', async () => {
		await page.viewport(...NARROW);
		const { trigger } = await setupOpenFromTrigger();

		await page.getByRole('button', { name: 'Close' }).click();

		expect(document.activeElement).toBe(trigger);
		trigger.remove();
	});

	it("returns focus to the opener on the caller's own close, for example after a send", async () => {
		await page.viewport(...NARROW);
		const { trigger, rerender } = await setupOpenFromTrigger();

		await rerender({ open: false, heading: HEADING, children: CONTENT });

		expect(document.activeElement).toBe(trigger);
		trigger.remove();
	});
});

describe('when it is narrower than the full width', () => {
	it('leaves the uncovered screen interactive', async () => {
		await page.viewport(...WIDE);
		const outside = document.createElement('button');
		outside.textContent = 'Elsewhere on the screen';
		document.body.append(outside);
		await setupOpenFromTrigger();

		outside.focus();

		expect(document.activeElement).toBe(outside);
		outside.remove();
	});

	it('still closes on Escape and returns focus to the opener', async () => {
		await page.viewport(...WIDE);
		const { trigger } = await setupOpenFromTrigger();

		await userEvent.keyboard('{Escape}');

		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
		expect(document.activeElement).toBe(trigger);
		trigger.remove();
	});

	/*
	 * The uncovered screen stays interactive, so a person can be focused
	 * out there when she presses Escape -- a listener on the dialog alone
	 * would never hear it.
	 */
	it('still closes on Escape while focus is on the uncovered screen', async () => {
		await page.viewport(...WIDE);
		const outside = document.createElement('button');
		outside.textContent = 'Elsewhere on the screen';
		document.body.append(outside);
		await setupOpenFromTrigger();
		outside.focus();

		await userEvent.keyboard('{Escape}');

		await expect.element(page.getByRole('dialog')).not.toBeInTheDocument();
		outside.remove();
	});

	it('does nothing on Escape while closed', async () => {
		await page.viewport(...WIDE);
		const { rerender } = await setup();

		await userEvent.keyboard('{Escape}');
		await rerender({ open: true, heading: HEADING, children: CONTENT });

		await expect.element(page.getByRole('dialog')).toBeVisible();
	});

	it('returns focus to the opener on Close', async () => {
		await page.viewport(...WIDE);
		const { trigger } = await setupOpenFromTrigger();

		await page.getByRole('button', { name: 'Close' }).click();

		expect(document.activeElement).toBe(trigger);
		trigger.remove();
	});

	it("returns focus to the opener on the caller's own close, for example after a send", async () => {
		await page.viewport(...WIDE);
		const { trigger, rerender } = await setupOpenFromTrigger();

		await rerender({ open: false, heading: HEADING, children: CONTENT });

		expect(document.activeElement).toBe(trigger);
		trigger.remove();
	});
});
