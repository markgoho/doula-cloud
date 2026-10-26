import { expect, test } from '@playwright/test';
import { seedPortalClient } from './portalClient';
import { enterPractice } from './auth';
import { seedMessageWithMissingAttachment } from './stack';

// #305: contract-lifecycle.e2e.ts already proves a Signed PDF round-trips
// through the real, fake-gcs-server-backed ObjectStore; no spec proved the
// same for a Message attachment, the object store's other write path. A
// unit-test round trip (message package's handlers_test.go) uses an
// in-memory ObjectStore fake, which never exercised the real GCS client's
// read path -- the same read path #684 found 500ing against
// fake-gcs-server. This is that missing proof: upload an image attachment
// through the real compose form, and confirm it decodes back out as a
// real image, not a broken or empty download.
test('A Message attachment uploads and downloads through the real object store', async ({ page }) => {
	const { practiceId, engagementId, staffHeaders } = await seedPortalClient(page.request, 'Riverside Doulas');

	await enterPractice(page.context(), page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	// #2004: the thread is read first; its form opens when asked.
	await page.getByRole('button', { name: 'Write a message' }).click();

	// A minimal valid 1x1 PNG -- the same bytes api/internal/message's own
	// pngBytes fixture uses, so a corrupted round trip through the real
	// store would fail the same way here as it does there.
	const pngBytes = Buffer.from(
		'89504e470d0a1a0a0000000d49484452000000010000000108060000' +
			'001f15c4890000000a49444154789c63000100000500010d0a2db400' +
			'00000049454e44ae426082',
		'hex'
	);
	await page.getByLabel('Attachment', { exact: true }).setInputFiles({
		name: 'photo.png',
		mimeType: 'image/png',
		buffer: pngBytes
	});
	const [createResponse] = await Promise.all([
		page.waitForResponse(
			(response) => response.url().includes('/messages') && response.request().method() === 'POST'
		),
		page.getByRole('button', { name: 'Send', exact: true }).click()
	]);
	expect(createResponse.ok(), `send failed: ${createResponse.status()}`).toBe(true);
	const created: { messageId: string } = await createResponse.json();
	const { messageId } = created;

	// Sending re-downloads the attachment straight away to render its
	// preview (refreshAttachmentPreviews) -- an image that decodes with a
	// non-zero naturalWidth proves the bytes came back intact, not just
	// that some response arrived.
	const preview = page.getByRole('img', { name: 'photo.png' });
	await expect(preview).toBeVisible();
	await expect
		.poll(() => preview.evaluate((img: HTMLImageElement) => img.naturalWidth))
		.toBeGreaterThan(0);

	// The preview only proves the browser's image decoder was satisfied;
	// fetching the same endpoint directly proves the Content-Type header
	// and the exact bytes both round-tripped, not just something
	// image-shaped.
	const dlResp = await page.request.get(
		`/api/practices/${practiceId}/engagements/${engagementId}/messages/${messageId}/attachment`
	);
	expect(dlResp.ok(), `attachment fetch failed: ${dlResp.status()}`).toBe(true);
	expect(dlResp.headers()['content-type']).toBe('image/png');
	const downloadedBytes = await dlResp.body();
	expect(downloadedBytes.equals(pngBytes)).toBe(true);
});

// #1952: the page cannot see inside a file, so a file that says it is a
// PNG passes the page's own check. The server sniffs its bytes, refuses it,
// and keys the refusal to `attachment`, so it lands against the control in
// GOV.UK's words rather than in a Notice above the thread.
test('A file whose bytes are not an image is refused by the server against the attachment', async ({ page }) => {
	const { practiceId, engagementId, staffHeaders } = await seedPortalClient(page.request, 'Riverside Doulas');
	await enterPractice(page.context(), page, staffHeaders, practiceId);
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	// #2004: the thread is read first; its form opens when asked.
	await page.getByRole('button', { name: 'Write a message' }).click();

	const attachment = page.getByLabel('Attachment', { exact: true });
	await attachment.setInputFiles({
		name: 'photo.png',
		mimeType: 'image/png',
		buffer: Buffer.from('plain text pretending to be a photo')
	});
	const [createResponse] = await Promise.all([
		page.waitForResponse(
			(response) => response.url().includes('/messages') && response.request().method() === 'POST'
		),
		page.getByRole('button', { name: 'Send', exact: true }).click()
	]);
	expect(createResponse.status()).toBe(400);

	const words = 'The selected file must be a JPG, PNG, GIF, WEBP or PDF';
	await expect(page.getByRole('link', { name: words })).toBeVisible();
	await expect(attachment).toHaveAttribute('aria-invalid', 'true');
	await expect(attachment).toHaveAccessibleDescription(`A JPG, PNG, GIF, WEBP or PDF, up to 10MB ${words}`);
});

// #1977: an iPhone photo the browser hands over as HEIC is refused on the
// page, before anything is sent, in the words that name the accepted
// types -- the server cannot sniff HEIC and most browsers cannot show it.
test('An iPhone HEIC photo is refused before sending, naming the accepted types', async ({ page }) => {
	const { practiceId, engagementId, staffHeaders } = await seedPortalClient(page.request, 'Riverside Doulas');
	await enterPractice(page.context(), page, staffHeaders, practiceId);
	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);
	// #2004: the thread is read first; its form opens when asked.
	await page.getByRole('button', { name: 'Write a message' }).click();

	let wasPosted = false;
	page.on('request', (request) => {
		if (request.url().includes('/messages') && request.method() === 'POST') wasPosted = true;
	});

	// The ftyp box that opens an iPhone's HEIC photo, the same bytes
	// api/internal/message's type-list test refuses.
	const attachment = page.getByLabel('Attachment', { exact: true });
	await attachment.setInputFiles({
		name: 'IMG_0001.HEIC',
		mimeType: 'image/heic',
		buffer: Buffer.from('000000186674797068656963000000006d69663168656963', 'hex')
	});
	await page.getByRole('button', { name: 'Send', exact: true }).click();

	const words = 'The selected file must be a JPG, PNG, GIF, WEBP or PDF';
	await expect(page.getByRole('link', { name: words })).toBeVisible();
	await expect(attachment).toHaveAttribute('aria-invalid', 'true');
	expect(wasPosted).toBe(false);
});

// #305's other half: a Message row whose attachment metadata points at
// an object the real store never received -- seeded directly (the real
// upload path can't produce this state) -- must 404 with an identifying
// message, not the generic 500 every other store failure gets. Proven
// against the real fake-gcs-server-backed store, not the MemoryStore fake
// api/internal/message's own unit tests inject.
test('A Message whose attachment object is missing from the store 404s', async ({ page }) => {
	const { practiceId, engagementId, staffId, staffHeaders } = await seedPortalClient(
		page.request,
		'Riverside Doulas'
	);
	await enterPractice(page.context(), page, staffHeaders, practiceId);

	const messageId = seedMessageWithMissingAttachment(engagementId, staffId, 'ghost.png');

	const resp = await page.request.get(
		`/api/practices/${practiceId}/engagements/${engagementId}/messages/${messageId}/attachment`
	);
	expect(resp.status(), `attachment fetch: ${resp.status()} ${await resp.text()}`).toBe(404);
});
