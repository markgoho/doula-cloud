import { createRawSnippet } from 'svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
import FilterBar from './FilterBar.svelte';
// The skip link is hidden by the design tokens' spacing, so they must load.
import '#lib/styles/app.css';

// `createRawSnippet` renders one root element, so the field is one
// wrapper holding a label and its date control: what the bar needs from
// its children is only that they are named fields inside its form.
function fromField() {
	return createRawSnippet(() => ({
		render: () =>
			'<div><label for="from">From</label><input id="from" type="date" name="from" value="2026-10-26"></div>'
	}));
}

function switchRow() {
	return createRawSnippet(() => ({
		render: () => '<label><input type="checkbox" name="all" value="true">Also show everyone</label>'
	}));
}

// A switch the route does not draw right now, as the Clients list marks it.
function heldSwitch() {
	return createRawSnippet(() => ({
		render: () =>
			'<div data-holds-space aria-hidden="true" inert><label><input type="checkbox" name="held" value="true">Held switch</label></div>'
	}));
}

// The selected filters: one li each, which the bar lists above its result line.
function appliedTags() {
	return createRawSnippet(() => ({
		render: () => '<li><a href="/practices/practice-1/clients">Remove filter: Who owe money</a></li>'
	}));
}

// The field a bar of switches keeps in view, beside the one it folds.
function nameField() {
	return createRawSnippet(() => ({
		render: () => '<div><label for="name">Name</label><input id="name" type="search" name="q"></div>'
	}));
}

interface SetupOptions {
	// A second field that folds, with `foldsFieldsOnly`; `withFields: false` for none in view.
	withFolded?: boolean;
	isBusy?: boolean;
	empty?: boolean;
	clearHref?: string;
	activeLabels?: string[];
	statusId?: string;
	withSwitches?: boolean;
	withHeldSwitch?: boolean;
	// One removable tag, as the Clients list draws its selected filters.
	withApplied?: boolean;
	appliedCount?: number;
	foldsFieldsOnly?: boolean;
	withFields?: boolean;
	status?: string;
	statusRuler?: string;
	onSubmit?: (event: SubmitEvent) => void;
	// The window the bar is drawn in. The bar answers to its own container,
	// which here is the whole window: wide enough to show every control
	// unless a test asks for a phone's.
	width?: number;
}

const WIDE = 1280;
const PHONE = 414;

const applyBox = () => page.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect();
const statusBox = () => page.getByRole('status').element().getBoundingClientRect();

async function setup({
	withFolded = false,
	isBusy = false,
	empty,
	clearHref,
	activeLabels,
	statusId,
	withSwitches = false,
	withHeldSwitch = false,
	withApplied = false,
	appliedCount,
	foldsFieldsOnly = false,
	withFields = true,
	status = 'Showing 2 scheduled visits.',
	statusRuler,
	onSubmit = vi.fn((event: SubmitEvent) => event.preventDefault()),
	width = WIDE
}: SetupOptions = {}) {
	await page.viewport(width, 900);
	// The gap a list screen leaves above its bar, where the skip link sits.
	document.body.style.paddingBlockStart = 'var(--space-group)';
	const { component, rerender } = await render(FilterBar, {
		action: '/practices/practice-1/schedule',
		onSubmit,
		legend: 'Narrow the schedule',
		status,
		statusRuler,
		empty,
		clearHref,
		activeLabels,
		statusId,
		switches: withSwitches ? switchRow() : (withHeldSwitch ? heldSwitch() : undefined),
		foldsFieldsOnly,
		isBusy,
		applied: withApplied ? appliedTags() : undefined,
		appliedCount,
		folded: withFolded ? fromField() : undefined,
		children: withFields ? (withFolded ? nameField() : fromField()) : undefined
	});
	return { onSubmit, component, rerender };
}

