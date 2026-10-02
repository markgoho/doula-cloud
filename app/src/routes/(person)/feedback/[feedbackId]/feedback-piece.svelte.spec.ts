import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { fixture, piece, unopenedClientPiece } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
if (!customElements.get('center-l')) registerLayoutPrimitives();
Object.assign(pageState, toPageState(fixture));

async function setup(shown = piece) {
	await render(Page, { params: fixture.params, data: { piece: shown } });
}

/**
A row's own label in the summary list.
*/
function fact(label: string) {
	return testPage.getByRole('term').filter({ hasText: label }).first();
}

describe('one piece of Feedback (#1526)', () => {
	it('is titled by when it was sent, and leads back to the list', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1 })).toHaveTextContent('Feedback sent');
		await expect.element(testPage.getByRole('link', { name: 'Back to Feedback' })).toHaveAttribute('href', '/feedback');
	});

	it('prints what she wrote, a paragraph for each line she typed', async () => {
		await setup();

		await expect.element(testPage.getByText('The invoice total is wrong on this page.')).toBeVisible();
		await expect.element(testPage.getByText('and it does not match.', { exact: false })).toBeVisible();
	});

	it('shows every stored field as a summary list', async () => {
		await setup();

		for (const label of [
			'Kind',
			'Sent',
			'Sender',
			'Email',
			'Role',
			'Practice',
			'Page',
			'Route',
			'App build',
			'Screen width',
			'Browser'
		]) {
			await expect.element(fact(label)).toBeVisible();
		}
		for (const value of [
			'Something is not working',
			'Anne-Marie Ochieng-Whitfield',
			'anne-marie@example.test',
			'owner, admin, doula',
			'Riverside Doula Collective',
			piece.pageUrl,
			piece.routeId,
			'abc1234',
			'390px',
			'Safari 18'
		]) {
			await expect.element(testPage.getByRole('definition').filter({ hasText: value }).first()).toBeVisible();
		}
	});

	it('links to the GitHub issue when there is one', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Issue #12 in the feedback repository' }))
			.toHaveAttribute('href', 'https://github.com/markgoho/doula-cloud-feedback/issues/12');
	});

	it('names the issue without a link where no repository is configured', async () => {
		await setup({ ...piece, issue: { state: 'opened', number: 12, attempts: 0 } });

		await expect.element(testPage.getByText('Issue #12', { exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Issue #12', exact: false })).not.toBeInTheDocument();
	});

	it('says so when nothing was written, and says why the issue did not open', async () => {
		await setup(unopenedClientPiece);

		await expect
			.element(testPage.getByText('Nothing was written. This piece is its kind and its context only.'))
			.toBeVisible();
		await expect.element(testPage.getByText('Not opened: all 5 attempts failed')).toBeVisible();
		await expect
			.element(testPage.getByText(`The last attempt said: ${unopenedClientPiece.issue.lastError}`))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('definition').filter({ hasText: 'A Client, named by her sign-in address' }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('definition').filter({ hasText: 'None: sent from a screen outside any Practice' }))
			.toBeVisible();
	});
});
