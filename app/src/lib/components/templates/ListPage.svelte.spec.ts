import type { ComponentProps } from 'svelte';
import { createRawSnippet, mount, unmount } from 'svelte';
import Link from '#lib/components/atoms/Link.svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ListPage from './ListPage.svelte';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See practice-landing.svelte.spec.ts's identical import.
import '#lib/styles/app.css';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';

// The title's cluster aligns through the primitive's own attribute handling.
if (!customElements.get('cluster-l')) registerLayoutPrimitives();

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

type SetupOptions = Partial<ComponentProps<typeof ListPage>>;

async function setup(overrides: SetupOptions = {}) {
	return render(ListPage, {
		props: {
			title: 'Staff',
			content: textSnippet('A table of Staff members'),
			...overrides
		}
	});
}

// The button a list draws in its title row: the real start Link, mounted into
// a raw snippet's element, so its own box is what the row is measured against.
function startButton() {
	return createRawSnippet(() => ({
		render: () => '<div></div>',
		setup: (node) => {
			const link = mount(Link, { target: node, props: { href: '/x', label: 'Invite a Staff member', variant: 'start' } });
			return () => unmount(link);
		}
	}));
}

const titleTop = () => page.getByRole('heading', { level: 1 }).element().getBoundingClientRect().top;
// The title's row, which holds an action's height whether or not one is given.
const titleRowHeight = () => page.getByRole('heading', { level: 1 }).element().parentElement!.getBoundingClientRect().height;

describe('ListPage.svelte', () => {
	it('renders the title as the page h1', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Staff' })).toBeVisible();
	});

	// #2041: the roster is where a removal lands, and it says what was done
	// the way RecordDetail does -- before the h1, as a status that takes focus.
	it('renders the outcome before the title, as a status that takes focus', async () => {
		await setup({ outcome: 'Pat Lee was removed from this Practice.' });

		const outcome = page.getByRole('status');
		await expect.element(outcome).toHaveTextContent('Pat Lee was removed from this Practice.');
		await expect.element(outcome).toHaveFocus();
		const heading = page.getByRole('heading', { level: 1, name: 'Staff' }).element();
		expect(outcome.element().compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('always renders the content region', async () => {
		await setup();

		await expect.element(page.getByText('A table of Staff members')).toBeVisible();
	});

	it('omits the intro and actions regions when neither is given', async () => {
		await setup();

		await expect.element(page.getByText('Invite a Staff member')).not.toBeInTheDocument();
	});

	it('renders the actions right after the title, ahead of the intro', async () => {
		await setup({
			intro: textSnippet('Work states are self-reported'),
			actions: textSnippet('Invite a Staff member')
		});

		const title = page.getByRole('heading', { level: 1 }).element();
		const actions = page.getByText('Invite a Staff member').element();
		const intro = page.getByText('Work states are self-reported').element();

		expect(title.compareDocumentPosition(actions) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(actions.compareDocumentPosition(intro) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('renders the intro between the actions and the content', async () => {
		await setup({
			intro: textSnippet('Work states are self-reported'),
			actions: textSnippet('Invite a Staff member')
		});

		const intro = page.getByText('Work states are self-reported').element();
		const content = page.getByText('A table of Staff members').element();

		expect(intro.compareDocumentPosition(content) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('caps the page at no width, unlike a form, and renders no chrome', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('nav')).toBeNull();
		expect(container.querySelector('header')).toBeNull();
	});

	it('renders the title and a skeleton while loading, instead of the content (#480)', async () => {
		const { container } = await setup({ loading: 'Loading Staff' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByRole('heading', { level: 1, name: 'Staff' })).toBeVisible();
		await expect.element(page.getByRole('status', { name: 'Loading Staff' })).toBeVisible();
		await expect.element(page.getByText('A table of Staff members')).not.toBeInTheDocument();
	});

	it('renders the title and a Notice on a load failure, instead of the content (#480)', async () => {
		const { container } = await setup({ loadError: 'Failed to load Staff' });

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		await expect.element(page.getByRole('heading', { level: 1, name: 'Staff' })).toBeVisible();
		await expect.element(page.getByText('Failed to load Staff')).toBeVisible();
		await expect.element(page.getByText('A table of Staff members')).not.toBeInTheDocument();
	});

	it('centers the title and the action on one another', async () => {
		await setup({
			actions: createRawSnippet(() => ({ render: () => '<a href="/x" style="display:block;block-size:3rem">Invite a Staff member</a>' }))
		});

		const title = page.getByRole('heading', { level: 1 }).element().getBoundingClientRect();
		const action = page.getByText('Invite a Staff member').element().getBoundingClientRect();
		expect(Math.abs(title.top + title.height / 2 - (action.top + action.height / 2))).toBeLessThanOrEqual(1);
	});

	it('holds the title at the same place on a list with an action and a list without one', async () => {
		const withAction = await setup({
			actions: createRawSnippet(() => ({ render: () => '<a href="/x" style="display:block;block-size:2.75rem">Invite a Staff member</a>' }))
		});
		const [topWith, heightWith] = [titleTop(), titleRowHeight()];
		await withAction.unmount();

		await setup();

		expect(Math.abs(titleTop() - topWith)).toBeLessThanOrEqual(1);
		expect(Math.abs(titleRowHeight() - heightWith)).toBeLessThanOrEqual(1);
	});

	// The row reserves the start button's own box, not a nearby size: a raw
	// snippet of any height proves nothing about the button a list draws.
	it('holds the title at the same place with the real start button and without it', async () => {
		const withButton = await setup({ actions: startButton() });
		const [topWith, heightWith] = [titleTop(), titleRowHeight()];
		const button = page.getByRole('link', { name: 'Invite a Staff member' }).element().getBoundingClientRect();
		expect(button.height, 'the start button fits the row').toBeLessThanOrEqual(heightWith + 0.5);
		await withButton.unmount();

		await setup();

		expect(Math.abs(titleTop() - topWith), `title top ${topWith} with, ${titleTop()} without`).toBeLessThanOrEqual(0.5);
		expect(Math.abs(titleRowHeight() - heightWith)).toBeLessThanOrEqual(0.5);
	});

	it('puts the content a group step under the title row, not a section step', async () => {
		await setup({ actions: startButton() });

		const row = page.getByRole('heading', { level: 1 }).element().parentElement!;
		const content = page.getByText('A table of Staff members').element().getBoundingClientRect();
		const gap = content.top - row.getBoundingClientRect().bottom;
		// The rhythm's steps are fluid lengths; read each as the box it makes
		// inside the same container the page's stack sits in.
		const stepOf = (token: string) => {
			const probe = document.createElement('div');
			probe.style.cssText = `position:absolute;block-size:var(${token})`;
			row.parentElement!.append(probe);
			const height = probe.getBoundingClientRect().height;
			probe.remove();
			return height;
		};
		const [groupStep, sectionStep] = [stepOf('--space-group'), stepOf('--space-section')];
		expect(sectionStep).toBeGreaterThan(groupStep);
		expect(gap).toBeGreaterThanOrEqual(groupStep - 0.5);
		expect(gap).toBeLessThan(sectionStep - 0.5);
	});

	it('prefers loadError over loading when both are somehow given', async () => {
		await setup({ loadError: 'Failed to load Staff', loading: 'Loading Staff' });

		await expect.element(page.getByText('Failed to load Staff')).toBeVisible();
		await expect.element(page.getByRole('status', { name: 'Loading Staff' })).not.toBeInTheDocument();
	});
});
