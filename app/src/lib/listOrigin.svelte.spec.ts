import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ListBack, originFrom } from './listOrigin.svelte.js';

const navigation = vi.hoisted(() => ({
	goto: vi.fn(),
	afterNavigate: undefined as ((navigation: { from: { url: URL } | undefined }) => void) | undefined
}));
vi.mock('$app/navigation', () => ({
	goto: navigation.goto,
	afterNavigate: (callback: (navigation: { from: { url: URL } | undefined }) => void) => {
		navigation.afterNavigate = callback;
	}
}));

const paths = { schedule: '/p/schedule', invoices: '/p/invoices' };
const here = '/p/engagements/e1';
const arrive = (path: string | undefined) =>
	navigation.afterNavigate?.({ from: path ? { url: new URL(`https://example.test${path}`) } : undefined });
const click = (init: MouseEventInit = {}) => new MouseEvent('click', { cancelable: true, ...init });

function setup() {
	return new ListBack(
		() => paths,
		() => here
	);
}

describe('originFrom', () => {
	it('names the list an address is, keeping its filters', () => {
		expect(originFrom(new URL('https://example.test/p/invoices?view=unpaid'), paths)).toEqual({
			list: 'invoices',
			address: '/p/invoices?view=unpaid'
		});
	});

	it('names nothing for another address or none', () => {
		expect(originFrom(new URL('https://example.test/p/clients'), paths)).toBeUndefined();
		expect(originFrom(undefined, paths)).toBeUndefined();
	});
});

describe('ListBack', () => {
	beforeEach(() => {
		navigation.goto.mockReset();
	});

	it('keeps the list it came from while the page moves within itself', () => {
		const back = setup();
		arrive('/p/schedule?mine=true');
		arrive(here);

		expect(back.label('Clients')).toBe('Back to the Schedule');
		expect(back.href('/p/clients')).toBe('/p/schedule?mine=true');
	});

	it('forgets it when the page is reached from anywhere else', () => {
		const back = setup();
		arrive('/p/schedule');
		arrive('/p/clients/c1');

		expect(back.label('Clients')).toBe('Clients');
		expect(back.href('/p/clients')).toBe('/p/clients');
	});

	it('leaves a click to the browser when it came from no list', () => {
		const back = setup();
		const event = click();
		arrive(undefined);

		back.go(event);

		expect(event.defaultPrevented).toBe(false);
	});

	it.each([{ button: 1 }, { metaKey: true }, { ctrlKey: true }, { shiftKey: true }, { altKey: true }])(
		'leaves a modified click %o to the browser',
		(init) => {
			const back = setup();
			const event = click(init);
			arrive('/p/invoices');

			back.go(event);

			expect(event.defaultPrevented).toBe(false);
			expect(navigation.goto).not.toHaveBeenCalled();
		}
	);
});