describe('FilterBar', () => {
	it('takes focus on its result line when the route asks, without joining the tab order', async () => {
		const { component } = await setup();
		const status = page.getByRole('status');

		await expect.element(status).toHaveAttribute('tabindex', '-1');
		await expect.element(status).not.toHaveFocus();
		component.focusStatus();

		await expect.element(status).toHaveFocus();
	});

	it('opens with a "Skip to results" stop that lands on the result line', async () => {
		await setup();

		await userEvent.keyboard('{Tab}');
		const skip = page.getByRole('link', { name: 'Skip to results' });
		await expect.element(skip).toHaveFocus();
		await userEvent.keyboard('{Enter}');

		await expect.element(page.getByRole('status')).toHaveFocus();
	});

	it('draws the skip link only while it is focused', async () => {
		await setup();
		const skip = page.getByRole('link', { name: 'Skip to results' });

		// Hit-testing ignores a clipped-away element, so at rest the point
		// at the link's own center belongs to whatever is under it; once
		// focused, it is the link. A link moved above its bar instead was
		// left on screen over the page title (seen live on the Clients list).
		const isDrawn = () => {
			const box = skip.element().getBoundingClientRect();
			const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
			return hit !== null && skip.element().contains(hit);
		};
		expect(isDrawn()).toBe(false);
		await userEvent.keyboard('{Tab}');

		expect(isDrawn()).toBe(true);
	});

	it.each([1280, PHONE])('keeps the focused skip link clear of every control and label, at %ipx', async (width) => {
		await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true, width });
		const skip = page.getByRole('link', { name: 'Skip to results' });
		await userEvent.keyboard('{Tab}');
		await expect.element(skip).toHaveFocus();

		const box = skip.element().getBoundingClientRect();
		const others = [...document.querySelectorAll<HTMLElement>('input, label, button, select, summary, a, legend')].filter(
			(element) => element !== skip.element() && !element.closest('.visually-hidden') && element.getClientRects().length > 0
		);
		expect(others.length).toBeGreaterThan(3);
		for (const other of others) {
			const rect = other.getBoundingClientRect();
			const isOverlapping = rect.left < box.right && rect.right > box.left && rect.top < box.bottom && rect.bottom > box.top;
			expect(isOverlapping, `the skip link covers "${other.textContent?.trim() || other.tagName}"`).toBe(false);
		}
	});

	it('names its result line with the id it is given, and one of its own otherwise', async () => {
		await setup({ statusId: 'result-line' });
		await expect.element(page.getByRole('status')).toHaveAttribute('id', 'result-line');
		await expect.element(page.getByRole('link', { name: 'Skip to results' })).toHaveAttribute('href', '#result-line');
	});

	it('groups its fields under the legend', async () => {
		await setup();

		const group = page.getByRole('group', { name: 'Narrow the schedule' });
		await expect.element(group).toBeVisible();
		await expect.element(group.getByLabelText('From')).toHaveValue('2026-10-26');
	});

	// The fields are named and the form is a GET to the list's own URL, so
	// it narrows the list with no script running at all.
	it('is a GET form to the list’s own address', async () => {
		await setup();

		const form = page.getByRole('group', { name: 'Narrow the schedule' }).element().closest('form');
		expect(form).toHaveAttribute('method', 'get');
		expect(form).toHaveAttribute('action', '/practices/practice-1/schedule');
	});

	it('hands the submit to the route when Apply is pressed', async () => {
		const { onSubmit } = await setup();

		await page.getByRole('button', { name: 'Apply' }).click();

		expect(onSubmit).toHaveBeenCalledOnce();
	});

	it('draws no "Clear filters" link without a clearHref', async () => {
		await setup();

		await expect.element(page.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});

	it('puts "Clear filters" after Apply, so Apply sits where it does without the link', async () => {
		await setup({ clearHref: '/practices/practice-1/schedule' });

		const apply = page.getByRole('button', { name: 'Apply' });
		const clear = page.getByRole('link', { name: 'Clear filters' });
		await expect.element(clear).toHaveAttribute('href', '/practices/practice-1/schedule');
		expect(apply.element().compareDocumentPosition(clear.element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('gives switches a row above the fields', async () => {
		await setup({ withSwitches: true });

		const switchBox = page.getByRole('checkbox', { name: 'Also show everyone' });
		const field = page.getByLabelText('From');
		await expect.element(switchBox).toBeVisible();
		expect(switchBox.element().getBoundingClientRect().bottom).toBeLessThanOrEqual(field.element().getBoundingClientRect().top);
	});

	it('says the result as its own status line', async () => {
		await setup();

		const status = page.getByRole('status');
		await expect.element(status).toBeVisible();
		await expect.element(status).toHaveTextContent('Showing 2 scheduled visits.');
	});

	// #2849: the focus ring Apply moves to it hugs the sentence, not the bar.
	it('draws its result line’s box, and so its focus ring, as wide as its sentence', async () => {
		await setup();

		const status = page.getByRole('status').element();
		const range = document.createRange();
		range.selectNodeContents(status);
		expect(Math.round(statusBox().width)).toBe(Math.round(range.getBoundingClientRect().width));
	});

	// The list's empty state says it on screen (#2015); the line stays in
	// the document as a live region, so a screen reader still hears what
	// the new range found.
	it('keeps the status line for a screen reader only when the list is empty', async () => {
		await setup({ empty: true });

		await expect.element(page.getByRole('status')).toHaveClass('visually-hidden');
	});
});

// GOV.UK/MOJ's filter on a small screen: the controls fold behind one
// control, and the result line stays where it is. The switch is the bar's
// own container (ADR-0024), so these draw the bar in a window the width of
// the container.
describe('FilterBar in a narrow column', () => {
	it('folds the controls behind a "Filters" summary, closed', async () => {
		await setup({ width: PHONE });

		await expect.element(page.getByText('Filters', { exact: true })).toBeVisible();
		await expect.element(page.getByLabelText('From')).not.toBeVisible();
	});

	it('keeps the result line visible while the controls are folded', async () => {
		await setup({ width: PHONE });

		await expect.element(page.getByRole('status')).toBeVisible();
	});

	it('shows the controls once the summary is pressed', async () => {
		await setup({ width: PHONE });

		await page.getByText('Filters', { exact: true }).click();

		await expect.element(page.getByLabelText('From')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Apply' })).toBeVisible();
	});

	it('counts the filters that are on, and stays closed until the person opens it', async () => {
		await setup({ width: PHONE, activeLabels: ['Only my Clients', 'Name "Cam"'] });

		await expect.element(page.getByText('Filters: 2 on')).toBeVisible();
		await expect.element(page.getByLabelText('From')).not.toBeVisible();
		await page.getByText('Filters: 2 on').click();
		await expect.element(page.getByLabelText('From')).toBeVisible();
	});

	it('names the one filter that is on', async () => {
		await setup({ width: PHONE, activeLabels: ['Only my Clients'] });

		await expect.element(page.getByText('Filters: Only my Clients')).toBeVisible();
	});

	it('keeps an opened summary open when the filters change', async () => {
		const { component } = await setup({ width: PHONE });
		await page.getByText('Filters', { exact: true }).click();
		await expect.element(page.getByLabelText('From')).toBeVisible();

		component.focusStatus();

		await expect.element(page.getByLabelText('From')).toBeVisible();
	});

	it('still submits a folded control with the form', async () => {
		const { onSubmit } = await setup({ width: PHONE });
		const form = page.getByRole('status').element().closest('form')!;

		const sent = new FormData(form).get('from');

		expect(sent).toBe('2026-10-26');
		expect(onSubmit).not.toHaveBeenCalled();
	});
});

describe('FilterBar in a wide column', () => {
	it('draws every control and no summary', async () => {
		await setup({ activeLabels: ['Only my Clients', 'Name "Cam"'] });

		await expect.element(page.getByLabelText('From')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Apply' })).toBeVisible();
		await expect.element(page.getByText('Filters: 2 on')).not.toBeVisible();
	});

	it('keeps the controls out of the accessibility tree when a narrow column folds them', async () => {
		await setup({ width: PHONE });

		await expect.element(page.getByRole('button', { name: 'Apply' })).not.toBeInTheDocument();
	});
});

describe('FilterBar that folds only its fields', () => {
	it('keeps the switches, the main field, Apply and "Clear filters" in view in a narrow column, with only the folded field behind the summary', async () => {
		await setup({
			width: PHONE,
			withSwitches: true,
			foldsFieldsOnly: true,
			withFolded: true,
			clearHref: '/practices/practice-1/schedule'
		});

		await expect.element(page.getByLabelText('Also show everyone')).toBeVisible();
		await expect.element(page.getByLabelText('Name')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Apply' })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Clear filters' })).toBeVisible();
		await expect.element(page.getByLabelText('From')).not.toBeVisible();
		await page.getByText('More filters', { exact: true }).click();
		await expect.element(page.getByLabelText('From')).toBeVisible();
	});

	it('holds Apply to the width of its words and puts "Clear filters" on its row, centered on it', async () => {
		await setup({
			width: PHONE,
			foldsFieldsOnly: true,
			withFolded: true,
			clearHref: '/practices/practice-1/schedule'
		});

		const apply = page.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect();
		const clear = page.getByRole('link', { name: 'Clear filters' }).element().getBoundingClientRect();
		expect(apply.width).toBeLessThan(PHONE / 2);
		expect(Math.abs(apply.top + apply.height / 2 - (clear.top + clear.height / 2))).toBeLessThanOrEqual(1);
		expect(clear.left).toBeGreaterThan(apply.right);
	});

	it('shows Apply busy while a newer result is on its way', async () => {
		await setup({ foldsFieldsOnly: true, isBusy: true });

		await expect.element(page.getByRole('button', { name: 'Apply' })).toHaveAttribute('aria-busy', 'true');
	});

	it('names the fields that are on in its summary', async () => {
		await setup({ width: PHONE, foldsFieldsOnly: true, withFolded: true, activeLabels: ['Doula Kimiko'] });

		await expect.element(page.getByText('More filters: Doula Kimiko')).toBeVisible();
	});

	it('draws the fields beside Apply, with no summary, in a wide column', async () => {
		await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true });

		await expect.element(page.getByLabelText('From')).toBeVisible();
		await expect.element(page.getByLabelText('Name')).toBeVisible();
		await expect.element(page.getByText('More filters', { exact: true })).not.toBeVisible();
		const field = page.getByLabelText('From').element().getBoundingClientRect();
		const apply = page.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect();
		expect(Math.abs(field.bottom - apply.bottom)).toBeLessThan(apply.height);
	});

	it('holds the controls apart from the result line by the group step', async () => {
		await setup();

		const status = page.getByRole('status').element();
		const fieldset = status.closest('form')!.querySelector('fieldset')!;
		// The controls and their result line are two groups (the fieldset and the
		// line have no accessible role that tells them apart from the form).
		expect(status.getBoundingClientRect().top - fieldset.getBoundingClientRect().bottom).toBeGreaterThanOrEqual(28);
	});

	it('names the group for a screen reader and does not draw its legend, at any width', async () => {
		await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true });

		const group = page.getByRole('group', { name: 'Narrow the schedule' });
		await expect.element(group).toBeInTheDocument();
		const legend = group.element().querySelector('legend')!;
		// A legend is not a role an accessible query reaches: the fact is that it takes no room.
		expect(legend.getBoundingClientRect().height).toBeLessThanOrEqual(1);
	});

	it('puts the switches on a row above the fields and Apply, at every width', async () => {
		await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true, clearHref: '/practices/practice-1/schedule' });

		const apply = () => page.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect();
		const switchBox = () => page.getByLabelText('Also show everyone').element().getBoundingClientRect();
		expect(switchBox().bottom).toBeLessThanOrEqual(apply().top);

		await page.viewport(PHONE, 900);
		expect(switchBox().bottom).toBeLessThanOrEqual(apply().top);
	});

	it('holds no box for "Clear filters" while no filter is on, and Apply does not move when the link comes', async () => {
		const { rerender } = await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true });
		const before = applyBox();

		await expect.element(page.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
		// A held box is out of the accessibility tree, so no role query could see one: the fact is that none is drawn.
		expect(page.getByRole('button', { name: 'Apply' }).element().closest('form')!.querySelector('.clear')).toBeNull();

		await rerender({ clearHref: '/practices/practice-1/schedule' });
		await expect.element(page.getByRole('link', { name: 'Clear filters' })).toBeVisible();
		expect(applyBox().left).toBe(before.left);
		expect(applyBox().top).toBe(before.top);
	});

	it('holds the box of a switch the route marks `data-holds-space`, out of the tab order and the accessibility tree', async () => {
		await setup({ foldsFieldsOnly: true, withHeldSwitch: true });

		await expect.element(page.getByRole('checkbox', { name: 'Held switch' })).not.toBeInTheDocument();
		const held = document.querySelector<HTMLElement>('[data-holds-space]')!;
		expect(getComputedStyle(held).visibility).toBe('hidden');
		expect(held.getBoundingClientRect().width).toBeGreaterThan(60);
		expect(held.getBoundingClientRect().height).toBeGreaterThan(10);
	});

	it('puts the count on a row of its own under the controls, on the bar’s left edge, at every width', async () => {
		await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true });
		const form = page.getByRole('status').element().closest('form')!.getBoundingClientRect();
		const status = page.getByRole('status').element().getBoundingClientRect();
		const apply = page.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect();
		expect(status.top).toBeGreaterThanOrEqual(apply.bottom);
		expect(status.left).toBe(form.left);

		await page.viewport(PHONE, 900);
		const narrow = page.getByRole('status').element().getBoundingClientRect();
		expect(narrow.top).toBeGreaterThanOrEqual(page.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect().bottom);
		expect(narrow.left).toBe(page.getByRole('status').element().closest('form')!.getBoundingClientRect().left);
	});

	it('keeps Apply and the count in one place whatever the count says, wrapping a long one inside its own box', async () => {
		const { rerender } = await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true });
		const before = { apply: applyBox(), status: statusBox() };

		await rerender({
			status: 'Showing the first 30 Clients, including Clients with no care or request yet, soonest due first, then most recently ended, for Kimiko Nakashima.'
		});
		expect(applyBox().top).toBe(before.apply.top);
		expect(applyBox().left).toBe(before.apply.left);
		expect(statusBox().left).toBe(before.status.left);
		await rerender({ status: '' });
		expect(applyBox().top).toBe(before.apply.top);
	});

	it('reserves two lines for the count, or what the longest wording it is given takes at the bar’s width', async () => {
		const lineHeight = () => Number(getComputedStyle(page.getByRole('status').element()).lineHeight.replace('px', ''));
		// The line's row, and the row's box that holds it and the unseen ruler.
		const reserved = () => page.getByRole('status').element().parentElement!.getBoundingClientRect().height;
		const longest = 'Showing the first 25 of Kimiko Nakashima’s active Clients.';
		const { rerender } = await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true });
		expect(reserved()).toBeGreaterThan(lineHeight() * 1.5);

		await rerender({ statusRuler: longest });
		expect(reserved(), 'one line where the longest wording fits on one').toBeLessThan(lineHeight() * 1.5);
		await expect.element(page.getByText(longest)).not.toBeVisible();

		await rerender({ statusRuler: `${longest} ${longest} ${longest} ${longest}` });
		expect(reserved(), 'two lines where it does not').toBeGreaterThan(lineHeight() * 1.5);
	});

	it('holds a wording longer than a reading line on one line where the bar can, and reserves one line for it', async () => {
		const wording =
			'Showing the first 99 of Kimiko Nakashima’s active Clients who owe money and have no Contract, soonest due first, then most recently ended.';
		await setup({ foldsFieldsOnly: true, withSwitches: true, withFolded: true, width: 1920, status: wording, statusRuler: wording });
		const status = page.getByRole('status').element();
		const lineHeight = Number(getComputedStyle(status).lineHeight.replace('px', ''));

		expect(status.getBoundingClientRect().height, 'one line').toBeLessThan(lineHeight * 1.5);
		expect(status.parentElement!.getBoundingClientRect().height, 'one line reserved').toBeLessThan(lineHeight * 1.5);
	});

	it.each([WIDE, 320])('lists the applied filters after Apply, on its line and centered on it, at %ipx', async (width) => {
		await setup({ foldsFieldsOnly: true, withFields: false, withApplied: true, width });

		const tags = page.getByRole('list', { name: 'Applied filters' });
		await expect.element(tags.getByRole('link', { name: 'Remove filter: Who owe money' })).toBeVisible();
		const tag = tags.getByRole('link').element().getBoundingClientRect();
		expect(tag.left).toBeGreaterThan(applyBox().right);
		expect(Math.abs((tag.top + tag.bottom) / 2 - (applyBox().top + applyBox().bottom) / 2)).toBeLessThanOrEqual(1);
	});

	it('puts "Clear filters" after the tags, where two or more are on', async () => {
		await setup({ foldsFieldsOnly: true, withApplied: true, appliedCount: 2, clearHref: '/practices/practice-1/schedule' });

		const tags = page.getByRole('list', { name: 'Applied filters' }).element().getBoundingClientRect();
		expect(page.getByRole('link', { name: 'Clear filters' }).element().getBoundingClientRect().left).toBeGreaterThan(tags.right);
	});

	it('draws no "Clear filters" beside exactly one tag, whose × does the same', async () => {
		await setup({ foldsFieldsOnly: true, withApplied: true, appliedCount: 1, clearHref: '/practices/practice-1/schedule' });

		await expect.element(page.getByRole('link', { name: 'Remove filter: Who owe money' })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Clear filters' })).not.toBeInTheDocument();
	});

	it('keeps "Clear filters" while a filter is on that draws no tag', async () => {
		await setup({ foldsFieldsOnly: true, clearHref: '/practices/practice-1/schedule' });

		await expect.element(page.getByRole('link', { name: 'Clear filters' })).toBeVisible();
	});

	it('draws no list of applied filters while none is given', async () => {
		await setup();

		await expect.element(page.getByRole('list', { name: 'Applied filters' })).not.toBeInTheDocument();
	});

	it.each([WIDE, 320])('holds no room for a tag, and applying one filter moves neither Apply nor the result line, at %ipx', async (width) => {
		const status = 'Showing the first 25 of 40 Invoices, earliest payment due first.';
		const bar = { foldsFieldsOnly: true, withFields: false, statusRuler: status, status, width };
		await setup(bar);
		const apply = applyBox();
		const line = statusBox();
		// Nothing but the related step between Apply and the line it reports.
		expect(line.top - apply.bottom).toBeLessThanOrEqual(12);

		cleanup();
		await setup({ ...bar, withApplied: true });

		await expect.element(page.getByRole('link', { name: 'Remove filter: Who owe money' })).toBeVisible();
		expect(applyBox().top).toBe(apply.top);
		expect(statusBox().top).toBe(line.top);
	});

	it('draws no fold when the bar has no fields', async () => {
		await setup({ width: PHONE, foldsFieldsOnly: true, withFields: false });

		await expect.element(page.getByText('More filters', { exact: true })).not.toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'Apply' })).toBeVisible();
	});
});
