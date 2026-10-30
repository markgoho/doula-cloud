import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PageTitle from './PageTitle.svelte';
import RouteAnnouncer from './RouteAnnouncer.svelte';

/*
 * #2246. SvelteKit's own announcer reads `document.title` once, as the
 * new page mounts, which on a page titled from its own read is a
 * placeholder or nothing. This one waits until no Template is still
 * loading, then reads it. `afterNavigate` is captured so a test can say
 * when a move has happened.
 */
const navigation = vi.hoisted(() => ({ callbacks: [] as (() => void)[] }));
vi.mock('$app/navigation', () => ({
	afterNavigate: (callback: () => void) => {
		navigation.callbacks.push(callback);
	},
}));

function navigate(): void {
	for (const callback of navigation.callbacks) callback();
}

beforeEach(() => {
	navigation.callbacks = [];
});

afterEach(() => {
	document.title = '';
});

async function setup() {
	await render(RouteAnnouncer);
	// SvelteKit calls `afterNavigate` once for the first load, which is no
	// move and is not announced.
	navigate();
}

// A live region with no role is in the accessibility tree only by what it
// says, and an empty one says nothing: no accessible query finds it before
// it speaks, so it is found by the one attribute that makes it one.
function liveRegion(): HTMLElement {
	return document.querySelector<HTMLElement>('[aria-live="assertive"]')!;
}

const announced = () => liveRegion().textContent?.trim();

describe('RouteAnnouncer (#2246)', () => {
	it('says nothing for the first load', async () => {
		await setup();
		await render(PageTitle, { page: 'Clients' });

		await expect.poll(() => document.title).toBe('Clients — DoulaCloud');
		expect(announced()).toBe('');
	});

	it('announces the title of a page that had nothing to load', async () => {
		await setup();
		await render(PageTitle, { page: 'Clients' });

		navigate();

		await expect.poll(announced).toBe('Clients — DoulaCloud');
	});

	it('waits for a loading page to settle, and announces its real title', async () => {
		await setup();
		const loading = await render(PageTitle, { page: '', isPending: true });

		navigate();
		await expect.poll(() => document.title).toBe('— DoulaCloud');
		expect(announced()).toBe('');

		await loading.rerender({ page: 'Dee Walker-Whitlock', isPending: false });

		await expect.poll(announced).toBe('Dee Walker-Whitlock — DoulaCloud');
	});

	it('names a page that has no title', async () => {
		await setup();

		navigate();

		await expect.poll(announced).toBe('untitled page');
	});

	it('is read as one whole announcement', async () => {
		await setup();

		expect(liveRegion().getAttribute('aria-atomic')).toBe('true');
	});
});
