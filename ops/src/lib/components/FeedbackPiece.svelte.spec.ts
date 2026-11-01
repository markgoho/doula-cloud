import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import FeedbackPiece from './FeedbackPiece.svelte';
import type { FeedbackPiece as Piece } from '../feedback.ts';

const piece: Piece = {
	id: 'f-2',
	kind: 'not_working',
	text: 'The page froze.\n\nI had to reload it.',
	pageUrl: '/portal/x',
	routeId: '/portal/[clientId]',
	appBuild: 'abc1234',
	screenWidth: 320,
	browser: 'Safari 18',
	sentAt: '2026-09-20T15:30:00Z',
	role: 'Client',
	practiceName: 'Rooted Birth Collective',
	sender: { kind: 'client', name: 'Nadia Haddad', email: 'nadia@example.com' },
	issue: { state: 'opened', number: 17, url: 'https://github.com/acme/feedback/issues/17', attempts: 0 }
};

describe('FeedbackPiece', () => {
	it('prints the words a paragraph a line, the sender and every field, and links the issue', async () => {
		await render(FeedbackPiece, { piece });

		await expect.element(page.getByText('The page froze.')).toBeVisible();
		await expect.element(page.getByText('I had to reload it.')).toBeVisible();
		await expect.element(page.getByText('Nadia Haddad, a Client')).toBeVisible();
		await expect.element(page.getByText('nadia@example.com')).toBeVisible();
		const link = page.getByRole('link', { name: 'Issue #17 in the feedback repository' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', 'https://github.com/acme/feedback/issues/17');
	});

	it('says when nothing was written, and states an issue with no link and its last error', async () => {
		await render(FeedbackPiece, {
			piece: {
				...piece,
				text: ' ',
				issue: { state: 'dead_lettered', attempts: 8, lastError: '401 Bad credentials', deadLetterReason: 'retries_exhausted' }
			}
		});

		await expect.element(page.getByText('Nothing was written. This piece is its kind and its context only.')).toBeVisible();
		await expect.element(page.getByText('Not opened: all 8 attempts failed, and nothing will try again')).toBeVisible();
		await expect.element(page.getByText('The last attempt said: 401 Bad credentials')).toBeVisible();
		await expect.element(page.getByRole('link')).not.toBeInTheDocument();
	});
});
