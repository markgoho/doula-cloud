import { expect, test } from '@playwright/test';
import { seedPortalClient } from './portalClient';
import { enterPracticeAsEnrolled } from './mfa';
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

	await enterPracticeAsEnrolled(page.context(), page, staffHeaders, practiceId);
	await expect(page).toHaveURL(new RegExp(`/practices/${practiceId}$`));

	await page.goto(`/practices/${practiceId}/engagements/${engagementId}`);

	// A minimal valid 1x1 PNG -- the same bytes api/internal/message's own
	// pngBytes fixture uses, so a corrupted round trip through the real
	// store would fail the same way here as it does there.
	const pngBytes = Buffer.from(
		'89504e470d0a1a0a0000000d49484452000000010000000108060000' +
			'001f15c4890000000a49444154789c63000100000500010d0a2db400' +
			'00000049454e44ae426082',
		'hex'
	);
	await page.getByLabel('Attachment (image or PDF, up to 10MB)').setInputFiles({
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
	await enterPracticeAsEnrolled(page.context(), page, staffHeaders, practiceId);

	const messageId = seedMessageWithMissingAttachment(engagementId, staffId, 'ghost.png');

	const resp = await page.request.get(
		`/api/practices/${practiceId}/engagements/${engagementId}/messages/${messageId}/attachment`
	);
	expect(resp.status(), `attachment fetch: ${resp.status()} ${await resp.text()}`).toBe(404);
});
